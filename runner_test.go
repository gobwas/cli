package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"
)

type testCommand struct {
	name     string
	synopsis string
	flags    func(*flag.FlagSet)
	run      func(context.Context, []string) error
}

func (c *testCommand) Name() string     { return c.name }
func (c *testCommand) Synopsis() string { return c.synopsis }

func (c *testCommand) DefineFlags(fs *flag.FlagSet) {
	if c.flags != nil {
		c.flags(fs)
	}
}

func (c *testCommand) Run(ctx context.Context, args []string) error {
	if c.run != nil {
		return c.run(ctx, args)
	}
	return nil
}

func TestRunnerRun(t *testing.T) {
	var (
		actArgs []string
		verbose bool
	)
	root := Commands{
		"sleep": &testCommand{
			name:     "Sleeps.",
			synopsis: "[-v] [arg...]",
			flags: func(fs *flag.FlagSet) {
				fs.BoolVar(&verbose, "v", false, "be verbose")
			},
			run: func(_ context.Context, args []string) error {
				actArgs = args
				return nil
			},
		},
		"fail": CommandFunc(func(context.Context, []string) error {
			return errors.New("boom")
		}),
		"exit": CommandFunc(func(context.Context, []string) error {
			return Exitf(42, "exit with %d%%", 42)
		}),
		"silent": CommandFunc(func(context.Context, []string) error {
			return &ExitError{Code: 1}
		}),
		"stop": CommandFunc(func(context.Context, []string) error {
			return &ExitError{Code: 0}
		}),
		"wrap": CommandFunc(func(context.Context, []string) error {
			return Exitf(3, "wrapped: %w", os.ErrNotExist)
		}),
		"usage": CommandFunc(func(context.Context, []string) error {
			return ErrUsage
		}),
		"wait": CommandFunc(func(ctx context.Context, _ []string) error {
			<-ctx.Done()
			return ctx.Err()
		}),
	}
	sleepHelp := strings.Join([]string{
		"Sleeps.",
		"",
		"Usage:",
		"",
		"  prog sleep [-v] [arg...]",
		"",
		"Options:",
		"",
		"  -v\tbe verbose",
		"",
	}, "\n")
	rootUsage := strings.Join([]string{
		"Usage:",
		"",
		"  prog [help] <command>",
		"",
		"Commands:",
		"  exit",
		"  fail",
		"  silent",
		"  sleep   Sleeps.",
		"  stop",
		"  usage",
		"  wait",
		"  wrap",
		"",
		"",
	}, "\n")

	for _, test := range []struct {
		name   string
		args   []string
		cancel bool
		signal os.Signal

		code    int
		stdout  string
		stderr  string
		expArgs []string
		verbose bool
	}{
		{
			name: "no command",
			args: nil,

			code:   2,
			stderr: rootUsage,
		},
		{
			name: "help",
			args: []string{"help"},

			code:   0,
			stdout: rootUsage,
		},
		{
			name: "root -h",
			args: []string{"-h"},

			code:   0,
			stdout: rootUsage,
		},
		{
			name: "help command",
			args: []string{"help", "sleep"},

			code:   0,
			stdout: sleepHelp,
		},
		{
			name: "command -h",
			args: []string{"sleep", "-h"},

			code:   0,
			stdout: sleepHelp,
		},
		{
			name: "unknown command",
			args: []string{"nope"},

			code: 2,
			stderr: "`prog nope`: unknown command\n" +
				"Run `prog help` for help.\n",
		},
		{
			name: "unknown flag",
			args: []string{"sleep", "-x"},

			code:   2,
			stderr: "flag provided but not defined: -x\n",
		},
		{
			name: "unknown flag with percent",
			args: []string{"sleep", "-%d"},

			code:   2,
			stderr: "flag provided but not defined: -%d\n",
		},
		{
			name: "run with flags and args",
			args: []string{"sleep", "-v", "a", "-x", "--", "b"},

			code:    0,
			expArgs: []string{"a", "-x", "--", "b"},
			verbose: true,
		},
		{
			name: "run with args after separator",
			args: []string{"sleep", "--", "-x"},

			code:    0,
			expArgs: []string{"-x"},
		},
		{
			name: "error",
			args: []string{"fail"},

			code:   1,
			stderr: "boom\n",
		},
		{
			name: "exit error",
			args: []string{"exit"},

			code:   42,
			stderr: "exit with 42%\n",
		},
		{
			name: "exit error without reason",
			args: []string{"silent"},

			code:   1,
			stderr: "",
		},
		{
			name: "exit error with zero code",
			args: []string{"stop"},

			code:   0,
			stderr: "",
		},
		{
			name: "exit error wrapping",
			args: []string{"wrap"},

			code:   3,
			stderr: "wrapped: file does not exist\n",
		},
		{
			name: "usage error from command",
			args: []string{"usage"},

			code: 2,
			stderr: strings.Join([]string{
				"Usage:",
				"",
				"  prog usage ",
				"",
				"",
			}, "\n"),
		},
		{
			name:   "cancelled",
			args:   []string{"wait"},
			cancel: true,

			code: 130,
		},
		{
			name:   "cancelled by signal",
			args:   []string{"wait"},
			cancel: true,
			signal: syscall.SIGTERM,

			code: 143,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			actArgs = nil
			verbose = false

			var stdout, stderr bytes.Buffer
			r := Runner{
				Stdout: &stdout,
				Stderr: &stderr,
			}
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			if test.cancel {
				var cause error
				if test.signal != nil {
					cause = &signalError{test.signal}
				}
				cancel(cause)
			}
			code := r.Run(ctx, root, "prog", test.args)
			if code != test.code {
				t.Errorf("exit code is %d; want %d", code, test.code)
			}
			if act, exp := stdout.String(), test.stdout; act != exp {
				t.Errorf("stdout:\n%s\nwant:\n%s", act, exp)
			}
			if act, exp := stderr.String(), test.stderr; act != exp {
				t.Errorf("stderr:\n%s\nwant:\n%s", act, exp)
			}
			if act, exp := actArgs, test.expArgs; !slices.Equal(act, exp) {
				t.Errorf("args are %q; want %q", act, exp)
			}
			if verbose != test.verbose {
				t.Errorf("verbose is %t; want %t", verbose, test.verbose)
			}
		})
	}
}
