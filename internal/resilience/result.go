package resilience

type SubrequestResult struct {
	Status  Status `json:"status"`
	Payload []byte `json:"payload"`
}
