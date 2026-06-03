// Package config reads the runtime config the GNOME extension writes for the
// daemon (currently just the poll interval).
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	IntervalSeconds int `json:"interval_seconds"`
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
