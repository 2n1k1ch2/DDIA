package testutil

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// TestFakeSleeperRecordsDelays проверяет, что fake sleeper не ждёт реальное
// время, а сохраняет значения задержек для проверки в тесте.
func TestFakeSleeperRecordsDelays(t *testing.T) {
	sleeper := &FakeSleeper{}

	if err := sleeper.Sleep(context.Background(), 10*time.Millisecond); err != nil {
		t.Fatalf("unexpected sleep error: %v", err)
	}
	if err := sleeper.Sleep(context.Background(), 20*time.Millisecond); err != nil {
		t.Fatalf("unexpected sleep error: %v", err)
	}

	want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	if got := sleeper.Delays(); !reflect.DeepEqual(got, want) {
		t.Fatalf("delays = %v, want %v", got, want)
	}
}
