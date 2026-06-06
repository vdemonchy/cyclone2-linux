#!/usr/bin/env python3
"""Decode Cyclone 2 RGB OUT reports captured by rgb-capture.sh.

Reads a usbmon text dump, keeps host->device interrupt-OUT reports whose first
byte is 0x0f (the vendor command report), and prints them as a TLV view:
  0f 03 20 00 | <reg> <len> <data...>
"""
import re
import sys

path = sys.argv[1] if len(sys.argv) > 1 else "docs/rgb-capture.txt"

EFFECTS = {1: "?eff01", 2: "?eff02", 5: "?eff05", 6: "?eff06", 8: "?eff08"}


def decode(data):
    b = bytes.fromhex(data)
    if len(b) < 5 or b[0] != 0x0F:
        return None
    hdr = b[:4]                       # 0f 03 20 00
    reg, ln = b[4], b[5]
    payload = b[6:6 + ln]
    out = f"hdr={hdr.hex(' ')}  reg=0x{reg:02x} len={ln:<2d} "
    if reg == 0x01 and len(payload) >= 4:
        eff, c1, p2, c2 = payload[0], payload[1], payload[2], payload[3]
        colors = payload[4:]
        trips = [colors[i:i+3].hex() for i in range(0, len(colors) - len(colors) % 3, 3)]
        trips = [t for t in trips if t != "000000"] or ["(none)"]
        out += f"LIGHTING eff=0x{eff:02x} c1=0x{c1:02x} p2=0x{p2:02x} c2=0x{c2:02x} colors=[{' '.join(trips)}]"
    elif reg == 0x03:
        out += f"REG03(speed?)={payload[0] if payload else '?'}"
    elif reg == 0x04:
        out += f"BRIGHTNESS={payload[0] if payload else '?'}"
    elif reg == 0x75:
        out += f"REG75 data={payload.hex(' ')}"
    elif reg == 0x3b:
        out += f"REG3b data={payload.hex(' ')}"
    else:
        out += f"data={payload.hex(' ')}"
    return out


seen_order = []
for line in open(path):
    m = re.search(r" Io:\d+:\d+:\d+ .*? = ([0-9a-f ]+)$", line.strip())
    if not m:
        continue
    data = m.group(1).replace(" ", "")
    if not data.startswith("0f"):
        continue
    ts = line.split()[1]
    d = decode(data)
    if d:
        print(f"[{ts}] {d}")
