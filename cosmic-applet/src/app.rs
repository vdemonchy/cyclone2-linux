use crate::config::{self, AppletConfig, DisplayMode, ZONE_COUNT, ZONE_NAMES};
use crate::state::{self, Display, State};
use crate::watcher;

use cosmic::app::{Core, Task};
use cosmic::iced::window::Id;
use cosmic::iced::{Alignment, Color, Length, Rectangle, Subscription, Vector};
use cosmic::surface::action::{app_popup, destroy_popup};
use cosmic::widget::color_picker::color_button;
use cosmic::widget;
use cosmic::Element;
use cosmic_config::CosmicConfigEntry;
use std::path::PathBuf;

/// Quick-pick palette offered under each zone's hex field.
const SWATCHES: [&str; 9] = [
    "ff0000", "ff8800", "ffff00", "00ff00", "00ffff", "0000ff", "ff00ff", "ffffff", "000000",
];

/// Normalise a user-typed colour to lowercase 6-digit hex, or None if invalid.
fn normalize_hex(s: &str) -> Option<String> {
    let s = s.trim().trim_start_matches('#');
    if s.len() == 6 && s.bytes().all(|b| b.is_ascii_hexdigit()) {
        Some(s.to_ascii_lowercase())
    } else {
        None
    }
}

/// Parse an "RRGGBB" hex string into an iced Color (white on failure).
fn color_from_hex(s: &str) -> Color {
    let s = s.trim().trim_start_matches('#');
    if s.len() == 6 {
        if let Ok(v) = u32::from_str_radix(s, 16) {
            return Color::from_rgb8((v >> 16) as u8, (v >> 8) as u8, v as u8);
        }
    }
    Color::WHITE
}

pub struct Cyclone2Applet {
    core: Core,
    config: AppletConfig,
    state: State,
    display: Display,
    popup: Option<Id>,
    state_path: PathBuf,
    /// Advances each animation tick to breathe the icon brightness while charging.
    charge_phase: f32,
    /// Live text-field contents for each zone's hex entry (ordered like
    /// ZONE_NAMES). Applied to the config on submit / swatch click.
    zone_hex: Vec<String>,
}

#[derive(Debug, Clone)]
pub enum Message {
    StateChanged,
    PopupClosed(Id),
    SetInterval(i32),
    SetDisplayMode(DisplayMode),
    SetThreshold(i32),
    SetLevelHigh(i32),
    SetLevelLow(i32),
    ToggleRgb(bool),
    /// Apply a colour to a zone (from a swatch or a submitted hex field).
    SetZone(usize, String),
    /// Live edit of a zone's hex field (not yet applied).
    ZoneHexEdit(usize, String),
    /// Commit the typed hex for a zone (Enter in the field).
    ZoneHexSubmit(usize),
    SetBrightness(i32),
    BrightnessReleased,
    Tick,
    Surface(cosmic::surface::Action),
}

impl Cyclone2Applet {
    fn reload_state(&mut self) {
        self.state = std::fs::read(&self.state_path)
            .ok()
            .and_then(|b| state::parse(&b).ok())
            .unwrap_or_default();
        self.display = Display::from(&self.state);
    }

    fn battery_line(&self) -> String {
        let s = &self.state;
        if !s.present {
            return "Battery: \u{2014}".into();
        }
        if !s.battery_known {
            return "Battery: unavailable".into();
        }
        let amount = if s.level.is_empty() {
            format!("{}%", s.percent)
        } else {
            s.level.clone()
        };
        let status = if s.charging { "Charging" } else { "On battery" };
        let mut batt = format!("{amount} \u{2014} {status}");
        if s.stale {
            batt.push_str(" (stale)");
        }
        format!("Battery: {batt}")
    }

    fn mode_line(&self) -> String {
        if !self.state.present {
            return "Cyclone 2 mode: disconnected".into();
        }
        format!("Cyclone 2 mode: {}", state::mode_name(&self.state.mode))
    }

    fn persist(&self) {
        if let Ok(cfg) =
            cosmic_config::Config::new(<Self as cosmic::Application>::APP_ID, config::CONFIG_VERSION)
        {
            let _ = self.config.write_entry(&cfg);
        }
    }

