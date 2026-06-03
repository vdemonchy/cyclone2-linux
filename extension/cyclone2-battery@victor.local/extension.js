import GObject from 'gi://GObject';
import St from 'gi://St';
import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import Clutter from 'gi://Clutter';

import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import * as PanelMenu from 'resource:///org/gnome/shell/ui/panelMenu.js';
import * as PopupMenu from 'resource:///org/gnome/shell/ui/popupMenu.js';
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';

const STATE_PATH = GLib.build_filenamev([GLib.get_user_runtime_dir(), 'cyclone2-battery.json']);

const Indicator = GObject.registerClass(
class Indicator extends PanelMenu.Button {
    _init(settings) {
        super._init(0.0, 'Cyclone 2 Battery');
        this._settings = settings;

        const box = new St.BoxLayout({style_class: 'panel-status-menu-box'});
        this._controllerIcon = new St.Icon({icon_name: 'input-gaming-symbolic', style_class: 'system-status-icon'});
        this._icon = new St.Icon({icon_name: 'battery-missing-symbolic', style_class: 'system-status-icon'});
        this._label = new St.Label({y_align: Clutter.ActorAlign.CENTER, text: ''});
        box.add_child(this._controllerIcon);
        box.add_child(this._icon);
        box.add_child(this._label);
        this.add_child(box);

        this.track_hover = true;
        this._lastState = null;
        this._tooltip = new St.Label({style_class: 'dash-label', text: ''});
        this._tooltip.hide();
        Main.layoutManager.addChrome(this._tooltip);
        this._hoverId = this.connect('notify::hover', () => this._onHover());

        this._modeItem = new PopupMenu.PopupMenuItem('Mode: —', {reactive: false});
        this._batteryItem = new PopupMenu.PopupMenuItem('Battery: —', {reactive: false});
        this.menu.addMenuItem(this._modeItem);
        this.menu.addMenuItem(this._batteryItem);

        this._modeId = this._settings.connect('changed::display-mode', () => this._applyMode());
        this._applyMode();
        this._ctrlIconId = this._settings.connect('changed::show-controller-icon', () => this._applyControllerIcon());
        this._applyControllerIcon();
    }

    _applyMode() {
        const mode = this._settings.get_string('display-mode');
        this._icon.visible = mode !== 'text-only';
        this._label.visible = mode !== 'icon-only';
    }

    _applyControllerIcon() {
        if (this._controllerIcon)
            this._controllerIcon.visible = this._settings.get_boolean('show-controller-icon');
    }

    update(state) {
        this._lastState = state;
        this._updateMenu(state);
        if (!state || !state.present) {
            this.visible = false;
            if (this._tooltip) this._tooltip.hide();
            return;
        }
        if (state.battery_known === false) {
            this.visible = true;
            this._label.text = '';
            this._icon.icon_name = 'battery-missing-symbolic';
            return;
        }
        this.visible = true;
        if (state.stale) {
            this._label.text = `${state.percent}%?`;
            this._icon.icon_name = 'battery-missing-symbolic';
            return;
        }
        this._label.text = state.level ? state.level : `${state.percent}%`;
        this._icon.icon_name = this._iconFor(state);
    }

    _iconFor(state) {
        const p = state.percent;
        const lvl = p >= 90 ? 'full' : p >= 60 ? 'good' : p >= 30 ? 'low' : p >= 10 ? 'caution' : 'empty';
        return state.charging ? `battery-${lvl}-charging-symbolic` : `battery-${lvl}-symbolic`;
    }

    _onHover() {
        if (!this._tooltip) return;
        if (this.hover && this._lastState && this._lastState.present) {
            this._tooltip.text = this._tooltipText(this._lastState);
            const [x, y] = this.get_transformed_position();
            this._tooltip.set_position(Math.round(x), Math.round(y + this.height + 4));
            this._tooltip.show();
        } else {
            this._tooltip.hide();
        }
    }

    _tooltipText() {
        return 'GameSir Cyclone 2';
    }

    _updateMenu(state) {
        if (!this._modeItem) return;
        const names = {xinput: 'XInput', ds4: 'DS4', switch: 'Switch', hid: 'HID', unknown: 'Unknown'};
        if (!state || !state.present) {
            this._modeItem.label.text = 'Mode: disconnected';
            this._batteryItem.label.text = 'Battery: —';
            return;
        }
        this._modeItem.label.text = `Mode: ${names[state.mode] || state.mode || 'Unknown'}`;
        if (state.battery_known === false) {
            this._batteryItem.label.text = 'Battery: unavailable';
            return;
        }
        let batt = state.level ? state.level : `${state.percent}%`;
        if (state.charging) batt += ' (charging)';
        if (state.stale) batt += ' (stale)';
        this._batteryItem.label.text = `Battery: ${batt}`;
    }

    cleanup() {
        if (this._modeId) {
            this._settings.disconnect(this._modeId);
            this._modeId = 0;
        }
        if (this._ctrlIconId) {
            this._settings.disconnect(this._ctrlIconId);
            this._ctrlIconId = 0;
        }
        if (this._hoverId) {
            this.disconnect(this._hoverId);
            this._hoverId = 0;
        }
        if (this._tooltip) {
            Main.layoutManager.removeChrome(this._tooltip);
            this._tooltip.destroy();
            this._tooltip = null;
        }
        this._modeItem = null;
        this._batteryItem = null;
    }
});

