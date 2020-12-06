package cli

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
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
		return errHelp
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
	var sb strings.Builder
	cs := make([]string, 0, len(c))
	for key := range c {
		cs = append(cs, key)
	}
	sort.Strings(cs)
	fmt.Fprintln(&sb, "Commands:")
	tw := tabwriter.NewWriter(&sb, 0, 1, 2, ' ', 0)
	for i, key := range cs {
		if i > 0 {
			fmt.Fprintln(tw)
		}
		cmd := c[key]
		fmt.Fprintf(tw, "  %s\t%s", key, name(cmd))
	}
	tw.Flush()

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
///
// NOTE: we are explicit here (and not embed Command) to now allow the use of
// non-pointer Container type as a Command. In that case Container would not
// implement all helper interfaces.
func (c *Container) Run(ctx context.Context, args []string) error {
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
