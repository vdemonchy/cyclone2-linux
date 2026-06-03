package uevent

import "testing"

func nullJoin(parts ...string) []byte {
	var b []byte
	for _, p := range parts {
		b = append(b, []byte(p)...)
		b = append(b, 0)
	}
	return b
}

func TestParseKernelFormat(t *testing.T) {
	buf := append([]byte("add@/devices/pci/usb5/5-1\x00"),
		nullJoin("ACTION=add", "SUBSYSTEM=usb", "PRODUCT=3537/100b/121")...)
	ev := ParseMessage(buf)
	if ev.Action != "add" || ev.Product != "3537/100b/121" {
		t.Fatalf("got %+v", ev)
	}
	if !ev.Matches("3537/100b") {
		t.Fatalf("expected match for 3537/100b")
	}
}

func TestParseLibudevFormat(t *testing.T) {
	buf := append([]byte("libudev\x00\xfe\xed\xca\xfe........"),
		nullJoin("ACTION=remove", "PRODUCT=3537/100b/121")...)
	ev := ParseMessage(buf)
	if ev.Action != "remove" || !ev.Matches("3537/100b") {
		t.Fatalf("got %+v", ev)
	}
}

func TestNonMatchingProduct(t *testing.T) {
	buf := nullJoin("ACTION=add", "PRODUCT=046d/c548/100")
	ev := ParseMessage(buf)
	if ev.Matches("3537/100b") {
		t.Fatalf("should not match a different device")
	}
}

func TestOpenClose(t *testing.T) {
	m, err := Open()
	if err != nil {
		t.Skipf("netlink unavailable in this environment: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
