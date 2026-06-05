#!/usr/bin/env bash
set -euo pipefail

# ---- shared: daemon + udev + systemd (DE-independent) ----
go build -o "$HOME/.local/bin/cyclone2" .
echo "installed $HOME/.local/bin/cyclone2"

sudo install -m0644 packaging/udev/60-gamesir-cyclone2.rules /etc/udev/rules.d/60-gamesir-cyclone2.rules
sudo udevadm control --reload-rules && sudo udevadm trigger --subsystem-match=hidraw
echo "udev rule installed"

mkdir -p "$HOME/.config/systemd/user"
install -m0644 packaging/systemd/cyclone2-battery.service "$HOME/.config/systemd/user/cyclone2-battery.service"
systemctl --user daemon-reload
systemctl --user enable --now cyclone2-battery.service
echo "service enabled; state file: ${XDG_RUNTIME_DIR}/cyclone2-battery.json"

# ---- frontend: pick by desktop environment ----
# Override with CYCLONE2_FRONTEND=cosmic|gnome (useful outside a graphical session).
frontend="${CYCLONE2_FRONTEND:-}"
if [ -z "$frontend" ]; then
  case "${XDG_CURRENT_DESKTOP:-}" in
    *COSMIC*) frontend=cosmic ;;
    *GNOME*)  frontend=gnome ;;
    *)        frontend=gnome ;;
  esac
fi
echo "frontend: $frontend"

if [ "$frontend" = "cosmic" ]; then
  ( cd cosmic-applet && cargo build --release )
  install -m0755 cosmic-applet/target/release/cyclone2-applet "$HOME/.local/bin/cyclone2-applet"
  mkdir -p "$HOME/.local/share/applications"
  install -m0644 cosmic-applet/data/io.github.vdemonchy.Cyclone2Battery.desktop \
    "$HOME/.local/share/applications/io.github.vdemonchy.Cyclone2Battery.desktop"
  echo "COSMIC applet installed."
  echo "Add it via: Settings → Desktop → Panel (or Dock) → Configure applets → add 'Cyclone 2 Battery'."
else
  EXT_SRC="extension/cyclone2-battery@vdemonchy.github.io"
  EXT_DST="$HOME/.local/share/gnome-shell/extensions/cyclone2-battery@vdemonchy.github.io"
  mkdir -p "$EXT_DST"
  cp -r "$EXT_SRC/." "$EXT_DST/"
  glib-compile-schemas "$EXT_DST/schemas"
  echo "extension installed; log out/in, then: gnome-extensions enable cyclone2-battery@vdemonchy.github.io"
fi
