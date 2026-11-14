package backoff

import (
	"context"
	"time"

	"ddia-hw2/internal/resilience"
	"ddia-hw2/internal/resilience/strategies/retry"
)

type Client struct {
	sender       resilience.SubrequestSender
	latency      resilience.LatencyBudget
	budget       resilience.RetryBudget
	initialDelay time.Duration
	sleeper      resilience.Sleeper
}

func NewClient(sender resilience.SubrequestSender, latency resilience.LatencyBudget,
	budget resilience.RetryBudget, initialDelay time.Duration, sleeper resilience.Sleeper) *Client {
	if initialDelay < 0 {
		panic("initialDelay must not be negative")
	}
	if sleeper == nil {
		sleeper = resilience.RealSleeper{}
	}

	return &Client{
		sender:       sender,
		latency:      latency,
		budget:       budget,
		initialDelay: initialDelay,
		sleeper:      sleeper,
	}
}

func (c *Client) Execute(ctx context.Context, request resilience.Request) (resilience.SubrequestResult, *resilience.StrategyError) {
	waiter := &backoffWaiter{
		nextDelay: c.initialDelay,
		sleeper:   c.sleeper,
	}
	retryClient := retry.NewClientWithWaiter(c.sender, c.latency, c.budget, waiter)
	return retryClient.Execute(ctx, request)
}

type backoffWaiter struct {
	nextDelay time.Duration
	sleeper   resilience.Sleeper
}

func (w *backoffWaiter) Wait(ctx context.Context, status resilience.Status, _ uint32) error {
	if status != resilience.ServerFailure && status != resilience.Timeout {
		return nil
	}
	if !delayFits(ctx, w.nextDelay) {
		return context.DeadlineExceeded
	}
	if err := w.sleeper.Sleep(ctx, w.nextDelay); err != nil {
		return err
	}
	w.nextDelay = w.nextDelay * 2
	return nil
}

func delayFits(ctx context.Context, delay time.Duration) bool {
	deadline, ok := ctx.Deadline()
	if !ok {
		return true
	}
	return time.Until(deadline) > delay
}
