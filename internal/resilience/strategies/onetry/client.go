package onetry

import (
	"context"
	"ddia-hw2/internal/resilience"
)

type Client struct {
	sender  resilience.SubrequestSender
	latency resilience.LatencyBudget
}

func NewClient(sender resilience.SubrequestSender, latency resilience.LatencyBudget) *Client {
	return &Client{
		sender:  sender,
		latency: latency,
	}
}

func (c *Client) Execute(mainCtx context.Context, request resilience.Request) (resilience.SubrequestResult, *resilience.StrategyError) {
	requestContext, RequestCancelFunc := context.WithTimeout(mainCtx, c.latency.RequestTimeout)
	defer RequestCancelFunc()
	attemptContext, AttemptCancelFunc := context.WithTimeout(requestContext, c.latency.SubrequestTimeout)
	defer AttemptCancelFunc()
	response := c.sender.Send(attemptContext, request)
	if response.Status == resilience.Success {
		return response, nil
	}
	if requestContext.Err() != nil {
		return resilience.SubrequestResult{}, &resilience.StrategyError{
			Reason:     resilience.RequestTimeout,
			LastStatus: response.Status,
			Attempts:   1,
		}
	} else if attemptContext.Err() != nil {
		return resilience.SubrequestResult{}, &resilience.StrategyError{
			Reason:     resilience.SubrequestTimeout,
			LastStatus: response.Status,
			Attempts:   1,
		}
	}
	return response, nil
}
