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
}

impl Default for AppletConfig {
    fn default() -> Self {
        AppletConfig {
            poll_interval: 60,
            display_mode: DisplayMode::IconText,
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

/// Serialize exactly like the GNOME prefs.js write: {"interval_seconds":N}.
pub fn daemon_interval_bytes(secs: i32) -> Vec<u8> {
    format!("{{\"interval_seconds\":{secs}}}").into_bytes()
}

/// Write the daemon config.json into `dir`, creating the dir if needed.
pub fn write_daemon_interval(dir: &Path, secs: i32) -> std::io::Result<()> {
    std::fs::create_dir_all(dir)?;
    let path = dir.join("config.json");
    std::fs::write(path, daemon_interval_bytes(secs))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn defaults_match_gschema() {
        let c = AppletConfig::default();
        assert_eq!(c.poll_interval, 60);
        assert_eq!(c.display_mode, DisplayMode::IconText);
    }

    #[test]
    fn interval_bytes_are_exact() {
        assert_eq!(daemon_interval_bytes(30), b"{\"interval_seconds\":30}");
        assert_eq!(daemon_interval_bytes(300), b"{\"interval_seconds\":300}");
    }

    #[test]
    fn write_creates_dir_and_file() {
        let tmp = std::env::temp_dir()
            .join(format!("cyclone2-cfg-test-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp);
        write_daemon_interval(&tmp, 10).unwrap();
        let got = std::fs::read(tmp.join("config.json")).unwrap();
        assert_eq!(got, b"{\"interval_seconds\":10}");
        let _ = std::fs::remove_dir_all(&tmp);
    }
}
