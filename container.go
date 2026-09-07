package cli

import (
	"context"
	"flag"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

// Commands holds a mapping of sub command name to its implementation.
type Commands map[string]Command

var _ interface { // Compile time checks of desired interfaces implementation.
	Command
	SynopsisProvider
	DescriptionProvider
} = Commands{}

// Run implements Command interface.
func (c Commands) Run(ctx context.Context, args []string) error {
	var help bool
top:
	if len(args) == 0 {
		if help {
			return errHelp
		}
		return errUsage
	}
	if args[0] == "help" {
		help = true
		args = args[1:]
		goto top
	}
	name := args[0]
	cmd := c[name]
	if cmd == nil {
		return Exitf(2,
			"`%[1]s %[2]s`: unknown command\nRun `%[1]s help` for help.",
			commandPath(ctx), name,
		)
	}
	if help {
		setup(ctx, cmd, name)
		return errHelp
	}
	return run(ctx, cmd, name, args[1:])
}

// Synopsis implements SynopsisProvider interface.
func (c Commands) Synopsis() string {
	return "[help] <command>"
}

// Description implements DescriptionProvider interface.
func (c Commands) Description() string {
	keys := slices.Sorted(maps.Keys(c))
	width := 0
	for _, key := range keys {
		width = max(width, utf8.RuneCountInString(key))
	}
	// NOTE: we align columns by hand instead of using text/tabwriter since it
	// pads every tab-terminated cell, leaving trailing whitespace on the rows
	// without a name, and it treats the last row without trailing newline
	// differently from the rest, making the output inconsistent.
	var sb strings.Builder
	sb.WriteString("Commands:")
	for _, key := range keys {
		fmt.Fprintf(&sb, "\n  %s", key)
		// Pad only when there is something to align, to not leave trailing
		// whitespace.
		if s := name(c[key]); s != "" {
			pad := width - utf8.RuneCountInString(key) + 2
			fmt.Fprintf(&sb, "%*s%s", pad, "", s)
		}
	}
	return sb.String()
}

var _ interface { // Compile time checks of desired interfaces implementation.
	Command
	NameProvider
	SynopsisProvider
	DescriptionProvider
	FlagDefiner
} = (*Container)(nil)

// Container is a Command wrapper which allows to modify behaviour of the
// Command it wraps.
type Container struct {
	Command Command

	// DoRun allows to override Command behaviour.
	DoRun func(context.Context, []string) error
	// DoName allows to override NameProvider behaviour.
	DoName func() string
	// DoSynopsis allows to override SynopsisProvider behaviour.
	DoSynopsis func() string
	// DoDescription allows to override DescriptionProvider behaviour.
	DoDescription func() string
	// DoDefineFlags allows to override FlagDefiner behaviour.
	DoDefineFlags func(*flag.FlagSet)
}

// Run implements Command interface.
//
// NOTE: we are explicit here (and are not embedding Command) to not allow the
// use of non-pointer Container type as a Command. Otherwise Container would
// not implement helper interfaces but still be a valid Command.
func (c *Container) Run(ctx context.Context, args []string) error {
	if f := c.DoRun; f != nil {
		return f(ctx, args)
	}
	return c.Command.Run(ctx, args)
}

// Name implements NameProvider interface.
func (c *Container) Name() string {
	if f := c.DoName; f != nil {
		return f()
	}
	return name(c.Command)
}

// Synopsis implements SynopsisProvider interface.
func (c *Container) Synopsis() string {
	if f := c.DoSynopsis; f != nil {
		return f()
	}
	return synopsis(c.Command)
}

// Description implements DescriptionProvider interface.
func (c *Container) Description() string {
	if f := c.DoDescription; f != nil {
		return f()
	}
	return description(c.Command)
}

// DefineFlags implements FlagDefiner interface.
func (c *Container) DefineFlags(fs *flag.FlagSet) {
	if f := c.DoDefineFlags; f != nil {
		f(fs)
		return
	}
	defineFlags(c.Command, fs)
}
