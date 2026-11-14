package resilience

import "time"

type LatencyBudget struct {
	RequestTimeout    time.Duration
	SubrequestTimeout time.Duration
}

func NewLatencyBudget(requestTimeout, subrequestTimeout time.Duration) *LatencyBudget {
	if subrequestTimeout > requestTimeout {
		panic("subrequestTimeout > requestTimeout")
	}
	return &LatencyBudget{
		RequestTimeout:    requestTimeout,
		SubrequestTimeout: subrequestTimeout,
	}
}

type RetryBudget struct {
	ClientFailure uint32
	ServerFailure uint32
	Timeouts      uint32
}

func NewRetryBudget(clientFailure, serverFailure, timeouts uint32) *RetryBudget {
	return &RetryBudget{
		ClientFailure: clientFailure,
		ServerFailure: serverFailure,
		Timeouts:      timeouts,
	}
}
