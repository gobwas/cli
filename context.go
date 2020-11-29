package cli

import (
	"container/list"
	"context"
	"flag"
	"strings"
)

type contextRunnerInfoKey struct{}

type runnerInfo struct {
	runner   *Runner
	commands list.List
}

func withRunnerInfo(ctx context.Context, r *runnerInfo) context.Context {
	return context.WithValue(ctx, contextRunnerInfoKey{}, r)
}

func contextRunnerInfo(ctx context.Context) *runnerInfo {
	r, _ := ctx.Value(contextRunnerInfoKey{}).(*runnerInfo)
	if r == nil {
		panic("cli: internal error: no runner info associated with context")
	}
	return r
}

func contextRunner(ctx context.Context) *Runner {
	return contextRunnerInfo(ctx).runner
}

type commandInfo struct {
	cmd     Command
	name    string
	flagSet *flag.FlagSet
}

func pushCommandInfo(ctx context.Context, c *commandInfo) {
	r := contextRunnerInfo(ctx)
	r.commands.PushBack(c)
}

func forEachCommandInfo(ctx context.Context, it func(*commandInfo)) {
	info := contextRunnerInfo(ctx)
	for el := info.commands.Front(); el != nil; el = el.Next() {
		it(el.Value.(*commandInfo))
	}
}

func lastCommandInfo(ctx context.Context) *commandInfo {
	info := contextRunnerInfo(ctx)
	return info.commands.Back().Value.(*commandInfo)
}

func commandPath(ctx context.Context) string {
	var sb strings.Builder
	forEachCommandInfo(ctx, func(info *commandInfo) {
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(info.name)
	})
	return sb.String()
}
