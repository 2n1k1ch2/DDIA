package circuitbreaker

import (
	"context"
	"sync"
	"time"

	"ddia-hw2/internal/resilience"
	"ddia-hw2/internal/resilience/strategies/retry"
)

type Config struct {
	WindowSize       int
	FailureThreshold float64
	Cooldown         time.Duration
}

type Client struct {
	senders []resilience.SubrequestSender
	latency resilience.LatencyBudget
	budget  resilience.RetryBudget
	config  Config

	mu     sync.Mutex
	states []senderState
}

type senderState struct {
	history     *history
	openedUntil time.Time
}

func NewClient(senders []resilience.SubrequestSender, latency resilience.LatencyBudget, budget resilience.RetryBudget, config Config) *Client {
	validateConfig(config)
	states := make([]senderState, len(senders))
	for i := range states {
		states[i].history = newHistory(config.WindowSize)
	}

	return &Client{
		senders: append([]resilience.SubrequestSender(nil), senders...),
		latency: latency,
		budget:  budget,
		config:  config,
		states:  states,
	}
}

func (c *Client) Execute(mainCtx context.Context, request resilience.Request) (resilience.SubrequestResult, *resilience.StrategyError) {
	if len(c.senders) == 0 {
		return resilience.SubrequestResult{}, &resilience.StrategyError{
			Reason:     resilience.NoAvailableSender,
			LastStatus: resilience.Unknown,
			Attempts:   0,
		}
	}

	circuitBreaker := &circuitBreakerSender{
		client:     c,
		lastStatus: resilience.Unknown,
	}
	retryClient := retry.NewClient(circuitBreaker, c.latency, c.budget)
	response, strategyErr := retryClient.Execute(mainCtx, request)
	if circuitBreaker.noAvailableSender {
		return resilience.SubrequestResult{}, &resilience.StrategyError{
			Reason:     resilience.NoAvailableSender,
			LastStatus: circuitBreaker.lastStatus,
			Attempts:   circuitBreaker.attempts,
		}
	}
	return response, strategyErr
}

type circuitBreakerSender struct {
	client            *Client
	next              int
	attempts          uint32
	lastStatus        resilience.Status
	noAvailableSender bool
}

func (s *circuitBreakerSender) Send(ctx context.Context, request resilience.Request) resilience.SubrequestResult {
	index, ok := s.nextAvailableSender()
	if !ok {
		s.noAvailableSender = true
		return resilience.SubrequestResult{Status: resilience.Unknown}
	}

	response := s.client.senders[index].Send(ctx, request)
	s.attempts++
	s.lastStatus = response.Status
	s.client.recordResult(index, response.Status)
	return response
}

func (s *circuitBreakerSender) nextAvailableSender() (int, bool) {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	now := time.Now()
	for offset := 0; offset < len(s.client.senders); offset++ {
		index := (s.next + offset) % len(s.client.senders)
		state := &s.client.states[index]
		if state.openedUntil.IsZero() || !now.Before(state.openedUntil) {
			if !state.openedUntil.IsZero() {
				state.openedUntil = time.Time{}
				state.history.reset()
			}
			s.next = (index + 1) % len(s.client.senders)
			return index, true
		}
	}
	return 0, false
}

func (c *Client) recordResult(index int, status resilience.Status) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := &c.states[index]
	state.history.add(status)
	if state.history.len() == c.config.WindowSize && state.history.failureRate() >= c.config.FailureThreshold {
		state.openedUntil = time.Now().Add(c.config.Cooldown)
	}
}

func validateConfig(config Config) {
	if config.WindowSize <= 0 {
		panic("window size must be positive")
	}
	if config.FailureThreshold <= 0 || config.FailureThreshold > 1 {
		panic("failure threshold must be in (0, 1]")
	}
	if config.Cooldown <= 0 {
		panic("cooldown must be positive")
	}
}
