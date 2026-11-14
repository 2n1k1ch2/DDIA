package testutil

import (
	"context"
	"sync"
	"time"
)

type FakeSleeper struct {
	mu     sync.Mutex
	delays []time.Duration
}

func (s *FakeSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.delays = append(s.delays, delay)
	return nil
}

func (s *FakeSleeper) Delays() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	delays := make([]time.Duration, len(s.delays))
	copy(delays, s.delays)
	return delays
}
