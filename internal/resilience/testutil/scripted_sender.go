// Package testutil contains deterministic fakes for resilience tests.
package testutil

import (
	"context"
	"ddia-hw2/internal/resilience"
	"sync"
	"time"
)

type Step struct {
	Result resilience.SubrequestResult
	Delay  time.Duration
}

type Call struct {
	Request   resilience.Request
	StartedAt time.Time
	Cancelled bool
}

type ScriptedSender struct {
	mu    sync.Mutex
	steps []Step
	next  int
	calls []Call
}

func NewScriptedSender(steps ...Step) *ScriptedSender {
	return &ScriptedSender{steps: append([]Step(nil), steps...)}
}

func (s *ScriptedSender) Send(ctx context.Context, request resilience.Request) resilience.SubrequestResult {
	step, callIndex := s.takeStep(request)

	if step.Delay > 0 {
		timer := time.NewTimer(step.Delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			return step.Result
		case <-ctx.Done():
			s.markCancelled(callIndex)
			return timeoutResult()
		}
	}

	select {
	case <-ctx.Done():
		s.markCancelled(callIndex)
		return timeoutResult()
	default:
		return step.Result
	}
}

func (s *ScriptedSender) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Call, len(s.calls))
	copy(result, s.calls)
	for i := range result {
		result[i].Request.Payload = append([]byte(nil), result[i].Request.Payload...)
	}
	return result
}

func (s *ScriptedSender) takeStep(request resilience.Request) (Step, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	step := Step{Result: resilience.SubrequestResult{
		Status: resilience.Unknown,
	}}
	if s.next < len(s.steps) {
		step = s.steps[s.next]
		s.next++
	}

	callIndex := len(s.calls)
	s.calls = append(s.calls, Call{
		Request:   resilience.Request{Payload: append([]byte(nil), request.Payload...)},
		StartedAt: time.Now(),
	})
	return step, callIndex
}

func (s *ScriptedSender) markCancelled(callIndex int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls[callIndex].Cancelled = true
}

func timeoutResult() resilience.SubrequestResult {
	return resilience.SubrequestResult{
		Status: resilience.Timeout,
	}
}
