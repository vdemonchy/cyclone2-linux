import Adw from 'gi://Adw';
import Gtk from 'gi://Gtk';
import {ExtensionPreferences} from 'resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js';

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
        window.add(page);
    }
}
