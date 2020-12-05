# cli

[![PkgGoDev][pkggodev:badge]][pkggodev:url]

> Package cli is a tiny and minimalistic CLI library for Go.

## Why make another one?

This package is aimed to reach out these goals:

1. To be simple as possible.
2. To be flexible until you not violate (1).

There are few libraries to build CLI for sure, but sometimes you don't want to
spend much time on learning their philosophy. To be honest it's almost always
much easier to write your own `map[string]func()`. This library allows you to
save the time spent on writing that.

## Concepts

There is a `cli.Command` interface which has single `Run()` method.
You can group commands by using `cli.Commands` map type.
Additional behaviour can be added by implementing few optional interfaces such
as `cli.FlagDefiner`.

## Limitations

There is no "persistent" or "global" flags. That is, flags are parsed exactly
for command they appear after.

Implementing this is not a big deal, but it complicates library API enough.
However, it's possible to "mixin" global flags into each command's flag set
manually (optionally resolving cases when global flag was set twice on the path
or was rewritten by env value if custom flags parsing used).

## Usage

```go
package main

import (
	"context"

	"github.com/gobwas/cli"
)

func main() {
	cli.Main(cli.Commands{
		"sleep": new(sleepCommand),
	})
}

type sleepCommand struct {
	duration time.Duration
}

func (s *sleepCommand) DefineFlags(fs *flag.FlagSet) {
	fs.DurationVar(&s.duration,
		"d", s.duration,
		"how long to sleep",
	)
}

func (s *sleepCommand) Run(ctx context.Context, _ []string) error {
	select {
	case <-ctx.Done(): // SIGINT or SIGTERM received.
		return ctx.Err()
	case <-time.After(s.duration):
		return nil
	}
}

```

> Note that `context.Context` instance passed to the `Run()` method will be
> cancelled by default if process receives _SIGTERM_ or _SIGINT_ signals. See
> [`cli.Runner`][docs:Runner] and [`cli.DefaultRunner`][docs:DefaultRunner]
> docs for more info.

Without help message customization, help request will output this:

```
$ go run ./example sleep -h
Usage:

  example sleep

Options:

  -d duration
        how long to sleep (default 1s)
```

However, you can implement optional `cli.NameProvider` and, say,
`cli.SynopsisProvider` to make help message more specific:

```go
func (s *sleepCommand) Name() string {
	return "Suspends execution for a given amount of time."
}

func (s *sleepCommand) Synopsis() string {
	return "[-d duration]"
}
```

Now the output will look like this:

```
$ go run ./example help sleep
Suspends execution for a given amount of time.

Usage:

  example sleep [-d duration]

Options:

  -d duration
        how long to sleep (default 1s)
```

Help for whole binary will look like this:

```
$ go run ./example help
Usage:

  example [help] <command>

Commands:
  sleep  Suspends execution for a given amount of time.
```

To customize the `cli.Commands` help output you can use `cli.Container`
wrapper.

See the [example][example] folder for more info.

[flagutil]:           https://github.com/gobwas/flagutil
[example]:            https://github.com/gobwas/cli/tree/main/example
[docs:Runner]:        https://pkg.go.dev/github.com/gobwas/cli#Runner
[docs:DefaultRunner]: https://pkg.go.dev/github.com/gobwas/cli#DefaultRunner
[pkggodev:badge]:     https://pkg.go.dev/badge/github.com/gobwas/cli
[pkggodev:url]:       https://pkg.go.dev/github.com/gobwas/cli
