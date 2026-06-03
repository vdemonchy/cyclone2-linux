package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"strconv"
	"time"

	"github.com/victordemonchy/cyclone2-battery/internal/hidraw"
)

func runProbe(args []string) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	passive := fs.Bool("passive", false, "read input reports for 5s and dump them")
	feature := fs.Bool("feature", false, "GET_FEATURE on report IDs 0x10 and 0x12")
	send := fs.String("send", "", "send a 0x0F command with this hex opcode (e.g. 0x01) and dump replies")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// No mode selected: print hint and exit without requiring a device.
	if !*passive && !*feature && *send == "" {
		fmt.Println("pick one of --passive, --feature, --send 0xNN")
		return nil
	}

	path, err := hidraw.FindDevicePath("/sys", "/dev")
	if err != nil {
		return err
	}
	fmt.Println("device:", path)
	dev, err := hidraw.Open(path)
	if err != nil {
		return err
	}
	defer dev.Close()

	switch {
	case *passive:
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			rep, err := dev.Read(500 * time.Millisecond)
			if err == hidraw.ErrTimeout {
				continue
			}
			if err != nil {
				return err
			}
			fmt.Printf("IN  %s\n", hex.EncodeToString(rep))
		}
	case *feature:
		for _, id := range []byte{0x10, 0x12} {
			buf, err := dev.GetFeature(id, 64)
			if err != nil {
				fmt.Printf("FEATURE 0x%02x: error %v\n", id, err)
				continue
			}
			fmt.Printf("FEATURE 0x%02x: %s\n", id, hex.EncodeToString(buf))
		}
	case *send != "":
		op, err := strconv.ParseUint(*send, 0, 8)
		if err != nil {
			return err
		}
		frame := make([]byte, 64)
		frame[0] = 0x0F
		frame[1] = byte(op)
		if err := dev.Write(frame); err != nil {
			return err
		}
		fmt.Printf("OUT %s\n", hex.EncodeToString(frame))
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			rep, err := dev.Read(500 * time.Millisecond)
			if err == hidraw.ErrTimeout {
				continue
			}
			if err != nil {
				return err
			}
			fmt.Printf("IN  %s\n", hex.EncodeToString(rep))
		}
	}
	return nil
}
