#!/bin/sh
# install.sh — install cyclone2-linux from the pre-built GitHub release artefacts.
#
# No build toolchain needed: downloads the latest release (or VERSION=vX.Y.Z),
# installs the core (daemon + udev rule + systemd --user service), detects your
# desktop from $XDG_CURRENT_DESKTOP, and installs the matching frontend
# (GNOME extension, COSMIC applet, or KDE Plasma plasmoid).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/vdemonchy/cyclone2-linux/main/scripts/install.sh | sh
#
# Environment overrides:
#   VERSION=v1.2.0    install a specific release tag (default: latest)
#   FRONTEND=gnome    force the frontend: gnome | cosmic | kde | none
#
# The udev rule is the only step that needs root (sudo); everything else goes
# to your user prefix (~/.local, ~/.config). Mirrors `make install` from a clone.
set -eu

REPO="vdemonchy/cyclone2-linux"
EXT_UUID="cyclone2-linux@vdemonchy.github.io"
PLASMOID_ID="io.github.vdemonchy.cyclone2"

VERSION="${VERSION:-}"
FRONTEND="${FRONTEND:-}"

BINDIR="$HOME/.local/bin"
DESKTOPDIR="$HOME/.local/share/applications"
SYSTEMD_USER="$HOME/.config/systemd/user"
UDEV_RULE="60-gamesir-cyclone2.rules"
SERVICE="cyclone2-linux.service"

info() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mwarning:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || die "curl is required"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM

fetch() { # fetch <url> <dest>
    curl -fsSL --retry 3 -o "$2" "$1" || die "download failed: $1"
}

# ---- resolve the release tag ------------------------------------------------
if [ -z "$VERSION" ]; then
    info "Resolving the latest release..."
    # The API redirect target of releases/latest carries the tag; parse the
    # JSON tag_name with sed so the script doesn't depend on jq.
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    [ -n "$VERSION" ] || die "could not determine the latest release tag (rate-limited?). Set VERSION=vX.Y.Z and retry."
fi
info "Installing cyclone2-linux ${VERSION}"

RELEASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
RAW_URL="https://raw.githubusercontent.com/${REPO}/${VERSION}"

# ---- arch check (pre-built binaries are x86_64 only) ------------------------
arch=$(uname -m)
[ "$arch" = "x86_64" ] || die "pre-built artefacts are x86_64 only (this machine: ${arch}). Build from source instead: git clone https://github.com/${REPO} && cd cyclone2-linux && make install"

# ---- pick the frontend -------------------------------------------------------
if [ -z "$FRONTEND" ]; then
    de=$(printf '%s' "${XDG_CURRENT_DESKTOP:-}" | tr '[:upper:]' '[:lower:]')
    case ":${de}:" in
        *cosmic*)       FRONTEND=cosmic ;;
        *gnome*)        FRONTEND=gnome ;;
        *kde*|*plasma*) FRONTEND=kde ;;
        *)              FRONTEND=unknown ;;
    esac
fi
case "$FRONTEND" in
    gnome|cosmic|kde|none|unknown) ;;
    *) die "unknown FRONTEND='${FRONTEND}' (expected gnome | cosmic | kde | none)" ;;
esac

# ---- core: daemon binary -----------------------------------------------------
info "Installing the daemon to ${BINDIR}/cyclone2"
fetch "${RELEASE_URL}/cyclone2-${VERSION}-x86_64-linux" "$tmpdir/cyclone2"
mkdir -p "$BINDIR"
install -m0755 "$tmpdir/cyclone2" "$BINDIR/cyclone2"
case ":${PATH}:" in
    *:"$BINDIR":*) ;;
    *) warn "${BINDIR} is not on your PATH — add it to use the 'cyclone2' CLI" ;;
esac

# ---- core: udev rule (the one sudo step) -------------------------------------
info "Installing the udev rule (needs sudo) for root-free HID access"
fetch "${RAW_URL}/packaging/udev/${UDEV_RULE}" "$tmpdir/${UDEV_RULE}"
sudo install -m0644 "$tmpdir/${UDEV_RULE}" "/etc/udev/rules.d/${UDEV_RULE}"
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hidraw

# ---- core: systemd --user service --------------------------------------------
info "Enabling the systemd --user service"
fetch "${RAW_URL}/packaging/systemd/${SERVICE}" "$tmpdir/${SERVICE}"
mkdir -p "$SYSTEMD_USER"
install -m0644 "$tmpdir/${SERVICE}" "$SYSTEMD_USER/${SERVICE}"
systemctl --user daemon-reload
systemctl --user enable --now "$SERVICE"

