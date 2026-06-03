// Package powersupply reads a controller battery from the kernel power_supply
// device that hid-playstation (DS4) and hid-nintendo (Switch) expose under the
// controller's hid device. It handles numeric `capacity` (DS4) and coarse
// `capacity_level` (Switch).
package powersupply

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Read returns the battery under hidSysPath/power_supply/*. percent is 0-100
// (-1 for an Unknown coarse level). level is the coarse string ("Full", …) for
// capacity_level sources, else "". charging is true for status Charging/Full.
// ok is false when no power_supply device is present.
func Read(hidSysPath string) (percent int, level string, charging bool, ok bool) {
	matches, _ := filepath.Glob(filepath.Join(hidSysPath, "power_supply", "*"))
	if len(matches) == 0 {
		return 0, "", false, false
	}
	ps := matches[0]

	status := strings.TrimSpace(readFile(filepath.Join(ps, "status")))
	charging = status == "Charging" || status == "Full"

	if c := strings.TrimSpace(readFile(filepath.Join(ps, "capacity"))); c != "" {
		if n, err := strconv.Atoi(c); err == nil {
			return clamp(n), "", charging, true
		}
	}
	if lvl := strings.TrimSpace(readFile(filepath.Join(ps, "capacity_level"))); lvl != "" {
		return levelToPercent(lvl), lvl, charging, true
	}
	return 0, "", charging, true
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func clamp(n int) int {
	switch {
	case n < 0:
		return 0
	case n > 100:
		return 100
	default:
		return n
	}
}

func levelToPercent(level string) int {
	switch level {
	case "Full":
		return 100
	case "High":
		return 80
	case "Normal":
		return 55
	case "Low":
		return 25
	case "Critical":
		return 5
	default:
		return -1
	}
}
