# Installing cyclone2-linux

The easiest install is one command: `make install` builds the daemon, installs
the core (udev rule + systemd user service), **detects your desktop**, and
installs the matching frontend — the GNOME Shell extension, the COSMIC applet, or
the KDE Plasma plasmoid.

Every working setup is the same shape: the **core** (daemon + udev rule + systemd
user service, identical on every desktop) plus **one frontend**. `make install`
wires up both for you; the two frontends stay independent under the hood
(`install-gnome` never touches COSMIC and vice-versa).

> See the [disclaimer](README.md#disclaimer): this is an unofficial project, not
> affiliated with GameSir. Use at your own risk.

## Prerequisites

- **Go 1.24+** — to build the daemon (always required).
- **GNOME Shell 49** *or* **COSMIC** *or* **KDE Plasma 6** — your desktop's
  frontend.
- For the COSMIC applet only: **Rust stable ≥ 1.93** plus the libcosmic build
  dependencies (see [CONTRIBUTING.md](CONTRIBUTING.md)).
- For the KDE plasmoid only: **Plasma 6** (Qt6/KF6); no extra build toolchain.

No toolchain? Skip to [Installing without a toolchain](#installing-without-a-toolchain).

## Quick install

```bash
git clone https://github.com/vdemonchy/cyclone2-linux.git
cd cyclone2-linux
make install            # core + the frontend for your desktop (sudo for the udev rule)
```

`make install` reads `$XDG_CURRENT_DESKTOP`:

- **GNOME** → installs the core and the GNOME Shell extension.
- **COSMIC** → installs the core and builds + installs the COSMIC applet (needs
  Rust; if it's missing, the core still installs and you'll be told how to finish).
- **KDE** → installs the core and the KDE Plasma 6 plasmoid.
- **anything else / headless** → installs the core and prints the commands so
  you can pick a frontend manually.

Force the choice instead of auto-detecting:

```bash
make install FRONTEND=gnome     # force the GNOME extension
make install FRONTEND=cosmic    # force the COSMIC applet
make install FRONTEND=kde        # force the KDE Plasma plasmoid
make install FRONTEND=none      # core only, no frontend
```

Run `make help` for every target (build, test, per-component install/uninstall,
clean). Override install paths with `PREFIX=...`, `BINDIR=...`, etc.

## Finish the frontend

The daemon and service start immediately, but each frontend needs one manual
step to show up — the desktop can't load it for you.

### GNOME

**Log out and back in** — on Wayland a full shell reload is required before a
freshly installed extension can be enabled. Then:

```bash
gnome-extensions enable cyclone2-linux@vdemonchy.github.io
```

Configure it from the **Extensions** app → *Cyclone 2* (poll interval, display
mode, low-battery threshold, battery colors, RGB lighting).

### COSMIC

Add the applet to your panel: **Settings → Desktop → Panel (or Dock) → Configure
applets → add "Cyclone 2"**. If it doesn't appear right away, run
`update-desktop-database ~/.local/share/applications` and/or log out and back in
so COSMIC rescans the desktop entries.

Configure it from the applet popup (poll interval, display mode, low-battery
alert, battery colors, RGB lighting); settings persist via `cosmic-config` and
the poll interval is mirrored to `~/.config/cyclone2-linux/config.json`.

### KDE Plasma

Add the plasmoid to a panel: **right-click the panel → Add Widgets → Cyclone 2**.
If *Cyclone 2* doesn't appear right away, log out and back in so Plasma rescans
installed widgets.

Configure it from the widget's **Configure Cyclone 2…** dialog (poll interval,
display mode, low-battery alert, battery colors, RGB lighting); settings persist
via KConfig and the daemon-relevant subset is mirrored to
`~/.config/cyclone2-linux/config.json`.

## Verifying the install

1. Connect the controller in **XInput, DS4, or Switch** mode (the indicator is
   hidden in HID mode or when the controller is off — that's expected).
2. The top-bar icon should appear, tinted by battery level.
3. `cyclone2 status` prints the mode + battery from the command line (e.g.
   `72% (xinput)`).
4. The popup/menu shows the current **Mode** and **Battery**.

Check the daemon is running: `systemctl --user status cyclone2-linux.service`.
If nothing shows up, see [Troubleshooting](#troubleshooting).

## Uninstalling

```bash
make uninstall          # remove the core (daemon + service + udev rule)
make uninstall-gnome    # remove the GNOME extension
make uninstall-cosmic   # remove the COSMIC applet
make uninstall-kde      # remove the KDE Plasma plasmoid
```

(`make uninstall` leaves the frontends in place; remove them with the
frontend-specific targets.)

---

## Installing without a toolchain

If you can't (or don't want to) build from source, every [GitHub
Release](https://github.com/vdemonchy/cyclone2-linux/releases) attaches pre-built
**x86_64 Linux** artefacts. On another architecture, build from source with
`make install` above.

The fastest path is the install script, which downloads the latest release,
installs the core, detects your desktop, and installs the matching frontend
(`sudo` is prompted once, for the udev rule):

```bash
curl -fsSL https://raw.githubusercontent.com/vdemonchy/cyclone2-linux/main/scripts/install.sh | sh
```

Pin a release with `VERSION=vX.Y.Z` and/or force a frontend with
`FRONTEND=gnome|cosmic|kde|none` (environment variables). The rest of this
section installs the same artefacts by hand.

Each release attaches four files (`<tag>` is the version, e.g. `v1.0.0`):

| Artefact | What it is | For |
|---|---|---|
| `cyclone2-<tag>-x86_64-linux` | the daemon binary (Go, static) | **core** — every desktop |
| `cyclone2-linux@vdemonchy.github.io.shell-extension.zip` | the GNOME Shell extension | GNOME frontend |
| `cyclone2-applet-<tag>-x86_64-linux.tar.gz` | the COSMIC applet + a bundled daemon copy + `INSTALL.txt` | COSMIC frontend |
| `cyclone2-plasmoid-<tag>.plasmoid` | the KDE Plasma 6 plasmoid (kpackage zip) | KDE frontend |

Set the version once so the commands copy/paste cleanly:

```bash
VERSION=v1.0.0   # the release tag you're installing
```

### Core (every desktop)

The core is the daemon binary, a udev rule (for root-free access to the
controller's HID node in XInput mode), and a systemd `--user` service.

```bash
# 1a. daemon binary
mkdir -p ~/.local/bin
curl -L -o ~/.local/bin/cyclone2 \
  "https://github.com/vdemonchy/cyclone2-linux/releases/download/${VERSION}/cyclone2-${VERSION}-x86_64-linux"
chmod +x ~/.local/bin/cyclone2

# 1b. udev rule (needs sudo)
curl -L -o /tmp/60-gamesir-cyclone2.rules \
  "https://raw.githubusercontent.com/vdemonchy/cyclone2-linux/${VERSION}/packaging/udev/60-gamesir-cyclone2.rules"
sudo install -m0644 /tmp/60-gamesir-cyclone2.rules /etc/udev/rules.d/60-gamesir-cyclone2.rules
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hidraw

# 1c. systemd --user service
curl -L -o /tmp/cyclone2-linux.service \
  "https://raw.githubusercontent.com/vdemonchy/cyclone2-linux/${VERSION}/packaging/systemd/cyclone2-linux.service"
mkdir -p ~/.config/systemd/user
install -m0644 /tmp/cyclone2-linux.service ~/.config/systemd/user/cyclone2-linux.service
systemctl --user daemon-reload
systemctl --user enable --now cyclone2-linux.service
```

Make sure `~/.local/bin` is on your `PATH` (most distros add it automatically).

> **Arch / CachyOS note:** the udev rule uses `TAG+="uaccess"` only (no `plugdev`
> group), which is the portable approach. Don't add `GROUP="plugdev"` — that
> group doesn't exist on Arch and breaks the whole rule.

### GNOME frontend (release zip)

Requires **GNOME Shell 49**.

```bash
curl -L -o /tmp/cyclone2-ext.zip \
  "https://github.com/vdemonchy/cyclone2-linux/releases/download/${VERSION}/cyclone2-linux@vdemonchy.github.io.shell-extension.zip"
gnome-extensions install --force /tmp/cyclone2-ext.zip
```

Then [finish the frontend](#gnome) — log out/in, then `gnome-extensions enable`.

### COSMIC frontend (release tarball)

The tarball bundles the applet, the desktop entry, a copy of the daemon, and an
`INSTALL.txt`. If you already did the core above, you only need the applet and
desktop entry:

```bash
curl -L -o /tmp/cyclone2-applet.tar.gz \
  "https://github.com/vdemonchy/cyclone2-linux/releases/download/${VERSION}/cyclone2-applet-${VERSION}-x86_64-linux.tar.gz"
tar -xzf /tmp/cyclone2-applet.tar.gz -C /tmp
stage="/tmp/cyclone2-applet-${VERSION}-x86_64-linux"

install -m0755 "$stage/cyclone2-applet" ~/.local/bin/cyclone2-applet
mkdir -p ~/.local/share/applications
install -m0644 "$stage/io.github.vdemonchy.Cyclone2Linux.desktop" \
  ~/.local/share/applications/
update-desktop-database ~/.local/share/applications 2>/dev/null || true
```

> The tarball also contains a `cyclone2` daemon binary identical to the core
> step's — install it from there (`install -m0755 "$stage/cyclone2"
> ~/.local/bin/`) if you prefer over downloading the standalone daemon.

Then [finish the frontend](#cosmic) — add *Cyclone 2* to your panel.

### KDE Plasma frontend (release .plasmoid)

The `.plasmoid` artefact is a kpackage zip that `kpackagetool6` installs
directly:

```bash
curl -L -o /tmp/cyclone2.plasmoid \
  "https://github.com/vdemonchy/cyclone2-linux/releases/download/${VERSION}/cyclone2-plasmoid-${VERSION}.plasmoid"
kpackagetool6 --type Plasma/Applet --upgrade /tmp/cyclone2.plasmoid \
  || kpackagetool6 --type Plasma/Applet --install /tmp/cyclone2.plasmoid
```

The plasmoid is plain QML, so from a clone it also installs straight from the
repo — no artefact needed:

```bash
kpackagetool6 --type Plasma/Applet --upgrade plasmoid/package
# or, without kpackagetool6:
mkdir -p ~/.local/share/plasma/plasmoids/io.github.vdemonchy.cyclone2
cp -r plasmoid/package/. ~/.local/share/plasma/plasmoids/io.github.vdemonchy.cyclone2/
```

Then add **Cyclone 2** to a panel (right-click → Add Widgets). Remove with:

```bash
kpackagetool6 --type Plasma/Applet --remove io.github.vdemonchy.cyclone2
# or: rm -rf ~/.local/share/plasma/plasmoids/io.github.vdemonchy.cyclone2
```

To uninstall the hand-installed artefacts, remove the same files
(`~/.local/bin/cyclone2*`, the systemd unit, the udev rule, and the GNOME/COSMIC
frontend files).

---

## Troubleshooting

- **Indicator never appears.** Confirm the controller mode is battery-readable
  (XInput/DS4/Switch — not HID) and the service is running:
  `systemctl --user status cyclone2-linux.service`. Check the state file exists:
  `cat "$XDG_RUNTIME_DIR/cyclone2-linux.json"`.
- **`cyclone2 status` works with `sudo` but not without.** The udev rule didn't
  apply. Re-run the udev step, then unplug/replug the dongle (or
  `sudo udevadm trigger --subsystem-match=hidraw`).
- **GNOME extension won't enable** right after install. You must log out and back
  in first (Wayland requires a full shell reload).
- **`make install` only installed the core on COSMIC.** Rust wasn't found — the
  applet builds from source. Install Rust (≥ 1.93) + libcosmic deps, then run
  `make install-cosmic`.
- **RGB controls are greyed out.** RGB works in **XInput mode only** — a hardware
  limitation. Switch the controller to XInput to use lighting.
- **A "controller battery low" popup appears in DS4 mode.** That's UPower reading
  the dongle's bogus constant ~5%; the indicator shows the real level. See the
  [README](README.md#supported-modes) for why it can't be suppressed.
