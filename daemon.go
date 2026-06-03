package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/victordemonchy/cyclone2-battery/internal/config"
	"github.com/victordemonchy/cyclone2-battery/internal/device"
	"github.com/victordemonchy/cyclone2-battery/internal/hidraw"
	"github.com/victordemonchy/cyclone2-battery/internal/powersupply"
	"github.com/victordemonchy/cyclone2-battery/internal/reader"
	"github.com/victordemonchy/cyclone2-battery/internal/state"
	"github.com/victordemonchy/cyclone2-battery/internal/uevent"

	"golang.org/x/sys/unix"
)

func isOurController(ev uevent.Event) bool {
	return ev.Matches("3537") || ev.Matches("54c/9cc") || ev.Matches("57e/2009")
}

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	intervalFlag := fs.String("interval", "", "poll interval (Go duration, e.g. 30s); overrides config")
	statePath := fs.String("state", state.DefaultPath(), "state file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	curInterval, err := currentInterval(*intervalFlag)
	if err != nil {
		return err
	}
	log.Printf("cyclone2 daemon: interval=%v state=%s", curInterval, *statePath)

	events := make(chan uevent.Event, 8)
	if mon, err := uevent.Open(); err != nil {
		log.Printf("hotplug monitor unavailable (%v); disconnect detected only on poll", err)
	} else {
		defer mon.Close()
		go func() {
			for {
				ev, err := mon.Read()
				if err != nil {
					return
				}
				if isOurController(ev) && (ev.Action == "add" || ev.Action == "remove") {
					events <- ev
				}
			}
		}()
	}

	configCh := make(chan struct{}, 1)
	if cw, err := watchConfig(configCh); err != nil {
		log.Printf("config watch unavailable (%v); interval changes apply on next tick", err)
	} else {
		defer cw.Close()
	}

	var last state.State
	pollOnce(*statePath, &last)

	ticker := time.NewTicker(curInterval)
	defer ticker.Stop()
	reapply := func() {
		if ni, err := currentInterval(*intervalFlag); err != nil {
			log.Printf("ignoring invalid interval change: %v", err)
		} else if ni != curInterval {
			curInterval = ni
			ticker.Reset(curInterval)
			log.Printf("poll interval changed to %v", curInterval)
		}
	}
	for {
		select {
		case <-ticker.C:
			pollOnce(*statePath, &last)
			reapply() // fallback pickup if inotify is unavailable
		case ev := <-events:
			if ev.Action == "remove" {
				last = state.State{Present: false}
				writeState(*statePath, last)
				log.Printf("controller disconnected")
			} else {
				pollOnce(*statePath, &last)
			}
		case <-configCh:
			reapply()
		}
	}
}

// currentInterval resolves flag > env > config-file > default.
func currentInterval(flagVal string) (time.Duration, error) {
	cfg, _ := config.Read()
	return resolveInterval(flagVal, os.Getenv("CYCLONE2_INTERVAL"), cfg.IntervalSeconds)
}

// configWatcher watches the config directory for changes via inotify.
type configWatcher struct{ fd int }

func (w *configWatcher) Close() error { return unix.Close(w.fd) }

func watchConfig(ch chan<- struct{}) (*configWatcher, error) {
	dir := filepath.Dir(config.Path())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	if _, err := unix.InotifyAddWatch(fd, dir, unix.IN_CLOSE_WRITE|unix.IN_MOVED_TO|unix.IN_CREATE); err != nil {
		unix.Close(fd)
		return nil, err
	}
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := unix.Read(fd, buf); err != nil {
				return
			}
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}()
	return &configWatcher{fd: fd}, nil
}

// pollOnce reads battery and writes the state file.
func pollOnce(statePath string, last *state.State) {
	m, ok := device.Find("/sys", "/dev")
	if !ok {
		*last = state.State{Present: false}
		writeState(statePath, *last)
		return
	}
	switch m.Mode.Source {
	case device.SourceHID:
		*last = readHID(m, last)
	case device.SourceHIDFeature:
		*last = readDS4(m, last)
	case device.SourcePowerSupply:
		pct, level, charging, psok := powersupply.Read(m.SysPath)
		*last = state.State{Present: true, Mode: m.Mode.Name, BatteryKnown: psok, Percent: pct, Level: level, Charging: charging}
	default: // SourceNone — HID mode / dongle idle / controller off: no battery, treat as not connected (hide indicator)
		*last = state.State{Present: false}
	}
	writeState(statePath, *last)
}

func readHID(m device.Match, last *state.State) state.State {
	dev, err := openHID(m.DevPath)
	if err != nil {
		log.Printf("cannot open %s: %v (is the udev rule installed?)", m.DevPath, err)
		return state.State{Present: true, Mode: m.Mode.Name, BatteryKnown: true, Stale: true, Percent: last.Percent, Charging: last.Charging}
	}
	defer dev.Close()
	st, err := reader.Read(dev)
	if err != nil {
		return state.State{Present: true, Mode: m.Mode.Name, BatteryKnown: true, Stale: true, Percent: last.Percent, Charging: last.Charging}
	}
	return state.State{Present: true, Mode: m.Mode.Name, BatteryKnown: true, Percent: st.Percent, Charging: st.Charging}
}

// readDS4 reads DS4-mode battery via the vendor feature report; charging comes
// from the kernel power_supply status (the cable-state signal is reliable even
// though its capacity is not).
func readDS4(m device.Match, last *state.State) state.State {
	dev, err := openHID(m.DevPath)
	if err != nil {
		log.Printf("cannot open %s: %v (is the udev rule installed?)", m.DevPath, err)
		return state.State{Present: true, Mode: m.Mode.Name, BatteryKnown: true, Stale: true, Percent: last.Percent, Charging: last.Charging}
	}
	defer dev.Close()
	st, err := reader.ReadDS4(dev)
	if err != nil {
		return state.State{Present: true, Mode: m.Mode.Name, BatteryKnown: true, Stale: true, Percent: last.Percent, Charging: last.Charging}
	}
	_, _, charging, _ := powersupply.Read(m.SysPath)
	return state.State{Present: true, Mode: m.Mode.Name, BatteryKnown: true, Percent: st.Percent, Charging: charging}
}

// openHID opens a hidraw node, retrying briefly on permission errors to ride out
// the udev uaccess race right after a hotplug "add".
func openHID(path string) (*hidraw.Device, error) {
	var err error
	for i := 0; i < 15; i++ {
		var dev *hidraw.Device
		dev, err = hidraw.Open(path)
		if err == nil {
			return dev, nil
		}
		if !os.IsPermission(err) {
			return nil, err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}

func writeState(path string, s state.State) {
	if err := state.Write(path, s); err != nil {
		log.Printf("state write failed: %v", err)
	}
}
