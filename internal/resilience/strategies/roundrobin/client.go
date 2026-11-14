package roundrobin

import (
	"context"

	"ddia-hw2/internal/resilience"
	"ddia-hw2/internal/resilience/strategies/retry"
)

type Client struct {
	senders []resilience.SubrequestSender
	latency resilience.LatencyBudget
	budget  resilience.RetryBudget
}

func NewClient(senders []resilience.SubrequestSender, latency resilience.LatencyBudget, budget resilience.RetryBudget) *Client {
	return &Client{
		senders: append([]resilience.SubrequestSender(nil), senders...),
		latency: latency,
		budget:  budget,
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

	roundRobin := &roundRobinSender{senders: c.senders}
	retryClient := retry.NewClient(roundRobin, c.latency, c.budget)
	return retryClient.Execute(mainCtx, request)
}

type roundRobinSender struct {
	senders []resilience.SubrequestSender
	next    int
}

func (s *roundRobinSender) Send(ctx context.Context, request resilience.Request) resilience.SubrequestResult {
	sender := s.senders[s.next]
	s.next = (s.next + 1) % len(s.senders)
	return sender.Send(ctx, request)
}
