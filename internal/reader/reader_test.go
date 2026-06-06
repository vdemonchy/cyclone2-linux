package reader

import (
	"testing"
	"time"

	"github.com/vdemonchy/cyclone2-linux/internal/hidraw"
)

type fakeDev struct {
	written [][]byte
	replies [][]byte // returned in order on Read
	feature []byte
}

func (f *fakeDev) Write(r []byte) error { f.written = append(f.written, r); return nil }
func (f *fakeDev) Read(_ time.Duration) ([]byte, error) {
	if len(f.replies) == 0 {
		return nil, hidraw.ErrTimeout
	}
	r := f.replies[0]
	f.replies = f.replies[1:]
	return r, nil
}
func (f *fakeDev) GetFeature(byte, int) ([]byte, error) { return f.feature, nil }

func batteryFrame() []byte { b := make([]byte, 64); b[0] = 0x12; b[36] = 72; return b }
func eventFrame() []byte   { b := make([]byte, 64); b[0] = 0x10; b[1] = 0x06; return b }

func TestReadSkipsNonBatteryFrames(t *testing.T) {
	dev := &fakeDev{replies: [][]byte{eventFrame(), batteryFrame()}}
	st, err := Read(dev)
	if err != nil {
		t.Fatal(err)
	}
	if st.Percent != 72 {
		t.Fatalf("got %d%%, want 72%%", st.Percent)
	}
	if len(dev.written) != 1 {
		t.Fatalf("expected exactly one request write, got %d", len(dev.written))
	}
}

func TestReadTimesOut(t *testing.T) {
	dev := &fakeDev{}
	if _, err := Read(dev); err != hidraw.ErrTimeout {
		t.Fatalf("got %v, want ErrTimeout", err)
	}
}

func TestReadDS4(t *testing.T) {
	buf := make([]byte, 64)
	buf[0] = 0x12
	buf[10] = 72
	dev := &fakeDev{feature: buf}
	st, err := ReadDS4(dev)
	if err != nil {
		t.Fatal(err)
	}
	if st.Percent != 72 {
		t.Fatalf("got %d%%, want 72%%", st.Percent)
	}
}
