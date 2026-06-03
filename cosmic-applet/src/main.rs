// Deviation from task template: uses cosmic::SingleThreadExecutor (not cosmic::executor::Default)
// and wraps the icon button with Element::from() — both match the actual libcosmic API at
// rev f0f68933f1552857e2165fc0fa953228107bddef (confirmed from examples/applet/src/window.rs).

use cosmic::app::{Core, Task};
use cosmic::Element;

fn main() -> cosmic::iced::Result {
    cosmic::applet::run::<Cyclone2Applet>(())
}

struct Cyclone2Applet {
    core: Core,
}

#[derive(Debug, Clone)]
enum Message {}

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
        (Cyclone2Applet { core }, Task::none())
    }

    fn update(&mut self, _message: Message) -> Task<Message> {
        Task::none()
    }

    fn view(&self) -> Element<'_, Message> {
        Element::from(self.core.applet.icon_button("input-gaming-symbolic"))
    }
}
