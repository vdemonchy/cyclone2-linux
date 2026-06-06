// Package state persists the controller battery snapshot that the daemon writes
// and the GNOME extension reads.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	Present      bool   `json:"present"`
	Percent      int    `json:"percent"`
	Charging     bool   `json:"charging"`
	Stale        bool   `json:"stale"`
	Error        string `json:"error,omitempty"`
	TS           int64  `json:"ts"`
	Mode         string `json:"mode,omitempty"`
	BatteryKnown bool   `json:"battery_known"`
	Level        string `json:"level,omitempty"`
}

// DefaultPath is $XDG_RUNTIME_DIR/cyclone2-linux.json (falling back to the
// OS temp dir if XDG_RUNTIME_DIR is unset).
func DefaultPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "cyclone2-linux.json")
}

// Write atomically replaces the state file: it writes a temp file in the same
// directory then renames it over the target, so readers never see a partial file.
func Write(path string, s State) error {
	if s.TS == 0 {
		s.TS = time.Now().Unix()
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Read(path string) (State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, err
	}
	return s, nil
}
