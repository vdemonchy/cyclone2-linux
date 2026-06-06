package protocol

import "testing"

// trimmed compares the leading non-padded bytes of a 64-byte report against want.
func leading(b []byte, n int) []byte { return b[:n] }

func TestBuildBrightnessMatchesCapture(t *testing.T) {
	// GameSir Connect, brightness slider at 100: 0f 03 20 00 04 01 64
	got := BuildBrightness(100)
	want := []byte{0x0f, 0x03, 0x20, 0x00, 0x04, 0x01, 0x64}
	if len(got) != 64 {
		t.Fatalf("report length = %d, want 64", len(got))
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("byte[%d] = 0x%02x, want 0x%02x (got %x)", i, got[i], w, leading(got, len(want)))
		}
	}
}

func TestBuildBrightnessClamps(t *testing.T) {
	if BuildBrightness(250)[6] != 100 {
		t.Errorf("over-range brightness not clamped to 100")
	}
	if BuildBrightness(-5)[6] != 0 {
		t.Errorf("negative brightness not clamped to 0")
	}
}

func TestBuildEnterStaticMatchesCapture(t *testing.T) {
	// GameSir Connect entering static mode: reg 0x01, effect 0x01,
	// 0f 03 20 00 01 3a 01 05 0a 32 <colour x7> 00...
	got := BuildEnterStatic(0xff, 0x00, 0x00)
	want := []byte{
		0x0f, 0x03, 0x20, 0x00, 0x01, 0x3a,
		0x01, 0x05, 0x0a, 0x32,
		0xff, 0x00, 0x00, 0xff, 0x00, 0x00, 0xff, 0x00, 0x00,
	}
	if len(got) != 64 {
		t.Fatalf("report length = %d, want 64", len(got))
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("byte[%d] = 0x%02x, want 0x%02x\n got=%x", i, got[i], w, leading(got, len(want)))
		}
	}
	if got[5] != modePayLen {
		t.Errorf("len byte = 0x%02x, want 0x%02x", got[5], modePayLen)
	}
}

func TestBuildStaticColorSequence(t *testing.T) {
	frames := BuildStaticColor(0x00, 0xff, 0x00)
	// 1 mode-enter frame + one per zone register.
	if len(frames) != 1+len(LEDZoneRegs) {
		t.Fatalf("got %d frames, want %d", len(frames), 1+len(LEDZoneRegs))
	}
	if frames[0][4] != ledRegMode || frames[0][6] != effectStatic {
		t.Errorf("first frame is not enter-static (reg=0x%02x eff=0x%02x)", frames[0][4], frames[0][6])
	}
	for i, reg := range LEDZoneRegs {
		f := frames[1+i]
		// 0f 03 20 00 <reg> 03 00 ff 00
		want := []byte{0x0f, 0x03, 0x20, 0x00, reg, 0x03, 0x00, 0xff, 0x00}
		for j, w := range want {
			if f[j] != w {
				t.Fatalf("zone %d byte[%d] = 0x%02x, want 0x%02x", i, j, f[j], w)
			}
		}
	}
}
