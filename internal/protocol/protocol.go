// Package protocol encodes the battery request and decodes the reply for the
// GameSir Cyclone 2 vendor HID interface. Offsets are confirmed in docs/protocol.md.
package protocol

import "errors"

const (
	reportIDCommand = 0x0F // OUTPUT command report
	reportIDBattery = 0x12 // INPUT full-state report that carries battery
	opcodeBattery   = 0x03 // any of 0x01/0x03 wakes the stream; 0x03 used
	reportLen       = 64

	offsetFlags   = 35 // byte[35] = charging/cable flag: 0 on battery, 1 plugged in
	offsetPercent = 36 // byte[36] = battery percent, raw 0-100 (CONFIRMED)
)

// ErrUnexpectedReport means the report is not the 0x12 battery report.
var ErrUnexpectedReport = errors.New("protocol: not a battery report")

// BatteryStatus holds the decoded battery state from a 0x12 report.
type BatteryStatus struct {
	Present  bool
	Percent  int
	Charging bool
}

// BuildBatteryRequest returns the 64-byte output report that wakes the vendor
// interface so it streams the 0x12 state report.
func BuildBatteryRequest() []byte {
	f := make([]byte, reportLen)
	f[0] = reportIDCommand
	f[1] = opcodeBattery
	return f
}

// ParseBattery decodes a 0x12 battery report. It rejects any other report ID
// (notably the 0x10 event report, whose byte[36] is 0) and short frames.
func ParseBattery(report []byte) (BatteryStatus, error) {
	if len(report) <= offsetPercent || report[0] != reportIDBattery {
		return BatteryStatus{}, ErrUnexpectedReport
	}
	pct := int(report[offsetPercent])
	if pct > 100 {
		pct = 100
	}
	return BatteryStatus{
		Present:  true,
		Percent:  pct,
		Charging: report[offsetFlags] != 0, // byte[35], confirmed by plug/unplug capture
	}, nil
}

const (
	// DS4FeatureReportID is the vendor GET_FEATURE report carrying DS4-mode battery.
	DS4FeatureReportID = 0x12
	ds4BatteryOffset   = 10 // byte 10 = battery percent (0-100); confirmed 0x64 at full
)

// ParseDS4Feature decodes the DS4 vendor feature report 0x12: byte 10 = battery percent.
func ParseDS4Feature(report []byte) (BatteryStatus, error) {
	if len(report) <= ds4BatteryOffset || report[0] != DS4FeatureReportID {
		return BatteryStatus{}, ErrUnexpectedReport
	}
	pct := int(report[ds4BatteryOffset])
	if pct > 100 {
		pct = 100
	}
	return BatteryStatus{Present: true, Percent: pct}, nil
}

// --- RGB LED control -------------------------------------------------------
//
// The Cyclone 2's lighting is driven over the same vendor OUTPUT report 0x0F as
// the battery wake-up, but with opcode 0x03 and byte[2]=0x20 selecting the LED
// subsystem. The payload is a register write: byte[4]=register, byte[5]=length,
// byte[6:] = data. Reverse-engineered 2026-06-06 by capturing GameSir Connect
// (running under WinBoat) with usbmon; see docs/protocol.md.
const (
	ledOpcode = 0x03 // byte[1]: same command opcode as the battery wake
	ledTarget = 0x20 // byte[2]: selects the LED subsystem

	ledRegMode       = 0x01 // effect/mode select: payload[0] = effect id
	ledRegBrightness = 0x04 // brightness 0-100, 1 byte

	effectStatic    = 0x01 // payload[0] of reg 0x01: solid/static mode
	modeConst       = 0x05 // reg 0x01 payload[1], constant in every capture
	modeSpeed       = 0x0a // reg 0x01 payload[2] (speed; irrelevant for static)
	modeBrightness  = 0x32 // reg 0x01 payload[3] (brightness echo; not load-bearing)
	modePayLen      = 0x3a // 58-byte payload GameSir Connect sends entering a mode
	modeFillTriples = 7    // RGB triples the app writes into the mode payload
)

// LEDZoneRegs are the four per-zone colour registers, in the order the physical
// zones light up: Left, Right, Logo, Center (confirmed on hardware). In static
// mode, writing an RGB triple to each sets that zone. They must be written with
// a short gap between them or the firmware drops some.
var LEDZoneRegs = []byte{0x05, 0x08, 0x0e, 0x11}

// LEDZoneNames labels LEDZoneRegs positionally for the frontends.
var LEDZoneNames = []string{"Left", "Right", "Logo", "Center"}

// NumZones is the number of independently addressable LED zones.
const NumZones = 4

// buildLEDCommand frames a single LED register write into a 64-byte report.
func buildLEDCommand(reg byte, data []byte) []byte {
	f := make([]byte, reportLen)
	f[0] = reportIDCommand
	f[1] = ledOpcode
	f[2] = ledTarget
	f[4] = reg
	f[5] = byte(len(data))
	copy(f[6:], data)
	return f
}

// BuildBrightness sets overall LED brightness (clamped to 0-100). Register 0x04.
func BuildBrightness(pct int) []byte {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return buildLEDCommand(ledRegBrightness, []byte{byte(pct)})
}

// BuildEnterStatic returns the register-0x01 command that switches the lighting
// to solid/static mode (effect 0x01), mirroring GameSir Connect. The colour is
// also seeded into the mode payload; the per-zone writes from BuildZoneColor
// then set it definitively.
func BuildEnterStatic(r, g, b byte) []byte {
	payload := make([]byte, 0, modePayLen)
	payload = append(payload, effectStatic, modeConst, modeSpeed, modeBrightness)
	for i := 0; i < modeFillTriples && len(payload)+3 <= modePayLen; i++ {
		payload = append(payload, r, g, b)
	}
	for len(payload) < modePayLen {
		payload = append(payload, 0)
	}
	return buildLEDCommand(ledRegMode, payload)
}

// BuildZoneColor sets one LED zone (a register from LEDZoneRegs) to an RGB value.
func BuildZoneColor(zoneReg, r, g, b byte) []byte {
	return buildLEDCommand(zoneReg, []byte{r, g, b})
}

// BuildStaticColor returns the full ordered sequence of reports that set a solid
// colour on every zone: enter static mode, then set each zone. Send them in
// order with a short delay between each (the firmware drops back-to-back writes).
func BuildStaticColor(r, g, b byte) [][]byte {
	colors := make([][3]byte, NumZones)
	for i := range colors {
		colors[i] = [3]byte{r, g, b}
	}
	return BuildZones(colors)
}

// BuildZones returns the ordered reports that set each zone to its own colour:
// enter static mode (seeded with the first zone's colour), then one write per
// zone. colors must have NumZones entries, ordered like LEDZoneRegs/LEDZoneNames.
func BuildZones(colors [][3]byte) [][]byte {
	frames := [][]byte{BuildEnterStatic(colors[0][0], colors[0][1], colors[0][2])}
	for i, reg := range LEDZoneRegs {
		c := colors[i]
		frames = append(frames, BuildZoneColor(reg, c[0], c[1], c[2]))
	}
	return frames
}
