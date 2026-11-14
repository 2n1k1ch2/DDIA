package resilience

import "context"

type SubrequestSender interface {
	Send(ctx context.Context, request Request) SubrequestResult
}
