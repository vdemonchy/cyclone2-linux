package main

import (
	"testing"

	"github.com/vdemonchy/cyclone2-linux/internal/config"
	"github.com/vdemonchy/cyclone2-linux/internal/protocol"
	"github.com/vdemonchy/cyclone2-linux/internal/state"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

// zoneColor extracts the RGB triple a zone frame carries (payload at byte 6).
func zoneColor(frame []byte) [3]byte {
	return [3]byte{frame[6], frame[7], frame[8]}
}

// frameForReg returns the frame writing register reg, or nil.
func frameForReg(frames [][]byte, reg byte) []byte {
	for _, f := range frames {
		if f[4] == reg {
			return f
		}
	}
	return nil
}

var fullBattery = state.State{Present: true, Mode: "xinput", BatteryKnown: true, Percent: 90}

func TestRGBFramesFirstApplyEntersStaticAndSetsEveryZone(t *testing.T) {
	r := config.RGB{Zones: []string{"ff0000", "00ff00", "0000ff", "ffffff"}}
	frames, next := rgbFrames(r, fullBattery, freshRGBState())
	if len(frames) != 1+protocol.NumZones {
		t.Fatalf("got %d frames, want %d", len(frames), 1+protocol.NumZones)
	}
	if frames[0][4] != 0x01 {
		t.Errorf("first frame is not the enter-static command (reg 0x%02x)", frames[0][4])
	}
	if !next.static {
		t.Error("next state should record that static mode was entered")
	}
}

func TestRGBFramesSkipsUnchangedZones(t *testing.T) {
	r := config.RGB{Zones: []string{"ff0000", "00ff00", "0000ff", "ffffff"}}
	_, applied := rgbFrames(r, fullBattery, freshRGBState())

	// Nothing changed: no writes at all.
	if frames, _ := rgbFrames(r, fullBattery, applied); len(frames) != 0 {
		t.Fatalf("re-applying an unchanged config sent %d frames, want 0", len(frames))
	}

	// One zone changed: exactly one write, for that zone.
	r.Zones[1] = "123456"
	frames, _ := rgbFrames(r, fullBattery, applied)
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
	if frames[0][4] != protocol.LEDZoneRegs[1] {
		t.Errorf("wrote register 0x%02x, want 0x%02x", frames[0][4], protocol.LEDZoneRegs[1])
	}
}

func TestRGBFramesBrightnessOnlyOnChange(t *testing.T) {
	r := config.RGB{Zones: []string{"ffffff", "ffffff", "ffffff", "ffffff"}, Brightness: intPtr(80)}
	frames, applied := rgbFrames(r, fullBattery, freshRGBState())
	if f := frameForReg(frames, 0x04); f == nil || f[6] != 80 {
		t.Fatalf("first apply did not set brightness to 80: %v", f)
	}
	if frames, _ := rgbFrames(r, fullBattery, applied); len(frames) != 0 {
		t.Fatalf("unchanged brightness re-sent %d frames, want 0", len(frames))
	}
	r.Brightness = intPtr(20)
	frames, _ = rgbFrames(r, fullBattery, applied)
	if len(frames) != 1 || frames[0][6] != 20 {
		t.Fatalf("brightness change sent %v, want a single 20%% write", frames)
	}
}

func TestRGBFramesDisabledBlanksEveryZone(t *testing.T) {
	r := config.RGB{
		Enabled:    boolPtr(false),
		Zones:      []string{"ff0000", "00ff00", "0000ff", "ffffff"},
		Brightness: intPtr(100),
	}
	frames, _ := rgbFrames(r, fullBattery, freshRGBState())
	if len(frames) != 1+protocol.NumZones {
		t.Fatalf("got %d frames, want %d (enter-static + one per zone)", len(frames), 1+protocol.NumZones)
	}
	for i, reg := range protocol.LEDZoneRegs {
		f := frameForReg(frames, reg)
		if f == nil {
			t.Fatalf("zone %d (reg 0x%02x) not written", i, reg)
		}
		if c := zoneColor(f); c != [3]byte{0, 0, 0} {
			t.Errorf("zone %d = %v, want black", i, c)
		}
	}
	if f := frameForReg(frames, 0x04); f != nil {
		t.Errorf("brightness written while lighting is off: %v", f)
	}
}

func TestRGBFramesMissingEnabledKeyStaysOn(t *testing.T) {
	// Configs written before the "enabled" key existed must keep their colours.
	r := config.RGB{Zones: []string{"ff0000", "ff0000", "ff0000", "ff0000"}}
	frames, _ := rgbFrames(r, fullBattery, freshRGBState())
	f := frameForReg(frames, protocol.LEDZoneRegs[0])
	if f == nil || zoneColor(f) != [3]byte{0xff, 0, 0} {
		t.Fatalf("zone 0 not set to the configured red: %v", f)
	}
}

func TestBatteryLogoTracksThresholds(t *testing.T) {
	r := config.RGB{
		Zones:       []string{"ffffff", "ffffff", "ffffff", "ffffff"},
		BatteryLogo: true,
		LevelHigh:   60,
		LevelLow:    25,
	}
	for _, tc := range []struct {
		percent int
		want    [3]byte
	}{
		{100, levelColorHigh},
		{60, levelColorHigh},
		{59, levelColorMid},
		{25, levelColorMid},
		{24, levelColorLow},
		{0, levelColorLow},
	} {
		st := state.State{Present: true, Mode: "xinput", BatteryKnown: true, Percent: tc.percent}
		colors, ok := desiredZones(r, st)
		if !ok {
			t.Fatalf("%d%%: no zones resolved", tc.percent)
		}
		if got := colors[protocol.LEDZoneLogo]; got != tc.want {
			t.Errorf("%d%%: logo = %v, want %v", tc.percent, got, tc.want)
		}
		// Only the logo follows the battery; the other zones keep their colour.
		if colors[0] != [3]byte{0xff, 0xff, 0xff} {
			t.Errorf("%d%%: zone 0 = %v, want the configured white", tc.percent, colors[0])
		}
	}
}

func TestBatteryLogoFallsBackWhenLevelUnknown(t *testing.T) {
	r := config.RGB{
		Zones:       []string{"ffffff", "ffffff", "112233", "ffffff"},
		BatteryLogo: true,
	}
	for name, st := range map[string]state.State{
		"absent":            {Present: false},
		"no battery source": {Present: true, Mode: "ds4", BatteryKnown: false},
		"stale":             {Present: true, Mode: "xinput", BatteryKnown: true, Stale: true, Percent: 10},
	} {
		colors, ok := desiredZones(r, st)
		if !ok {
			t.Fatalf("%s: no zones resolved", name)
		}
		if got := colors[protocol.LEDZoneLogo]; got != [3]byte{0x11, 0x22, 0x33} {
			t.Errorf("%s: logo = %v, want the configured colour", name, got)
		}
	}
}

func TestBatteryLogoDefaultThresholds(t *testing.T) {
	// Thresholds left at zero fall back to the frontends' defaults (60/25).
	r := config.RGB{Zones: []string{"ffffff", "ffffff", "ffffff", "ffffff"}, BatteryLogo: true}
	st := state.State{Present: true, Mode: "xinput", BatteryKnown: true, Percent: 30}
	colors, _ := desiredZones(r, st)
	if got := colors[protocol.LEDZoneLogo]; got != levelColorMid {
		t.Errorf("logo = %v, want yellow at 30%% with default thresholds", got)
	}
}

func TestBatteryLogoRewritesOnlyTheLogoZone(t *testing.T) {
	r := config.RGB{
		Zones:       []string{"ffffff", "ffffff", "ffffff", "ffffff"},
		BatteryLogo: true,
	}
	full := state.State{Present: true, Mode: "xinput", BatteryKnown: true, Percent: 90}
	_, applied := rgbFrames(r, full, freshRGBState())

	empty := state.State{Present: true, Mode: "xinput", BatteryKnown: true, Percent: 10}
	frames, _ := rgbFrames(r, empty, applied)
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want just the logo rewrite", len(frames))
	}
	if frames[0][4] != protocol.LEDZoneRegs[protocol.LEDZoneLogo] {
		t.Errorf("wrote register 0x%02x, want the logo register", frames[0][4])
	}
	if c := zoneColor(frames[0]); c != levelColorLow {
		t.Errorf("logo = %v, want red at 10%%", c)
	}
}

func TestRGBFramesMalformedZonesLeaveLightingUntouched(t *testing.T) {
	for name, zones := range map[string][]string{
		"too few": {"ff0000"},
		"bad hex": {"zzzzzz", "00ff00", "0000ff", "ffffff"},
	} {
		frames, _ := rgbFrames(config.RGB{Zones: zones}, fullBattery, freshRGBState())
		if len(frames) != 0 {
			t.Errorf("%s: sent %d frames, want 0", name, len(frames))
		}
	}
}
