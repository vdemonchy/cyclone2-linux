package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/vdemonchy/cyclone2-linux/internal/config"
	"github.com/vdemonchy/cyclone2-linux/internal/device"
	"github.com/vdemonchy/cyclone2-linux/internal/hidraw"
	"github.com/vdemonchy/cyclone2-linux/internal/notify"
	"github.com/vdemonchy/cyclone2-linux/internal/powersupply"
	"github.com/vdemonchy/cyclone2-linux/internal/reader"
	"github.com/vdemonchy/cyclone2-linux/internal/state"
	"github.com/vdemonchy/cyclone2-linux/internal/uevent"

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
	notifier := &lowBatteryNotifier{send: notify.Send}
	// poll reads the battery, writes the state file, and fires a low-battery
	// notification if the level just crossed the configured threshold.
	poll := func() {
		pollOnce(*statePath, &last)
		notifier.consider(last, currentThreshold())
	}
	poll()
	applyRGBFromConfig() // push configured lighting on startup

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
			poll()
			reapply() // fallback pickup if inotify is unavailable
		case ev := <-events:
			if ev.Action == "remove" {
				last = state.State{Present: false}
				writeState(*statePath, last)
				notifier.consider(last, currentThreshold())
				resetRGBState() // a reconnect will need to re-enter static mode
				log.Printf("controller disconnected")
			} else {
				resetRGBState() // freshly connected: re-enter static mode once
				poll()
				applyRGBFromConfig() // re-apply lighting on reconnect
			}
		case <-configCh:
			reapply()
			applyRGBFromConfig() // lighting settings may have changed
		}
	}
}

// lowBatteryNotifier posts a desktop notification when the battery first drops
// to or below the threshold, then stays quiet until the level recovers (with a
// small hysteresis margin) or the controller charges/disconnects — so a battery
// hovering near the threshold doesn't spam the user.
type lowBatteryNotifier struct {
	notified bool
	send     func(icon, summary, body string) error
}

// lowBatteryHysteresis is how far above the threshold the battery must climb
// before a fresh low-battery notification can fire again.
const lowBatteryHysteresis = 5

func (n *lowBatteryNotifier) consider(s state.State, threshold int) {
	if threshold <= 0 {
		return // notifications disabled
	}
	// Charging or disconnected: re-arm so the next discharge can notify again.
	if s.Charging || !s.Present {
		n.notified = false
		return
	}
	// Can't trust the level: leave the armed state untouched.
	if !s.BatteryKnown || s.Stale {
		return
	}
	if s.Percent <= threshold {
		if !n.notified {
			body := fmt.Sprintf("%s battery at %d%%", notify.AppName, s.Percent)
			if err := n.send("input-gaming-symbolic", "Controller battery low", body); err != nil {
				log.Printf("low-battery notification failed: %v", err)
			}
			n.notified = true
		}
	} else if s.Percent >= threshold+lowBatteryHysteresis {
		n.notified = false
	}
}

// currentThreshold reads the live low-battery threshold from the config file.
func currentThreshold() int {
	cfg, _ := config.Read()
	return cfg.LowBatteryThreshold
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
		*last = readDS4(m)
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

// readDS4 reports the DS4-mode controller as present with an unknown battery.
// The GameSir dongle exposes no live battery in DS4 mode: the vendor feature
// 0x12 byte 10 is a frozen 0x64 (100) that never tracks real charge, and the
// standard DS4 battery field (input-report byte 30) stays 0. Reporting
// BatteryKnown=false makes both frontends show the controller without a (fake)
// percentage, rather than a hardcoded-looking 100%. See docs/protocol.md.
func readDS4(m device.Match) state.State {
	return state.State{Present: true, Mode: m.Mode.Name, BatteryKnown: false}
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
