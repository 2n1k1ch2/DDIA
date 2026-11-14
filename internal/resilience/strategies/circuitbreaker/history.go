package circuitbreaker

import (
	"container/list"

	"ddia-hw2/internal/resilience"
)

type history struct {
	windowSize int
	values     *list.List
	counts     map[resilience.Status]int
}

func newHistory(windowSize int) *history {
	return &history{
		windowSize: windowSize,
		values:     list.New(),
		counts:     make(map[resilience.Status]int),
	}
}

func (h *history) add(status resilience.Status) {
	h.values.PushBack(status)
	h.counts[status]++

	if h.values.Len() > h.windowSize {
		oldest := h.values.Front()
		oldestStatus := oldest.Value.(resilience.Status)
		h.counts[oldestStatus]--
		h.values.Remove(oldest)
	}
}

func (h *history) len() int {
	return h.values.Len()
}

func (h *history) failureRate() float64 {
	if h.values.Len() == 0 {
		return 0
	}

	failures := h.counts[resilience.ClientFailure] +
		h.counts[resilience.ServerFailure] +
		h.counts[resilience.Timeout]
	return float64(failures) / float64(h.values.Len())
}

func (h *history) reset() {
	h.values.Init()
	clear(h.counts)
}
