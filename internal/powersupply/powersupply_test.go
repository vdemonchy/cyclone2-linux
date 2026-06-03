package powersupply

import (
	"os"
	"path/filepath"
	"testing"
)

func mkPS(t *testing.T, hid, name string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(hid, "power_supply", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for f, v := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(v+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadDS4Numeric(t *testing.T) {
	hid := t.TempDir()
	mkPS(t, hid, "ps-controller-battery-x", map[string]string{"capacity": "5", "status": "Discharging"})
	pct, level, charging, ok := Read(hid)
	if !ok || pct != 5 || level != "" || charging {
		t.Fatalf("got pct=%d level=%q charging=%v ok=%v", pct, level, charging, ok)
	}
}

func TestReadSwitchCoarse(t *testing.T) {
	hid := t.TempDir()
	mkPS(t, hid, "nintendo_switch_controller_battery_x", map[string]string{"capacity_level": "Full", "status": "Discharging"})
	pct, level, charging, ok := Read(hid)
	if !ok || pct != 100 || level != "Full" || charging {
		t.Fatalf("got pct=%d level=%q charging=%v ok=%v", pct, level, charging, ok)
	}
}

func TestReadCharging(t *testing.T) {
	hid := t.TempDir()
	mkPS(t, hid, "ps-x", map[string]string{"capacity": "50", "status": "Charging"})
	_, _, charging, ok := Read(hid)
	if !ok || !charging {
		t.Fatalf("charging=%v ok=%v", charging, ok)
	}
}

func TestReadMissing(t *testing.T) {
	hid := t.TempDir()
	if _, _, _, ok := Read(hid); ok {
		t.Fatalf("expected ok=false when no power_supply")
	}
}
