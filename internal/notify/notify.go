// Package notify wraps cross-platform desktop notifications via
// github.com/gen2brain/beeep. Pure-Go on Windows/Linux (no cgo);
// uses NSUserNotification on macOS (the upstream library handles that).
//
// Wachat fires a notification for incoming messages when:
//   - the message is not from the local user, and
//   - the originating chat is not currently selected, and
//   - the chat is not muted.
//
// The trigger logic lives in main.go; this package just exposes the
// thin Send call so the call site stays portable.
package notify

import (
	"github.com/gen2brain/beeep"
)

// Send fires a desktop notification with the given title and body.
// Errors are swallowed — desktop notifications are best-effort and
// the user should not lose messages because the OS toast subsystem
// is unhappy.
func Send(title, body string) {
	// beeep.Notify(title, body, iconPath) — empty iconPath uses the
	// caller's default icon (Gio doesn't ship one; OS fallback applies).
	_ = beeep.Notify(title, body, "")
}
