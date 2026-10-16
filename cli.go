package cli

import (
	"context"
	"flag"
)

// Command is the interface that holds command execution logic.
type Command interface {
	Run(ctx context.Context, args []string) error
}

// CommandFunc is an adapter to allow the use of ordinary functions as
// Commands.
type CommandFunc func(ctx context.Context, args []string) error

// Run implements Command interface.
func (f CommandFunc) Run(ctx context.Context, args []string) error {
	return f(ctx, args)
}

type (
	// SummaryProvider gives the one-line summary listed next to the
	// command's name.
	SummaryProvider interface {
		Summary() string
	}
	// PrologProvider gives the text that opens the command's help, before its
	// usage.
	PrologProvider interface {
		Prolog() string
	}
	SynopsisProvider interface {
		Synopsis() string
	}
	DescriptionProvider interface {
		Description() string
	}
	// EpilogProvider gives the text that closes the command's help, after its
	// options.
	EpilogProvider interface {
		Epilog() string
	}
	FlagDefiner interface {
		DefineFlags(*flag.FlagSet)
	}
)

func defineFlags(cmd Command, fs *flag.FlagSet) {
	d, ok := cmd.(FlagDefiner)
	if !ok {
		return
	}
	d.DefineFlags(fs)
}

func synopsis(cmd Command) string {
	if s, ok := cmd.(SynopsisProvider); ok {
		return s.Synopsis()
	}
	return ""
}

func description(cmd Command) string {
	if s, ok := cmd.(DescriptionProvider); ok {
		return s.Description()
	}
	return ""
}

func summary(cmd Command) string {
	if s, ok := cmd.(SummaryProvider); ok {
		return s.Summary()
	}
	return ""
}

func prolog(cmd Command) string {
	if s, ok := cmd.(PrologProvider); ok {
		return s.Prolog()
	}
	return ""
}

func epilog(cmd Command) string {
	if s, ok := cmd.(EpilogProvider); ok {
		return s.Epilog()
	}
	return ""
}
