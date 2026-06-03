package hidraw

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ReadWriter is the minimal device surface the reader/daemon need, so tests can
// substitute a fake without real hardware.
type ReadWriter interface {
	Write(report []byte) error
	Read(timeout time.Duration) ([]byte, error)
	GetFeature(reportID byte, length int) ([]byte, error)
}

type Device struct{ f *os.File }

var _ ReadWriter = (*Device)(nil)

func Open(path string) (*Device, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &Device{f: f}, nil
}

func (d *Device) Close() error { return d.f.Close() }

// Write sends an output report. report[0] must be the report ID (e.g. 0x0F).
func (d *Device) Write(report []byte) error {
	_, err := d.f.Write(report)
	return err
}

// Read blocks up to timeout for one input report; report[0] is the report ID.
func (d *Device) Read(timeout time.Duration) ([]byte, error) {
	// NOTE: os.File.Fd() puts the file into blocking mode, removing it from
	// Go's runtime network poller. That's acceptable here because we always
	// unix.Poll for readiness before issuing the (blocking) Read below.
	fds := []unix.PollFd{{Fd: int32(d.f.Fd()), Events: unix.POLLIN}}
	// Ceiling division so any positive sub-millisecond timeout waits at least
	// 1ms rather than truncating to a non-blocking poll.
	ms := int((timeout + time.Millisecond - 1) / time.Millisecond)
	n, err := unix.Poll(fds, ms)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrTimeout
	}
	buf := make([]byte, 64)
	c, err := d.f.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:c], nil
}

// GetFeature issues HIDIOCGFEATURE(length); buf[0] is the requested report ID.
func (d *Device) GetFeature(reportID byte, length int) ([]byte, error) {
	if length < 1 {
		return nil, fmt.Errorf("hidraw: GetFeature length must be >= 1, got %d", length)
	}
	buf := make([]byte, length)
	buf[0] = reportID
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, d.f.Fd(),
		uintptr(hidiocgfeature(length)), uintptr(unsafe.Pointer(&buf[0])))
	if errno != 0 {
		return nil, errno
	}
	return buf, nil
}

// _IOC(dir,type,nr,size) for HIDIOCGFEATURE: dir = READ|WRITE, type='H', nr=0x07.
func hidiocgfeature(length int) uint {
	const (
		iocWrite    = 1
		iocRead     = 2
		nrShift     = 0
		typeShift   = 8
		sizeShift   = 16
		dirShift    = 30
		hidrawMagic = 'H'
	)
	return (uint(iocRead|iocWrite) << dirShift) |
		(uint(hidrawMagic) << typeShift) |
		(0x07 << nrShift) |
		(uint(length) << sizeShift)
}
