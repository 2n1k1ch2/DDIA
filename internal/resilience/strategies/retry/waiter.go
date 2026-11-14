package retry

import (
	"context"

	"ddia-hw2/internal/resilience"
)

type Waiter interface {
	Wait(ctx context.Context, status resilience.Status, retryNumber uint32) error
}

type noWaiter struct{}

func (noWaiter) Wait(context.Context, resilience.Status, uint32) error { return nil }
