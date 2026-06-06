use serde::Deserialize;
use std::path::PathBuf;

/// Mirror of internal/state/state.go State. Go is the source of truth.
#[derive(Debug, Clone, Default, Deserialize, PartialEq)]
pub struct State {
    #[serde(default)]
    pub present: bool,
    #[serde(default)]
    pub percent: i32,
    #[serde(default)]
    pub charging: bool,
    #[serde(default)]
    pub stale: bool,
    #[serde(default)]
    pub error: String,
    #[serde(default)]
    pub ts: i64,
    #[serde(default)]
    pub mode: String,
    #[serde(default)]
    pub battery_known: bool,
    #[serde(default)]
    pub level: String,
}

/// What the panel should show, derived from a State.
#[derive(Debug, Clone, PartialEq)]
pub enum Display {
    /// Not present: render nothing (zero-size view), applet effectively hidden.
    Hidden,
    /// Visible but no battery source (HID mode) or stale: missing icon.
    Missing { text: String },
    /// Normal: mapped battery icon + text (level or "NN%").
    Battery { icon: String, text: String },
}

impl State {
    /// freedesktop battery icon name, matching extension.js:_iconFor.
    pub fn icon_name(&self) -> String {
        let p = self.percent;
        let lvl = if p >= 90 {
            "full"
        } else if p >= 60 {
            "good"
        } else if p >= 30 {
            "low"
        } else if p >= 10 {
            "caution"
        } else {
            "empty"
        };
        if self.charging {
            format!("battery-{lvl}-charging-symbolic")
        } else {
            format!("battery-{lvl}-symbolic")
        }
    }
}

impl From<&State> for Display {
    fn from(s: &State) -> Self {
        if !s.present {
            return Display::Hidden;
        }
        if !s.battery_known {
            return Display::Missing { text: String::new() };
        }
        if s.stale {
            return Display::Missing {
                text: format!("{}%?", s.percent),
            };
        }
        let text = if s.level.is_empty() {
            format!("{}%", s.percent)
        } else {
            s.level.clone()
        };
        Display::Battery {
            icon: s.icon_name(),
            text,
        }
    }
}

/// Human-readable mode name, matching extension.js _updateMenu `names`.
pub fn mode_name(mode: &str) -> String {
    match mode {
        "xinput" => "XInput",
        "ds4" => "DS4",
        "switch" => "Switch",
        "hid" => "HID",
        "unknown" | "" => "Unknown",
        other => other,
    }
    .to_string()
}

/// $XDG_RUNTIME_DIR/cyclone2-linux.json, fallback to temp dir. Mirrors state.DefaultPath().
pub fn state_path() -> PathBuf {
    let dir = std::env::var_os("XDG_RUNTIME_DIR")
        .map(PathBuf::from)
        .unwrap_or_else(std::env::temp_dir);
    dir.join("cyclone2-linux.json")
}

pub fn parse(bytes: &[u8]) -> Result<State, serde_json::Error> {
    serde_json::from_slice(bytes)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_full_xinput_sample() {
        let json = br#"{"present":true,"percent":72,"charging":false,"stale":false,"ts":1,"mode":"xinput","battery_known":true}"#;
        let s = parse(json).unwrap();
        assert_eq!(s.percent, 72);
        assert_eq!(s.mode, "xinput");
        assert!(s.present);
        assert!(s.battery_known);
    }

    #[test]
    fn icon_thresholds_match_extension() {
        let mk = |p: i32, charging: bool| State { percent: p, charging, ..Default::default() };
        assert_eq!(mk(95, false).icon_name(), "battery-full-symbolic");
        assert_eq!(mk(90, false).icon_name(), "battery-full-symbolic");
        assert_eq!(mk(89, false).icon_name(), "battery-good-symbolic");
        assert_eq!(mk(60, false).icon_name(), "battery-good-symbolic");
        assert_eq!(mk(30, false).icon_name(), "battery-low-symbolic");
        assert_eq!(mk(10, false).icon_name(), "battery-caution-symbolic");
        assert_eq!(mk(9, false).icon_name(), "battery-empty-symbolic");
        assert_eq!(mk(95, true).icon_name(), "battery-full-charging-symbolic");
    }

    #[test]
    fn display_hidden_when_absent() {
        let s = State { present: false, ..Default::default() };
        assert_eq!(Display::from(&s), Display::Hidden);
    }

    #[test]
    fn display_missing_when_battery_unknown() {
        let s = State { present: true, battery_known: false, ..Default::default() };
        assert_eq!(Display::from(&s), Display::Missing { text: String::new() });
    }

    #[test]
    fn display_stale_shows_question_mark() {
        let s = State { present: true, battery_known: true, stale: true, percent: 50, ..Default::default() };
        assert_eq!(Display::from(&s), Display::Missing { text: "50%?".into() });
    }

    #[test]
    fn display_switch_level_prefers_level_text() {
        let s = State { present: true, battery_known: true, level: "Full".into(), mode: "switch".into(), ..Default::default() };
        match Display::from(&s) {
            Display::Battery { text, .. } => assert_eq!(text, "Full"),
            other => panic!("expected Battery, got {other:?}"),
        }
    }

    #[test]
    fn display_normal_shows_percent() {
        let s = State { present: true, battery_known: true, percent: 72, ..Default::default() };
        match Display::from(&s) {
            Display::Battery { icon, text } => {
                assert_eq!(text, "72%");
                assert_eq!(icon, "battery-good-symbolic");
            }
            other => panic!("expected Battery, got {other:?}"),
        }
    }

    #[test]
    fn mode_names_map() {
        assert_eq!(mode_name("ds4"), "DS4");
        assert_eq!(mode_name("switch"), "Switch");
        assert_eq!(mode_name(""), "Unknown");
        assert_eq!(mode_name("weird"), "weird");
    }
}
