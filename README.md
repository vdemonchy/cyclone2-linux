# cyclone2-linux

Battery, connection mode, and RGB lighting control for the **GameSir Cyclone 2**
on Linux — as a top-bar indicator on GNOME (Shell extension) and COSMIC (native
applet).

The Cyclone 2 can present as several different USB controllers, each exposing the
battery differently. A dependency-free Go daemon detects the current mode, reads
the battery from the right source, drives the RGB lighting, and writes a small
JSON state file that the GNOME extension or COSMIC applet displays.

## Features

- **Battery level in every readable mode** — XInput, DS4, and Switch, each read
  from the source that's actually correct for it (two are reverse-engineered;
  see [Supported modes](#supported-modes)).
- **Automatic mode detection** with instant connect/disconnect/mode-change
  updates over udev hotplug events.
- **Charging detection** in all battery modes.
- **Top-bar indicator** — controller icon tinted by battery level (configurable
  green/yellow/red thresholds), optional level text, a pulse while charging, the
  controller name on hover, and a menu showing the current mode and battery.
- **Low-battery desktop notifications** from the daemon, with hysteresis so a
  battery near the threshold doesn't spam you. Accurate in every mode, including
  the ones the system power panel gets wrong.
- **RGB lighting control** — four addressable zones (Left/Right/Logo/Center) plus
  brightness, set from the UI or the CLI and re-applied on reconnect. XInput mode
  only (a hardware limitation — see [RGB lighting](#rgb-lighting)).
- **Two desktop frontends** — a GNOME Shell extension and a native COSMIC applet,
  sharing the same daemon.
- **CLI** for a one-shot battery read, the daemon, and lighting control.
- **Configurable poll interval** with live config reload (no restart).

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

**Indicator vs. system power panel.** This project's indicator (applet/extension)
shows the correct battery in every battery-readable mode, because the daemon reads
each mode at its real source. The desktop's *system* power panel is different: it
only sees devices the kernel exposes through UPower, which here means Switch mode
(`hid-nintendo`, accurate but coarse) and DS4. So for accurate per-mode battery,
use the indicator; the system power panel is only reliable in Switch mode.

UPower has no userspace API to publish a custom battery, so getting the daemon's
values into the system panel would need a dedicated kernel driver — out of scope.

**DS4 false low-battery popup.** In DS4 mode the dongle never populates the
standard DualShock battery byte, so `hid-playstation` reports a constant ~5%
(`0 × 10 + 5`). GNOME may pop up a "controller battery low" warning as a result.
Nothing in the config can suppress it — UPower 1.90+ dropped `UPOWER_IGNORE` and
has no per-device ignore, leaving only a patched `upowerd` (overwritten on
updates) or intercepting the notification. The indicator shows the real DS4 level,
so the popup is just cosmetic.

## Requirements

- Go 1.24+ to build. For the indicator: GNOME Shell 49 (extension) **or** COSMIC
  with Rust stable ≥ 1.93 (applet — see [COSMIC (CachyOS)](#cosmic-cachyos)).

## Install

Every setup is the **core** (daemon + udev rule + systemd `--user` service,
identical on every desktop) plus **one frontend** — the GNOME extension *or* the
COSMIC applet. Pick the section for your desktop below.

There are two routes either way: **pre-built release artefacts** (no toolchain
needed) or **from source** with the `Makefile`. Full step-by-step instructions,
including the release-artefact commands, verification, and troubleshooting, are
in **[INSTALL.md](INSTALL.md)**; `make help` lists every target. The GNOME and
COSMIC installs are fully separated — `install-gnome` never touches COSMIC and
vice-versa.

### GNOME Shell

Requires **GNOME Shell 49**. Install the core and the GNOME frontend:

```bash
make install         # core: daemon + udev rule + systemd service (sudo for udev)
make install-gnome   # GNOME Shell extension
```

`make install-gnome` copies the extension into place and compiles its gschema.
Then load the indicator:

```bash
# Wayland: log out and back in (a full shell reload is required), then:
gnome-extensions enable cyclone2-linux@vdemonchy.github.io
```

Configure it from the **Extensions** app → *Cyclone 2* (poll interval, display
mode, low-battery threshold, battery colors, RGB lighting).

### COSMIC (CachyOS)

On the COSMIC desktop the GNOME extension does not apply; a native libcosmic
applet provides the same indicator (the daemon, udev rule, and systemd service
are identical — only the frontend differs). Install the core and the COSMIC
frontend:

```bash
make install         # core: daemon + udev rule + systemd service (sudo for udev)
make install-cosmic  # COSMIC applet
```

`make install-cosmic` builds `cyclone2-applet` (needs **Rust stable ≥ 1.93** +
libcosmic build deps), installs it to `~/.local/bin`, and drops a `.desktop`
entry into `~/.local/share/applications`. Then add **Cyclone 2** to your panel:
*Settings → Desktop → Panel (or Dock) → Configure applets*.

Settings (poll interval, display mode, low-battery alert, battery level colors) live in
the applet's popup and persist via `cosmic-config`; the poll interval is also written to
`~/.config/cyclone2-linux/config.json`, which the daemon reads live.

If *Cyclone 2* doesn't appear in the applet configurator right away, run
`update-desktop-database ~/.local/share/applications` and/or log out and back in
so COSMIC rescans the desktop entries.

#### Manual test checklist (on COSMIC hardware)

1. `cd cosmic-applet && cargo build && ./target/debug/cyclone2-applet` — runs
   standalone for dev (a small window).
2. With a controller connected in XInput/DS4/Switch mode, the panel shows the
   controller icon tinted by battery level + the level; the popup shows the
   correct Mode and Battery.
3. Power the controller off or switch to HID mode: the indicator hides (no
   readable battery).
4. Hand-edit `$XDG_RUNTIME_DIR/cyclone2-linux.json` (e.g. flip `percent`) and
   confirm the panel updates within a second.
5. Change the poll interval in the popup; confirm
   `~/.config/cyclone2-linux/config.json` updates and the daemon honors it.

## Usage

- `cyclone2 status` — print mode + battery once (`72% (xinput)`); `--json` for machine output.
- `cyclone2 daemon` — the poll loop (normally run by the systemd user service).
- `cyclone2 rgb …` — control the RGB lighting (see below).

## RGB lighting

The Cyclone 2 has four addressable RGB zones — **Left, Right, Logo, Center** —
plus a global brightness, normally only configurable via GameSir's Windows app.
cyclone2 drives them natively over the vendor HID interface (the lighting
protocol was reverse-engineered — see [`docs/protocol.md`](docs/protocol.md)).

**XInput mode only.** RGB control works *exclusively* in XInput mode (USB
`3537:100b`). In DS4 and Switch modes the controller masquerades as a Sony/
Nintendo device and **hides the vendor LED interface entirely** — so there is no
way to set the lighting in those modes. This is a hardware/firmware limitation,
not a cyclone2 one: GameSir's own software [requires XInput mode to connect](https://gamesir.com/pages/gamesir-connect-software)
as well. The applet/extension therefore **disable the lighting controls entirely
unless the controller is in XInput mode** (showing why instead); the daemon
applies the saved settings whenever an XInput controller is connected.

**From the UI** (recommended): in the COSMIC applet popup or the GNOME extension
preferences, enable **Control lighting**, then set per-zone colours and
brightness. Settings are written to `config.json`; the **daemon** applies them
and re-applies on reconnect, so they persist. Left off, the controller's lighting
is untouched.

**From the CLI:**
```
cyclone2 rgb color ff0000              # solid red on all zones
cyclone2 rgb zones ff0000 00ff00 0000ff ffffff   # Left Right Logo Center
cyclone2 rgb brightness 50             # 0–100
cyclone2 rgb off                       # lights off
```
The CLI writes directly to the controller; the daemon-managed `config.json`
settings are what survive restarts and reconnects.

## The indicator

- **Top bar:** a game-controller icon tinted by battery level — green (high) /
  yellow (medium) / red (low), with **configurable thresholds** (defaults: green
  ≥60%, yellow ≥25%, red below) — plus the level text (`NN%`, or the coarse level
  like `Full` in Switch mode) when *Icon + text* is selected. The icon **pulses
  while charging**, and falls back to the default foreground color when the
  level is unknown (stale reading).
- **Hover:** shows the controller name (`GameSir Cyclone 2`).
- **Click:** a dropdown menu with the current **Mode** and **Battery** — the
  battery line always shows the charge state (`— Charging` / `— On battery`).

Charging detection works in all battery modes: **DS4** and **Switch** read the
kernel `power_supply` cable-state, and **XInput** reads byte 35 of the vendor
`0x12` report — the charging/cable flag, confirmed by plug/unplug captures.

## Low-battery notifications

The **daemon** posts a desktop notification (via the freedesktop notification
service) when the battery first drops to or below a configurable threshold, then
stays quiet until the level recovers (with a small hysteresis margin) or the
controller charges/disconnects — so a battery hovering near the threshold doesn't
spam you. Because the daemon reads the **correct** per-mode value, this works
accurately in *all* battery modes — including XInput and DS4, which the system
power panel (UPower) can't report correctly.

Set the threshold in the applet popup (COSMIC) or extension preferences (GNOME)
with a numeric stepper — **0–50% in steps of 5** (default **20%**; **0
disables**). Requires a running notification daemon and `gdbus` (part of glib —
present on GNOME/COSMIC).

## Configuration

Open the GNOME **Extensions** app → *Cyclone 2* (COSMIC: the applet
popup):

- **Battery poll interval** — `10s / 30s / 1 min / 5 min` (default 1 min). The
  frontend writes it to `~/.config/cyclone2-linux/config.json`, which the
  daemon reads live (no restart). CLI override precedence: `--interval` flag >
  `CYCLONE2_INTERVAL` env > config file > 60s default (5s minimum).
- **Top-bar display** — *Icon only* / *Icon + text*.
- **Low battery alert** — percentage at or below which the daemon notifies,
  set with a 0–50% stepper (default 20%, `0` disables). Also written to
  `config.json`.
- **Battery level colors** — battery % thresholds for the icon: green at or
  above the high threshold, yellow at or above the low threshold, red below it
  (defaults: green ≥60%, yellow ≥25%). The green threshold can't be set at or
  below the yellow one.
- **Controller lighting** — opt-in **Control lighting** toggle, a brightness
  slider, and a colour picker per zone (Left / Right / Logo / Center). Written to
  `config.json` as an `rgb` block; the daemon applies it (XInput mode only) and
  re-applies on reconnect. Off by default, so battery-only setups are untouched.

## How it works

```
controller (USB: 3537:100b | 054c:09cc | 057e:2009 | 3537:0575)
   │  cyclone2 daemon  (device.Find → mode)
   │    • XInput: raw HID read (no cgo)
   │    • DS4: vendor HID feature report; Switch: kernel power_supply sysfs
   │    • + udev netlink for instant connect/disconnect
   ▼
$XDG_RUNTIME_DIR/cyclone2-linux.json
   {"present":true,"mode":"ds4","percent":72,"charging":false,"battery_known":true,...}
   │  Gio.FileMonitor
   ▼
GNOME Shell extension → top-bar indicator + menu
```

## Protocol / discovery

See [`docs/protocol.md`](docs/protocol.md): the reverse-engineered XInput vendor
HID battery (report `0x0F` request → report `0x12` byte 36), the DS4/Switch
`power_supply` sysfs layout, and the RGB lighting command protocol. The capture
and decode helpers used for the reverse-engineering live in `docs/rgb-capture.sh`
and `docs/rgb-decode.py`.

## Contributing

Bug reports, hardware captures, code, and docs are all welcome. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the project layout, development setup
(Go daemon, Rust COSMIC applet, GNOME extension), and the PR workflow.

## License

[GPL-3.0](LICENSE).

## Disclaimer

This is an **unofficial**, community-developed project. It is **not affiliated
with, endorsed by, or supported by GameSir** (Guangzhou Chicken Run Network
Technology Co., Ltd.) in any way. "GameSir" and "Cyclone 2" are trademarks of
their respective owners and are used here only to describe the hardware this
software interoperates with.

The battery and RGB lighting protocols were **reverse-engineered** for
interoperability on Linux; they are not documented or sanctioned by GameSir and
may stop working after any firmware update.

This software is provided **"as is", without warranty of any kind**, express or
implied (see the [LICENSE](LICENSE) for the full terms). Neither GameSir nor the
developer(s) of this project are responsible for any damage — to your controller,
your computer, your data, or anything else — that may result from using it.
**Use it at your own risk.**
