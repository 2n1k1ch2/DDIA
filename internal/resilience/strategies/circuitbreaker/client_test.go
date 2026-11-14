package circuitbreaker

import (
	"context"
	"testing"
	"time"

	"ddia-hw2/internal/resilience"
	"ddia-hw2/internal/resilience/testutil"
)

// TestExecuteSkipsOpenedSender проверяет, что sender с открытым circuit
// исключается из следующего round-robin запроса.
func TestExecuteSkipsOpenedSender(t *testing.T) {
	broken := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ServerFailure}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	healthy := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	client := newClient(
		[]resilience.SubrequestSender{broken, healthy},
		resilience.RetryBudget{ServerFailure: 1},
		Config{WindowSize: 1, FailureThreshold: 1, Cooldown: time.Second},
	)

	first, firstErr := client.Execute(context.Background(), resilience.Request{})
	second, secondErr := client.Execute(context.Background(), resilience.Request{})

	if firstErr != nil || first.Status != resilience.Success {
		t.Fatalf("first result = %+v, error = %+v, want success", first, firstErr)
	}
	if secondErr != nil || second.Status != resilience.Success {
		t.Fatalf("second result = %+v, error = %+v, want success", second, secondErr)
	}
	assertCalls(t, broken, 1)
	assertCalls(t, healthy, 2)
}

// TestExecuteKeepsSenderBelowFailureThreshold проверяет, что sender не
// исключается, когда доля ошибок в заполненном окне ниже порога.
func TestExecuteKeepsSenderBelowFailureThreshold(t *testing.T) {
	sender := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ServerFailure}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	client := newClient(
		[]resilience.SubrequestSender{sender},
		resilience.RetryBudget{ServerFailure: 1},
		Config{WindowSize: 2, FailureThreshold: 0.75, Cooldown: time.Second},
	)

	_, firstErr := client.Execute(context.Background(), resilience.Request{})
	_, secondErr := client.Execute(context.Background(), resilience.Request{})

	if firstErr != nil || secondErr != nil {
		t.Fatalf("errors = first:%+v second:%+v, want nil", firstErr, secondErr)
	}
	if !client.states[0].openedUntil.IsZero() {
		t.Fatal("sender must remain in rotation below failure threshold")
	}
}

// TestExecuteReturnsNoAvailableSender проверяет, что стратегия сообщает
// понятную ошибку, когда все sender-ы уже открыли circuit.
func TestExecuteReturnsNoAvailableSender(t *testing.T) {
	first := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ServerFailure},
	})
	second := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ServerFailure},
	})
	client := newClient(
		[]resilience.SubrequestSender{first, second},
		resilience.RetryBudget{ServerFailure: 2},
		Config{WindowSize: 1, FailureThreshold: 1, Cooldown: time.Second},
	)

	_, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr == nil || strategyErr.Reason != resilience.NoAvailableSender || strategyErr.Attempts != 2 {
		t.Fatalf("strategy error = %+v, want no available sender after two attempts", strategyErr)
	}
}

// TestExecuteRestoresSenderAfterCooldown проверяет, что по завершении cooldown
// sender снова участвует в ротации с очищенной историей.
func TestExecuteRestoresSenderAfterCooldown(t *testing.T) {
	sender := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ServerFailure}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	client := newClient(
		[]resilience.SubrequestSender{sender},
		resilience.RetryBudget{ServerFailure: 1},
		Config{WindowSize: 1, FailureThreshold: 1, Cooldown: 10 * time.Millisecond},
	)

	_, firstErr := client.Execute(context.Background(), resilience.Request{})
	if firstErr == nil || firstErr.Reason != resilience.NoAvailableSender {
		t.Fatalf("first error = %+v, want no available sender", firstErr)
	}

	time.Sleep(20 * time.Millisecond)
	got, secondErr := client.Execute(context.Background(), resilience.Request{})

	if secondErr != nil || got.Status != resilience.Success {
		t.Fatalf("second result = %+v, error = %+v, want restored sender success", got, secondErr)
	}
	assertCalls(t, sender, 2)
}

// TestHistoryKeepsOnlyLatestOutcomes проверяет, что скользящее окно удаляет
// самый старый статус и корректно пересчитывает долю ошибок.
func TestHistoryKeepsOnlyLatestOutcomes(t *testing.T) {
	history := newHistory(2)
	history.add(resilience.ServerFailure)
	history.add(resilience.Success)
	history.add(resilience.Success)

	if history.len() != 2 {
		t.Fatalf("history length = %d, want 2", history.len())
	}
	if history.failureRate() != 0 {
		t.Fatalf("failure rate = %f, want 0", history.failureRate())
	}
}

func newClient(senders []resilience.SubrequestSender, budget resilience.RetryBudget, config Config) *Client {
	return NewClient(senders, *resilience.NewLatencyBudget(time.Second, time.Second), budget, config)
}

func assertCalls(t *testing.T, sender *testutil.ScriptedSender, want int) {
	t.Helper()
	if got := len(sender.Calls()); got != want {
		t.Fatalf("calls = %d, want %d", got, want)
	}
}
