package testutil

import (
	"context"
	"ddia-hw2/internal/resilience"
	"testing"
	"time"
)

func TestScriptedSenderReturnsStepsInOrder(t *testing.T) {
	sender := NewScriptedSender(
		Step{Result: result(resilience.ServerFailure, nil)},
		Step{Result: result(resilience.Success, []byte("ok"))},
	)

	first := sender.Send(context.Background(), resilience.Request{Payload: []byte("request")})
	second := sender.Send(context.Background(), resilience.Request{})

	if first.Status != resilience.ServerFailure {
		t.Fatalf("first outcome = %q, want %q", first.Status, resilience.ServerFailure)
	}
	if second.Status != resilience.Success || string(second.Payload) != "ok" {
		t.Fatalf("second result = %+v, want successful payload", second)
	}
	if calls := sender.Calls(); len(calls) != 2 || string(calls[0].Request.Payload) != "request" {
		t.Fatalf("calls = %+v, want two recorded calls", calls)
	}
}

func TestScriptedSenderReturnsTimeoutOnContextCancellation(t *testing.T) {
	sender := NewScriptedSender(Step{
		Result: result(resilience.Success, []byte("too late")),
		Delay:  time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := sender.Send(ctx, resilience.Request{})
	if got.Status != resilience.Timeout {
		t.Fatalf("outcome = %q, want %q", got.Status, resilience.Timeout)
	}
	if calls := sender.Calls(); len(calls) != 1 || !calls[0].Cancelled {
		t.Fatalf("calls = %+v, want one cancelled call", calls)
	}
}

func result(outcome resilience.Status, payload []byte) resilience.SubrequestResult {
	return resilience.SubrequestResult{
		Status:  outcome,
		Payload: payload,
	}
}
