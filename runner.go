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
	"syscall"
)

// DefaultRunner is an instance of Runner used by Main().
var DefaultRunner = Runner{
	TermSignals: []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
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
	if n := r.ForceTerm; n > 0 {
		trapSeq(n, r.TermSignals, func(os.Signal) {
			os.Exit(130)
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
// 130.
func (r *Runner) Run(ctx context.Context, cmd Command, name string, args []string) int {
	baseCtx := ctx
	ctx = withRuntimeInfo(ctx, &runtimeInfo{
		runner: r,
	})
	err := run(ctx, cmd, name, args)
	if errors.Is(err, errHelp) {
		// Help was asked for: it is the output, so stdout and success.
		var buf bytes.Buffer
		r.printUsage(ctx, &buf)
		r.printFlags(ctx, &buf)
		io.Copy(r.stdout(), &buf)
		return 0
	}
	if errors.Is(err, errUsage) {
		// Help was not asked for; the invocation was wrong. Usage goes
		// where errors go, with the usage exit code.
		var buf bytes.Buffer
		r.printUsage(ctx, &buf)
		r.printFlags(ctx, &buf)
		io.Copy(r.stderr(), &buf)
		return 2
	}
	if baseCtx.Err() != nil {
		return 130
	}
	if e, ok := errors.AsType[*exitError](err); ok {
		fmt.Fprintln(r.stderr(), err)
		return e.code
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
	if s := name(cmd); s != "" {
		fmt.Fprintln(dst, s)
		fmt.Fprintln(dst)
	}
	fmt.Fprintln(dst, "Usage:")
	fmt.Fprintln(dst)
	fmt.Fprintf(dst, "  %s %s\n", commandPath(ctx), synopsis(cmd))
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
	io.Copy(dst, &buf)
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
	// Every command parses, flags or not: -h works everywhere, and a stray
	// flag is refused instead of being passed on as an argument.
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
	// NOTE: we are using errors.Is() here to allow the use of fmt.Errorf()
	// with `%w` verb.
	if errors.Is(err, flag.ErrHelp) {
		return errHelp
	}
	if err != nil {
		// A flag the parser refused is a usage error, like a missing
		// command: the parser's message, exit 2.
		return Exitf(2, "%v", err)
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

var (
	// errHelp is help asked for explicitly (-h or the help command).
	errHelp = errors.New("help requested")
	// errUsage is an invocation that cannot run, such as a command
	// container given no command.
	errUsage = errors.New("usage error")
)

// Exitf creates an error which reception cause Runner.Main() to exit with
// given code preceded by formatted message.
func Exitf(code int, f string, args ...any) error {
	return &exitError{
		code:   code,
		reason: fmt.Sprintf(f, args...),
	}
}

type exitError struct {
	code   int
	reason string
}

func (e *exitError) Error() string {
	return e.reason
}
