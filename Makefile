# cyclone2-linux — build & install from source
#
# The project has three installable pieces:
#   1. core      — the Go daemon + udev rule + systemd --user service (DE-independent)
#   2. gnome      — the GNOME Shell extension frontend
#   3. cosmic     — the native COSMIC applet frontend
#
# A working install is always "core" plus exactly one frontend. The frontends
# are kept strictly separate: `make install-gnome` never touches COSMIC and
# `make install-cosmic` never touches GNOME. Pick the one for your desktop.
#
# Quick start:
#   make install install-gnome     # GNOME Shell
#   make install install-cosmic    # COSMIC desktop
#
# Everything installs into your user prefix ($HOME/.local) except the udev rule,
# which needs root and so prompts for sudo. Override paths with PREFIX=... etc.

# ---- configuration ---------------------------------------------------------
PREFIX         ?= $(HOME)/.local
BINDIR         ?= $(PREFIX)/bin
DESKTOPDIR     ?= $(PREFIX)/share/applications
SYSTEMD_USER   ?= $(HOME)/.config/systemd/user
GNOME_EXT_DIR  ?= $(HOME)/.local/share/gnome-shell/extensions
UDEV_RULES_DIR ?= /etc/udev/rules.d

GO    ?= go
CARGO ?= cargo

EXT_UUID    := cyclone2-linux@vdemonchy.github.io
EXT_SRC     := extension/$(EXT_UUID)
UDEV_RULE   := 60-gamesir-cyclone2.rules
SERVICE     := cyclone2-linux.service

# Strip debug info from release builds (matches the CI release artifacts).
GO_LDFLAGS  := -s -w
GO_BUILD    := $(GO) build -trimpath -ldflags="$(GO_LDFLAGS)"

.DEFAULT_GOAL := help

# ---- meta ------------------------------------------------------------------
.PHONY: help
help: ## Show this help
	@echo "cyclone2-linux — make targets:"
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Typical install (GNOME):  make install install-gnome"
	@echo "Typical install (COSMIC): make install install-cosmic"

# ---- build -----------------------------------------------------------------
.PHONY: build
build: ## Build the cyclone2 daemon binary into ./cyclone2
	$(GO_BUILD) -o cyclone2 ./cmd/cyclone2

.PHONY: build-cosmic
build-cosmic: ## Build the COSMIC applet (release) — needs Rust >= 1.93 + libcosmic deps
	cd cosmic-applet && $(CARGO) build --release --locked

.PHONY: test
test: ## Run the Go test suite
	$(GO) test ./...

# ---- core install (DE-independent): daemon + udev + systemd ----------------
.PHONY: install
install: install-daemon install-udev install-service ## Install core: daemon + udev rule + systemd user service
	@echo "core installed. Now install a frontend: 'make install-gnome' or 'make install-cosmic'."

.PHONY: install-daemon
install-daemon: ## Build and install the cyclone2 daemon to $(BINDIR)
	@mkdir -p "$(BINDIR)"
	$(GO_BUILD) -o "$(BINDIR)/cyclone2" ./cmd/cyclone2
	@echo "installed $(BINDIR)/cyclone2"

.PHONY: install-udev
install-udev: ## Install the udev rule (needs sudo) for root-free HID access
	sudo install -m0644 packaging/udev/$(UDEV_RULE) "$(UDEV_RULES_DIR)/$(UDEV_RULE)"
	sudo udevadm control --reload-rules
	sudo udevadm trigger --subsystem-match=hidraw
	@echo "udev rule installed to $(UDEV_RULES_DIR)/$(UDEV_RULE)"

.PHONY: install-service
install-service: ## Install and enable the systemd --user service
	@mkdir -p "$(SYSTEMD_USER)"
	install -m0644 packaging/systemd/$(SERVICE) "$(SYSTEMD_USER)/$(SERVICE)"
	systemctl --user daemon-reload
	systemctl --user enable --now $(SERVICE)
	@echo "service enabled; state file: $${XDG_RUNTIME_DIR}/cyclone2-linux.json"

# ---- GNOME frontend (only touches GNOME) -----------------------------------
.PHONY: install-gnome
install-gnome: ## Install the GNOME Shell extension (does not touch COSMIC)
	@mkdir -p "$(GNOME_EXT_DIR)/$(EXT_UUID)"
	cp -r "$(EXT_SRC)/." "$(GNOME_EXT_DIR)/$(EXT_UUID)/"
	glib-compile-schemas "$(GNOME_EXT_DIR)/$(EXT_UUID)/schemas"
	@echo "extension installed."
	@echo "Log out and back in (Wayland needs a full shell reload), then:"
	@echo "  gnome-extensions enable $(EXT_UUID)"

.PHONY: uninstall-gnome
uninstall-gnome: ## Remove the GNOME Shell extension
	-gnome-extensions disable $(EXT_UUID) 2>/dev/null || true
	rm -rf "$(GNOME_EXT_DIR)/$(EXT_UUID)"
	@echo "GNOME extension removed."

# ---- COSMIC frontend (only touches COSMIC) ---------------------------------
.PHONY: install-cosmic
install-cosmic: build-cosmic ## Build and install the COSMIC applet (does not touch GNOME)
	@mkdir -p "$(BINDIR)" "$(DESKTOPDIR)"
	install -m0755 cosmic-applet/target/release/cyclone2-applet "$(BINDIR)/cyclone2-applet"
	install -m0644 cosmic-applet/data/io.github.vdemonchy.Cyclone2Linux.desktop \
		"$(DESKTOPDIR)/io.github.vdemonchy.Cyclone2Linux.desktop"
	-update-desktop-database "$(DESKTOPDIR)" 2>/dev/null || true
	@echo "COSMIC applet installed."
	@echo "Add it via: Settings -> Desktop -> Panel (or Dock) -> Configure applets -> add 'Cyclone 2'."

.PHONY: uninstall-cosmic
uninstall-cosmic: ## Remove the COSMIC applet
	rm -f "$(BINDIR)/cyclone2-applet"
	rm -f "$(DESKTOPDIR)/io.github.vdemonchy.Cyclone2Linux.desktop"
	-update-desktop-database "$(DESKTOPDIR)" 2>/dev/null || true
	@echo "COSMIC applet removed."

# ---- core uninstall --------------------------------------------------------
.PHONY: uninstall
uninstall: ## Remove core (daemon + service + udev rule); leaves frontends
	-systemctl --user disable --now $(SERVICE) 2>/dev/null || true
	rm -f "$(SYSTEMD_USER)/$(SERVICE)"
	systemctl --user daemon-reload 2>/dev/null || true
	rm -f "$(BINDIR)/cyclone2"
	sudo rm -f "$(UDEV_RULES_DIR)/$(UDEV_RULE)"
	sudo udevadm control --reload-rules 2>/dev/null || true
	@echo "core removed. Frontends (if installed) remain: see uninstall-gnome / uninstall-cosmic."

# ---- housekeeping ----------------------------------------------------------
.PHONY: clean
clean: ## Remove build artifacts
	rm -f cyclone2
	rm -rf dist
	cd cosmic-applet && $(CARGO) clean 2>/dev/null || true
