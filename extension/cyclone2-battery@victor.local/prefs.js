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

        page.add(group);
        window.add(page);
    }
}
