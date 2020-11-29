package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/gobwas/cli"
)

func main() {
	cli.Main(cli.Commands{
		"sleep": &sleepCommand{
			duration: time.Second, // Default value.
		},
	})
}

// Compile time check that sleepCommand implements all interfaces we want.
var _ interface {
	cli.Command
	cli.FlagDefiner
	cli.NameProvider
	cli.SynopsisProvider
} = new(sleepCommand)

type sleepCommand struct {
	duration time.Duration
}

func (s *sleepCommand) DefineFlags(fs *flag.FlagSet) {
	fs.DurationVar(&s.duration,
		"d", s.duration,
		"how long to sleep",
	)
}

func (s *sleepCommand) Name() string {
	return "Suspends execution for a given amount of time."
}

func (s *sleepCommand) Synopsis() string {
	return "[-d duration]"
}

func (s *sleepCommand) Run(ctx context.Context, _ []string) error {
	fmt.Printf("going to sleep for %s\n", s.duration)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.duration):
		return nil
	}
}
