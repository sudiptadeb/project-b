//go:build !android

package main

import "testing"

func TestOzonePlatformFollowsTheSession(t *testing.T) {
	for _, c := range []struct{ wayland, session, x, want string }{
		{"wayland-0", "wayland", ":0", "wayland"},
		{"", "wayland", "", "wayland"},
		{"", "x11", ":0", "x11"},
		{"", "", "", "headless"},
	} {
		t.Setenv("WAYLAND_DISPLAY", c.wayland)
		t.Setenv("XDG_SESSION_TYPE", c.session)
		t.Setenv("DISPLAY", c.x)
		if got := ozonePlatform(); got != c.want {
			t.Errorf("WAYLAND_DISPLAY=%q XDG_SESSION_TYPE=%q DISPLAY=%q: %s, want %s", c.wayland, c.session, c.x, got, c.want)
		}
	}
}
