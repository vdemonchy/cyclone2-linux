package main

import (
	"encoding/hex"
	"log"
	"strings"
	"time"

	"github.com/vdemonchy/cyclone2-linux/internal/config"
	"github.com/vdemonchy/cyclone2-linux/internal/device"
	"github.com/vdemonchy/cyclone2-linux/internal/protocol"
	"github.com/vdemonchy/cyclone2-linux/internal/state"
)

// rgbWriteGap spaces consecutive LED writes; the firmware drops zone writes sent
// back-to-back (confirmed on hardware). Kept in sync with rgb.go's writeGap.
const rgbWriteGap = 60 * time.Millisecond

// Battery-level colours for the logo zone, matching the tints the frontends put
// on their tray icon: green (high) / yellow (medium) / red (low).
var (
	levelColorHigh = [3]byte{0x2e, 0xc2, 0x7e}
	levelColorMid  = [3]byte{0xf5, 0xc2, 0x11}
	levelColorLow  = [3]byte{0xe0, 0x1b, 0x24}
)

// Fallback thresholds when the config carries none, matching the frontends'
// defaults for the icon tint.
const (
	defaultLevelHigh = 60
	defaultLevelLow  = 25
)

// rgbApplied is what the daemon has pushed to the controller since it last
// connected. Frames are computed as the delta against it, so a poll that doesn't
// change the lighting costs no HID writes at all.
type rgbApplied struct {
	static     bool      // static mode already entered
	zones      [][3]byte // nil until the first zone write
	brightness int       // -1 until the first brightness write
}

// rgbState tracks the applied lighting. Safe without locking: only touched from
// the daemon's single-goroutine event loop.
var rgbState = freshRGBState()

func freshRGBState() rgbApplied { return rgbApplied{brightness: -1} }

// resetRGBState forgets what was applied, so the next apply re-enters static
// mode and rewrites every zone (e.g. after the controller reconnects, when it
// has reverted to its default animation).
func resetRGBState() { rgbState = freshRGBState() }

// applyRGBFromConfig pushes the configured lighting to the controller, given the
// battery snapshot the logo zone may follow. It is a no-op when no RGB is
// configured at all (CLI-only setups keep their lighting), when the controller
// is not in XInput mode (the only mode whose vendor interface accepts the LED
// protocol), or when nothing has changed since the last apply. Called from the
// daemon's single-goroutine event loop, so it never races the battery poll.
func applyRGBFromConfig(st state.State) {
	cfg, err := config.Read()
	if err != nil || cfg.RGB == nil {
		return
	}
	m, ok := device.Find("/sys", "/dev")
	if !ok || m.Mode.Name != "xinput" {
		return
	}
	frames, next := rgbFrames(*cfg.RGB, st, rgbState)
	if len(frames) == 0 {
		return
	}
	dev, err := openHID(m.DevPath)
	if err != nil {
		log.Printf("rgb: cannot open %s: %v", m.DevPath, err)
		return
	}
	defer dev.Close()
	for i, f := range frames {
		if i > 0 {
			time.Sleep(rgbWriteGap)
		}
		if err := dev.Write(f); err != nil {
			// Leave rgbState untouched so the next apply retries the whole
			// sequence rather than assuming a half-written frame stuck.
			log.Printf("rgb: write failed: %v", err)
			return
		}
	}
	rgbState = next
}

// rgbFrames turns an RGB config plus the current battery state into the ordered
// reports to send, given what was already applied: optionally the "enter static
// mode" command, then the zone colours that actually changed, then brightness.
// It returns the frames and the state to record once they are all written.
func rgbFrames(r config.RGB, st state.State, prev rgbApplied) ([][]byte, rgbApplied) {
	next := prev
	var frames [][]byte

	if colors, ok := desiredZones(r, st); ok {
		// Entering static mode briefly forces every zone to one colour, so send
		// it only when the controller may still be animating (first apply /
		// after reconnect) and rewrite all zones behind it.
		if !prev.static {
			c0 := colors[0]
			frames = append(frames, protocol.BuildEnterStatic(c0[0], c0[1], c0[2]))
			next.static = true
			next.zones = nil
		}
		for i, reg := range protocol.LEDZoneRegs {
			if next.zones != nil && next.zones[i] == colors[i] {
				continue // already showing this colour
			}
			c := colors[i]
			frames = append(frames, protocol.BuildZoneColor(reg, c[0], c[1], c[2]))
		}
		next.zones = colors
	}
	// Brightness is meaningless while the zones are black, so skip it when the
	// lighting is off — re-enabling then restores the configured value.
	if r.On() && r.Brightness != nil && *r.Brightness != prev.brightness {
		frames = append(frames, protocol.BuildBrightness(*r.Brightness))
		next.brightness = *r.Brightness
	}
	if len(frames) == 0 {
		return nil, prev
	}
	return frames, next
}

// desiredZones resolves the colours the controller should show: all black when
// lighting is disabled, otherwise the configured zones with the logo zone
// replaced by the battery-level colour when that option is on. ok is false when
// the config carries no usable zone list, which leaves the zones untouched.
func desiredZones(r config.RGB, st state.State) ([][3]byte, bool) {
	if !r.On() {
		return make([][3]byte, protocol.NumZones), true // lighting off
	}
	colors, ok := parseZones(r.Zones)
	if !ok {
		return nil, false
	}
	if r.BatteryLogo {
		if c, ok := batteryLevelColor(r, st); ok {
			colors[protocol.LEDZoneLogo] = c
		}
	}
	return colors, true
}

// batteryLevelColor maps the battery percentage onto the configured thresholds.
// ok is false when there is no level to map (no controller, no battery source,
// or a stale reading), so the logo keeps its configured colour instead of
// claiming a level the daemon can't vouch for.
func batteryLevelColor(r config.RGB, st state.State) ([3]byte, bool) {
	if !st.Present || !st.BatteryKnown || st.Stale {
		return [3]byte{}, false
	}
	high, low := r.LevelHigh, r.LevelLow
	if high <= 0 {
		high = defaultLevelHigh
	}
	if low <= 0 {
		low = defaultLevelLow
	}
	switch {
	case st.Percent >= high:
		return levelColorHigh, true
	case st.Percent >= low:
		return levelColorMid, true
	default:
		return levelColorLow, true
	}
}

// parseZones converts the config's hex zone strings into RGB triples. It returns
// ok=false unless exactly NumZones valid colours are present, so a partial or
// malformed list leaves the lighting untouched rather than half-applied.
func parseZones(zones []string) ([][3]byte, bool) {
	if len(zones) != protocol.NumZones {
		return nil, false
	}
	out := make([][3]byte, protocol.NumZones)
	for i, z := range zones {
		v, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(z), "#"))
		if err != nil || len(v) != 3 {
			return nil, false
		}
		out[i] = [3]byte{v[0], v[1], v[2]}
	}
	return out, true
}