    /// Write the daemon's config.json (poll interval, low-battery threshold and,
    /// when lighting control is enabled, the RGB settings).
    fn write_daemon_config(&self) {
        let _ = config::write_daemon_config(&config::daemon_config_dir(), &self.config);
    }

    /// Seed each zone's hex field from the saved colours (padded to ZONE_COUNT).
    fn zone_hex_from(cfg: &AppletConfig) -> Vec<String> {
        (0..ZONE_COUNT)
            .map(|i| {
                cfg.rgb_zones
                    .get(i)
                    .cloned()
                    .unwrap_or_else(|| "ffffff".to_string())
            })
            .collect()
    }

    /// Apply a validated hex colour to zone `i`: update config, enable lighting,
    /// persist, and push to the daemon.
    fn apply_zone(&mut self, i: usize, hex: String) {
        while self.config.rgb_zones.len() < ZONE_COUNT {
            self.config.rgb_zones.push("ffffff".to_string());
        }
        if let Some(slot) = self.config.rgb_zones.get_mut(i) {
            *slot = hex.clone();
        }
        if let Some(buf) = self.zone_hex.get_mut(i) {
            *buf = hex;
        }
        self.config.rgb_enabled = true;
        self.persist();
        self.write_daemon_config();
    }
}

/// Build the message that toggles the popup: closes it if open, otherwise opens
/// one anchored to the pressed button's rectangle. Shared by all panel buttons.
fn popup_toggle(popup_id: Option<Id>, offset: Vector, bounds: Rectangle) -> Message {
    if let Some(id) = popup_id {
        Message::Surface(destroy_popup(id))
    } else {
        Message::Surface(app_popup::<Cyclone2Applet>(
            move |state: &mut Cyclone2Applet| {
                let new_id = Id::unique();
                state.popup = Some(new_id);
                let mut settings = state.core.applet.get_popup_settings(
                    state.core.main_window_id().expect("applet main window"),
                    new_id,
                    None,
                    None,
                    None,
                );
                settings.positioner.anchor_rect = Rectangle {
                    x: (bounds.x - offset.x) as i32,
                    y: (bounds.y - offset.y) as i32,
                    width: bounds.width as i32,
                    height: bounds.height as i32,
                };
                settings
            },
            None,
        ))
    }
}

impl cosmic::Application for Cyclone2Applet {
    type Executor = cosmic::SingleThreadExecutor;
    type Flags = ();
    type Message = Message;
    const APP_ID: &'static str = "io.github.vdemonchy.Cyclone2Linux";

    fn core(&self) -> &Core {
        &self.core
    }

    fn core_mut(&mut self) -> &mut Core {
        &mut self.core
    }

    fn init(core: Core, _flags: Self::Flags) -> (Self, Task<Message>) {
        let config = cosmic_config::Config::new(
            <Cyclone2Applet as cosmic::Application>::APP_ID,
            config::CONFIG_VERSION,
        )
        .ok()
        .and_then(|c| AppletConfig::get_entry(&c).ok())
        .unwrap_or_default();

        let zone_hex = Self::zone_hex_from(&config);
        let mut app = Cyclone2Applet {
            core,
            config,
            state: State::default(),
            display: Display::Hidden,
            popup: None,
            state_path: state::state_path(),
            charge_phase: 0.0,
            zone_hex,
        };
        app.reload_state();
        // Sync settings to the daemon's config.json on startup so the configured
        // poll interval and low-battery threshold take effect without waiting for
        // the user to change a setting.
        app.write_daemon_config();
        (app, Task::none())
    }

    fn subscription(&self) -> Subscription<Message> {
        let watch = watcher::subscription(self.state_path.clone()).map(|_| Message::StateChanged);
        // While charging, add a timer that pulses the icon. iced starts/stops the
        // timer automatically as this set changes with the charging state.
        if self.state.present && self.state.charging {
            let pulse = cosmic::iced::time::every(std::time::Duration::from_millis(60))
                .map(|_| Message::Tick);
            Subscription::batch([watch, pulse])
        } else {
            watch
        }
    }

