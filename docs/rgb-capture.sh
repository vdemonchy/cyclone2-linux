#!/usr/bin/env bash
# Capture USB traffic to/from the Cyclone 2 while you change RGB in GameSir
# Connect (running under WinBoat). Run with sudo; it loads usbmon, finds the
# controller on the bus, and dumps every URB to a file. Ctrl-C when done.
#
#   sudo bash docs/rgb-capture.sh
#
# Then changing the LED colour/effect in the Windows app produces the bytes we
# need. The output file is docs/rgb-capture.txt (parsed afterwards).
set -euo pipefail

OUT="$(dirname "$0")/rgb-capture.txt"

# 1. Load usbmon and make sure debugfs is mounted.
modprobe usbmon
if [ ! -d /sys/kernel/debug/usb/usbmon ]; then
  mount -t debugfs none /sys/kernel/debug 2>/dev/null || true
fi

# 2. Locate the Cyclone 2 (any of its USB ids) -> bus + device number.
line="$(lsusb | grep -iE '3537:100b|054c:09cc|057e:2009|3537:0575' | head -n1 || true)"
if [ -z "$line" ]; then
  echo "Controller not found on USB. Is it connected / passed through to WinBoat?" >&2
  exit 1
fi
bus=$(echo "$line"  | sed -E 's/Bus 0*([0-9]+) Device.*/\1/')
dev=$(echo "$line"  | sed -E 's/.*Device 0*([0-9]+):.*/\1/')
devpad=$(printf '%03d' "$dev")
echo "Capturing bus $bus device $devpad  ($line)"
echo "Output -> $OUT"
echo
echo ">>> Now switch to WinBoat / GameSir Connect and change the RGB colour,"
echo ">>> brightness and effect a few times. Press Ctrl-C here when finished."
echo

mon="/sys/kernel/debug/usb/usbmon/${bus}u"
# Keep only lines for this device; tee so you can watch it live.
grep --line-buffered ":${bus}:${devpad}:" "$mon" | tee "$OUT"
