// Package notify posts desktop notifications through the freedesktop D-Bus
// service. It shells out to `gdbus` (part of glib, present on any GNOME/COSMIC
// session) so the daemon stays dependency-free — no D-Bus Go module needed.
package notify

import "os/exec"

// AppName is shown to the notification server as the originating application.
const AppName = "GameSir Cyclone 2"

// Send posts a notification with the given themed icon name, summary and body.
// It is best-effort: a missing `gdbus`, missing notification daemon, or absent
// session bus all surface as an error for the caller to log and otherwise
// ignore. Notifications use critical urgency so the desktop keeps them visible
// until dismissed (matching how the OS surfaces a low battery).
func Send(icon, summary, body string) error {
	path, err := exec.LookPath("gdbus")
	if err != nil {
		return err
	}
	// org.freedesktop.Notifications.Notify signature: (susssasa{sv}i) —
	// app_name, replaces_id, app_icon, summary, body, actions, hints, timeout.
	cmd := exec.Command(path, "call", "--session",
		"--dest", "org.freedesktop.Notifications",
		"--object-path", "/org/freedesktop/Notifications",
		"--method", "org.freedesktop.Notifications.Notify",
		AppName,
		"0",
		icon,
		summary,
		body,
		"[]",
		"{'urgency': <byte 2>}",
		"0",
	)
	return cmd.Run()
}
