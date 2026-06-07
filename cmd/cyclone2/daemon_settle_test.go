package main

import (
	"testing"
	"time"
)

// scheduleSettle should arm one timer per configured delay, preserving order.
func TestScheduleSettleArmsOnePerDelay(t *testing.T) {
	var got []time.Duration
	ch := make(chan struct{}, 1)
	scheduleSettle(func(d time.Duration, _ func()) { got = append(got, d) }, ch)

	if len(got) != len(settleDelays) {
		t.Fatalf("armed %d timers, want %d", len(got), len(settleDelays))
	}
	for i, d := range settleDelays {
		if got[i] != d {
			t.Fatalf("delay[%d]=%v, want %v", i, got[i], d)
		}
	}
}

// When several settle timers fire near-simultaneously, the non-blocking send
// into a size-1 channel must coalesce them to a single queued poll trigger.
func TestScheduleSettleSendsCoalesce(t *testing.T) {
	ch := make(chan struct{}, 1)
	// after invokes fn immediately, simulating all timers firing at once.
	scheduleSettle(func(_ time.Duration, fn func()) { fn() }, ch)

	if len(ch) != 1 {
		t.Fatalf("coalesced sends = %d, want 1 (channel buffer size)", len(ch))
	}
}
