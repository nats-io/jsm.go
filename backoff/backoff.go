package backoff

import (
	"context"
	"math/rand"
	"time"
)

// Policy implements a backoff policy, randomizing its delays
// and saturating at the final value in Millis.
type Policy struct {
	Millis []int
}

// FiveSecStartGrace Like FiveSec but allows for a few fairly rapid initial tries
var FiveSecStartGrace = Policy{
	Millis: []int{0, 25, 50, 75, 100, 125, 150, 200, 250, 500, 750, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000},
}

// FiveSec is a backoff policy ranging up to 5 seconds.
var FiveSec = Policy{
	Millis: []int{500, 750, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000},
}

// TwentySec is a backoff policy ranging up to 20 seconds
var TwentySec = Policy{
	Millis: []int{
		500, 750, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000,
		5500, 5750, 6000, 6500, 7000, 7500, 8000, 8500, 9000, 9500, 10000,
		10500, 10750, 11000, 11500, 12000, 12500, 13000, 13500, 14000, 14500, 15000,
		15500, 15750, 16000, 16500, 17000, 17500, 18000, 18500, 19000, 19500, 20000,
	},
}

// Default is the default backoff policy to use
var Default = FiveSec

// Duration returns the time duration of the n'th wait cycle in a
// backoff policy. This is b.Millis[n], randomized to avoid thundering
// herds.
func (b Policy) Duration(n int) time.Duration {
	if n >= len(b.Millis) {
		n = len(b.Millis) - 1
	}

	return time.Duration(jitter(b.Millis[n])) * time.Millisecond
}

// TrySleep sleeps for the duration of the n'th try cycle
// in a way that can be interrupted by the context.  An error is returned
// if the context cancels the sleep
func (b Policy) TrySleep(ctx context.Context, n int) error {
	return b.Sleep(ctx, b.Duration(n))
}

// Sleep sleeps for the duration t and can be interrupted by ctx. An error
// is returns if the context cancels the sleep
func (b Policy) Sleep(ctx context.Context, t time.Duration) error {
	timer := time.NewTimer(t)

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	}
}

// jitter returns a random integer uniformly distributed in the range
// [0.5 * millis .. 1.5 * millis]
func jitter(millis int) int {
	if millis == 0 {
		return 0
	}

	return millis/2 + rand.Intn(millis)
}
