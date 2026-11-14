package retry

import (
	"context"

	"ddia-hw2/internal/resilience"
)

type Client struct {
	sender  resilience.SubrequestSender
	latency resilience.LatencyBudget
	budget  resilience.RetryBudget
	waiter  Waiter
}

func NewClient(sender resilience.SubrequestSender, latency resilience.LatencyBudget, budget resilience.RetryBudget) *Client {
	return NewClientWithWaiter(sender, latency, budget, noWaiter{})
}

func NewClientWithWaiter(sender resilience.SubrequestSender, latency resilience.LatencyBudget, budget resilience.RetryBudget, waiter Waiter) *Client {
	if waiter == nil {
		waiter = noWaiter{}
	}

	return &Client{
		sender:  sender,
		latency: latency,
		budget:  budget,
		waiter:  waiter,
	}
}

func (c *Client) Execute(mainCtx context.Context, request resilience.Request) (resilience.SubrequestResult, *resilience.StrategyError) {
	localBudget := c.budget
	requestCtx, requestCancel := context.WithTimeout(mainCtx, c.latency.RequestTimeout)
	defer requestCancel()

	var attempts uint32
	lastStatus := resilience.Unknown
	for {
		if requestCtx.Err() != nil {
			return resilience.SubrequestResult{}, &resilience.StrategyError{
				Reason:     resilience.RequestTimeout,
				LastStatus: lastStatus,
				Attempts:   attempts,
			}
		}

		attemptCtx, attemptCancel := context.WithTimeout(requestCtx, c.latency.SubrequestTimeout)
		response := c.sender.Send(attemptCtx, request)
		requestTimedOut := requestCtx.Err() != nil
		attemptCancel()

		attempts++
		lastStatus = response.Status
		if requestTimedOut {
			return resilience.SubrequestResult{}, &resilience.StrategyError{
				Reason:     resilience.RequestTimeout,
				LastStatus: lastStatus,
				Attempts:   attempts,
			}
		}

		switch response.Status {
		case resilience.Success:
			return response, nil
		case resilience.ClientFailure:
			if localBudget.ClientFailure == 0 {
				return retryBudgetError(response.Status, attempts)
			}
			localBudget.ClientFailure--
		case resilience.ServerFailure:
			if localBudget.ServerFailure == 0 {
				return retryBudgetError(response.Status, attempts)
			}
			localBudget.ServerFailure--
		case resilience.Timeout:
			if localBudget.Timeouts == 0 {
				return retryBudgetError(response.Status, attempts)
			}
			localBudget.Timeouts--
		case resilience.Unknown:
			return response, nil
		}

		if err := c.waiter.Wait(requestCtx, response.Status, attempts); err != nil {
			return resilience.SubrequestResult{}, &resilience.StrategyError{
				Reason:     resilience.RequestTimeout,
				LastStatus: response.Status,
				Attempts:   attempts,
			}
		}
	}
}

func retryBudgetError(status resilience.Status, attempts uint32) (resilience.SubrequestResult, *resilience.StrategyError) {
	return resilience.SubrequestResult{}, &resilience.StrategyError{
		Reason:     resilience.RetryBudgetExhausted,
		LastStatus: status,
		Attempts:   attempts,
	}
}
