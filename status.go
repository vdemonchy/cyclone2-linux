package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/victordemonchy/cyclone2-battery/internal/device"
	"github.com/victordemonchy/cyclone2-battery/internal/hidraw"
	"github.com/victordemonchy/cyclone2-battery/internal/powersupply"
	"github.com/victordemonchy/cyclone2-battery/internal/reader"
)

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	m, ok := device.Find("/sys", "/dev")
	if !ok {
		return hidraw.ErrDeviceNotFound
	}
	switch m.Mode.Source {
	case device.SourcePowerSupply:
		pct, level, charging, psok := powersupply.Read(m.SysPath)
		printStatus(*asJSON, m.Mode.Name, pct, level, charging, psok)
		return nil
	case device.SourceHID:
		dev, err := hidraw.Open(m.DevPath)
		if err != nil {
			return err
		}
		defer dev.Close()
		st, err := reader.Read(dev)
		if err != nil {
			return err
		}
		printStatus(*asJSON, m.Mode.Name, st.Percent, "", st.Charging, true)
		return nil
	case device.SourceHIDFeature:
		dev, err := hidraw.Open(m.DevPath)
		if err != nil {
			return err
		}
		defer dev.Close()
		st, err := reader.ReadDS4(dev)
		if err != nil {
			return err
		}
		_, _, charging, _ := powersupply.Read(m.SysPath)
		printStatus(*asJSON, m.Mode.Name, st.Percent, "", charging, true)
		return nil
	default: // SourceNone — HID mode / dongle idle / controller off: no battery
		if *asJSON {
			b, _ := json.Marshal(map[string]any{"present": false, "mode": m.Mode.Name})
			fmt.Println(string(b))
		} else {
			fmt.Println("no controller connected (HID mode or dongle idle — no battery)")
		}
		return nil
	}
}

func printStatus(asJSON bool, mode string, percent int, level string, charging, known bool) {
	if asJSON {
		b, _ := json.Marshal(map[string]any{
			"present": true, "mode": mode, "percent": percent,
			"level": level, "charging": charging, "battery_known": known,
		})
		fmt.Println(string(b))
		return
	}
	if !known {
		fmt.Printf("%s mode (battery unavailable)\n", mode)
		return
	}
	batt := fmt.Sprintf("%d%%", percent)
	if level != "" {
		batt = level
	}
	chg := ""
	if charging {
		chg = " charging"
	}
	fmt.Printf("%s (%s)%s\n", batt, mode, chg)
}
