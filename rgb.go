package main

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/victordemonchy/cyclone2-battery/internal/hidraw"
	"github.com/victordemonchy/cyclone2-battery/internal/protocol"
)

// writeGap spaces out consecutive LED writes; the firmware drops zone writes
// sent back-to-back (confirmed on hardware: without a gap, some zones lag a
// colour behind, inconsistently).
const writeGap = 60 * time.Millisecond

func rgbUsage() {
	fmt.Println(`cyclone2 rgb — control the Cyclone 2 RGB lighting

usage:
  cyclone2 rgb color <RRGGBB>        solid colour on all zones (e.g. ff0000)
  cyclone2 rgb zones <L> <R> <Logo> <Center>
                                     per-zone colours (4x RRGGBB)
  cyclone2 rgb off                   turn the lighting off (solid black)
  cyclone2 rgb brightness <0-100>    overall brightness
  cyclone2 rgb raw <hex>             send a raw 0x0F report (right-padded to 64)

Requires write access to the controller's vendor hidraw node (the install.sh
udev rule grants it) and the controller in XInput mode (USB 3537:100b).`)
}

func runRGB(args []string) error {
	if len(args) == 0 {
		rgbUsage()
		return nil
	}

	var frames [][]byte
	switch args[0] {
	case "color":
		if len(args) != 2 {
			rgbUsage()
			return fmt.Errorf("rgb color: need one RRGGBB argument")
		}
		r, g, b, err := parseHexColor(args[1])
		if err != nil {
			return err
		}
		frames = protocol.BuildStaticColor(r, g, b)

	case "zones":
		if len(args) != 1+protocol.NumZones {
			rgbUsage()
			return fmt.Errorf("rgb zones: need %d RRGGBB arguments (%s)",
				protocol.NumZones, strings.Join(protocol.LEDZoneNames, ", "))
		}
		colors := make([][3]byte, protocol.NumZones)
		for i := 0; i < protocol.NumZones; i++ {
			r, g, b, err := parseHexColor(args[1+i])
			if err != nil {
				return err
			}
			colors[i] = [3]byte{r, g, b}
		}
		frames = protocol.BuildZones(colors)

	case "off":
		frames = protocol.BuildStaticColor(0, 0, 0)

	case "brightness":
		if len(args) != 2 {
			rgbUsage()
			return fmt.Errorf("rgb brightness: need a 0-100 argument")
		}
		pct, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("rgb brightness: %w", err)
		}
		frames = [][]byte{protocol.BuildBrightness(pct)}

	case "raw":
		if len(args) != 2 {
			rgbUsage()
			return fmt.Errorf("rgb raw: need one hex argument")
		}
		raw, err := hex.DecodeString(strings.ReplaceAll(args[1], " ", ""))
		if err != nil {
			return fmt.Errorf("rgb raw: %w", err)
		}
		frame := make([]byte, 64)
		copy(frame, raw)
		frames = [][]byte{frame}

	default:
		rgbUsage()
		return fmt.Errorf("unknown rgb command %q", args[0])
	}

	path, err := hidraw.FindDevicePath("/sys", "/dev")
	if err != nil {
		return err
	}
	dev, err := hidraw.Open(path)
	if err != nil {
		return err
	}
	defer dev.Close()

	for i, f := range frames {
		if i > 0 {
			time.Sleep(writeGap)
		}
		if err := dev.Write(f); err != nil {
			return err
		}
	}
	return nil
}

// parseHexColor parses "RRGGBB" or "#RRGGBB" into r,g,b bytes.
func parseHexColor(s string) (r, g, b byte, err error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("colour %q: want 6 hex digits RRGGBB", s)
	}
	v, err := hex.DecodeString(s)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("colour %q: %w", s, err)
	}
	return v[0], v[1], v[2], nil
}