    fn update(&mut self, message: Message) -> Task<Message> {
        match message {
            Message::StateChanged => {
                self.reload_state();
                Task::none()
            }
            Message::Tick => {
                // Advance the breathing phase (~2s cycle at the 60ms tick rate).
                self.charge_phase = (self.charge_phase + 0.19) % std::f32::consts::TAU;
                Task::none()
            }
            Message::Surface(action) => cosmic::task::message(cosmic::Action::Cosmic(
                cosmic::app::Action::Surface(action),
            )),
            Message::PopupClosed(id) => {
                if self.popup == Some(id) {
                    self.popup = None;
                }
                Task::none()
            }
            Message::SetInterval(secs) => {
                self.config.poll_interval = secs;
                self.persist();
                self.write_daemon_config();
                Task::none()
            }
            Message::SetDisplayMode(mode) => {
                self.config.display_mode = mode;
                self.persist();
                Task::none()
            }
            Message::SetThreshold(threshold) => {
                self.config.low_battery_threshold = threshold;
                self.persist();
                self.write_daemon_config();
                Task::none()
            }
            Message::SetLevelHigh(v) => {
                // Green must stay strictly above the yellow threshold.
                self.config.level_high = v.max(self.config.level_low + 5);
                self.persist();
                Task::none()
            }
            Message::SetLevelLow(v) => {
                // Yellow must stay strictly below the green threshold.
                self.config.level_low = v.min(self.config.level_high - 5);
                self.persist();
                Task::none()
            }
            Message::ToggleRgb(on) => {
                self.config.rgb_enabled = on;
                self.persist();
                self.write_daemon_config();
                Task::none()
            }
            Message::SetZone(i, hex) => {
                if let Some(h) = normalize_hex(&hex) {
                    self.apply_zone(i, h);
                }
                Task::none()
            }
            Message::ZoneHexEdit(i, s) => {
                if let Some(buf) = self.zone_hex.get_mut(i) {
                    *buf = s;
                }
                Task::none()
            }
            Message::ZoneHexSubmit(i) => {
                let typed = self.zone_hex.get(i).cloned().unwrap_or_default();
                match normalize_hex(&typed) {
                    Some(h) => self.apply_zone(i, h),
                    // Invalid entry: revert the field to the stored colour.
                    None => {
                        if let (Some(buf), Some(saved)) =
                            (self.zone_hex.get_mut(i), self.config.rgb_zones.get(i))
                        {
                            *buf = saved.clone();
                        }
                    }
                }
                Task::none()
            }
            Message::SetBrightness(v) => {
                // Live update for slider feedback; written out on release.
                self.config.rgb_brightness = v.clamp(0, 100);
                Task::none()
            }
            Message::BrightnessReleased => {
                self.config.rgb_enabled = true;
                self.persist();
                self.write_daemon_config();
                Task::none()
            }
        }
    }

    fn view(&self) -> Element<'_, Message> {
        let text_val = match &self.display {
            Display::Hidden => {
                // Wrap the empty element in autosize_window so the panel shrinks
                // the applet surface to zero; returning a bare Space leaves the
                // surface at its last size (an empty gap) when the controller
                // disconnects.
                return self
                    .core
                    .applet
                    .autosize_window(widget::Space::new())
                    .into();
            }
            Display::Missing { text } => text.clone(),
            Display::Battery { text, .. } => text.clone(),
        };

        // The controller icon is the indicator; it is tinted by battery level:
        // green (high) / yellow (medium) / red (low), falling back to the panel
        // foreground colour when the level is unknown (missing / stale / no
        // battery).
        let suggested = self.core.applet.suggested_size(true);
        let colorize = matches!(self.display, Display::Battery { .. });
        let pct = self.state.percent;
        // While charging, smoothly breathe the icon brightness via the pulse phase.
        let charging = self.state.charging;
        let phase = self.charge_phase;
        let level_high = self.config.level_high;
        let level_low = self.config.level_low;
        let icon_class = cosmic::theme::Svg::Custom(std::rc::Rc::new(move |theme| {
            let c = theme.cosmic();
            let base = if !colorize {
                c.background.on
            } else if pct >= level_high {
                c.success.base
            } else if pct >= level_low {
                c.warning.base
            } else {
                c.destructive.base
            };
            let mut color: cosmic::iced::Color = base.into();
            if charging {
                // sin() ∈ [-1, 1] → brightness factor ∈ [0.4, 1.0] (~40% floor),
                // ~2s cycle (see the 60ms tick + 0.19 phase step). Kept in sync
                // with the GNOME extension's opacity pulse for a matching feel.
                let f = 0.7 + 0.3 * phase.sin();
                color.r *= f;
                color.g *= f;
                color.b *= f;
            }
            cosmic::widget::svg::Style { color: Some(color) }
        }));
        let controller_icon = widget::icon::from_name("input-gaming-symbolic")
            .symbolic(true)
            .size(suggested.0)
            .icon()
            .class(icon_class)
            .width(Length::Fixed(suggested.0 as f32))
            .height(Length::Fixed(suggested.1 as f32));

