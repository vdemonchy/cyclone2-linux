package hidraw

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// TargetHIDID is the `HID_ID` value from the hidraw uevent for the GameSir
// Cyclone 2 over USB (bus 0003, vendor 3537, product 100B).
const TargetHIDID = "0003:00003537:0000100B"

// FindDevicePath scans <sysRoot>/class/hidraw/*/device/uevent for a node whose
// HID_ID matches TargetHIDID and returns <devRoot>/<node>. sysRoot/devRoot are
// parameters for testability (normally "/sys" and "/dev").
func FindDevicePath(sysRoot, devRoot string) (string, error) {
	base := filepath.Join(sysRoot, "class", "hidraw")
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		id, err := hidIDFromUevent(filepath.Join(base, e.Name(), "device", "uevent"))
		if err != nil {
			continue
		}
		if strings.EqualFold(id, TargetHIDID) {
			return filepath.Join(devRoot, e.Name()), nil
		}
	}
	return "", ErrDeviceNotFound
}

func hidIDFromUevent(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "HID_ID=") {
			return strings.TrimPrefix(line, "HID_ID="), nil
		}
	}
	return "", ErrDeviceNotFound
}
