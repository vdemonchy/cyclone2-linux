package hidraw

import "errors"

var (
	ErrDeviceNotFound = errors.New("cyclone2: controller not found (is the 2.4GHz dongle plugged in?)")
	ErrTimeout        = errors.New("cyclone2: timed out waiting for a HID report")
)
