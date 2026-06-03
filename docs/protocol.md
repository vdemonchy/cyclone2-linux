# Cyclone 2 vendor HID — battery protocol

Device: USB `3537:100b` ("Xbox 360 Controller for Windows"), vendor HID
interface (interface 1, `/dev/hidrawN`). Reports: `0x0F` OUTPUT (command),
`0x10` + `0x12` INPUT (replies). Reverse-engineered 2026-06-03 against the
2.4GHz dongle.

## How to read battery

1. The interface emits **nothing** while idle (passive read for 5s → no frames).
   `GET_FEATURE` on `0x10`/`0x12` returns EPIPE (they are Input reports, not
   Feature reports).
2. Writing **any** OUTPUT report on `0x0F` wakes the interface, after which it
   streams the full controller state on report **`0x12`** (high rate) plus an
   occasional small event report on `0x10`.
3. Read `0x12` frames and take the battery byte.

### Battery request
Output report (64 bytes): `0F 03 00 00 ...` (report ID `0x0F`, opcode `0x03`,
rest zero). Opcodes `0x01`/`0x03` reliably trigger the `0x12` stream; `0x00`
elicits no reply.

### Battery reply
Input report **ID `0x12`**, 64 bytes. Battery is at a fixed offset:

- **byte[36] = battery percent** (raw 0–100; observed `0x64` = 100 at full charge).
- byte[37] = status/charging flag **(UNCONFIRMED)** — `0x00` in every observed
  state. Could not be confirmed because the pack was at 100% (plugged-full and
  on-battery-full read identically). Revisit when the battery is mid-charge.

Report **ID `0x10`** (`10 06 00…`) is a separate event report and does **NOT**
carry battery (its byte[36] is `0x00`). The parser must accept **only `0x12`**.

### Sample captured frames (byte[36] = 0x64)
```
plugged, full (opcode 0x03):
12808080800f00000000fd6500feff00001000e9ff44203f0000000000000000000000006400000000...
                                    offset:                              ^36=0x64=100  ^37=0x00
on battery, full (opcode 0x03):
12808080800f00000000ed0d00feff00000e00a5009b20f9fd00000000000000000000006400010118...
                                                                          ^36=0x64=100 ^37=0x00
```

## Cross-check
- Battery byte = `0x64` = 100% while the controller was plugged in and full —
  consistent. A definitive Windows GameSir-app cross-check at a *non-full* level
  is still desirable to confirm scaling at lower charge, but the 0–100 raw
  interpretation is high-confidence.

## Status
- **byte[36] = battery percent: CONFIRMED** (value matches full charge; sole
  in-range constant in the report; stable across plugged/unplugged).
- **byte[37] = charging flag: TENTATIVE** (not observable at full charge).
- Request = Output `0F 03 00…`; reply = Input report `0x12`.

---

# DS4 mode (PlayStation) — battery via vendor HID feature report

Discovered 2026-06-03. In DS4 mode the Cyclone 2 **enumerates as a genuine Sony
DualShock 4 v2**, not as a GameSir device:

- USB ID: **`054c:09cc`** (Sony Corp. DualShock 4 [CUH-ZCT2x]).
- `HID_ID=0003:0000054C:000009CC`, bound to the kernel **`playstation`** driver.

**The kernel `power_supply` `capacity` is WRONG for this dongle.** The GameSir DS4
emulation does not populate the *standard* DS4 battery byte (input-report offset
30 stays `0`), so `hid-playstation` derives a constant **~5%** regardless of real
charge (`0 × 10 + 5`). There is no `capacity_level` for DS4. (Cross-check: the
same physical battery reads `Full` via `hid-nintendo` in Switch mode.) So do NOT
use the DS4 `power_supply` `capacity` for the percentage.

**Real battery = vendor HID `GET_FEATURE` report `0x12`, byte 10 (percent 0–100).**
This is what the Windows GameSir app reads. Confirmed stable at `0x64` = 100 at
full charge. Example feature `0x12` (full):
```
12 f0050cf2418c 08 25 00 64 78705696 5c 00...
   └ 6-byte MAC ┘       ^^ byte[10] = 0x64 = 100% (battery)
```
- Read via `HIDIOCGFEATURE` on `/dev/hidrawN` of the `054c:09cc` device.
- Requires a udev rule granting access to `054c:09cc` (the DS4 node is otherwise
  root-only; the kernel `playstation` driver coexists with hidraw `GET_FEATURE`).
- **Charging:** the kernel `power_supply` `status` (`Charging`/`Discharging`/`Full`)
  is a separate cable-state signal and is reliable; use it for the charging flag.
- **Caveat:** byte-10 scaling confirmed only at full (=100). Re-confirm at low
  charge; if it turns out coarse/stepped, adjust the mapping.

# Switch mode (Nintendo) — battery via kernel power_supply (coarse)

Discovered 2026-06-03. In Switch mode the Cyclone 2 enumerates as a **Nintendo
Switch Pro Controller**:

- USB ID: **`057e:2009`** (Nintendo Co., Ltd Switch Pro Controller).
- `HID_ID=0003:0000057E:00002009`, `HID_NAME=Pro Controller`.
- Bound to the kernel **`nintendo`** (`hid-nintendo`) driver.

Battery is exposed as a `power_supply` device under the controller's hid device,
but the Pro Controller hardware reports only a **coarse level**, so there is
**no numeric `capacity`** — only `capacity_level`:

```
/sys/class/hidraw/hidrawN/device/power_supply/nintendo_switch_controller_battery_*/capacity_level
    # one of: Full | High | Normal | Low | Critical | Unknown   (observed: "Full")
.../status     # "Charging" | "Discharging" | ...
```

Map level → approximate percent for the icon while showing the level text in the
menu: `Full=100, High=80, Normal=55, Low=25, Critical=5, Unknown=-1`.

It is also already in UPower
(`/org/freedesktop/UPower/devices/battery_nintendo_switch_controller_battery_*`).

## Generic power_supply read (DS4 + Switch)

For any mode whose battery comes from the kernel, the source is found under the
matched controller's hid device: `/sys/class/hidraw/hidrawN/device/power_supply/*/`.
Read `capacity` if present (numeric percent); else read `capacity_level` (coarse
→ approximate percent + label). `status` ⇒ charging (`Charging`/`Full`).

## Mode → battery source summary
| USB id | Mode | Battery source |
|---|---|---|
| `3537:100b` | xinput | vendor HID report `0x12` byte 36 (this doc, top) |
| `054c:09cc` | ds4 | vendor HID `GET_FEATURE 0x12` byte 10 (kernel `power_supply` capacity is bogus); charging from `power_supply` `status` |
| `057e:2009` | switch | kernel `power_supply` (coarse `capacity_level`) |
| `3537:0575` | hid | none — indicator hidden (HID mode; also the dongle's id when the controller is off) |
