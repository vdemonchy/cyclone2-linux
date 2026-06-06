use cosmic_config::CosmicConfigEntry;
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

pub const CONFIG_VERSION: u64 = 1;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum DisplayMode {
    IconOnly,
    IconText,
}

impl Default for DisplayMode {
    fn default() -> Self {
        DisplayMode::IconText
    }
}

/// Number of independently addressable LED zones, ordered [Left, Right, Logo,
/// Center] to match the daemon's protocol.LEDZoneNames.
pub const ZONE_COUNT: usize = 4;
pub const ZONE_NAMES: [&str; ZONE_COUNT] = ["Left", "Right", "Logo", "Center"];

#[derive(
    Debug,
    Clone,
    PartialEq,
    Serialize,
    Deserialize,
    cosmic_config::cosmic_config_derive::CosmicConfigEntry,
)]
#[version = 1]
pub struct AppletConfig {
    pub poll_interval: i32,
    pub display_mode: DisplayMode,
    /// Percentage at or below which the daemon posts a low-battery notification.
    /// 0 disables notifications.
    pub low_battery_threshold: i32,
    /// Battery % at or above which the icon is green (high level).
    pub level_high: i32,
    /// Battery % at or above which the icon is yellow (medium); below is red (low).
    pub level_low: i32,
    /// When true, the daemon manages the controller lighting from the settings
    /// below. When false, RGB is omitted from config.json and the daemon leaves
    /// the controller's lighting untouched.
    pub rgb_enabled: bool,
    /// Overall LED brightness, 0-100.
    pub rgb_brightness: i32,
    /// Per-zone colours as "RRGGBB" hex, ordered like ZONE_NAMES.
    pub rgb_zones: Vec<String>,
}

impl Default for AppletConfig {
    fn default() -> Self {
        AppletConfig {
            poll_interval: 60,
            display_mode: DisplayMode::IconText,
            low_battery_threshold: 20,
            level_high: 60,
            level_low: 25,
            rgb_enabled: false,
            rgb_brightness: 100,
            rgb_zones: vec!["ffffff".to_string(); ZONE_COUNT],
        }
    }
}

/// $XDG_CONFIG_HOME/cyclone2-battery, fallback ~/.config/cyclone2-battery.
/// Mirrors the daemon's config.Path() dir.
pub fn daemon_config_dir() -> PathBuf {
    let base = std::env::var_os("XDG_CONFIG_HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|| {
            let home = std::env::var_os("HOME")
                .map(PathBuf::from)
                .unwrap_or_default();
            home.join(".config")
        });
    base.join("cyclone2-battery")
}

/// The RGB block of the daemon config.json (omitted entirely when lighting is
/// not managed). Field names/shape match the Go config.RGB struct.
#[derive(Serialize)]
struct DaemonRgb {
    brightness: i32,
    zones: Vec<String>,
}

/// The daemon config.json. `rgb` is None unless the user enabled lighting
/// control, so battery-only setups leave the controller's LEDs untouched.
#[derive(Serialize)]
struct DaemonConfig {
    interval_seconds: i32,
    low_battery_threshold: i32,
    #[serde(skip_serializing_if = "Option::is_none")]
    rgb: Option<DaemonRgb>,
}

/// Serialize the daemon config.json from the applet config.
pub fn daemon_config_bytes(cfg: &AppletConfig) -> Vec<u8> {
    let rgb = cfg.rgb_enabled.then(|| DaemonRgb {
        brightness: cfg.rgb_brightness,
        zones: cfg.rgb_zones.clone(),
    });
    let dc = DaemonConfig {
        interval_seconds: cfg.poll_interval,
        low_battery_threshold: cfg.low_battery_threshold,
        rgb,
    };
    serde_json::to_vec(&dc).unwrap_or_default()
}

/// Write the daemon config.json into `dir`, creating the dir if needed.
pub fn write_daemon_config(dir: &Path, cfg: &AppletConfig) -> std::io::Result<()> {
    std::fs::create_dir_all(dir)?;
    let path = dir.join("config.json");
    std::fs::write(path, daemon_config_bytes(cfg))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn defaults_match_gschema() {
        let c = AppletConfig::default();
        assert_eq!(c.poll_interval, 60);
        assert_eq!(c.display_mode, DisplayMode::IconText);
        assert_eq!(c.low_battery_threshold, 20);
        assert_eq!(c.level_high, 60);
        assert_eq!(c.level_low, 25);
        assert!(!c.rgb_enabled);
        assert_eq!(c.rgb_brightness, 100);
        assert_eq!(c.rgb_zones.len(), ZONE_COUNT);
    }

    #[test]
    fn config_bytes_omit_rgb_when_disabled() {
        let mut c = AppletConfig::default();
        c.poll_interval = 30;
        c.low_battery_threshold = 20;
        assert_eq!(
            daemon_config_bytes(&c),
            b"{\"interval_seconds\":30,\"low_battery_threshold\":20}"
        );
    }

    #[test]
    fn config_bytes_include_rgb_when_enabled() {
        let mut c = AppletConfig::default();
        c.poll_interval = 60;
        c.low_battery_threshold = 0;
        c.rgb_enabled = true;
        c.rgb_brightness = 80;
        c.rgb_zones = vec![
            "ff0000".into(),
            "00ff00".into(),
            "0000ff".into(),
            "ffffff".into(),
        ];
        assert_eq!(
            daemon_config_bytes(&c),
            br#"{"interval_seconds":60,"low_battery_threshold":0,"rgb":{"brightness":80,"zones":["ff0000","00ff00","0000ff","ffffff"]}}"#
        );
    }

    #[test]
    fn write_creates_dir_and_file() {
        let tmp = std::env::temp_dir()
            .join(format!("cyclone2-cfg-test-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp);
        let mut c = AppletConfig::default();
        c.poll_interval = 10;
        c.low_battery_threshold = 15;
        write_daemon_config(&tmp, &c).unwrap();
        let got = std::fs::read(tmp.join("config.json")).unwrap();
        assert_eq!(got, b"{\"interval_seconds\":10,\"low_battery_threshold\":15}");
        let _ = std::fs::remove_dir_all(&tmp);
    }
}
