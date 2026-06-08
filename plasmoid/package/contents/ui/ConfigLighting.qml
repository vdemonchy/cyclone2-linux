import QtQuick
import QtQuick.Controls as QQC2
import QtQuick.Layouts
import QtQuick.Dialogs
import Qt.labs.platform as Platform
import org.kde.kirigami as Kirigami
import org.kde.kcmutils as KCM

KCM.SimpleKCM {
    id: page

    property bool cfg_rgbEnabled
    property bool cfg_rgbEnabledDefault: false
    property int cfg_rgbBrightness
    property int cfg_rgbBrightnessDefault: 100
    property var cfg_rgbZones
    property var cfg_rgbZonesDefault: ["ffffff", "ffffff", "ffffff", "ffffff"]

    readonly property var zoneNames: ["Left", "Right", "Logo", "Center"]

    property string controllerMode: ""
    readonly property bool isXInput: controllerMode === "xinput"

    readonly property string stateFileUrl:
        Platform.StandardPaths.writableLocation(Platform.StandardPaths.RuntimeLocation)
        + "/cyclone2-linux.json"

    Timer {
        interval: 1000; running: true; repeat: true; triggeredOnStart: true
        onTriggered: page.readMode()
    }

    function readMode() {
        var xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function() {
            if (xhr.readyState !== XMLHttpRequest.DONE) return;
            try {
                var s = JSON.parse(xhr.responseText);
                page.controllerMode = (s && s.present) ? (s.mode || "") : "";
            } catch (e) {
                page.controllerMode = "";
            }
        };
        try { xhr.open("GET", page.stateFileUrl); xhr.send(); }
        catch (e) { page.controllerMode = ""; }
    }

    function modeLabel(m) {
        switch (m) {
        case "xinput": return "XInput";
        case "ds4":    return "DS4";
        case "switch": return "Switch";
        case "hid":    return "HID";
        default:       return m ? m : "";
        }
    }

    function hexToColor(hex) { return "#" + (hex && hex.length === 6 ? hex : "ffffff"); }
    function colorToHex(c) {
        function h(v) { var s = Math.round(v * 255).toString(16); return s.length === 1 ? "0" + s : s; }
        return h(c.r) + h(c.g) + h(c.b);
    }
    function zoneHex(i) {
        return (cfg_rgbZones && cfg_rgbZones[i]) ? cfg_rgbZones[i] : "ffffff";
    }
    function setZone(i, hex) {
        var z = cfg_rgbZones ? cfg_rgbZones.slice() : [];
        while (z.length < 4) z.push("ffffff");
        z[i] = hex;
        cfg_rgbZones = z;
    }

    ColumnLayout {
        spacing: Kirigami.Units.smallSpacing

        Kirigami.InlineMessage {
            Layout.fillWidth: true
            visible: !page.isXInput
            type: Kirigami.MessageType.Information
            text: {
                var where = page.controllerMode
                    ? (page.modeLabel(page.controllerMode) + " mode")
                    : "no controller connected";
                return "Unavailable (" + where + "). RGB control works only in XInput mode (USB 3537:100b).";
            }
        }

        Kirigami.FormLayout {
            Layout.fillWidth: true
            enabled: page.isXInput

            QQC2.Switch {
                id: enableSwitch
                Kirigami.FormData.label: "Control lighting:"
                checked: page.cfg_rgbEnabled
                onToggled: page.cfg_rgbEnabled = checked
            }

            QQC2.SpinBox {
                Kirigami.FormData.label: "Brightness (%):"
                enabled: enableSwitch.checked
                from: 0; to: 100; stepSize: 5
                value: page.cfg_rgbBrightness
                onValueModified: page.cfg_rgbBrightness = value
            }

            Repeater {
                model: page.zoneNames
                delegate: QQC2.Button {
                    required property int index
                    required property string modelData
                    Kirigami.FormData.label: modelData + ":"
                    enabled: enableSwitch.checked
                    implicitWidth: Kirigami.Units.gridUnit * 4
                    contentItem: Rectangle {
                        radius: 3
                        color: page.hexToColor(page.zoneHex(index))
                        border.color: Kirigami.Theme.textColor
                        border.width: 1
                    }
                    onClicked: {
                        colorDialog.zoneIndex = index;
                        colorDialog.selectedColor = page.hexToColor(page.zoneHex(index));
                        colorDialog.open();
                    }
                }
            }
        }
    }

    ColorDialog {
        id: colorDialog
        property int zoneIndex: 0
        onAccepted: page.setZone(zoneIndex, page.colorToHex(selectedColor))
    }
}
