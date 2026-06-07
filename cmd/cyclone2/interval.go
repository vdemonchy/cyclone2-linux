package main

import (
	"fmt"
	"time"
)

const (
	defaultInterval = 60 * time.Second
	minInterval     = 5 * time.Second
)

// resolveInterval applies precedence: flag > env > config-file > default, with
// a 5s floor. configSeconds <= 0 means "no configured value".
func resolveInterval(flagVal, envVal string, configSeconds int) (time.Duration, error) {
	raw := defaultInterval
	switch {
	case flagVal != "":
		d, err := time.ParseDuration(flagVal)
		if err != nil {
			return 0, fmt.Errorf("invalid --interval %q: %w", flagVal, err)
		}
		raw = d
	case envVal != "":
		d, err := time.ParseDuration(envVal)
		if err != nil {
			return 0, fmt.Errorf("invalid CYCLONE2_INTERVAL %q: %w", envVal, err)
		}
		raw = d
	case configSeconds > 0:
		raw = time.Duration(configSeconds) * time.Second
	}
	if raw < minInterval {
		return 0, fmt.Errorf("interval %v below %v floor", raw, minInterval)
	}
	return raw, nil
}
