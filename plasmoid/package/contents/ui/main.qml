import QtQuick
import Qt.labs.platform as Platform
import org.kde.plasma.plasmoid
import org.kde.plasma.core as PlasmaCore

PlasmoidItem {
    id: root

    // Parsed daemon state file. Default = no controller.
    property var ctrl: ({ present: false })

    readonly property string stateFileUrl:
        Platform.StandardPaths.writableLocation(Platform.StandardPaths.RuntimeLocation)
        + "/cyclone2-linux.json"

    // QML has no native file-watch; a 1s poll on a few-hundred-byte file is cheap
    // and keeps hotplug / mode changes feeling instant.
    Timer {
        interval: 1000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: root.readState()
    }

    function readState() {
        var xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function() {
            if (xhr.readyState !== XMLHttpRequest.DONE)
                return;
            try {
                if (!xhr.responseText)
                    throw "empty";
                root.ctrl = JSON.parse(xhr.responseText);
            } catch (e) {
                root.ctrl = { present: false };
            }
            root.applyStatus();
        };
        try {
            xhr.open("GET", root.stateFileUrl);
            xhr.send();
        } catch (e) {
            root.ctrl = { present: false };
            root.applyStatus();
        }
    }

    // Hide the plasmoid from the panel entirely when no controller is present
    // (mirrors the GNOME extension's `this.visible = false`).
    function applyStatus() {
        root.Plasmoid.status = (root.ctrl && root.ctrl.present)
            ? PlasmaCore.Types.ActiveStatus
            : PlasmaCore.Types.HiddenStatus;
    }

    function modeName(mode) {
        switch (mode) {
        case "xinput": return "XInput";
        case "ds4":    return "DS4";
        case "switch": return "Switch";
        case "hid":    return "HID";
        default:       return mode ? mode : "Unknown";
        }
    }

    toolTipMainText: "GameSir Cyclone 2"
    toolTipSubText: (root.ctrl && root.ctrl.present) ? root.modeName(root.ctrl.mode) : ""

    compactRepresentation: CompactRepresentation { ctrl: root.ctrl }
    fullRepresentation: FullRepresentation { ctrl: root.ctrl }
}
