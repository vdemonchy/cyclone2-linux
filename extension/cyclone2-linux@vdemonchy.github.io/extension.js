import GObject from 'gi://GObject';
import St from 'gi://St';
import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import Clutter from 'gi://Clutter';

import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import * as PanelMenu from 'resource:///org/gnome/shell/ui/panelMenu.js';
import * as PopupMenu from 'resource:///org/gnome/shell/ui/popupMenu.js';
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';

const STATE_PATH = GLib.build_filenamev([GLib.get_user_runtime_dir(), 'cyclone2-linux.json']);

const Indicator = GObject.registerClass(
class Indicator extends PanelMenu.Button {
    _init(settings, openPrefs) {
        super._init(0.0, 'Cyclone 2');
        this._settings = settings;

        const box = new St.BoxLayout({style_class: 'panel-status-menu-box'});
        this._controllerIcon = new St.Icon({icon_name: 'input-gaming-symbolic', style_class: 'system-status-icon'});
        this._label = new St.Label({y_align: Clutter.ActorAlign.CENTER, text: ''});
        box.add_child(this._controllerIcon);
        box.add_child(this._label);
        this.add_child(box);

        this.track_hover = true;
        this._lastState = null;
        this._tooltip = new St.Label({style_class: 'dash-label', text: ''});
        this._tooltip.hide();
        Main.layoutManager.addChrome(this._tooltip);
        this._hoverId = this.connect('notify::hover', () => this._onHover());

        this._modeItem = new PopupMenu.PopupImageMenuItem('Cyclone 2 mode: —', 'input-gaming-symbolic', {reactive: false});
        this._batteryItem = new PopupMenu.PopupImageMenuItem('Battery: —', 'battery-symbolic', {reactive: false});
        this.menu.addMenuItem(this._modeItem);
        this.menu.addMenuItem(this._batteryItem);

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());
        const settingsItem = new PopupMenu.PopupImageMenuItem('Settings', 'preferences-system-symbolic');
        settingsItem.connect('activate', () => openPrefs());
        this.menu.addMenuItem(settingsItem);

        this._modeId = this._settings.connect('changed::display-mode', () => this._applyMode());
        this._applyMode();
        this._levelHiId = this._settings.connect('changed::level-high-threshold', () => this._reapply());
        this._levelLoId = this._settings.connect('changed::level-low-threshold', () => this._reapply());
    }

    // Re-render from the last state (used when level-colour thresholds change).
    _reapply() {
        if (this._lastState) this.update(this._lastState);
    }

    _applyMode() {
        const mode = this._settings.get_string('display-mode');
        this._label.visible = mode !== 'icon-only';
    }

    update(state) {
        this._lastState = state;
        this._updateMenu(state);
        if (!state || !state.present) {
            this.visible = false;
            this._setChargingPulse(false);
            if (this._tooltip) this._tooltip.hide();
            return;
        }
        this.visible = true;
        if (state.battery_known === false) {
            this._label.text = '';
            this._setIconColor(null);
            this._setChargingPulse(false);
            return;
        }
        if (state.stale) {
            this._label.text = `${state.percent}%?`;
            this._setIconColor(null);
            this._setChargingPulse(false);
            return;
        }
        this._label.text = state.level ? state.level : `${state.percent}%`;
        this._setIconColor(this._colorFor(state.percent));
        this._setChargingPulse(!!state.charging);
    }

    // Tint the controller icon by battery level: green (high) / yellow (medium)
    // / red (low). A null colour clears the tint so the icon follows the theme's
    // default top-bar foreground (used when the level is unknown: stale / no
    // battery source).
    _colorFor(percent) {
        const high = this._settings.get_int('level-high-threshold');
        const low = this._settings.get_int('level-low-threshold');
        if (percent >= high) return '#2ec27e';
        if (percent >= low) return '#f5c211';
        return '#e01b24';
    }

    _setIconColor(color) {
        if (this._controllerIcon)
            this._controllerIcon.style = color ? `color: ${color};` : null;
    }

    // Pulse the controller icon's opacity while charging (a looping, auto-reversing
    // transition), and clear it otherwise.
    _setChargingPulse(on) {
        if (!this._controllerIcon) return;
        if (on) {
            if (this._controllerIcon.get_transition('charge-pulse')) return;
            // ~40% floor, ~2s full cycle (1000ms each way, auto-reversed), sine
            // easing — kept in sync with the COSMIC applet's breathing pulse.
            const t = new Clutter.PropertyTransition({property_name: 'opacity'});
            t.set_from(255);
            t.set_to(102);
            t.set_duration(1000);
            t.set_auto_reverse(true);
            t.set_repeat_count(-1);
            t.set_progress_mode(Clutter.AnimationMode.EASE_IN_OUT_SINE);
            this._controllerIcon.add_transition('charge-pulse', t);
        } else {
            this._controllerIcon.remove_transition('charge-pulse');
            this._controllerIcon.opacity = 255;
        }
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
            this._modeItem.label.text = 'Cyclone 2 mode: disconnected';
            this._batteryItem.label.text = 'Battery: —';
            return;
        }
        this._modeItem.label.text = `Cyclone 2 mode: ${names[state.mode] || state.mode || 'Unknown'}`;
        if (state.battery_known === false) {
            this._batteryItem.label.text = 'Battery: unavailable';
            return;
        }
        let batt = state.level ? state.level : `${state.percent}%`;
        batt += state.charging ? ' — Charging' : ' — On battery';
        if (state.stale) batt += ' (stale)';
        this._batteryItem.label.text = `Battery: ${batt}`;
    }

    cleanup() {
        this._setChargingPulse(false);
        if (this._modeId) {
            this._settings.disconnect(this._modeId);
            this._modeId = 0;
        }
        if (this._levelHiId) {
            this._settings.disconnect(this._levelHiId);
            this._levelHiId = 0;
        }
        if (this._levelLoId) {
            this._settings.disconnect(this._levelLoId);
            this._levelLoId = 0;
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

export default class Cyclone2Extension extends Extension {
    enable() {
        this._settings = this.getSettings();
        this._indicator = new Indicator(this._settings, () => this.openPreferences());
        Main.panel.addToStatusArea(this.uuid, this._indicator);
        this._intervalId = this._settings.connect('changed::poll-interval', () => this._writeConfig());
        this._thresholdId = this._settings.connect('changed::low-battery-threshold', () => this._writeConfig());
        this._rgbEnabledId = this._settings.connect('changed::rgb-enabled', () => this._writeConfig());
        this._rgbBrightnessId = this._settings.connect('changed::rgb-brightness', () => this._writeConfig());
        this._rgbZonesId = this._settings.connect('changed::rgb-zones', () => this._writeConfig());
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
            const dir = GLib.build_filenamev([GLib.get_user_config_dir(), 'cyclone2-linux']);
            GLib.mkdir_with_parents(dir, 0o755);
            const path = GLib.build_filenamev([dir, 'config.json']);
            const seconds = this._settings.get_int('poll-interval');
            const threshold = this._settings.get_int('low-battery-threshold');
            const config = {interval_seconds: seconds, low_battery_threshold: threshold};
            // Only emit rgb when the user opted in, so battery-only setups leave
            // the controller's lighting untouched.
            if (this._settings.get_boolean('rgb-enabled')) {
                config.rgb = {
                    brightness: this._settings.get_int('rgb-brightness'),
                    zones: this._settings.get_strv('rgb-zones'),
                };
            }
            const data = JSON.stringify(config);
            Gio.File.new_for_path(path).replace_contents(
                new TextEncoder().encode(data), null, false,
                Gio.FileCreateFlags.REPLACE_DESTINATION, null);
        } catch (e) {
            logError(e, 'cyclone2-linux: failed to write config.json');
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
        if (this._thresholdId) {
            this._settings.disconnect(this._thresholdId);
            this._thresholdId = 0;
        }
        for (const id of ['_rgbEnabledId', '_rgbBrightnessId', '_rgbZonesId']) {
            if (this[id]) {
                this._settings.disconnect(this[id]);
                this[id] = 0;
            }
        }
        this._settings = null;
    }
}
