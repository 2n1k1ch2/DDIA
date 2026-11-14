package resilience

type Status string

const (
	Success       Status = "success"
	ClientFailure Status = "client_failure"
	ServerFailure Status = "server_failure"
	Timeout       Status = "timeout"
	Unknown       Status = "unknown"
)
