package cli

import (
	"context"
	"os"
	"os/signal"
)

func withTrapCancel(ctx context.Context, ss ...os.Signal) (context.Context, context.CancelFunc) {
	ret, cancel := context.WithCancel(ctx)
	ch := make(chan os.Signal, len(ss))
	go func() {
		defer signal.Stop(ch)
		<-ch
		cancel()
	}()
	signal.Notify(ch, ss...)
	return ret, cancel
}

func trapSeq(n int, ss []os.Signal, fn func(os.Signal)) {
	var (
		ch  = make(chan os.Signal, len(ss))
		cnt = make(map[os.Signal]int)
	)
	go func() {
		defer signal.Stop(ch)
		for sig := range ch {
			m := cnt[sig] + 1
			if m != n {
				cnt[sig] = m
			} else {
				cnt[sig] = 0
				fn(sig)
			}
		}
	}()
	signal.Notify(ch, ss...)
}
