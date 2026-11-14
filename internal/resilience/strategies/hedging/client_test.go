package hedging

import (
	"context"
	"testing"
	"time"

	"ddia-hw2/internal/resilience"
	"ddia-hw2/internal/resilience/testutil"
)

// TestExecuteReturnsFirstSuccessfulResponse проверяет, что быстрый успешный
// ответ первого sender-а завершается без запуска hedge-запросов.
func TestExecuteReturnsFirstSuccessfulResponse(t *testing.T) {
	first := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success, Payload: []byte("first")},
	})
	second := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
	})
	client := newClient([]resilience.SubrequestSender{first, second}, time.Second, 50*time.Millisecond)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil || got.Status != resilience.Success || string(got.Payload) != "first" {
		t.Fatalf("result = %+v, strategy error = %+v, want first success", got, strategyErr)
	}
	assertCalls(t, first, 1)
	assertCalls(t, second, 0)
}

// TestExecuteHedgesSlowFirstSender проверяет, что после hedging delay запрос
// отправляется следующему sender-у, который может вернуть успех.
func TestExecuteHedgesSlowFirstSender(t *testing.T) {
	first := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
		Delay:  200 * time.Millisecond,
	})
	second := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success, Payload: []byte("hedged")},
	})
	client := newClient([]resilience.SubrequestSender{first, second}, time.Second, 10*time.Millisecond)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil || got.Status != resilience.Success || string(got.Payload) != "hedged" {
		t.Fatalf("result = %+v, strategy error = %+v, want hedged success", got, strategyErr)
	}
	assertCalls(t, first, 1)
	assertCalls(t, second, 1)
	if !first.Calls()[0].Cancelled {
		t.Fatal("first sender must be cancelled after hedged success")
	}
}

// TestExecuteCancelsAllLosingSubrequests проверяет, что успех одного hedge-
// запроса отменяет и первый, и остальные проигравшие подзапросы.
func TestExecuteCancelsAllLosingSubrequests(t *testing.T) {
	first := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
		Delay:  200 * time.Millisecond,
	})
	second := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
	})
	third := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
		Delay:  200 * time.Millisecond,
	})
	client := newClient([]resilience.SubrequestSender{first, second, third}, time.Second, 10*time.Millisecond)

	_, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil {
		t.Fatalf("unexpected strategy error: %+v", strategyErr)
	}
	assertCalls(t, first, 1)
	assertCalls(t, second, 1)
	assertCalls(t, third, 1)
	if !first.Calls()[0].Cancelled || !third.Calls()[0].Cancelled {
		t.Fatalf("losing calls = first:%+v third:%+v, want cancelled", first.Calls()[0], third.Calls()[0])
	}
}

// TestExecuteDoesNotHedgeAfterFastServerFailure проверяет буквальное правило:
// ранний 5xx является ответом, поэтому hedge-запросы не запускаются.
func TestExecuteDoesNotHedgeAfterFastServerFailure(t *testing.T) {
	first := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ServerFailure},
	})
	second := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
	})
	client := newClient([]resilience.SubrequestSender{first, second}, 15*time.Millisecond, 100*time.Millisecond)

	_, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr == nil || strategyErr.Reason != resilience.RequestTimeout || strategyErr.Attempts != 1 {
		t.Fatalf("strategy error = %+v, want request timeout after one attempt", strategyErr)
	}
	assertCalls(t, first, 1)
	assertCalls(t, second, 0)
}

// TestExecuteStopsAtRequestTimeout проверяет, что при отсутствии успешных
// ответов общий deadline отменяет все уже запущенные hedge-подзапросы.
func TestExecuteStopsAtRequestTimeout(t *testing.T) {
	first := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
		Delay:  200 * time.Millisecond,
	})
	second := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.Success},
		Delay:  200 * time.Millisecond,
	})
	client := newClient([]resilience.SubrequestSender{first, second}, 25*time.Millisecond, 5*time.Millisecond)

	_, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr == nil || strategyErr.Reason != resilience.RequestTimeout || strategyErr.Attempts != 2 {
		t.Fatalf("strategy error = %+v, want request timeout after two attempts", strategyErr)
	}
	assertCalls(t, first, 1)
	assertCalls(t, second, 1)
}

func newClient(senders []resilience.SubrequestSender, requestTimeout, hedgingDelay time.Duration) *Client {
	return NewClient(senders, *resilience.NewLatencyBudget(requestTimeout, requestTimeout), hedgingDelay)
}

func assertCalls(t *testing.T, sender *testutil.ScriptedSender, want int) {
	t.Helper()
	if got := len(sender.Calls()); got != want {
		t.Fatalf("calls = %d, want %d", got, want)
	}
}
