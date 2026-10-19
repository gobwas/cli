package cli

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
)

// SignalError is the cancellation cause of a context the Runner cancelled on a
// signal: context.Cause(ctx) returns it, so a command can tell an interruption
// from any other cancellation.
type SignalError struct {
	Signal os.Signal
}

// Error implements error interface.
func (e *SignalError) Error() string {
	return "cancelled by signal: " + e.Signal.String()
}

// signalExitCode returns the exit code conventional for a process terminated
// by sig: 128 plus the signal number, or 130 if the number is unknown.
func signalExitCode(sig os.Signal) int {
	if s, ok := sig.(syscall.Signal); ok {
		return 128 + int(s)
	}
	return 130
}

// contextExitCode returns the exit code for a cancelled ctx: the one of the
// signal which cancelled it, or 130 if it was cancelled for another reason.
func contextExitCode(ctx context.Context) int {
	if e, ok := errors.AsType[*SignalError](context.Cause(ctx)); ok {
		return signalExitCode(e.Signal)
	}
	return 130
}

// withTrapCancel returns a context which is cancelled on reception of any of
// the given signals. The signal is available as the cancellation cause.
//
// Only the first signal is trapped: after it, the signal handler is removed,
// so that the second signal (unless trapped by ForceTerm) terminates the
// process with the default disposition. That is, a command which does not
// respect context cancellation can still be interrupted.
func withTrapCancel(ctx context.Context, ss ...os.Signal) (context.Context, context.CancelFunc) {
	ret, cancel := context.WithCancelCause(ctx)
	ch := make(chan os.Signal, len(ss))
	go func() {
		defer signal.Stop(ch)
		sig := <-ch
		cancel(&SignalError{Signal: sig})
	}()
	signal.Notify(ch, ss...)
	return ret, func() { cancel(nil) }
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
