package onetry

import (
	"context"
	"ddia-hw2/internal/resilience"
	"ddia-hw2/internal/resilience/testutil"
	"testing"
	"time"
)

// TestExecuteReturnsSuccessfulResponse проверяет, что успешная первая попытка
// возвращает payload без ошибки стратегии.
func TestExecuteReturnsSuccessfulResponse(t *testing.T) {
	sender := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{
			Status:  resilience.Success,
			Payload: []byte("response"),
		},
	})
	client := newClient(sender, time.Second, time.Second)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{Payload: []byte("request")})

	if strategyErr != nil {
		t.Fatalf("unexpected strategy error: %+v", strategyErr)
	}
	if got.Status != resilience.Success || string(got.Payload) != "response" {
		t.Fatalf("result = %+v, want successful response", got)
	}
}

// TestExecuteReturnsClientFailure проверяет, что быстрый ответ уровня 4xx
// возвращается как есть: default-стратегия не выполняет повтор.
func TestExecuteReturnsClientFailure(t *testing.T) {
	sender := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ClientFailure},
	})
	client := newClient(sender, time.Second, time.Second)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil {
		t.Fatalf("unexpected strategy error: %+v", strategyErr)
	}
	if got.Status != resilience.ClientFailure {
		t.Fatalf("status = %q, want %q", got.Status, resilience.ClientFailure)
	}
}

// TestExecuteReturnsServerFailure проверяет, что быстрый ответ уровня 5xx
// возвращается как есть: default-стратегия выполняет ровно одну попытку.
func TestExecuteReturnsServerFailure(t *testing.T) {
	sender := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ServerFailure},
	})
	client := newClient(sender, time.Second, time.Second)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil {
		t.Fatalf("unexpected strategy error: %+v", strategyErr)
	}
	if got.Status != resilience.ServerFailure {
		t.Fatalf("status = %q, want %q", got.Status, resilience.ServerFailure)
	}
}

// TestExecuteStopsAtSubrequestTimeout проверяет, что медленный подзапрос
// отменяется по истечении лимита времени одной попытки.
func TestExecuteStopsAtSubrequestTimeout(t *testing.T) {
	sender := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
		Delay:  200 * time.Millisecond,
	})
	client := newClient(sender, time.Second, 10*time.Millisecond)

	_, strategyErr := client.Execute(context.Background(), resilience.Request{})

	assertStrategyError(t, strategyErr, resilience.SubrequestTimeout)
	if calls := sender.Calls(); len(calls) != 1 || !calls[0].Cancelled {
		t.Fatalf("calls = %+v, want one cancelled subrequest", calls)
	}
}

// TestExecuteStopsAtRequestTimeout проверяет, что общий deadline запроса
// сильнее более длительного лимита времени одной попытки.
func TestExecuteStopsAtRequestTimeout(t *testing.T) {
	sender := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
		Delay:  200 * time.Millisecond,
	})
	client := newClient(sender, time.Second, 100*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, strategyErr := client.Execute(ctx, resilience.Request{})

	assertStrategyError(t, strategyErr, resilience.RequestTimeout)
}

func newClient(sender resilience.SubrequestSender, requestTimeout, subrequestTimeout time.Duration) *Client {
	return NewClient(sender, *resilience.NewLatencyBudget(requestTimeout, subrequestTimeout))
}

func assertStrategyError(t *testing.T, got *resilience.StrategyError, wantReason resilience.Reason) {
	t.Helper()
	if got == nil {
		t.Fatal("expected strategy error, got nil")
	}
	if got.Reason != wantReason || got.LastStatus != resilience.Timeout || got.Attempts != 1 {
		t.Fatalf("strategy error = %+v, want reason=%q lastStatus=%q attempts=1", got, wantReason, resilience.Timeout)
	}
}
