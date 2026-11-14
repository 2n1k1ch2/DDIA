package backoff

import (
	"context"
	"ddia-hw2/internal/resilience"
	"ddia-hw2/internal/resilience/testutil"
	"reflect"
	"testing"
	"time"
)

// TestExecuteWaitsAfterServerFailure проверяет, что после 5xx перед повтором
// запрашивается начальная backoff-задержка.
func TestExecuteWaitsAfterServerFailure(t *testing.T) {
	sender := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ServerFailure}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	sleeper := &testutil.FakeSleeper{}
	client := newClient(sender, resilience.RetryBudget{ServerFailure: 1}, 10*time.Millisecond, sleeper)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil || got.Status != resilience.Success {
		t.Fatalf("result = %+v, strategy error = %+v, want success", got, strategyErr)
	}
	assertDelays(t, sleeper, []time.Duration{10 * time.Millisecond})
}

// TestExecuteWaitsAfterTimeout проверяет, что после timeout перед повтором
// также применяется начальная backoff-задержка.
func TestExecuteWaitsAfterTimeout(t *testing.T) {
	sender := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Timeout}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	sleeper := &testutil.FakeSleeper{}
	client := newClient(sender, resilience.RetryBudget{Timeouts: 1}, 10*time.Millisecond, sleeper)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil || got.Status != resilience.Success {
		t.Fatalf("result = %+v, strategy error = %+v, want success", got, strategyErr)
	}
	assertDelays(t, sleeper, []time.Duration{10 * time.Millisecond})
}

// TestExecuteDoesNotWaitAfterClientFailure проверяет, что 4xx допускает retry
// без backoff-задержки.
func TestExecuteDoesNotWaitAfterClientFailure(t *testing.T) {
	sender := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ClientFailure}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	sleeper := &testutil.FakeSleeper{}
	client := newClient(sender, resilience.RetryBudget{ClientFailure: 1}, 10*time.Millisecond, sleeper)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil || got.Status != resilience.Success {
		t.Fatalf("result = %+v, strategy error = %+v, want success", got, strategyErr)
	}
	assertDelays(t, sleeper, nil)
}

// TestExecuteDoublesDelay проверяет рост задержек по степеням двойки после
// последовательных 5xx.
func TestExecuteDoublesDelay(t *testing.T) {
	sender := testutil.NewScriptedSender(
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ServerFailure}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.ServerFailure}},
		testutil.Step{Result: resilience.SubrequestResult{Status: resilience.Success}},
	)
	sleeper := &testutil.FakeSleeper{}
	client := newClient(sender, resilience.RetryBudget{ServerFailure: 2}, 10*time.Millisecond, sleeper)

	got, strategyErr := client.Execute(context.Background(), resilience.Request{})

	if strategyErr != nil || got.Status != resilience.Success {
		t.Fatalf("result = %+v, strategy error = %+v, want success", got, strategyErr)
	}
	assertDelays(t, sleeper, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond})
}

// TestExecuteStopsWhenDelayDoesNotFit проверяет, что стратегия не ждёт и не
// запускает новую попытку, если backoff не помещается в общий deadline.
func TestExecuteStopsWhenDelayDoesNotFit(t *testing.T) {
	sender := testutil.NewScriptedSender(testutil.Step{
		Result: resilience.SubrequestResult{Status: resilience.ServerFailure},
	})
	sleeper := &testutil.FakeSleeper{}
	client := newClient(sender, resilience.RetryBudget{ServerFailure: 1}, time.Second, sleeper)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, strategyErr := client.Execute(ctx, resilience.Request{})

	if strategyErr == nil || strategyErr.Reason != resilience.RequestTimeout || strategyErr.Attempts != 1 {
		t.Fatalf("strategy error = %+v, want request timeout after one attempt", strategyErr)
	}
	assertDelays(t, sleeper, nil)
	if calls := sender.Calls(); len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
}

func newClient(sender resilience.SubrequestSender, budget resilience.RetryBudget, initialDelay time.Duration, sleeper resilience.Sleeper) *Client {
	return NewClient(sender, *resilience.NewLatencyBudget(time.Second, time.Second), budget, initialDelay, sleeper)
}

func assertDelays(t *testing.T, sleeper *testutil.FakeSleeper, want []time.Duration) {
	t.Helper()
	if got := sleeper.Delays(); len(got) != len(want) || (len(got) > 0 && !reflect.DeepEqual(got, want)) {
		t.Fatalf("delays = %v, want %v", got, want)
	}
}
