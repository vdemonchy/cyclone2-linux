mod app;
mod config;
mod state;
mod watcher;

fn main() -> cosmic::iced::Result {
    cosmic::applet::run::<app::Cyclone2Applet>(())
}
