// Package config reads the runtime config the frontends (GNOME extension /
// COSMIC applet) write for the daemon: the poll interval and the low-battery
// notification threshold.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	IntervalSeconds int `json:"interval_seconds"`
	// LowBatteryThreshold is the percentage at or below which the daemon posts
	// a low-battery desktop notification. 0 (or unset) disables notifications.
	LowBatteryThreshold int `json:"low_battery_threshold"`
	// RGB holds the lighting settings the frontends write. Nil means "not
	// configured" — the daemon then leaves the controller's lighting untouched.
	RGB *RGB `json:"rgb,omitempty"`
}

// RGB is the controller lighting configuration. Zones are "RRGGBB" hex strings
// in the order [Left, Right, Logo, Center]; an empty/short list leaves the
// missing zones untouched. Brightness is 0-100; nil leaves it untouched.
type RGB struct {
	Brightness *int     `json:"brightness,omitempty"`
	Zones      []string `json:"zones,omitempty"`
}

// Path is $XDG_CONFIG_HOME/cyclone2-battery/config.json (falling back to
// ~/.config/cyclone2-battery/config.json).
func Path() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "cyclone2-battery", "config.json")
}

// Read returns the config. A missing file is not an error — it yields a zero
// Config, signalling "no configured value" to the interval resolver.
func Read() (Config, error) {
	b, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}
