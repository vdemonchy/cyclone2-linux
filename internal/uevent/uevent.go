// Package uevent listens for udev hotplug events over a netlink socket so the
// daemon can react instantly to the controller connecting/disconnecting.
package uevent

import (
	"bytes"
	"strings"

	"golang.org/x/sys/unix"
)

// Event is a parsed hotplug event.
type Event struct {
	Action  string // "add", "remove", "change", ...
	Product string // USB PRODUCT field, e.g. "3537/100b/121"
}

// Matches reports whether the event concerns the given "vendor/product" (hex,
// lower-case, no leading zeros — the kernel's PRODUCT format is
// "vendor/product/bcdDevice").
func (e Event) Matches(vendorProduct string) bool {
	return e.Product == vendorProduct || strings.HasPrefix(e.Product, vendorProduct+"/")
}

// ParseMessage extracts ACTION and PRODUCT from a netlink uevent message. It
// scans for null-separated KEY=value tokens anywhere in buf, so it works for
// both the kernel message format and the libudev-framed udev-group format.
// Within each token, it searches for "KEY=value" as a substring to tolerate
// any leading framing bytes (e.g. the libudev header magic).
func ParseMessage(buf []byte) Event {
	var ev Event
	for _, tok := range bytes.Split(buf, []byte{0}) {
		s := string(tok)
		if i := strings.Index(s, "ACTION="); i >= 0 {
			ev.Action = s[i+len("ACTION="):]
		} else if i := strings.Index(s, "PRODUCT="); i >= 0 {
			ev.Product = s[i+len("PRODUCT="):]
		}
	}
	return ev
}

// Monitor is a netlink uevent socket.
type Monitor struct{ fd int }

// Open binds a NETLINK_KOBJECT_UEVENT socket to the udev multicast group
// (group 2), which unprivileged processes may join. Events on this group are
// the ones systemd-udevd rebroadcasts after processing.
func Open() (*Monitor, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, unix.NETLINK_KOBJECT_UEVENT)
	if err != nil {
		return nil, err
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: 2}); err != nil {
		unix.Close(fd)
		return nil, err
	}
	return &Monitor{fd: fd}, nil
}

// Read blocks for the next event.
func (m *Monitor) Read() (Event, error) {
	buf := make([]byte, 8192)
	n, err := unix.Read(m.fd, buf)
	if err != nil {
		return Event{}, err
	}
	return ParseMessage(buf[:n]), nil
}

func (m *Monitor) Close() error { return unix.Close(m.fd) }
