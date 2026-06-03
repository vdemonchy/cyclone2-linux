package hidraw

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
	body := "DRIVER=usbhid\nHID_ID=" + hidID + "\nHID_NAME=test\n"
	if err := os.WriteFile(filepath.Join(dir, "uevent"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindDevicePath(t *testing.T) {
	sys := t.TempDir()
	writeUevent(t, sys, "hidraw0", "0003:0000046D:0000C548") // some other device
	writeUevent(t, sys, "hidraw9", TargetHIDID)              // the Cyclone 2

	got, err := FindDevicePath(sys, "/dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/dev/hidraw9" {
		t.Fatalf("got %q, want /dev/hidraw9", got)
	}
}

func TestFindDevicePathNotFound(t *testing.T) {
	sys := t.TempDir()
	writeUevent(t, sys, "hidraw0", "0003:0000046D:0000C548")
	if _, err := FindDevicePath(sys, "/dev"); err != ErrDeviceNotFound {
		t.Fatalf("got %v, want ErrDeviceNotFound", err)
	}
}

