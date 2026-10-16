package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// DefaultRunner is an instance of Runner used by Main().
var DefaultRunner = Runner{
	TermSignals: []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
	},
}

// Main runs given command using DefaultRunner.
func Main(cmd Command) {
	DefaultRunner.Main(cmd)
}

// Runner holds options for running commands.
type Runner struct {
	// TermSignals specifies termination OS signals which must be used to
	// cancel context passed to Command's Run() method.
	TermSignals []os.Signal

	// ForceTerm specifies whether reception of same signal specified in
	// TermSignals this amount of times should result into os.Exit() call.
	ForceTerm int

	// DoParseFlags allows to override standard way of flags parsing.
	// It should return remaining arguments (same as flag.Args() does) or
	// error.
	DoParseFlags func(context.Context, *flag.FlagSet, []string) ([]string, error)

	// DoPrintFlags allows to override standard way of flags printing.
	// It should write all output into given io.Writer.
	DoPrintFlags func(context.Context, io.Writer, *flag.FlagSet) error

	// Stdout is where help that was asked for is printed.
	// Nil means os.Stdout.
	Stdout io.Writer

	// Stderr is where usage of a wrong invocation and errors are printed.
	// Nil means os.Stderr.
	Stderr io.Writer
}

// Main runs given command with os.Args and exits the process with the code
// returned by Run(). The base name of os.Args[0] is used as the root of the
// command path in help messages.
//
// It cancels the context passed to the command on reception of any of the
// TermSignals. See Runner fields docs for more info.
func (r *Runner) Main(cmd Command) {
	ctx := context.Background()
	if len(r.TermSignals) > 0 {
		var cancel context.CancelFunc
		ctx, cancel = withTrapCancel(ctx, r.TermSignals...)
		defer cancel()
	}
	// NOTE: signal.Notify() with no signals subscribes to all of them, hence
	// the length check.
	if n := r.ForceTerm; n > 0 && len(r.TermSignals) > 0 {
		trapSeq(n, r.TermSignals, func(sig os.Signal) {
			os.Exit(signalExitCode(sig))
		})
	}
	os.Exit(r.Run(ctx, cmd, filepath.Base(os.Args[0]), os.Args[1:]))
}

// Run runs given command with given name and arguments (without the program
// name) and returns the exit code of the process.
//
// It does some i/o, such that printing help messages to Stdout or usage and
// errors returned from cmd.Run() to Stderr. Unlike Main(), it neither traps
// OS signals nor exits the process: a cancelled ctx is reported as exit code
// 128 plus the number of the signal which cancelled it, or 130 if it was
// cancelled for another reason.
func (r *Runner) Run(ctx context.Context, cmd Command, name string, args []string) int {
	baseCtx := ctx
	ctx = withRuntimeInfo(ctx, &runtimeInfo{
		runner: r,
	})
	err := run(ctx, cmd, name, args)
	if errors.Is(err, ErrHelp) {
		// Help was asked for: it is the output, so stdout and success.
		var buf bytes.Buffer
		r.printUsage(ctx, &buf)
		r.printFlags(ctx, &buf)
		r.printEpilog(ctx, &buf)
		io.Copy(r.stdout(), &buf)
		return 0
	}
	if u, ok := errors.AsType[*UsageError](err); ok {
		if u.Err != nil {
			fmt.Fprintln(r.stderr(), err)
			fmt.Fprintln(r.stderr())
		}
		// Help was not asked for; the invocation was wrong.
		// Usage goes where errors go, with the usage exit code.
		var buf bytes.Buffer
		r.printUsage(ctx, &buf)
		r.printFlags(ctx, &buf)
		r.printEpilog(ctx, &buf)
		io.Copy(r.stderr(), &buf)
		return 2
	}
	if baseCtx.Err() != nil {
		return contextExitCode(baseCtx)
	}
	if e, ok := errors.AsType[*ExitError](err); ok {
		// A nil Err is a code alone, with nothing to say: a search that
		// found nothing, as grep exits 1 in silence.
		if e.Err != nil {
			fmt.Fprintln(r.stderr(), err)
		}
		return e.Code
	}
	if err != nil {
		fmt.Fprintln(r.stderr(), err)
		return 1
	}
	return 0
}

func (r *Runner) stdout() io.Writer {
	if r.Stdout != nil {
		return r.Stdout
	}
	return os.Stdout
}

func (r *Runner) stderr() io.Writer {
	if r.Stderr != nil {
		return r.Stderr
	}
	return os.Stderr
}

