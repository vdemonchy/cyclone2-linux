use crate::config::{self, AppletConfig, DisplayMode};
use crate::state::{self, Display, State};
use crate::watcher;

use cosmic::app::{Core, Task};
use cosmic::iced::window::Id;
use cosmic::iced::{Alignment, Rectangle, Subscription};
use cosmic::surface::action::{app_popup, destroy_popup};
use cosmic::widget;
use cosmic::Element;
use cosmic_config::CosmicConfigEntry;
use std::path::PathBuf;

pub struct Cyclone2Applet {
    core: Core,
    config: AppletConfig,
    state: State,
    display: Display,
    popup: Option<Id>,
    state_path: PathBuf,
}

#[derive(Debug, Clone)]
pub enum Message {
    StateChanged,
    PopupClosed(Id),
    SetInterval(i32),
    SetDisplayMode(DisplayMode),
    ToggleControllerIcon(bool),
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
        let mut batt = if s.level.is_empty() {
            format!("{}%", s.percent)
        } else {
            s.level.clone()
        };
        if s.charging {
            batt.push_str(" (charging)");
        }
        if s.stale {
            batt.push_str(" (stale)");
        }
        format!("Battery: {batt}")
    }

    fn mode_line(&self) -> String {
        if !self.state.present {
            return "Mode: disconnected".into();
        }
        format!("Mode: {}", state::mode_name(&self.state.mode))
    }

    fn persist(&self) {
        if let Ok(cfg) =
            cosmic_config::Config::new(<Self as cosmic::Application>::APP_ID, config::CONFIG_VERSION)
        {
            let _ = self.config.write_entry(&cfg);
        }
    }
}

impl cosmic::Application for Cyclone2Applet {
    type Executor = cosmic::SingleThreadExecutor;
    type Flags = ();
    type Message = Message;
    const APP_ID: &'static str = "dev.victor.Cyclone2Battery";

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

        let mut app = Cyclone2Applet {
            core,
            config,
            state: State::default(),
            display: Display::Hidden,
            popup: None,
            state_path: state::state_path(),
        };
        app.reload_state();
        (app, Task::none())
    }

    fn subscription(&self) -> Subscription<Message> {
        watcher::subscription(self.state_path.clone()).map(|_| Message::StateChanged)
    }

    fn update(&mut self, message: Message) -> Task<Message> {
        match message {
            Message::StateChanged => {
                self.reload_state();
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
                let _ = config::write_daemon_interval(&config::daemon_config_dir(), secs);
                Task::none()
            }
            Message::SetDisplayMode(mode) => {
                self.config.display_mode = mode;
                self.persist();
                Task::none()
            }
            Message::ToggleControllerIcon(on) => {
                self.config.show_controller_icon = on;
                self.persist();
                Task::none()
            }
        }
    }

    fn view(&self) -> Element<'_, Message> {
        let (icon_name, text_val) = match &self.display {
            Display::Hidden => {
                return widget::Space::new().into();
            }
            Display::Missing { text } => ("battery-missing-symbolic".to_string(), text.clone()),
            Display::Battery { icon, text } => (icon.clone(), text.clone()),
        };

        let have_popup = self.popup;
        let mut row = widget::Row::with_capacity(3)
            .align_y(Alignment::Center)
            .spacing(2);

        if self.config.show_controller_icon {
            let btn = self
                .core
                .applet
                .icon_button("input-gaming-symbolic")
                .on_press_with_rectangle(move |offset, bounds| {
                    if let Some(id) = have_popup {
                        Message::Surface(destroy_popup(id))
                    } else {
                        Message::Surface(app_popup::<Cyclone2Applet>(
                            move |state: &mut Cyclone2Applet| {
                                let new_id = Id::unique();
                                state.popup = Some(new_id);
                                let mut settings = state.core.applet.get_popup_settings(
                                    state.core.main_window_id().unwrap(),
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
                });
            row = row.push(btn);
        }

        if self.config.display_mode != DisplayMode::TextOnly {
            // Use icon_button_from_handle so the button doesn't borrow icon_name (a local String).
            let suggested_size = self.core.applet.suggested_size(true);
            let handle = widget::icon::from_name(icon_name.as_str())
                .symbolic(true)
                .size(suggested_size.0)
                .into();
            let btn = self
                .core
                .applet
                .icon_button_from_handle(handle)
                .on_press_with_rectangle(move |offset, bounds| {
                    if let Some(id) = have_popup {
                        Message::Surface(destroy_popup(id))
                    } else {
                        Message::Surface(app_popup::<Cyclone2Applet>(
                            move |state: &mut Cyclone2Applet| {
                                let new_id = Id::unique();
                                state.popup = Some(new_id);
                                let mut settings = state.core.applet.get_popup_settings(
                                    state.core.main_window_id().unwrap(),
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
                });
            row = row.push(btn);
        }

        if self.config.display_mode != DisplayMode::IconOnly && !text_val.is_empty() {
            let tv = text_val.clone();
            let btn = widget::button::custom(self.core.applet.text(tv))
                .class(cosmic::theme::Button::AppletIcon)
                .on_press_with_rectangle(move |offset, bounds| {
                    if let Some(id) = have_popup {
                        Message::Surface(destroy_popup(id))
                    } else {
                        Message::Surface(app_popup::<Cyclone2Applet>(
                            move |state: &mut Cyclone2Applet| {
                                let new_id = Id::unique();
                                state.popup = Some(new_id);
                                let mut settings = state.core.applet.get_popup_settings(
                                    state.core.main_window_id().unwrap(),
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
                });
            row = row.push(btn);
        }

        row.into()
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

        let display_modes: [(DisplayMode, &str); 3] = [
            (DisplayMode::IconOnly, "Icon only"),
            (DisplayMode::IconText, "Icon + text"),
            (DisplayMode::TextOnly, "Text only"),
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

        let content = widget::Column::with_capacity(8)
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
            .push(cosmic::applet::padded_control(
                widget::toggler(self.config.show_controller_icon)
                    .label("Show controller icon".to_string())
                    .on_toggle(Message::ToggleControllerIcon),
            ));

        self.core.applet.popup_container(content).into()
    }

    fn on_close_requested(&self, id: Id) -> Option<Message> {
        Some(Message::PopupClosed(id))
    }

    fn style(&self) -> Option<cosmic::iced::theme::Style> {
        Some(cosmic::applet::style())
    }
}
