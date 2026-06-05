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
