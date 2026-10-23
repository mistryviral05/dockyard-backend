package agentd

import (
	"math"
	"sync"
	"time"
)

const (
	loadWindow1  = time.Minute
	loadWindow5  = 5 * time.Minute
	loadWindow15 = 15 * time.Minute
)

type loadTracker struct {
	mu     sync.Mutex
	primed bool
	last   time.Time

	load1  float64
	load5  float64
	load15 float64
}

func newLoadTracker() *loadTracker {
	return &loadTracker{}
}

func (t *loadTracker) update(instant *float64, now time.Time) (*float64, *float64, *float64) {
	if instant == nil {
		return nil, nil, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	value := math.Max(*instant, 0)

	switch {
	case !t.primed:
		t.primed = true
		t.load1, t.load5, t.load15 = value, value, value

	default:
		elapsed := now.Sub(t.last)
		if elapsed <= 0 {
			break
		}
		t.load1 = decay(t.load1, value, elapsed, loadWindow1)
		t.load5 = decay(t.load5, value, elapsed, loadWindow5)
		t.load15 = decay(t.load15, value, elapsed, loadWindow15)
	}

	t.last = now

	return roundLoad(t.load1), roundLoad(t.load5), roundLoad(t.load15)
}

func decay(previous, value float64, elapsed, window time.Duration) float64 {
	weight := math.Exp(-elapsed.Seconds() / window.Seconds())
	return previous*weight + value*(1-weight)
}
