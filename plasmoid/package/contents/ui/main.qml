import QtQuick
import Qt.labs.platform as Platform
import org.kde.plasma.plasmoid
import org.kde.plasma.core as PlasmaCore
import org.kde.plasma.plasma5support as Plasma5Support

PlasmoidItem {
    id: root

    // Parsed daemon state file. Default = no controller.
    property var ctrl: ({ present: false })

    readonly property string stateFilePath:
        Platform.StandardPaths.writableLocation(Platform.StandardPaths.RuntimeLocation)
        .toString().replace(/^file:\/\//, "") + "/cyclone2-linux.json"

    readonly property string configDirPath:
        Platform.StandardPaths.writableLocation(Platform.StandardPaths.GenericConfigLocation)
        .toString().replace(/^file:\/\//, "") + "/cyclone2-linux"

    // QML has no native file-watch, and XHR on file:// URLs is blocked in Qt 6
    // (QML_XHR_ALLOW_FILE_READ), so poll the state file through the executable
    // engine; a 1s cat of a few-hundred-byte file is cheap and keeps hotplug /
    // mode changes feeling instant. A missing file (daemon stopped) is a normal
    // condition, hence no exit-code warning here unlike the config writer below.
    Plasma5Support.DataSource {
        engine: "executable"
        interval: 1000
        connectedSources: ["cat '" + root.stateFilePath + "' 2>/dev/null"]
        onNewData: function(source, data) {
            try {
                if (!data.stdout)
                    throw "empty";
                root.ctrl = JSON.parse(data.stdout);
            } catch (e) {
                root.ctrl = { present: false };
            }
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

    // Runs short shell commands (config.json writes). Plasma's executable engine
    // is the only way to write a file from a sandboxed plasmoid.
    Plasma5Support.DataSource {
        id: runner
        engine: "executable"
        connectedSources: []
        onNewData: function(source, data) {
            disconnectSource(source);
            if (data["exit code"] !== 0)
                console.warn("cyclone2: config.json write failed:", data["stderr"]);
        }
        function run(cmd) { connectSource(cmd); }
    }

    // Serialize the daemon-relevant settings to ~/.config/cyclone2-linux/config.json.
    // rgb is always emitted, enabled or not: enabled=false is what tells the daemon
    // to turn the controller's LEDs off. The level thresholds ride along so the
    // daemon can colour the logo zone like the panel icon (matches the GNOME
    // extension).
    function writeConfig() {
        var cfg = {
            interval_seconds: Plasmoid.configuration.pollInterval,
            low_battery_threshold: Plasmoid.configuration.lowBatteryThreshold,
            rgb: {
                enabled: Plasmoid.configuration.rgbEnabled,
                brightness: Plasmoid.configuration.rgbBrightness,
                zones: Plasmoid.configuration.rgbZones,
                battery_logo: Plasmoid.configuration.rgbBatteryLogo,
                level_high: Plasmoid.configuration.levelHighThreshold,
                level_low: Plasmoid.configuration.levelLowThreshold
            }
        };
        var b64 = Qt.btoa(JSON.stringify(cfg));
        var dir = root.configDirPath;
        var path = dir + "/config.json";
        // base64 avoids shell-escaping the hex colours; temp file + mv = atomic
        // (the QML analog of GNOME's replace_contents).
        var cmd = "mkdir -p '" + dir + "' && printf %s '" + b64 +
                  "' | base64 -d > '" + path + ".tmp' && mv '" + path + ".tmp' '" + path + "'";
        runner.run(cmd);
    }

    Connections {
        target: Plasmoid.configuration
        function onPollIntervalChanged()        { root.writeConfig(); }
        function onLowBatteryThresholdChanged() { root.writeConfig(); }
        function onRgbEnabledChanged()          { root.writeConfig(); }
        function onRgbBrightnessChanged()       { root.writeConfig(); }
        function onRgbZonesChanged()            { root.writeConfig(); }
        function onRgbBatteryLogoChanged()      { root.writeConfig(); }
        // The level thresholds tint the panel icon *and* the controller's logo
        // zone, so a change has to reach the daemon too.
        function onLevelHighThresholdChanged()  { root.writeConfig(); }
        function onLevelLowThresholdChanged()   { root.writeConfig(); }
    }

    Component.onCompleted: root.writeConfig()

    toolTipMainText: "GameSir Cyclone 2"
    toolTipSubText: (root.ctrl && root.ctrl.present) ? root.modeName(root.ctrl.mode) : ""

    compactRepresentation: CompactRepresentation { ctrl: root.ctrl }
    fullRepresentation: FullRepresentation { ctrl: root.ctrl }
}
