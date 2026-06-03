// Package reader performs one battery request/response exchange against a
// hidraw.ReadWriter, isolating the retry/skip logic so it is testable with a fake.
package reader

import (
	"time"

	"github.com/victordemonchy/cyclone2-battery/internal/hidraw"
	"github.com/victordemonchy/cyclone2-battery/internal/protocol"
)

// ReadDS4 reads battery from the DS4-mode vendor feature report 0x12 (byte 10 =
// percent). The kernel power_supply capacity is unreliable for the GameSir
// dongle, so we read the feature report directly.
func ReadDS4(dev hidraw.ReadWriter) (protocol.BatteryStatus, error) {
	buf, err := dev.GetFeature(protocol.DS4FeatureReportID, 64)
	if err != nil {
		return protocol.BatteryStatus{}, err
	}
	return protocol.ParseDS4Feature(buf)
}

// Read writes the battery request, then reads input frames until it gets the
// 0x12 battery report, skipping other frames (the 0x10 event report, noise).
// Returns hidraw.ErrTimeout if no battery frame arrives before the deadline.
func Read(dev hidraw.ReadWriter) (protocol.BatteryStatus, error) {
	if err := dev.Write(protocol.BuildBatteryRequest()); err != nil {
		return protocol.BatteryStatus{}, err
	}
	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		rep, err := dev.Read(200 * time.Millisecond)
		if err == hidraw.ErrTimeout {
			continue
		}
		if err != nil {
			return protocol.BatteryStatus{}, err
		}
		st, err := protocol.ParseBattery(rep)
		if err == protocol.ErrUnexpectedReport {
			continue
		}
		return st, err
	}
	return protocol.BatteryStatus{}, hidraw.ErrTimeout
}
