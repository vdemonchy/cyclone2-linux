import QtQuick
import org.kde.plasma.configuration

ConfigModel {
    ConfigCategory {
        name: "General"
        icon: "input-gaming-symbolic"
        source: "ConfigGeneral.qml"
    }
    ConfigCategory {
        name: "Lighting"
        icon: "color-management"
        source: "ConfigLighting.qml"
    }
}