        // Build the indicator as a single row (icon, optionally + text)...
        let mut content = widget::Row::new().align_y(Alignment::Center).spacing(4);
        content = content.push(controller_icon);
        if self.config.display_mode == DisplayMode::IconText && !text_val.is_empty() {
            content = content.push(self.core.applet.text(text_val));
        }

        // ...wrapped in a single AppletIcon button so it reads as one cohesive
        // indicator, and in autosize_window so the panel surface grows to fit.
        let popup_id = self.popup;
        let btn = widget::button::custom(content)
            .class(cosmic::theme::Button::AppletIcon)
            .on_press_with_rectangle(move |offset, bounds| popup_toggle(popup_id, offset, bounds));

        self.core.applet.autosize_window(btn).into()
    }

    fn view_window(&self, _id: Id) -> Element<'_, Message> {
        let interval = self.config.poll_interval;
        let intervals: [(i32, &str); 4] =
            [(10, "10 s"), (30, "30 s"), (60, "1 min"), (300, "5 min")];
        let mut interval_row = widget::Row::with_capacity(intervals.len()).spacing(4);
        for (secs, label) in intervals {
            let b = if interval == secs {
                widget::button::text(label)
                    .on_press(Message::SetInterval(secs))
                    .class(cosmic::theme::Button::Suggested)
            } else {
                widget::button::text(label).on_press(Message::SetInterval(secs))
            };
            interval_row = interval_row.push(b);
        }

        let display_modes: [(DisplayMode, &str); 2] = [
            (DisplayMode::IconOnly, "Icon only"),
            (DisplayMode::IconText, "Icon + text"),
        ];
        let mut display_row = widget::Row::with_capacity(display_modes.len()).spacing(4);
        for (mode, label) in display_modes {
            let b = if self.config.display_mode == mode {
                widget::button::text(label)
                    .on_press(Message::SetDisplayMode(mode))
                    .class(cosmic::theme::Button::Suggested)
            } else {
                widget::button::text(label).on_press(Message::SetDisplayMode(mode))
            };
            display_row = display_row.push(b);
        }

        // Numeric stepper (mirrors the GNOME SpinRow): 0–50% in steps of 5,
        // where 0 shows "Off". The spin_button widget displays the label we pass
        // and drives the +/- math from value/step/min/max.
        let threshold = self.config.low_battery_threshold;
        let threshold_label = if threshold <= 0 {
            "Off".to_string()
        } else {
            format!("{threshold}%")
        };
        let threshold_spin =
            widget::spin_button(threshold_label, threshold, 5, 0, 50, Message::SetThreshold);

        // Battery level colour thresholds (green ≥ high, yellow ≥ low, else red).
        let level_high = self.config.level_high;
        let level_low = self.config.level_low;
        // Green must stay above yellow: bound each stepper by the other (+/- one
        // step) so the constraint can't be violated from the UI.
        let high_spin = widget::spin_button(
            format!("{level_high}%"),
            level_high,
            5,
            level_low + 5,
            100,
            Message::SetLevelHigh,
        );
        let low_spin = widget::spin_button(
            format!("{level_low}%"),
            level_low,
            5,
            0,
            level_high - 5,
            Message::SetLevelLow,
        );

        let mut content = widget::Column::with_capacity(20)
            .spacing(8)
            .push(cosmic::applet::padded_control(widget::text::title4(
                self.mode_line(),
            )))
            .push(cosmic::applet::padded_control(widget::text(
                self.battery_line(),
            )))
            .push(cosmic::applet::padded_control(
                widget::divider::horizontal::default(),
            ))
            .push(cosmic::applet::padded_control(widget::text::heading(
                "Poll interval",
            )))
            .push(cosmic::applet::padded_control(interval_row))
            .push(cosmic::applet::padded_control(widget::text::heading(
                "Display",
            )))
            .push(cosmic::applet::padded_control(display_row))
            .push(cosmic::applet::padded_control(widget::text::heading(
                "Low battery alert",
            )))
            .push(cosmic::applet::padded_control(threshold_spin))
            .push(cosmic::applet::padded_control(widget::text::heading(
                "Battery level colors",
            )))
            .push(cosmic::applet::padded_control(widget::settings::item(
                "Green at \u{2265}",
                high_spin,
            )))
            .push(cosmic::applet::padded_control(widget::settings::item(
                "Yellow at \u{2265}",
                low_spin,
            )))
            .push(cosmic::applet::padded_control(widget::text::caption(
                "Below the yellow threshold the icon is red.",
            )));

        // Controller lighting (RGB). XInput mode only; a toggle gates whether the
        // daemon manages the lighting at all.
        content = content
            .push(cosmic::applet::padded_control(
                widget::divider::horizontal::default(),
            ))
            .push(cosmic::applet::padded_control(widget::text::heading(
                "Controller lighting",
            )));

        // RGB control is only possible over the vendor interface, which the
        // controller exposes only in XInput mode (GameSir Connect requires it
        // too). Outside XInput, render no controls at all — just explain why.
        let xinput = self.state.present && self.state.mode == "xinput";
        if !xinput {
            let why = if self.state.present {
                format!(
                    "Unavailable in {} mode — RGB control works only in XInput mode.",
                    state::mode_name(&self.state.mode)
                )
            } else {
                "Connect a controller in XInput mode to control the lighting.".into()
            };
            content = content.push(cosmic::applet::padded_control(widget::text::caption(why)));
        } else {
            content = content.push(cosmic::applet::padded_control(widget::settings::item(
                "Control lighting",
                widget::toggler(self.config.rgb_enabled).on_toggle(Message::ToggleRgb),
            )));

            if self.config.rgb_enabled {
                let brightness = self.config.rgb_brightness;
                content = content.push(cosmic::applet::padded_control(widget::settings::item(
                    format!("Brightness: {brightness}%"),
                    widget::slider(0..=100, brightness, Message::SetBrightness)
                        .on_release(Message::BrightnessReleased),
                )));
                for i in 0..ZONE_COUNT {
                    // Hex field with a live colour-swatch preview as its leading icon.
                    // Borrow the buffer from self so it outlives the returned element.
                    let field = widget::text_input("ffffff", &self.zone_hex[i])
                        .leading_icon(
                            color_button(None, Some(color_from_hex(&self.zone_hex[i])), Length::Fixed(16.0))
                                .into(),
                        )
                        .on_input(move |s| Message::ZoneHexEdit(i, s))
                        .on_submit(move |_| Message::ZoneHexSubmit(i))
                        .width(Length::Fixed(150.0));
                    content = content.push(cosmic::applet::padded_control(widget::settings::item(
                        ZONE_NAMES[i],
                        field,
                    )));
                    // Quick-pick palette row that applies immediately to this zone.
                    let mut palette = widget::Row::with_capacity(SWATCHES.len()).spacing(4);
                    for swatch in SWATCHES {
                        palette = palette.push(color_button(
                            Some(Message::SetZone(i, swatch.to_string())),
                            Some(color_from_hex(swatch)),
                            Length::Fixed(16.0),
                        ));
                    }
                    content = content.push(cosmic::applet::padded_control(palette));
                }
            }
        }

        // With lighting expanded the popup can exceed the screen height (the LED
        // zone controls are tall), clipping the lower zones. Cap the height and
        // let it scroll so every zone stays reachable.
        let body: Element<'_, Message> = if xinput && self.config.rgb_enabled {
            widget::scrollable(content)
                .height(Length::Fixed(720.0))
                .into()
        } else {
            content.into()
        };
        self.core.applet.popup_container(body).into()
    }

    fn on_close_requested(&self, id: Id) -> Option<Message> {
        Some(Message::PopupClosed(id))
    }

    fn style(&self) -> Option<cosmic::iced::theme::Style> {
        Some(cosmic::applet::style())
    }
}
