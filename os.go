package cli

import (
	"context"
	"os"
	"os/signal"
)

// withTrapCancel returns a context which is cancelled on reception of any of
// the given signals.
//
// Only the first signal is trapped: after it, the signal handler is removed,
// so that the second signal (unless trapped by ForceTerm) terminates the
// process with the default disposition. That is, a command which does not
// respect context cancellation can still be interrupted.
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
