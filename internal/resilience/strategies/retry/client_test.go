package retry

import (
	"context"
	"ddia-hw2/internal/resilience"
	"ddia-hw2/internal/resilience/testutil"
	"testing"
	"time"
)

// TestExecuteRetriesAfterServerFailure проверяет, что после 5xx выполняется
// разрешённая дополнительная попытка, которая может завершиться успехом.
func TestExecuteRetriesAfterServerFailure(t *testing.T) {
	sender := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ServerFailure}},
		testutil.Step{Result: resilience.SubrequestResult{
			Status:  resilience.Success,
			Payload: []byte("response"),
		}},
	)
	client := newClient(sender, resilience.RetryBudget{ServerFailure: 1}, time.Second, time.Second)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil {
		t.Fatalf("unexpected strategy error: %+v", strategyErr)
	}
	if got.Status != resilience.Success || string(got.Payload) != "response" {
		t.Fatalf("result = %+v, want successful response", got)
	}
	if calls := sender.Calls(); len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
}

// TestExecuteRetriesAfterClientFailure проверяет, что после 4xx выполняется
// разрешённая дополнительная попытка.
func TestExecuteRetriesAfterClientFailure(t *testing.T) {
	sender := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ClientFailure}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	client := newClient(sender, resilience.RetryBudget{ClientFailure: 1}, time.Second, time.Second)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil {
		t.Fatalf("unexpected strategy error: %+v", strategyErr)
	}
	if got.Status != resilience.Success {
		t.Fatalf("status = %q, want %q", got.Status, resilience.Success)
	}
}

// TestExecuteRetriesAfterTimeout проверяет, что timeout одной попытки расходует
// timeout-бюджет и допускает следующую попытку.
func TestExecuteRetriesAfterTimeout(t *testing.T) {
	sender := testutil.NewScriptedSender(
		testutil.Step{
			Result: resilience.SubrequestResult{Status: resilience.Success},
			Delay:  200 * time.Millisecond,
		},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	client := newClient(sender, resilience.RetryBudget{Timeouts: 1}, time.Second, 10*time.Millisecond)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil {
		t.Fatalf("unexpected strategy error: %+v", strategyErr)
	}
	if got.Status != resilience.Success {
		t.Fatalf("status = %q, want %q", got.Status, resilience.Success)
	}
	if calls := sender.Calls(); len(calls) != 2 || !calls[0].Cancelled {
		t.Fatalf("calls = %+v, want a cancelled first attempt and a retry", calls)
	}
}

// TestExecuteStopsWhenServerFailureBudgetIsExhausted проверяет, что 5xx без
// разрешённого повтора завершает запрос ошибкой исчерпания retry-бюджета.
func TestExecuteStopsWhenServerFailureBudgetIsExhausted(t *testing.T) {
	sender := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ServerFailure},
	})
	client := newClient(sender, resilience.RetryBudget{}, time.Second, time.Second)

	_, strategyErr := client.Execute(context.Background(), resilience.Request{})

	assertStrategyError(t, strategyErr, resilience.RetryBudgetExhausted, resilience.ServerFailure, 1)
	if calls := sender.Calls(); len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
}

// TestExecuteStopsAtRequestTimeout проверяет, что общий deadline запроса
// прекращает ожидание раньше более длительного лимита одной попытки.
func TestExecuteStopsAtRequestTimeout(t *testing.T) {
	sender := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
		Delay:  200 * time.Millisecond,
	})
	client := newClient(sender, resilience.RetryBudget{Timeouts: 1}, time.Second, 100*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, strategyErr := client.Execute(ctx, resilience.Request{})

	assertStrategyError(t, strategyErr, resilience.RequestTimeout, resilience.Timeout, 1)
}

// TestExecuteCallsWaiterBeforeRetry проверяет, что retry-цикл вызывает hook
// между неуспешной попыткой и разрешённым повтором.
func TestExecuteCallsWaiterBeforeRetry(t *testing.T) {
	sender := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ServerFailure}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	waiter := &recordingWaiter{}
	client := NewClientWithWaiter(
		sender,
		*resilience.NewLatencyBudget(time.Second, time.Second),
		resilience.RetryBudget{ServerFailure: 1},
		waiter,
	)

	_, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil {
		t.Fatalf("unexpected strategy error: %+v", strategyErr)
	}
	if len(waiter.statuses) != 1 || waiter.statuses[0] != resilience.ServerFailure {
		t.Fatalf("waiter statuses = %v, want [%q]", waiter.statuses, resilience.ServerFailure)
	}
}

func newClient(sender resilience.SubrequestSender, budget resilience.RetryBudget, requestTimeout, subrequestTimeout time.Duration) *Client {
	return NewClient(sender, *resilience.NewLatencyBudget(requestTimeout, subrequestTimeout), budget)
}

func assertStrategyError(t *testing.T, got *resilience.StrategyError, wantReason resilience.Reason, wantStatus resilience.Status, wantAttempts uint32) {
	t.Helper()
	if got == nil {
		t.Fatal("expected strategy error, got nil")
	}
	if got.Reason != wantReason || got.LastStatus != wantStatus || got.Attempts != wantAttempts {
		t.Fatalf("strategy error = %+v, want reason=%q lastStatus=%q attempts=%d", got, wantReason, wantStatus, wantAttempts)
	}
}

type recordingWaiter struct {
	statuses []resilience.Status
}

func (w *recordingWaiter) Wait(_ context.Context, status resilience.Status, _ uint32) error {
	w.statuses = append(w.statuses, status)
	return nil
}
