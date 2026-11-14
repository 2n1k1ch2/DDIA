package hedging

import (
	"context"
	"sync"
	"time"

	"ddia-hw2/internal/resilience"
)

type Client struct {
	senders      []resilience.SubrequestSender
	latency      resilience.LatencyBudget
	hedgingDelay time.Duration
}

func NewClient(senders []resilience.SubrequestSender, latency resilience.LatencyBudget, delay time.Duration) *Client {
	if delay < 0 {
		panic("delay cannot be negative")
	}
	return &Client{
		senders:      append([]resilience.SubrequestSender(nil), senders...),
		latency:      latency,
		hedgingDelay: delay,
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
	requestCtx, cancel := context.WithTimeout(mainCtx, c.latency.RequestTimeout)
	defer cancel()

	resultChan := make(chan resilience.SubrequestResult, len(c.senders))
	var workers sync.WaitGroup
	launch := func(sender resilience.SubrequestSender) {
		workers.Add(1)
		go func(sender resilience.SubrequestSender) {
			defer workers.Done()
			resultChan <- sender.Send(requestCtx, request)
		}(sender)
	}

	launch(c.senders[0])
	attempts := uint32(1)
	lastStatus := resilience.Unknown
	timer := time.NewTimer(c.hedgingDelay)
	timerChan := timer.C
	defer timer.Stop()

	for {
		select {
		case response := <-resultChan:
			lastStatus = response.Status
			if response.Status == resilience.Success {
				cancel()
				workers.Wait()
				return response, nil
			}
			if response.Status == resilience.ServerFailure || response.Status == resilience.ClientFailure {
				timerChan = nil
			}
		case <-timerChan:
			timerChan = nil
			for _, sender := range c.senders[1:] {
				launch(sender)
				attempts++
			}
		case <-requestCtx.Done():
			cancel()
			workers.Wait()
			return resilience.SubrequestResult{}, &resilience.StrategyError{
				Reason:     resilience.RequestTimeout,
				LastStatus: lastStatus,
				Attempts:   attempts,
			}
		}
	}
}
