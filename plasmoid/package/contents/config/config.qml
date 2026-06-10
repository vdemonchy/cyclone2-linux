import QtQuick
import org.kde.plasma.configuration

ConfigModel {
    ConfigCategory {
        name: "General"
        icon: "input-gamepad-symbolic"
        source: "ConfigGeneral.qml"
    }
    ConfigCategory {
        name: "Lighting"
        icon: "color-management"
        source: "ConfigLighting.qml"
    }
}
