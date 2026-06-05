# cyclone2-battery

Battery level + connection mode for the **GameSir Cyclone 2** on Linux, shown as
a GNOME top-bar indicator.

The Cyclone 2 can present as several different USB controllers. A dependency-free
Go daemon detects the current mode, reads the battery from the right source, and
writes a small JSON state file; a GNOME Shell extension displays it.

## Supported modes

| Mode | USB id | Battery |
|---|---|---|
| **XInput** (Xbox 360) | `3537:100b` | yes — read from the controller's vendor HID interface (XInput has no battery field, so this is reverse-engineered: byte 36 of report `0x12`) |
| **DS4** (PlayStation) | `054c:09cc` | yes — vendor HID feature report `0x12` (the kernel's `power_supply` value is wrong for this dongle) |
| **Switch** (Nintendo) | `057e:2009` | yes — kernel `hid-nintendo` `power_supply` (coarse level: Full/High/…) |
| **HID** | `3537:0575` | no battery source — indicator hidden (this is also what the dongle reports on its own when the controller is powered off) |

The Cyclone 2's four input modes are XInput, DS4, NS (Switch), and HID. The
indicator shows **only** when a battery-readable controller is connected
(XInput/DS4/Switch); in HID mode, or when the controller is off (dongle idle), or
fully disconnected, the indicator is hidden. Mode changes update it automatically
(instant, via udev hotplug events). In Switch mode GNOME also shows the battery
natively (accurate).

**Known limitation (DS4 mode):** the kernel `hid-playstation` driver derives a
bogus `power_supply` capacity (~5% always) because the dongle doesn't populate
the standard DS4 battery byte. GNOME may therefore show a spurious "controller
battery low" popup in DS4 mode. This can't be suppressed via configuration —
UPower 1.90+ removed `UPOWER_IGNORE` and offers no per-device ignore, so the only
fixes are patching `upowerd` (overwritten on updates) or intercepting the GNOME
notification. cyclone2-battery's own indicator shows the **correct** DS4 level
(read from the vendor HID feature report), so the popup is cosmetic.

**System power settings (UPower):** the indicator (applet/extension) reports the
battery in **all** battery-readable modes. The desktop's *system* power panel,
however, only sees what the kernel exposes as a `power_supply` device through
UPower — and that is **only Switch mode** (`hid-nintendo`, accurate but coarse)
plus DS4 (the bogus ~5% above). XInput battery is reverse-engineered from the
vendor HID interface with no kernel `power_supply`, so it never appears in the
system power panel. UPower has no userspace API to publish a custom battery, so
surfacing the daemon's correct values there would require a dedicated kernel
driver — intentionally out of scope. Use the indicator for accurate per-mode
battery; the system power panel is only reliable in Switch mode.

## Requirements

- Go 1.24+ to build. For the indicator: GNOME Shell 49 (extension) **or** COSMIC
  with Rust stable ≥ 1.93 (applet — see [COSMIC (CachyOS)](#cosmic-cachyos)).

## Install

```bash
bash install.sh
```

This builds `~/.local/bin/cyclone2`, installs a udev rule (needs `sudo`, for
root-free access to the XInput-mode HID node), and enables the `cyclone2-battery`
systemd `--user` service. Then load the indicator:

```bash
# Wayland: log out and back in (a full shell reload is required), then:
gnome-extensions enable cyclone2-battery@victor.local
```

## COSMIC (CachyOS)

On the COSMIC desktop the GNOME extension does not apply; a native libcosmic
applet provides the same indicator. The daemon, udev rule, and systemd service
are identical — only the frontend differs. `install.sh` auto-detects COSMIC (via
`XDG_CURRENT_DESKTOP`) and builds/installs the applet instead of the extension.
To force it (e.g. when running outside a graphical session):

```bash
CYCLONE2_FRONTEND=cosmic bash install.sh
```

This builds `cyclone2-applet` (needs **Rust stable ≥ 1.93** + libcosmic build
deps), installs it to `~/.local/bin`, and drops a `.desktop` entry into
`~/.local/share/applications`. Then add **Cyclone 2 Battery** to your panel:
*Settings → Desktop → Panel (or Dock) → Configure applets*.

Settings (poll interval, display mode) live in the applet's
popup and persist via `cosmic-config`; the poll interval is also written to
`~/.config/cyclone2-battery/config.json`, which the daemon reads live.

If *Cyclone 2 Battery* doesn't appear in the applet configurator right away, run
`update-desktop-database ~/.local/share/applications` and/or log out and back in
so COSMIC rescans the desktop entries.

### Manual test checklist (on COSMIC hardware)

1. `cd cosmic-applet && cargo build && ./target/debug/cyclone2-applet` — runs
   standalone for dev (a small window).
2. With a controller connected in XInput/DS4/Switch mode, the panel shows the
   controller icon tinted by battery level + the level; the popup shows the
   correct Mode and Battery.
3. Power the controller off or switch to HID mode: the indicator hides (no
   readable battery).
4. Hand-edit `$XDG_RUNTIME_DIR/cyclone2-battery.json` (e.g. flip `percent`) and
   confirm the panel updates within a second.
5. Change the poll interval in the popup; confirm
   `~/.config/cyclone2-battery/config.json` updates and the daemon honors it.

## Usage

- `cyclone2 status` — print mode + battery once (`72% (xinput)`); `--json` for machine output.
- `cyclone2 daemon` — the poll loop (normally run by the systemd user service).

## The indicator

- **Top bar:** a game-controller icon tinted by battery level — green (high,
  ≥60%) / yellow (medium, 25–59%) / red (low, <25%) — plus the level text
  (`NN%`, or the coarse level like `Full` in Switch mode) when *Icon + text* is
  selected. The icon falls back to the default foreground colour when the level
  is unknown (stale reading).
- **Hover:** shows the controller name (`GameSir Cyclone 2`).
- **Click:** a dropdown menu with the current **Mode** and **Battery** details.

## Configuration

Open the GNOME **Extensions** app → *Cyclone 2 Battery*:

- **Battery poll interval** — `10s / 30s / 1 min / 5 min` (default 1 min). The
  extension writes it to `~/.config/cyclone2-battery/config.json`, which the
  daemon reads live (no restart). CLI override precedence: `--interval` flag >
  `CYCLONE2_INTERVAL` env > config file > 60s default (5s minimum).
- **Top-bar display** — *Icon only* / *Icon + text* / *Text only*.

## How it works

```
controller (USB: 3537:100b | 054c:09cc | 057e:2009 | 3537:0575)
   │  cyclone2 daemon  (device.Find → mode)
   │    • XInput: raw HID read (no cgo)
   │    • DS4: vendor HID feature report; Switch: kernel power_supply sysfs
   │    • + udev netlink for instant connect/disconnect
   ▼
$XDG_RUNTIME_DIR/cyclone2-battery.json
   {"present":true,"mode":"ds4","percent":72,"charging":false,"battery_known":true,...}
   │  Gio.FileMonitor
   ▼
GNOME Shell extension → top-bar indicator + menu
```

## Protocol / discovery

See [`docs/protocol.md`](docs/protocol.md): the reverse-engineered XInput vendor
HID battery (report `0x0F` request → report `0x12` byte 36), and the DS4/Switch
`power_supply` sysfs layout. The XInput charging-flag byte is tentative; the
percentage is confirmed.

## Design docs

- Specs: [`docs/superpowers/specs/`](docs/superpowers/specs/) (v1 battery, v2 UX, v3 multi-mode)
- Plans: [`docs/superpowers/plans/`](docs/superpowers/plans/)
