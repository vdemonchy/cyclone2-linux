package device

import (
	"os"
	"path/filepath"
	"testing"
)

func writeUevent(t *testing.T, sysRoot, node, hidID string) {
	t.Helper()
	dir := filepath.Join(sysRoot, "class", "hidraw", node, "device")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "uevent"), []byte("HID_ID="+hidID+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindModes(t *testing.T) {
	cases := []struct {
		hidID    string
		wantMode string
		wantSrc  string
	}{
		{"0003:00003537:0000100B", "xinput", SourceHID},
		{"0003:0000054C:000009CC", "ds4", SourceHIDFeature},
		{"0003:0000057E:00002009", "switch", SourcePowerSupply},
		{"0003:00003537:00000575", "hid", SourceNone},
		{"0003:00003537:00009999", "unknown", SourceNone},
	}
	for _, c := range cases {
		sys := t.TempDir()
		writeUevent(t, sys, "hidraw9", c.hidID)
		m, ok := Find(sys, "/dev")
		if !ok {
			t.Fatalf("%s: not found", c.hidID)
		}
		if m.Mode.Name != c.wantMode || m.Mode.Source != c.wantSrc {
			t.Fatalf("%s: got %+v want %s/%s", c.hidID, m.Mode, c.wantMode, c.wantSrc)
		}
		if m.DevPath != "/dev/hidraw9" || m.SysPath != filepath.Join(sys, "class", "hidraw", "hidraw9", "device") {
			t.Fatalf("%s: bad paths %+v", c.hidID, m)
		}
	}
}

func TestFindSkipsNonController(t *testing.T) {
	sys := t.TempDir()
	writeUevent(t, sys, "hidraw0", "0003:0000046D:0000C548") // Logitech, not ours
	if _, ok := Find(sys, "/dev"); ok {
		t.Fatalf("should not match a non-controller device")
	}
}