func (r *Runner) printUsage(ctx context.Context, dst io.Writer) {
	info := lastCommandInfo(ctx)
	cmd := info.Command
	if s := prolog(cmd); s != "" {
		fmt.Fprintln(dst, s)
		fmt.Fprintln(dst)
	}
	fmt.Fprintln(dst, "Usage:")
	fmt.Fprintln(dst)
	// NOTE: trim the trailing space left by an empty synopsis.
	fmt.Fprintln(dst, strings.TrimRight(
		fmt.Sprintf("  %s %s", commandPath(ctx), synopsis(cmd)),
		" ",
	))
	fmt.Fprintln(dst)
	if s := description(cmd); s != "" {
		fmt.Fprintln(dst, s)
		fmt.Fprintln(dst)
	}
}

func (r *Runner) printFlags(ctx context.Context, dst io.Writer) {
	var buf bytes.Buffer
	info := lastCommandInfo(ctx)
	if info.FlagSet == nil {
		return
	}
	r.printDefaults(ctx, &buf, info.FlagSet)
	if buf.Len() == 0 {
		return
	}
	fmt.Fprintf(dst, "Options:\n")
	fmt.Fprintln(dst)
	// Every section ends with an empty line, so the epilog after the flags
	// stands apart whatever the flag printer leaves.
	b := buf.Bytes()
	dst.Write(b)
	if !bytes.HasSuffix(b, []byte("\n\n")) {
		fmt.Fprintln(dst)
	}
}

// printEpilog prints the command's epilog, if it has one, after the flags.
func (r *Runner) printEpilog(ctx context.Context, dst io.Writer) {
	info := lastCommandInfo(ctx)
	s := epilog(info.Command)
	if s == "" {
		return
	}
	fmt.Fprintln(dst, s)
	fmt.Fprintln(dst)
}

func (r *Runner) printDefaults(ctx context.Context, dst io.Writer, fs *flag.FlagSet) {
	print := r.DoPrintFlags
	if print == nil {
		print = defaultPrintFlags
	}
	print(ctx, dst, fs)
}

func (r *Runner) parseFlags(ctx context.Context, fs *flag.FlagSet, args []string) ([]string, error) {
	parse := r.DoParseFlags
	if parse == nil {
		parse = defaultParseFlags
	}
	return parse(ctx, fs, args)
}

func setup(ctx context.Context, cmd Command, name string) (context.Context, *flag.FlagSet) {
	fs := newFlagSet(name)
	defineFlags(cmd, fs)
	info := CommandInfo{
		Name:    name,
		Command: cmd,
		FlagSet: fs,
	}
	return withCommandInfo(ctx, info), fs
}

func run(ctx context.Context, cmd Command, name string, args []string) (err error) {
	ctx, fs := setup(ctx, cmd, name)
	args, err = contextRunner(ctx).parseFlags(ctx, fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return ErrHelp
	}
	if err != nil {
		return &ExitError{
			Code: 2,
			Err:  err,
		}
	}
	return cmd.Run(ctx, args)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {}
	fs.SetOutput(io.Discard)
	return fs
}

var defaultParseFlags = func(_ context.Context, fs *flag.FlagSet, args []string) ([]string, error) {
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return fs.Args(), nil
}

var defaultPrintFlags = func(_ context.Context, w io.Writer, fs *flag.FlagSet) error {
	prev := fs.Output()
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(prev)
	return nil
}

// ErrHelp is help asked for explicitly (-h or the help command).
//
// A Command may return it to have its help printed to Stdout with exit code 0.
var ErrHelp = errors.New("help requested")

// Usagef returns a UsageError with the message formatted as fmt.Errorf() does.
// That is, using %w verb in f wraps the error.
func Usagef(f string, args ...any) error {
	return &UsageError{
		Err: fmt.Errorf(f, args...),
	}
}

// UsageError represents an invocation that cannot run, such as a command
// container given no command.
//
// A Command may return it (for example, when given wrong positional arguments)
// to have its usage printed to Stderr with exit code 2.
//
// Err is the optional field describing what error caused this. It's printed to
// stderr before the usage text.
type UsageError struct {
	Err error
}

// Error implements error interface.
// It returns Err.Error() or, if Err is nil, a generic "usage error" message.
func (e *UsageError) Error() string {
	if e.Err == nil {
		return "usage error"
	}
	return e.Err.Error()
}

// Unwrap returns Err.
func (e *UsageError) Unwrap() error {
	return e.Err
}

// Exitf returns an error which makes Runner exit with the given code, printing
// the message formatted as fmt.Errorf() does to Stderr. That is, using %w verb
// in f wraps the error.
func Exitf(code int, f string, args ...any) error {
	return &ExitError{
		Code: code,
		Err:  fmt.Errorf(f, args...),
	}
}

// ExitError signals that program must be terminated with the given exit code.
// Non-nil Err is printed to Stderr before exit; nil Err means silent exit.
type ExitError struct {
	Code int
	Err  error
}

// Error implements error interface.
// It returns Err.Error() or, if Err is nil, a generic "exit status" message.
func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit status %d", e.Code)
	}
	return e.Err.Error()
}

// Unwrap returns Err.
func (e *ExitError) Unwrap() error {
	return e.Err
}
