package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
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
}

// Main runs given command.
// It does some i/o, such that printing help messages or errors returned from
// cmd.Error().
func (r *Runner) Main(cmd Command) {
	baseCtx := context.Background()
	if len(r.TermSignals) > 0 {
		var cancel context.CancelFunc
		baseCtx, cancel = withTrapCancel(baseCtx, r.TermSignals...)
		defer cancel()
	}
	if n := r.ForceTerm; n > 0 {
		trapSeq(n, r.TermSignals, func(os.Signal) {
			os.Exit(130)
		})
	}

	ctx := withRunner(baseCtx, r)

	exe := name(cmd)
	if exe == "" {
		exe = path.Base(os.Args[0])
	}
	err := run(ctx, cmd, exe, os.Args[1:])
	if err == errHelp {
		var buf bytes.Buffer
		r.printUsage(ctx, &buf)
		r.printFlags(ctx, &buf)
		r.output(ctx, &buf)
		os.Exit(0)
	}
	if baseCtx.Err() != nil {
		os.Exit(130)
	}
	var e *exitError
	if errors.As(err, &e) {
		fmt.Println(err)
		os.Exit(e.code)
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
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

func (r *Runner) output(ctx context.Context, src io.Reader) {
	io.Copy(os.Stdout, src)
}

func (r *Runner) parseFlags(ctx context.Context, fs *flag.FlagSet, args []string) ([]string, error) {
	parse := r.DoParseFlags
	if parse == nil {
		parse = defaultParseFlags
	}
	return parse(ctx, fs, args)
}

func setup(ctx context.Context, cmd Command, name string) (context.Context, *flag.FlagSet) {
	var fs *flag.FlagSet
	if _, ok := cmd.(FlagDefiner); ok {
		fs = newFlagSet(name)
		defineFlags(cmd, fs)
	}
	info := CommandInfo{
		Name:    name,
		Command: cmd,
		FlagSet: fs,
	}
	return WithCommandInfo(ctx, info), fs
}

func run(ctx context.Context, cmd Command, name string, args []string) (err error) {
	ctx, fs := setup(ctx, cmd, name)
	if fs != nil {
		args, err = contextRunner(ctx).parseFlags(ctx, fs, args)
		// NOTE: we are using errors.Is() here to allow the use of fmt.Errorf()
		// with `%w` verb.
		if errors.Is(err, flag.ErrHelp) {
			err = errHelp
		}
		if err != nil {
			return err
		}
	}
	return cmd.Run(ctx, args)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {}
	fs.SetOutput(ioutil.Discard)
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

var errHelp = errors.New("help requested")

// Exitf creates an error which reception cause Runner.Main() to exit with
// given code preceded by formatted message.
func Exitf(code int, f string, args ...interface{}) error {
	e := &exitError{
		code: code,
	}
	return fmt.Errorf(fmt.Sprintf(f, args...)+"%w", e)
}

type exitError struct {
	code int
}

func (e *exitError) Error() string {
	return ""
}
