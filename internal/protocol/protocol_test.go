package protocol

import (
	"encoding/hex"
	"testing"
)

func batteryFrame(percent, flags byte) []byte {
	f := make([]byte, 64)
	f[0] = 0x12
	f[35] = flags // charging/cable flag
	f[36] = percent
	return f
}

func TestParseBatteryFull(t *testing.T) {
	st, err := ParseBattery(batteryFrame(100, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !st.Present || st.Percent != 100 || st.Charging {
		t.Fatalf("got %+v, want {Present:true Percent:100 Charging:false}", st)
	}
}

func TestParseBatteryChargingFlag(t *testing.T) {
	st, err := ParseBattery(batteryFrame(80, 0x01))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Percent != 80 || !st.Charging {
		t.Fatalf("got %+v, want Percent:80 Charging:true", st)
	}
}

func TestParseBatteryRealCapturedFrame(t *testing.T) {
	// Real on-battery frame captured 2026-06-03 (docs/protocol.md). byte[36]=0x64=100.
	raw, _ := hex.DecodeString("12808080800f00000000ed0d00feff00000e00a5009b20f9fd000000000000000000000064000118293f00002b14000000000000000000808080800f0000000000")
	st, err := ParseBattery(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Percent != 100 || st.Charging {
		t.Fatalf("got %+v, want Percent:100 Charging:false (on-battery capture)", st)
	}
}

// Real frames captured 2026-06-05 by plugging/unplugging the controller in
// XInput mode: byte[35] is the charging/cable flag (0 unplugged, 1 plugged).
func TestParseBatteryRealChargingFrames(t *testing.T) {
	unplugged, _ := hex.DecodeString("12808080800f00000000ec6300feff00000700d200582083ff000000000000000000000064003f250005003a00342500000000000000808080800f0000000000")
	plugged, _ := hex.DecodeString("12808080800f0000000092e900fdff00000900c7005a2085ff00000000000000000000016400021b263f00002c1300000000000000007f8080800f0000000000")
	if st, _ := ParseBattery(unplugged); st.Charging {
		t.Fatalf("unplugged frame: got Charging:true, want false")
	}
	if st, _ := ParseBattery(plugged); !st.Charging || st.Percent != 100 {
		t.Fatalf("plugged frame: got %+v, want Percent:100 Charging:true", st)
	}
}

func TestParseBatteryRejectsEventReport0x10(t *testing.T) {
	f := make([]byte, 64)
	f[0] = 0x10
	f[1] = 0x06
	if _, err := ParseBattery(f); err != ErrUnexpectedReport {
		t.Fatalf("got %v, want ErrUnexpectedReport for 0x10", err)
	}
}

func TestParseBatteryRejectsShort(t *testing.T) {
	f := make([]byte, 20)
	f[0] = 0x12
	if _, err := ParseBattery(f); err != ErrUnexpectedReport {
		t.Fatalf("got %v, want ErrUnexpectedReport for short frame", err)
	}
}

func TestBuildBatteryRequest(t *testing.T) {
	f := BuildBatteryRequest()
	if len(f) != 64 || f[0] != 0x0F || f[1] != 0x03 {
		t.Fatalf("bad request frame: len=%d [0]=0x%02x [1]=0x%02x", len(f), f[0], f[1])
	}
}
