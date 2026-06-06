package main

import (
	"encoding/hex"
	"log"
	"strings"
	"time"

	"github.com/victordemonchy/cyclone2-battery/internal/config"
	"github.com/victordemonchy/cyclone2-battery/internal/device"
	"github.com/victordemonchy/cyclone2-battery/internal/protocol"
)

// rgbWriteGap spaces consecutive LED writes; the firmware drops zone writes sent
// back-to-back (confirmed on hardware). Kept in sync with rgb.go's writeGap.
const rgbWriteGap = 60 * time.Millisecond

// rgbStaticEntered tracks whether we've already switched the controller into
// static mode since it (re)connected. The "enter static" command briefly forces
// every zone to one colour, so re-sending it on each change makes all zones
// flash; we send it only once and then just update the zone colours. Reset on
// hotplug via resetRGBState. Safe without locking: only touched from the daemon's
// single-goroutine event loop.
var rgbStaticEntered bool

// resetRGBState forces the next apply to re-enter static mode (e.g. after the
// controller reconnects, when it has reverted to its default animation).
func resetRGBState() { rgbStaticEntered = false }

// applyRGBFromConfig pushes the configured lighting to the controller. It is a
// no-op when no RGB is configured or the controller is not in XInput mode (the
// only mode whose vendor interface accepts the LED protocol). Called from the
// daemon's single-goroutine event loop, so it never races the battery poll.
func applyRGBFromConfig() {
	cfg, err := config.Read()
	if err != nil || cfg.RGB == nil {
		return
	}
	m, ok := device.Find("/sys", "/dev")
	if !ok || m.Mode.Name != "xinput" {
		return
	}
	frames := rgbFrames(*cfg.RGB, !rgbStaticEntered)
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
			log.Printf("rgb: write failed: %v", err)
			return
		}
	}
	rgbStaticEntered = true
}

// rgbFrames turns an RGB config into the ordered reports to send: optionally the
// "enter static mode" command, then the per-zone colours, then brightness.
// enterStatic should be true only when the controller may still be in an
// animated mode (first apply / after reconnect) to avoid the all-zone flash.
func rgbFrames(r config.RGB, enterStatic bool) [][]byte {
	var frames [][]byte
	if colors, ok := parseZones(r.Zones); ok {
		if enterStatic {
			c0 := colors[0]
			frames = append(frames, protocol.BuildEnterStatic(c0[0], c0[1], c0[2]))
		}
		for i, reg := range protocol.LEDZoneRegs {
			c := colors[i]
			frames = append(frames, protocol.BuildZoneColor(reg, c[0], c[1], c[2]))
		}
	}
	if r.Brightness != nil {
		frames = append(frames, protocol.BuildBrightness(*r.Brightness))
	}
	return frames
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
