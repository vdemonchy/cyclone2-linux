// Package device identifies which mode the GameSir Cyclone 2 is in (by USB
// vendor:product) and where to read its battery.
package device

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Battery source kinds.
const (
	SourceHID         = "hid"
	SourceHIDFeature  = "hid_feature"
	SourcePowerSupply = "power_supply"
	SourceNone        = "none"
)

type Mode struct {
	Name   string // "xinput", "ds4", "switch", "hid", "unknown"
	Source string // SourceHID | SourcePowerSupply | SourceNone
}

type Match struct {
	DevPath string // /dev/hidrawN
	SysPath string // <sysRoot>/class/hidraw/hidrawN/device (holds power_supply/*)
	Mode    Mode
}

// modeFor maps a USB vendor:product to a mode. An empty Name means "not one of
// our controllers" (skip it).
func modeFor(vendor, product uint16) Mode {
	switch {
	case vendor == 0x3537 && product == 0x100b:
		return Mode{"xinput", SourceHID}
	case vendor == 0x054c && product == 0x09cc:
		return Mode{"ds4", SourceHIDFeature}
	case vendor == 0x057e && product == 0x2009:
		return Mode{"switch", SourcePowerSupply}
	case vendor == 0x3537 && product == 0x0575:
		// HID mode — also what the 2.4GHz dongle reports on its own when the
		// controller is powered off. No readable battery either way.
		return Mode{"hid", SourceNone}
	case vendor == 0x3537:
		return Mode{"unknown", SourceNone}
	default:
		return Mode{}
	}
}

// Find returns the first connected controller we recognize, or ok=false.
func Find(sysRoot, devRoot string) (Match, bool) {
	base := filepath.Join(sysRoot, "class", "hidraw")
	entries, err := os.ReadDir(base)
	if err != nil {
		return Match{}, false
	}
	for _, e := range entries {
		devDir := filepath.Join(base, e.Name(), "device")
		vendor, product, ok := vidPidFromUevent(filepath.Join(devDir, "uevent"))
		if !ok {
			continue
		}
		mode := modeFor(vendor, product)
		if mode.Name == "" {
			continue
		}
		return Match{
			DevPath: filepath.Join(devRoot, e.Name()),
			SysPath: devDir,
			Mode:    mode,
		}, true
	}
	return Match{}, false
}

func vidPidFromUevent(path string) (uint16, uint16, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		v, ok := strings.CutPrefix(sc.Text(), "HID_ID=")
		if !ok {
			continue
		}
		parts := strings.Split(v, ":")
		if len(parts) != 3 {
			return 0, 0, false
		}
		vendor, err1 := strconv.ParseUint(parts[1], 16, 32)
		product, err2 := strconv.ParseUint(parts[2], 16, 32)
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return uint16(vendor), uint16(product), true
	}
	return 0, 0, false
}
