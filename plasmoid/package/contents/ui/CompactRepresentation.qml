import QtQuick
import org.kde.kirigami as Kirigami
import org.kde.plasma.components as PlasmaComponents
import org.kde.plasma.plasmoid

MouseArea {
    id: compact

    // Injected from main.qml at instantiation.
    property var ctrl: ({ present: false })

    onClicked: Plasmoid.expanded = !Plasmoid.expanded

    // --- derived display state ---
    readonly property bool present: ctrl && ctrl.present === true
    readonly property bool batteryKnown: present && ctrl.battery_known !== false
    readonly property bool stale: present && ctrl.stale === true
    readonly property bool charging: batteryKnown && !stale && ctrl.charging === true

    readonly property string labelText: {
        if (!present || !batteryKnown) return "";
        if (stale) return ctrl.percent + "%?";
        return ctrl.level ? ctrl.level : (ctrl.percent + "%");
    }

    readonly property var tintColor: {
        if (!present || !batteryKnown || stale) return Kirigami.Theme.textColor;
        var p = ctrl.percent;
        var high = Plasmoid.configuration.levelHighThreshold;
        var low = Plasmoid.configuration.levelLowThreshold;
        if (p >= high) return "#2ec27e";
        if (p >= low) return "#f5c211";
        return "#e01b24";
    }

    Row {
        anchors.centerIn: parent
        spacing: Kirigami.Units.smallSpacing

        Kirigami.Icon {
            id: icon
            // Breeze has no input-gaming-symbolic (that's the GNOME name) and
            // silently falls back to the full-color input-gaming, which cannot
            // be tinted. isMask forces the tint regardless of icon theme.
            source: "input-gamepad-symbolic"
            fallback: "input-gaming-symbolic"
            isMask: true
            color: compact.tintColor
            width: Kirigami.Units.iconSizes.smallMedium
            height: width
            anchors.verticalCenter: parent.verticalCenter

            // Charging pulse: opacity 1.0<->0.4, 1s each way, sine easing, looping
            // (same curve as the GNOME/COSMIC pulse).
            SequentialAnimation {
                running: compact.charging
                loops: Animation.Infinite
                alwaysRunToEnd: true
                NumberAnimation { target: icon; property: "opacity"; from: 1.0; to: 0.4; duration: 1000; easing.type: Easing.InOutSine }
                NumberAnimation { target: icon; property: "opacity"; from: 0.4; to: 1.0; duration: 1000; easing.type: Easing.InOutSine }
                onRunningChanged: if (!running) icon.opacity = 1.0
            }
        }

        PlasmaComponents.Label {
            anchors.verticalCenter: parent.verticalCenter
            visible: compact.labelText.length > 0 && Plasmoid.configuration.displayMode !== "icon-only"
            text: compact.labelText
        }
    }
}
