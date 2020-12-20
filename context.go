package cli

import (
	"context"
	"flag"
	"strings"
)

type CommandInfo struct {
	Name    string
	Command Command
	FlagSet *flag.FlagSet
}

// NOTE: we don't provide custom context type.
// See https://github.com/golang/go/wiki/CodeReviewComments#contexts
type contextCommandsInfoKey struct{}

// WithCommandInfo returns a new context with given command info added to the
// execution path.
//
// WithCommandInfo is called internally during command chain execution.
// It might be called from third parties only for testing purposes.
func WithCommandInfo(ctx context.Context, info CommandInfo) context.Context {
	cs := append(ContextCommandsInfo(ctx), info)
	return context.WithValue(ctx, contextCommandsInfoKey{}, cs)
}

// ContextCommandsInfo returns commands execution path associated with context.
// Last element of the slice is the Command which Run() method is currently
// running.
//
// Callers must not mutate returned slice.
func ContextCommandsInfo(ctx context.Context) []CommandInfo {
	cs, _ := ctx.Value(contextCommandsInfoKey{}).([]CommandInfo)
	return cs
}

type contextRunnerKey struct{}

func withRunner(ctx context.Context, r *Runner) context.Context {
	return context.WithValue(ctx, contextRunnerKey{}, r)
}

func contextRunner(ctx context.Context) *Runner {
	r, _ := ctx.Value(contextRunnerKey{}).(*Runner)
	if r == nil {
		panic("cli: internal error: no runner associated with context")
	}
	return r
}

func lastCommandInfo(ctx context.Context) CommandInfo {
	cs := ContextCommandsInfo(ctx)
	n := len(cs)
	if n == 0 {
		panic("cli: internal error: no commands associated with context")
	}
	return cs[n-1]
}

func commandPath(ctx context.Context) string {
	var sb strings.Builder
	for _, c := range ContextCommandsInfo(ctx) {
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(c.Name)
	}
	return sb.String()
}