export default class Cyclone2BatteryExtension extends Extension {
    enable() {
        this._settings = this.getSettings();
        this._indicator = new Indicator(this._settings);
        Main.panel.addToStatusArea(this.uuid, this._indicator);
        this._intervalId = this._settings.connect('changed::poll-interval', () => this._writeConfig());
        this._writeConfig();

        this._file = Gio.File.new_for_path(STATE_PATH);
        this._monitor = this._file.monitor(Gio.FileMonitorFlags.NONE, null);
        this._monitorId = this._monitor.connect('changed', () => this._refresh());
        this._timer = GLib.timeout_add_seconds(GLib.PRIORITY_DEFAULT, 30, () => {
            this._refresh();
            return GLib.SOURCE_CONTINUE;
        });
        this._refresh();
    }

    _refresh() {
        try {
            const [ok, contents] = this._file.load_contents(null);
            if (!ok) return;
            const state = JSON.parse(new TextDecoder().decode(contents));
            this._indicator.update(state);
        } catch (_e) {
            this._indicator.update({present: false});
        }
    }

    _writeConfig() {
        try {
            const dir = GLib.build_filenamev([GLib.get_user_config_dir(), 'cyclone2-battery']);
            GLib.mkdir_with_parents(dir, 0o755);
            const path = GLib.build_filenamev([dir, 'config.json']);
            const seconds = this._settings.get_int('poll-interval');
            const data = JSON.stringify({interval_seconds: seconds});
            Gio.File.new_for_path(path).replace_contents(
                new TextEncoder().encode(data), null, false,
                Gio.FileCreateFlags.REPLACE_DESTINATION, null);
        } catch (e) {
            logError(e, 'cyclone2-battery: failed to write config.json');
        }
    }

    disable() {
        if (this._timer) {
            GLib.source_remove(this._timer);
            this._timer = 0;
        }
        if (this._monitor && this._monitorId) {
            this._monitor.disconnect(this._monitorId);
            this._monitorId = 0;
            this._monitor.cancel();
        }
        this._monitor = null;
        this._file = null;
        if (this._indicator) {
            this._indicator.cleanup();
            this._indicator.destroy();
            this._indicator = null;
        }
        if (this._intervalId) {
            this._settings.disconnect(this._intervalId);
            this._intervalId = 0;
        }
        this._settings = null;
    }
}
