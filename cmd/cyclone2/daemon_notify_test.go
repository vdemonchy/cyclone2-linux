package main

import (
	"testing"

	"github.com/vdemonchy/cyclone2-linux/internal/state"
)

// recorder builds a notifier whose send() just counts calls.
func recorder() (*lowBatteryNotifier, *int) {
	calls := 0
	n := &lowBatteryNotifier{send: func(_, _, _ string) error {
		calls++
		return nil
	}}
	return n, &calls
}

func batt(percent int) state.State {
	return state.State{Present: true, BatteryKnown: true, Percent: percent}
}

func TestNotifyDisabledWhenThresholdZero(t *testing.T) {
	n, calls := recorder()
	n.consider(batt(1), 0)
	if *calls != 0 {
		t.Fatalf("threshold 0 should disable notifications, got %d", *calls)
	}
}

func TestNotifyFiresOnceOnCrossing(t *testing.T) {
	n, calls := recorder()
	n.consider(batt(50), 20) // above threshold: silent
	n.consider(batt(15), 20) // crosses below: notify
	n.consider(batt(12), 20) // still low: stay quiet
	n.consider(batt(18), 20) // below hysteresis (20+5): still quiet
	if *calls != 1 {
		t.Fatalf("expected exactly 1 notification, got %d", *calls)
	}
}

func TestNotifyReArmsAfterRecovery(t *testing.T) {
	n, calls := recorder()
	n.consider(batt(10), 20) // notify
	n.consider(batt(30), 20) // >= threshold+hysteresis: re-arm
	n.consider(batt(10), 20) // notify again
	if *calls != 2 {
		t.Fatalf("expected 2 notifications across two low episodes, got %d", *calls)
	}
}

func TestNotifyReArmsOnChargeAndDisconnect(t *testing.T) {
	for _, tc := range []struct {
		name  string
		recov state.State
	}{
		{"charging", state.State{Present: true, BatteryKnown: true, Percent: 10, Charging: true}},
		{"disconnected", state.State{Present: false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, calls := recorder()
			n.consider(batt(10), 20) // notify
			n.consider(tc.recov, 20) // re-arm (charging / gone)
			n.consider(batt(10), 20) // notify again
			if *calls != 2 {
				t.Fatalf("expected 2 notifications, got %d", *calls)
			}
		})
	}
}

func TestNotifyIgnoresStaleAndUnknown(t *testing.T) {
	n, calls := recorder()
	n.consider(state.State{Present: true, BatteryKnown: true, Stale: true, Percent: 1}, 20)
	n.consider(state.State{Present: true, BatteryKnown: false, Percent: 1}, 20)
	if *calls != 0 {
		t.Fatalf("stale/unknown readings must not notify, got %d", *calls)
	}
}
