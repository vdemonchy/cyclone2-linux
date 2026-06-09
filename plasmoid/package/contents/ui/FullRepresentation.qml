import QtQuick
import QtQuick.Layouts
import org.kde.kirigami as Kirigami
import org.kde.plasma.components as PlasmaComponents
import org.kde.plasma.plasmoid

ColumnLayout {
    id: full

    // Injected from main.qml.
    property var ctrl: ({ present: false })

    Layout.minimumWidth: Kirigami.Units.gridUnit * 14
    Layout.minimumHeight: Kirigami.Units.gridUnit * 7
    spacing: Kirigami.Units.smallSpacing

    function modeName(m) {
        switch (m) {
        case "xinput": return "XInput";
        case "ds4":    return "DS4";
        case "switch": return "Switch";
        case "hid":    return "HID";
        default:       return m ? m : "Unknown";
        }
    }

    function batteryText() {
        if (!ctrl || !ctrl.present) return "—";
        if (ctrl.battery_known === false) return "unavailable";
        var b = ctrl.level ? ctrl.level : (ctrl.percent + "%");
        b += ctrl.charging ? " — Charging" : " — On battery";
        if (ctrl.stale) b += " (stale)";
        return b;
    }

    Kirigami.Heading {
        level: 3
        text: "GameSir Cyclone 2"
    }

    GridLayout {
        columns: 2
        columnSpacing: Kirigami.Units.largeSpacing
        PlasmaComponents.Label { text: "Mode:"; font.bold: true }
        PlasmaComponents.Label {
            text: (full.ctrl && full.ctrl.present) ? full.modeName(full.ctrl.mode) : "disconnected"
        }
        PlasmaComponents.Label { text: "Battery:"; font.bold: true }
        PlasmaComponents.Label { text: full.batteryText() }
    }

    Item { Layout.fillHeight: true }

    PlasmaComponents.Button {
        Layout.alignment: Qt.AlignRight
        icon.name: "configure"
        text: "Configure…"
        onClicked: Plasmoid.internalAction("configure").trigger()
    }
}
