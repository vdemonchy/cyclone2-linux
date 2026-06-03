package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	in := State{Present: true, Percent: 55, Charging: true}
	in.Mode = "switch"
	in.BatteryKnown = true
	in.Level = "Full"
	if err := Write(p, in); err != nil {
		t.Fatal(err)
	}
	out, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if out.Present != in.Present || out.Percent != in.Percent || out.Charging != in.Charging {
		t.Fatalf("round-trip mismatch: %+v vs %+v", out, in)
	}
	if out.Mode != "switch" || !out.BatteryKnown || out.Level != "Full" {
		t.Fatalf("mode/battery_known/level round-trip failed: %+v", out)
	}
	if out.TS == 0 {
		t.Fatal("expected TS to be stamped")
	}
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file was left behind")
	}
}

func TestDefaultPathHonoursXDG(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if got := DefaultPath(); got != "/run/user/1000/cyclone2-battery.json" {
		t.Fatalf("got %q", got)
	}
}
