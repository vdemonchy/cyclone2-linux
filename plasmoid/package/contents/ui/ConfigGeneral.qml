import QtQuick
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami
import org.kde.kcmutils as KCM

KCM.SimpleKCM {
    id: page

    property string cfg_displayMode
    property string cfg_displayModeDefault: "icon-text"
    property int cfg_pollInterval
    property int cfg_pollIntervalDefault: 60
    property int cfg_lowBatteryThreshold
    property int cfg_lowBatteryThresholdDefault: 20
    property int cfg_levelHighThreshold
    property int cfg_levelHighThresholdDefault: 60
    property int cfg_levelLowThreshold
    property int cfg_levelLowThresholdDefault: 25

    // Owned by ConfigLighting; declared here because Plasma pushes every config
    // key as an initial property to every page and warns about missing ones.
    property bool cfg_rgbEnabled
    property bool cfg_rgbEnabledDefault
    property int cfg_rgbBrightness
    property int cfg_rgbBrightnessDefault
    property var cfg_rgbZones
    property var cfg_rgbZonesDefault

    Kirigami.FormLayout {

        QQC2.ComboBox {
            Kirigami.FormData.label: "Top-bar display:"
            model: [
                { text: "Icon only",  value: "icon-only" },
                { text: "Icon + text", value: "icon-text" }
            ]
            textRole: "text"
            valueRole: "value"
            currentIndex: page.cfg_displayMode === "icon-only" ? 0 : 1
            onActivated: page.cfg_displayMode = currentValue
        }

        QQC2.ComboBox {
            Kirigami.FormData.label: "Battery poll interval:"
            readonly property var values: [10, 30, 60, 300]
            model: ["10 seconds", "30 seconds", "1 minute", "5 minutes"]
            currentIndex: Math.max(0, values.indexOf(page.cfg_pollInterval))
            onActivated: page.cfg_pollInterval = values[currentIndex]
        }

        QQC2.SpinBox {
            Kirigami.FormData.label: "Low battery alert (%):"
            from: 0; to: 50; stepSize: 5
            value: page.cfg_lowBatteryThreshold
            onValueModified: page.cfg_lowBatteryThreshold = value
        }

        Item {
            Kirigami.FormData.isSection: true
            Kirigami.FormData.label: "Battery level colors"
        }

        QQC2.SpinBox {
            id: greenSpin
            Kirigami.FormData.label: "Green at or above (%):"
            from: yellowSpin.value + 5; to: 100; stepSize: 5
            value: page.cfg_levelHighThreshold
            onValueModified: page.cfg_levelHighThreshold = value
        }

        QQC2.SpinBox {
            id: yellowSpin
            Kirigami.FormData.label: "Yellow at or above (%):"
            from: 0; to: greenSpin.value - 5; stepSize: 5
            value: page.cfg_levelLowThreshold
            onValueModified: page.cfg_levelLowThreshold = value
        }
    }
}
