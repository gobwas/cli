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
	NameProvider interface {
		Name() string
	}
	SynopsisProvider interface {
		Synopsis() string
	}
	DescriptionProvider interface {
		Description() string
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
	return
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

func name(cmd Command) string {
	if s, ok := cmd.(NameProvider); ok {
		return s.Name()
	}
	return ""
}
