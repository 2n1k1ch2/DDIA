package resilience

import "context"

type Client interface {
	Execute(ctx context.Context, request Request) (SubrequestResult, *StrategyError)
}
type MultiClient interface {
	Execute(ctx context.Context, request Request) (SubrequestResult, *StrategyError)
}
