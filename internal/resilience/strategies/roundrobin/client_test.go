package roundrobin

import (
	"context"
	"testing"
	"time"

	"ddia-hw2/internal/resilience"
	"ddia-hw2/internal/resilience/testutil"
)

// TestExecuteUsesFirstSender проверяет, что первая попытка отправляется
// первому sender-у из переданного списка.
func TestExecuteUsesFirstSender(t *testing.T) {
	first := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
	})
	second := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ServerFailure},
	})
	client := newClient([]resilience.SubrequestSender{first, second}, resilience.RetryBudget{})

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil || got.Status != resilience.Success {
		t.Fatalf("result = %+v, strategy error = %+v, want success", got, strategyErr)
	}
	assertCalls(t, first, 1)
	assertCalls(t, second, 0)
}

// TestExecuteMovesToNextSender проверяет, что после неуспешной первой попытки
// следующий retry отправляется второму sender-у.
func TestExecuteMovesToNextSender(t *testing.T) {
	first := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ServerFailure},
	})
	second := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
	})
	client := newClient([]resilience.SubrequestSender{first, second}, resilience.RetryBudget{ServerFailure: 1})

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil || got.Status != resilience.Success {
		t.Fatalf("result = %+v, strategy error = %+v, want success", got, strategyErr)
	}
	assertCalls(t, first, 1)
	assertCalls(t, second, 1)
}

// TestExecuteWrapsAroundSenderList проверяет переход с последнего sender-а
// обратно к первому при третьей попытке.
func TestExecuteWrapsAroundSenderList(t *testing.T) {
	first := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ServerFailure}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	second := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ServerFailure},
	})
	client := newClient([]resilience.SubrequestSender{first, second}, resilience.RetryBudget{ServerFailure: 2})

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil || got.Status != resilience.Success {
		t.Fatalf("result = %+v, strategy error = %+v, want success", got, strategyErr)
	}
	assertCalls(t, first, 2)
	assertCalls(t, second, 1)
}

// TestExecuteStopsWhenRetryBudgetIsExhausted проверяет, что стратегия не
// выбирает следующего sender-а, если retry-бюджет уже исчерпан.
func TestExecuteStopsWhenRetryBudgetIsExhausted(t *testing.T) {
	first := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ServerFailure},
	})
	second := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
	})
	client := newClient([]resilience.SubrequestSender{first, second}, resilience.RetryBudget{})

	_, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr == nil || strategyErr.Reason != resilience.RetryBudgetExhausted || strategyErr.Attempts != 1 {
		t.Fatalf("strategy error = %+v, want exhausted budget after one attempt", strategyErr)
	}
	assertCalls(t, first, 1)
	assertCalls(t, second, 0)
}

// TestExecuteRejectsEmptySenderList проверяет, что мультиклиент без sender-ов
// завершается понятной ошибкой без подзапросов.
func TestExecuteRejectsEmptySenderList(t *testing.T) {
	client := newClient(nil, resilience.RetryBudget{})

	_, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr == nil || strategyErr.Reason != resilience.NoAvailableSender || strategyErr.Attempts != 0 {
		t.Fatalf("strategy error = %+v, want no available sender", strategyErr)
	}
}

func newClient(senders []resilience.SubrequestSender, budget resilience.RetryBudget) *Client {
	return NewClient(senders, *resilience.NewLatencyBudget(time.Second, time.Second), budget)
}

func assertCalls(t *testing.T, sender *testutil.ScriptedSender, want int) {
	t.Helper()
	if got := len(sender.Calls()); got != want {
		t.Fatalf("calls = %d, want %d", got, want)
	}
}
