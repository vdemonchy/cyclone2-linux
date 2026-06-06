import Adw from 'gi://Adw';
import Gtk from 'gi://Gtk';
import Gdk from 'gi://Gdk';
import {ExtensionPreferences} from 'resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js';

// LED zones, ordered to match the daemon's protocol.LEDZoneNames.
const ZONE_NAMES = ['Left', 'Right', 'Logo', 'Center'];

function hexToRgba(hex) {
    const rgba = new Gdk.RGBA();
    if (!rgba.parse('#' + hex)) rgba.parse('#ffffff');
    return rgba;
}

function rgbaToHex(rgba) {
    const to8 = (v) => Math.round(Math.max(0, Math.min(1, v)) * 255);
    const h = (n) => n.toString(16).padStart(2, '0');
    return h(to8(rgba.red)) + h(to8(rgba.green)) + h(to8(rgba.blue));
}

export default class Cyclone2BatteryPrefs extends ExtensionPreferences {
    fillPreferencesWindow(window) {
        const settings = this.getSettings();
        const page = new Adw.PreferencesPage();
        const group = new Adw.PreferencesGroup({title: 'Display'});

        const modes = ['icon-only', 'icon-text'];
        const row = new Adw.ComboRow({
            title: 'Top-bar display',
            model: Gtk.StringList.new(['Icon only', 'Icon + text']),
        });
        row.selected = modes.indexOf(settings.get_string('display-mode'));
        row.connect('notify::selected', () => settings.set_string('display-mode', modes[row.selected]));

        group.add(row);

        const intervals = [10, 30, 60, 300];
        const intervalRow = new Adw.ComboRow({
            title: 'Battery poll interval',
            subtitle: 'How often the controller battery is read',
            model: Gtk.StringList.new(['10 seconds', '30 seconds', '1 minute', '5 minutes']),
        });
        const curIdx = intervals.indexOf(settings.get_int('poll-interval'));
        intervalRow.selected = curIdx >= 0 ? curIdx : 2;
        intervalRow.connect('notify::selected', () =>
            settings.set_int('poll-interval', intervals[intervalRow.selected]));
        group.add(intervalRow);

        const thresholdRow = new Adw.SpinRow({
            title: 'Low battery alert',
            subtitle: 'Notify at or below this percentage (0 disables)',
            adjustment: new Gtk.Adjustment({
                lower: 0, upper: 50, step_increment: 5, page_increment: 10,
            }),
        });
        thresholdRow.value = settings.get_int('low-battery-threshold');
        thresholdRow.connect('notify::value', () =>
            settings.set_int('low-battery-threshold', thresholdRow.value));
        group.add(thresholdRow);

        page.add(group);

        const colorGroup = new Adw.PreferencesGroup({
            title: 'Battery level colors',
            description: 'Battery % thresholds for the icon color. Below the yellow threshold is red.',
        });

        const high0 = settings.get_int('level-high-threshold');
        const low0 = settings.get_int('level-low-threshold');

        // Green must stay strictly above yellow: each spinner's bound tracks the
        // other so the constraint can't be crossed from the UI.
        const greenAdj = new Gtk.Adjustment({
            lower: low0 + 5, upper: 100, step_increment: 5, page_increment: 10,
        });
        const yellowAdj = new Gtk.Adjustment({
            lower: 0, upper: high0 - 5, step_increment: 5, page_increment: 10,
        });

        const greenRow = new Adw.SpinRow({
            title: 'Green at or above',
            subtitle: 'High level (%)',
            adjustment: greenAdj,
        });
        greenRow.value = high0;
        greenRow.connect('notify::value', () => {
            settings.set_int('level-high-threshold', greenRow.value);
            yellowAdj.set_upper(greenRow.value - 5);
        });
        colorGroup.add(greenRow);

        const yellowRow = new Adw.SpinRow({
            title: 'Yellow at or above',
            subtitle: 'Medium level (%); below this the icon is red',
            adjustment: yellowAdj,
        });
        yellowRow.value = low0;
        yellowRow.connect('notify::value', () => {
            settings.set_int('level-low-threshold', yellowRow.value);
            greenAdj.set_lower(yellowRow.value + 5);
        });
        colorGroup.add(yellowRow);

        page.add(colorGroup);

        // Controller lighting (RGB). XInput mode only. The switch gates whether
        // the daemon manages the lighting at all.
        const rgbGroup = new Adw.PreferencesGroup({
            title: 'Controller lighting',
            description: 'Per-zone RGB and brightness. Applied only in XInput mode.',
        });

        const enableRow = new Adw.SwitchRow({
            title: 'Control lighting',
            subtitle: 'Let the daemon manage the controller LEDs',
        });
        enableRow.active = settings.get_boolean('rgb-enabled');
        enableRow.connect('notify::active', () =>
            settings.set_boolean('rgb-enabled', enableRow.active));
        rgbGroup.add(enableRow);

        const brightnessRow = new Adw.SpinRow({
            title: 'Brightness',
            subtitle: 'Overall LED brightness (%)',
            adjustment: new Gtk.Adjustment({
                lower: 0, upper: 100, step_increment: 5, page_increment: 10,
            }),
        });
        brightnessRow.value = settings.get_int('rgb-brightness');
        brightnessRow.connect('notify::value', () =>
            settings.set_int('rgb-brightness', brightnessRow.value));
        rgbGroup.add(brightnessRow);

        // Track every RGB sub-control so they can be gated together by the switch.
        const gatedRows = [brightnessRow];

        ZONE_NAMES.forEach((name, i) => {
            const zoneRow = new Adw.ActionRow({title: name});
            const zones = settings.get_strv('rgb-zones');
            const button = new Gtk.ColorDialogButton({
                dialog: new Gtk.ColorDialog({with_alpha: false}),
                valign: Gtk.Align.CENTER,
            });
            button.set_rgba(hexToRgba(zones[i] || 'ffffff'));
            button.connect('notify::rgba', () => {
                const current = settings.get_strv('rgb-zones');
                while (current.length < ZONE_NAMES.length) current.push('ffffff');
                current[i] = rgbaToHex(button.get_rgba());
                settings.set_strv('rgb-zones', current);
            });
            zoneRow.add_suffix(button);
            rgbGroup.add(zoneRow);
            gatedRows.push(zoneRow);
        });

        // Keep brightness and all zone controls greyed out until lighting control
        // is enabled (mirrors the COSMIC applet, which hides them when off).
        const syncSensitive = () => {
            const on = settings.get_boolean('rgb-enabled');
            for (const r of gatedRows) r.sensitive = on;
            rgbGroup.set_description(on
                ? 'Per-zone RGB and brightness. Applied only in XInput mode.'
                : 'Enable "Control lighting" to manage the controller LEDs.');
        };
        settings.connect('changed::rgb-enabled', syncSensitive);
        syncSensitive();

        page.add(rgbGroup);
        window.add(page);
    }
}
