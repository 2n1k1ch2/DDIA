package resilience

import (
	"context"
	"time"
)

type Sleeper interface {
	Sleep(ctx context.Context, delay time.Duration) error
}

type RealSleeper struct{}

func (RealSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
