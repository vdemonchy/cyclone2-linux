package main

import (
	"testing"

	"github.com/vdemonchy/cyclone2-linux/internal/device"
)

// DS4 mode exposes no live battery (the dongle's feature 0x12 byte 10 is a frozen
// 0x64 and the standard battery field is dead), so the daemon must report the
// controller as present with an unknown battery rather than a fake percentage.
func TestReadDS4ReportsBatteryUnknown(t *testing.T) {
	s := readDS4(device.Match{Mode: device.Mode{Name: "ds4", Source: device.SourceHIDFeature}})
	if !s.Present {
		t.Errorf("Present = false, want true (controller is connected in DS4 mode)")
	}
	if s.BatteryKnown {
		t.Errorf("BatteryKnown = true, want false (DS4 has no live battery source)")
	}
	if s.Percent != 0 {
		t.Errorf("Percent = %d, want 0 (no battery value should be reported)", s.Percent)
	}
	if s.Mode != "ds4" {
		t.Errorf("Mode = %q, want \"ds4\"", s.Mode)
	}
}
