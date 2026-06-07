# Installing cyclone2-linux

There are two ways to install: from the **pre-built release artefacts** on GitHub
(no Go/Rust toolchain needed — recommended for most people) or **from source**
with the `Makefile`. This guide covers the release artefacts; for building from
source see [Building from source](#building-from-source) below and
[CONTRIBUTING.md](CONTRIBUTING.md).

Every working setup is the same shape: the **core** (daemon + udev rule + systemd
user service, identical on every desktop) plus **one frontend** — the GNOME Shell
extension *or* the COSMIC applet. The two frontends are independent; install only
the one for your desktop.

> See the [disclaimer](README.md#disclaimer): this is an unofficial project, not
> affiliated with GameSir. Use at your own risk.

## Release artefacts

Each [GitHub Release](https://github.com/vdemonchy/cyclone2-linux/releases)
attaches three files (`<tag>` is the version, e.g. `v1.0.0`):

| Artefact | What it is | For |
|---|---|---|
| `cyclone2-<tag>-x86_64-linux` | the daemon binary (Go, static) | **core** — both desktops |
| `cyclone2-linux@vdemonchy.github.io.shell-extension.zip` | the GNOME Shell extension | GNOME frontend |
| `cyclone2-applet-<tag>-x86_64-linux.tar.gz` | the COSMIC applet + a bundled daemon copy + `INSTALL.txt` | COSMIC frontend |

All artefacts are **x86_64 Linux**. On another architecture, build from source.

Set a variable for the version so the commands below copy/paste cleanly:

```bash
VERSION=v1.0.0   # the release tag you're installing
```

---

## Step 1 — Core (every desktop)

The core is the same regardless of frontend: the daemon binary, a udev rule (for
root-free access to the controller's HID node in XInput mode), and a systemd
`--user` service that runs the daemon.

### 1a. Install the daemon binary

```bash
mkdir -p ~/.local/bin
curl -L -o ~/.local/bin/cyclone2 \
  "https://github.com/vdemonchy/cyclone2-linux/releases/download/${VERSION}/cyclone2-${VERSION}-x86_64-linux"
chmod +x ~/.local/bin/cyclone2
```

Make sure `~/.local/bin` is on your `PATH` (most distros add it automatically).
Verify:

```bash
cyclone2 status   # prints e.g. "72% (xinput)" with a controller connected
```

### 1b. Install the udev rule

The rule grants the active desktop user access to the controller's vendor HID
node (needed for XInput-mode battery reads and RGB control). Download it from the
repo and install it (needs `sudo`):

```bash
curl -L -o /tmp/60-gamesir-cyclone2.rules \
  "https://raw.githubusercontent.com/vdemonchy/cyclone2-linux/${VERSION}/packaging/udev/60-gamesir-cyclone2.rules"
sudo install -m0644 /tmp/60-gamesir-cyclone2.rules /etc/udev/rules.d/60-gamesir-cyclone2.rules
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hidraw
```

> **Arch / CachyOS note:** the rule uses `TAG+="uaccess"` only (no `plugdev`
> group), which is the portable approach. Don't add `GROUP="plugdev"` — that
> group doesn't exist on Arch and breaks the whole rule.

### 1c. Install and enable the systemd user service

```bash
curl -L -o /tmp/cyclone2-linux.service \
  "https://raw.githubusercontent.com/vdemonchy/cyclone2-linux/${VERSION}/packaging/systemd/cyclone2-linux.service"
mkdir -p ~/.config/systemd/user
install -m0644 /tmp/cyclone2-linux.service ~/.config/systemd/user/cyclone2-linux.service
systemctl --user daemon-reload
systemctl --user enable --now cyclone2-linux.service
```

The daemon now writes its state file to `$XDG_RUNTIME_DIR/cyclone2-linux.json`.
Check it's running:

```bash
systemctl --user status cyclone2-linux.service
```

Now install **one** frontend below.

---

## Step 2a — GNOME frontend (GNOME Shell only)

Requires **GNOME Shell 49**. Download and install the extension zip:

```bash
curl -L -o /tmp/cyclone2-ext.zip \
  "https://github.com/vdemonchy/cyclone2-linux/releases/download/${VERSION}/cyclone2-linux@vdemonchy.github.io.shell-extension.zip"
gnome-extensions install --force /tmp/cyclone2-ext.zip
```

Then **log out and back in** — on Wayland a full shell reload is required before
a freshly installed extension can be enabled. After logging back in:

```bash
gnome-extensions enable cyclone2-linux@vdemonchy.github.io
```

Configure it from the **Extensions** app → *Cyclone 2* (poll interval, display
mode, low-battery threshold, battery colors, RGB lighting).

To remove it later:

```bash
gnome-extensions disable cyclone2-linux@vdemonchy.github.io
gnome-extensions uninstall cyclone2-linux@vdemonchy.github.io
```

---

## Step 2b — COSMIC frontend (COSMIC desktop only)

The COSMIC tarball bundles the applet, the desktop entry, and a copy of the
daemon (`cyclone2`) plus an `INSTALL.txt`. If you already did Step 1 you only
need the applet and desktop entry:

```bash
curl -L -o /tmp/cyclone2-applet.tar.gz \
  "https://github.com/vdemonchy/cyclone2-linux/releases/download/${VERSION}/cyclone2-applet-${VERSION}-x86_64-linux.tar.gz"
tar -xzf /tmp/cyclone2-applet.tar.gz -C /tmp
stage="/tmp/cyclone2-applet-${VERSION}-x86_64-linux"

# applet binary + desktop entry
install -m0755 "$stage/cyclone2-applet" ~/.local/bin/cyclone2-applet
mkdir -p ~/.local/share/applications
install -m0644 "$stage/io.github.vdemonchy.Cyclone2Linux.desktop" \
  ~/.local/share/applications/
update-desktop-database ~/.local/share/applications 2>/dev/null || true
```

> The tarball also contains a `cyclone2` daemon binary, identical to Step 1a — if
> you prefer, install it from there (`install -m0755 "$stage/cyclone2"
> ~/.local/bin/`) instead of downloading the standalone daemon.

Then add the applet to your panel: **Settings → Desktop → Panel (or Dock) →
Configure applets → add "Cyclone 2"**. If it doesn't appear right away, run
`update-desktop-database ~/.local/share/applications` and/or log out and back in
so COSMIC rescans the desktop entries.

Configure it from the applet popup (poll interval, display mode, low-battery
alert, battery colors, RGB lighting); settings persist via `cosmic-config` and
the poll interval is mirrored to `~/.config/cyclone2-linux/config.json`.

To remove it later:

```bash
rm -f ~/.local/bin/cyclone2-applet
rm -f ~/.local/share/applications/io.github.vdemonchy.Cyclone2Linux.desktop
update-desktop-database ~/.local/share/applications 2>/dev/null || true
```

---

## Verifying the install

1. Connect the controller in **XInput, DS4, or Switch** mode (the indicator is
   hidden in HID mode or when the controller is off — that's expected).
2. The top-bar icon should appear, tinted by battery level.
3. `cyclone2 status` prints the mode + battery from the command line.
4. The popup/menu shows the current **Mode** and **Battery**.

If nothing shows up, see [Troubleshooting](#troubleshooting).

---

## Building from source

If you have the Go (and, for COSMIC, Rust) toolchains, the `Makefile` does
everything the steps above do, with the two frontends kept separate:

```bash
git clone https://github.com/vdemonchy/cyclone2-linux.git
cd cyclone2-linux

make install            # core: daemon + udev rule + systemd service
make install-gnome      # GNOME frontend   (only one of these)
make install-cosmic     # COSMIC frontend  (only one of these)
```

Run `make help` for all targets (build, test, per-component install/uninstall,
clean). Building the COSMIC applet needs **Rust stable ≥ 1.93** plus the
libcosmic build dependencies. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full
list and a development workflow.

---

## Troubleshooting

- **Indicator never appears.** Confirm the controller mode is battery-readable
  (XInput/DS4/Switch — not HID) and the service is running:
  `systemctl --user status cyclone2-linux.service`. Check the state file exists:
  `cat "$XDG_RUNTIME_DIR/cyclone2-linux.json"`.
- **`cyclone2 status` works with `sudo` but not without.** The udev rule didn't
  apply. Re-run Step 1b, then unplug/replug the dongle (or
  `sudo udevadm trigger --subsystem-match=hidraw`).
- **GNOME extension won't enable** right after install. You must log out and back
  in first (Wayland requires a full shell reload).
- **RGB controls are greyed out.** RGB works in **XInput mode only** — a hardware
  limitation. Switch the controller to XInput to use lighting.
- **A "controller battery low" popup appears in DS4 mode.** That's UPower reading
  the dongle's bogus constant ~5%; the indicator shows the real level. See the
  [README](README.md#supported-modes) for why it can't be suppressed.
