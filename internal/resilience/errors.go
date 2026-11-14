package resilience

import "fmt"

type Reason string

const (
	RequestTimeout       Reason = "Request deadline exceeded"
	SubrequestTimeout    Reason = "Subrequest timeout"
	RetryBudgetExhausted Reason = "Retry budget exhausted"
	NoAvailableSender    Reason = "No available sender"
)

type StrategyError struct {
	Reason     Reason
	LastStatus Status
	Attempts   uint32
}

func (e *StrategyError) Error() string {
	return fmt.Sprintf("strategy stopped: reason=%s last_status=%s attempts=%d", e.Reason, e.LastStatus, e.Attempts)
}
