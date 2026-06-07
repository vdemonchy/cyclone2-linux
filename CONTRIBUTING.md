# Contributing to cyclone2-linux

Thanks for your interest in improving cyclone2-linux! This project controls
battery, connection mode, and RGB lighting for the GameSir Cyclone 2 on Linux.
Contributions — bug reports, hardware captures, code, docs — are all welcome.

> This is an unofficial project, not affiliated with GameSir (see the
> [disclaimer](README.md#disclaimer)). Contributions must respect that: don't
> add GameSir branding, and keep reverse-engineering work to interoperability.

## Project layout

```
cmd/cyclone2/          # CLI + daemon entrypoints (probe/status/daemon/rgb)
internal/              # the daemon's guts (Go, no cgo):
  device/               #   mode detection from USB ids
  hidraw/               #   raw HID device discovery + I/O
  protocol/             #   reverse-engineered battery + RGB command protocol
  powersupply/          #   kernel power_supply (sysfs) reads for DS4/Switch
  reader/ state/        #   battery reading + state-file writing
  uevent/               #   udev netlink hotplug events
  config/ notify/       #   live config reload + low-battery notifications
extension/             # GNOME Shell extension frontend (JS)
cosmic-applet/         # COSMIC applet frontend (Rust / libcosmic)
packaging/             # udev rule + systemd --user service
docs/                  # protocol notes + HID capture/decode helpers
```

The architecture: a dependency-free Go daemon detects the controller mode, reads
the battery from the correct per-mode source, drives RGB, and writes a small JSON
state file (`$XDG_RUNTIME_DIR/cyclone2-linux.json`) that either frontend watches
and displays. See the [README](README.md#how-it-works) for the data flow and
[`docs/protocol.md`](docs/protocol.md) for the reverse-engineered protocol.

## Development setup

**Daemon (Go):**

- Go **1.24+**.
- Build: `make build` (or `go build ./cmd/cyclone2`).
- Test: `make test` (or `go test ./...`). Please run before opening a PR.
- Format/vet: `gofmt -l .` should print nothing; `go vet ./...` should be clean.

**COSMIC applet (Rust):**

- Rust stable **≥ 1.93** (pinned in `cosmic-applet/rust-toolchain.toml`; rustup
  selects it automatically).
- libcosmic build deps. On Debian/Ubuntu:
  ```bash
  sudo apt-get install -y build-essential pkg-config cmake \
    libxkbcommon-dev libwayland-dev libudev-dev libssl-dev libfontconfig-dev
  ```
  (See `.github/workflows/release.yml` for the canonical, CI-tested list.)
- Build: `make build-cosmic` (or `cd cosmic-applet && cargo build --release --locked`).
- Run standalone for dev: `cd cosmic-applet && cargo build && ./target/debug/cyclone2-applet`.

**GNOME extension (JS):** no build step — `make install-gnome` copies it into
place and compiles the gschema. Iterate with `journalctl --user -f` (or Looking
Glass) for logs; on Wayland a shell reload (log out/in) is needed to pick up
changes.

## Installing your build

The `Makefile` keeps the two frontends strictly separate (see `make help`):

```bash
make install            # core: daemon + udev rule + systemd service
make install-gnome      # GNOME frontend only
make install-cosmic     # COSMIC frontend only
```

After changing the daemon: `make install-daemon && systemctl --user restart cyclone2-linux.service`.

## Making changes

1. **Open an issue first** for anything non-trivial, so the approach can be
   discussed before you invest time.
2. **Branch** off `main` (e.g. `fix/ds4-charging`, `feat/rgb-presets`).
3. **Keep the daemon dependency-free** where practical — the only Go dependency
   is `golang.org/x/sys`. New third-party deps need a good reason.
4. **Add tests** for protocol/parsing/state logic; most `internal/` packages have
   table-driven tests next to them (`*_test.go`).
5. **Match the surrounding style** — comment density, naming, and idiom. The
   codebase favors explanatory comments on the *why*, especially around the
   reverse-engineered byte offsets.
6. **Update docs** when behavior changes — `README.md`, `INSTALL.md`, and
   `docs/protocol.md` as relevant.

## Commit & PR conventions

- Commit messages follow the existing log: a `type: summary` subject
  (`fix:`, `feat:`, `refactor:`, `ci:`, `docs:`, `chore:`), imperative mood,
  with a body explaining the *why* when it isn't obvious.
- Keep PRs focused; one logical change per PR.
- In the PR description, note how you tested — ideally on real hardware, stating
  the controller mode(s) (XInput / DS4 / Switch / HID) you exercised.
- CI must be green (`go test ./...`, COSMIC build, extension packaging).

## Hardware captures & protocol work

Much of this project is reverse-engineered. If you're adding support for a new
mode, byte field, or RGB feature, please include the evidence:

- Use `docs/rgb-capture.sh` (usbmon) to capture HID traffic and
  `docs/rgb-decode.py` to decode it.
- Document new findings in [`docs/protocol.md`](docs/protocol.md) with the report
  ids, byte offsets, and how you confirmed them (e.g. plug/unplug deltas).
- Reverse-engineering here is strictly for **interoperability** with hardware the
  user owns — don't include firmware, proprietary assets, or GameSir code.

## Releases

Releases are tag-driven and only build from `main`. Pushing a tag like `v1.2.0`
(on `main`) triggers `.github/workflows/release.yml`, which builds and attaches
the daemon, the GNOME extension zip, and the COSMIC applet tarball to the GitHub
Release. See [INSTALL.md](INSTALL.md) for what each artefact is.

## License

By contributing, you agree that your contributions are licensed under the
project's [GPL-3.0](LICENSE) license.