# ---- frontend -----------------------------------------------------------------
install_gnome() {
    info "Installing the GNOME Shell extension"
    command -v gnome-extensions >/dev/null 2>&1 \
        || die "gnome-extensions CLI not found (is this really GNOME?)"
    fetch "${RELEASE_URL}/${EXT_UUID}.shell-extension.zip" "$tmpdir/extension.zip"
    gnome-extensions install --force "$tmpdir/extension.zip"
    cat <<EOF

GNOME extension installed. Log out and back in (Wayland needs a full shell
reload), then enable it:
  gnome-extensions enable ${EXT_UUID}
EOF
}

install_cosmic() {
    info "Installing the COSMIC applet"
    fetch "${RELEASE_URL}/cyclone2-applet-${VERSION}-x86_64-linux.tar.gz" "$tmpdir/applet.tar.gz"
    tar -xzf "$tmpdir/applet.tar.gz" -C "$tmpdir"
    stage="$tmpdir/cyclone2-applet-${VERSION}-x86_64-linux"
    install -m0755 "$stage/cyclone2-applet" "$BINDIR/cyclone2-applet"
    mkdir -p "$DESKTOPDIR"
    install -m0644 "$stage/io.github.vdemonchy.Cyclone2Linux.desktop" "$DESKTOPDIR/"
    update-desktop-database "$DESKTOPDIR" 2>/dev/null || true
    cat <<'EOF'

COSMIC applet installed. Add it to your panel:
  Settings -> Desktop -> Panel (or Dock) -> Configure applets -> add "Cyclone 2"
EOF
}

install_kde() {
    info "Installing the KDE Plasma 6 plasmoid"
    plasmoid="$tmpdir/cyclone2.plasmoid"
    # Releases older than the plasmoid artefact don't attach it — fall back to
    # the source tarball at the same tag, which has the identical package.
    if ! curl -fsSL --retry 3 -o "$plasmoid" \
            "${RELEASE_URL}/cyclone2-plasmoid-${VERSION}.plasmoid" 2>/dev/null; then
        warn "release ${VERSION} has no plasmoid artefact; using the source tree at that tag"
        fetch "https://github.com/${REPO}/archive/refs/tags/${VERSION}.tar.gz" "$tmpdir/src.tar.gz"
        tar -xzf "$tmpdir/src.tar.gz" -C "$tmpdir"
        pkg="$tmpdir/cyclone2-linux-${VERSION#v}/plasmoid/package"
        [ -d "$pkg" ] || die "release ${VERSION} predates the KDE plasmoid — install a newer release (or the core with FRONTEND=none)"
    fi
    if command -v kpackagetool6 >/dev/null 2>&1; then
        src="${pkg:-$plasmoid}"
        kpackagetool6 --type Plasma/Applet --upgrade "$src" \
            || kpackagetool6 --type Plasma/Applet --install "$src"
    else
        warn "kpackagetool6 not found — copying the package manually"
        dest="$HOME/.local/share/plasma/plasmoids/${PLASMOID_ID}"
        mkdir -p "$dest"
        if [ -n "${pkg:-}" ]; then
            cp -r "$pkg/." "$dest/"
        else
            command -v unzip >/dev/null 2>&1 || die "need unzip (or kpackagetool6) to install the plasmoid"
            unzip -oq "$plasmoid" -d "$dest"
        fi
    fi
    cat <<'EOF'

KDE plasmoid installed. Add it: right-click your panel -> Add Widgets -> Cyclone 2
(If it doesn't appear, log out and back in so Plasma rescans widgets.)
EOF
}

case "$FRONTEND" in
    gnome)  install_gnome ;;
    cosmic) install_cosmic ;;
    kde)    install_kde ;;
    none)
        info "FRONTEND=none — core installed, no frontend" ;;
    unknown)
        warn "no supported desktop detected (XDG_CURRENT_DESKTOP='${XDG_CURRENT_DESKTOP:-}')"
        cat <<'EOF'
Core installed. Pick a frontend explicitly by re-running with FRONTEND=:
  FRONTEND=gnome  sh install.sh   # GNOME Shell
  FRONTEND=cosmic sh install.sh   # COSMIC desktop
  FRONTEND=kde    sh install.sh   # KDE Plasma
EOF
        ;;
esac

info "Done. Verify with: cyclone2 status   (controller in XInput/Switch mode)"
