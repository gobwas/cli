package main

import (
	"context"
	"flag"
	"io"
	"os"
	"path"

	"github.com/gobwas/cli"
	"github.com/gobwas/flagutil"
	"github.com/gobwas/flagutil/parse/file"
	"github.com/gobwas/flagutil/parse/file/yaml"
	"github.com/gobwas/flagutil/parse/pargs"
)

type core struct {
	verbose bool
}

func main() {
	var core core
	r := cli.Runner{
		// Override flags parsing to use flagutil package.
		// It allows us to have fancy things like flag shortucts and
		// posix-compatible flags syntax.
		DoParseFlags: func(ctx context.Context, fs *flag.FlagSet, args []string) ([]string, error) {
			opts, rest := parseOptions(fs, args)
			err := flagutil.Parse(ctx, fs, opts...)
			if err != nil {
				return nil, err
			}
			return rest(), err
		},
		// Override help message printing. It will print pretty help message
		// which is aware of flag shortcuts used by flagutil package.
		DoPrintFlags: func(ctx context.Context, w io.Writer, fs *flag.FlagSet) error {
			// Be kind and restore original output writer.
			orig := fs.Output()
			fs.SetOutput(w)
			defer fs.SetOutput(orig)

			// Note that to print right help message we have to use same parse
			// options we used in DoParseFlags() above. That's why here is
			// parseOptions() helper func.
			opts, _ := parseOptions(fs, nil)

			return flagutil.PrintDefaults(ctx, fs, opts...)
		},
	}
	r.Main(&cli.Container{
		Command: cli.Commands{
			// NOTE: we wrap original command here to make it parse
			// configuration file before actual Run() happens.
			//
			// This is an easy way to determine the "target" command. That is,
			// the command which is the latest in the execution path.
			// We need this since we don't want to parse configuration file per
			// each command in the path because not all of commands defined
			// their flags yet.
			"ping": wrap(&ping{
				core: &core,
			}),
		},
		// Define the "core" global flags which we can use then in sub commands
		// (if the core struct were injected).
		DoDefineFlags: func(fs *flag.FlagSet) {
			fs.BoolVar(&core.verbose,
				"verbose", false,
				"be verbose",
			)
		},
	})
}

func wrap(cmd cli.Command) cli.Command {
	return &cli.Container{
		Command: cmd,
		DoRun: func(ctx context.Context, args []string) error {
			if err := parseConfigFile(ctx); err != nil {
				return err
			}
			return cmd.Run(ctx, args)
		},
	}
}

func parseConfigFile(ctx context.Context) error {
	all := mergeFlags(ctx)
	return flagutil.Parse(ctx, all,
		flagutil.WithParser(
			&file.Parser{
				Lookup: file.PathLookup(configPath()),
				Syntax: new(yaml.Syntax),
			},
		),
	)
}

// mergeFlags prepares merge of every command's flag set into one superset.
// It adds command name as a prefix for every subset.
func mergeFlags(ctx context.Context) *flag.FlagSet {
	all := flag.NewFlagSet("all", flag.PanicOnError)
	for i, cmd := range cli.ContextCommandsInfo(ctx) {
		if cmd.FlagSet == nil {
			continue
		}
		name := cmd.Name
		if i == 0 {
			name = "core"
		}
		flagutil.Subset(all, name, func(sub *flag.FlagSet) {
			// Combine command flag set into a new empty subset.
			// This makes setting flag value of a subset also change original
			// command flag set.
			*sub = *flagutil.CombineSets(sub, cmd.FlagSet)
		})
		// Mark already specified flags in command flag set as specified in
		// superset as well. This makes command line options prioritized over
		// file configuration.
		cmd.FlagSet.Visit(func(f *flag.Flag) {
			flagutil.SetActual(all, name+"."+f.Name)
		})
	}
	return all
}

func parseOptions(fs *flag.FlagSet, args []string) (
	opts []flagutil.ParseOption,
	rest func() []string,
) {
	posix := &pargs.Parser{
		Args:      args,
		Shorthand: true,
	}
	opts = []flagutil.ParseOption{
		flagutil.WithParser(posix),
	}
	return opts, posix.NonOptionArgs
}

func configPath() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path.Join(dir, "examples/complex/config.yaml")
}
