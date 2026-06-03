#!/usr/bin/env bash
set -euo pipefail

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

EXT_SRC="extension/cyclone2-battery@victor.local"
EXT_DST="$HOME/.local/share/gnome-shell/extensions/cyclone2-battery@victor.local"
mkdir -p "$EXT_DST"
cp -r "$EXT_SRC/." "$EXT_DST/"
glib-compile-schemas "$EXT_DST/schemas"
echo "extension installed; log out/in, then: gnome-extensions enable cyclone2-battery@victor.local"
