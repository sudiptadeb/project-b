package commands

import (
	"fmt"
	"strings"

	"projectb/engine"
)

// Platforms, as the keymap file names them.
const (
	Mac     = "mac"
	Windows = "windows"
	Linux   = "linux"
)

// keyCodes are the key names a keymap may use, with their Windows virtual-key codes.
// Punctuation means the key that types it on a US keyboard.
var keyCodes = map[string]int{
	"Backspace": 0x08, "Tab": 0x09, "Enter": 0x0D, "Escape": 0x1B, "Space": 0x20,
	"PageUp": 0x21, "PageDown": 0x22, "End": 0x23, "Home": 0x24,
	"Left": 0x25, "Up": 0x26, "Right": 0x27, "Down": 0x28,
	"Insert": 0x2D, "Delete": 0x2E,
	";": 0xBA, "=": 0xBB, ",": 0xBC, "-": 0xBD, ".": 0xBE, "/": 0xBF, "`": 0xC0,
	"[": 0xDB, "\\": 0xDC, "]": 0xDD, "'": 0xDE,
}

// keyNames is keyCodes the other way round, with letters, digits and F keys added.
var keyNames = map[int]string{}

func init() {
	for c := 'A'; c <= 'Z'; c++ {
		keyCodes[string(c)] = int(c)
	}
	for c := '0'; c <= '9'; c++ {
		keyCodes[string(c)] = int(c)
	}
	for i := 1; i <= 24; i++ {
		keyCodes[fmt.Sprintf("F%d", i)] = 0x6F + i
	}
	for name, code := range keyCodes {
		keyNames[code] = name
	}
}

// modNames are the modifiers in the one order a combination is written in. Option is
// the Mac's name for Alt; either is read on any platform.
var modNames = []struct {
	mod        engine.Mods
	mac, other string
}{
	{engine.ModCtrl, "Ctrl", "Ctrl"},
	{engine.ModAlt, "Option", "Alt"},
	{engine.ModShift, "Shift", "Shift"},
	{engine.ModCmd, "Cmd", "Win"},
}

// ParseKeys reads a combination such as "Cmd+Shift+T".
func ParseKeys(s string) (engine.Key, error) {
	parts := strings.Split(s, "+")
	var k engine.Key
	for i, p := range parts {
		if i == len(parts)-1 {
			code, ok := keyCodes[p]
			if !ok && len(p) == 1 {
				code, ok = keyCodes[strings.ToUpper(p)]
			}
			if !ok {
				return engine.Key{}, fmt.Errorf("unknown key %q in %q", p, s)
			}
			k.Code = code
			break
		}
		mod, ok := modOf(p)
		if !ok {
			return engine.Key{}, fmt.Errorf("unknown modifier %q in %q", p, s)
		}
		k.Mods |= mod
	}
	return k, nil
}

func modOf(name string) (engine.Mods, bool) {
	for _, m := range modNames {
		if name == m.mac || name == m.other {
			return m.mod, true
		}
	}
	return 0, false
}

// FormatKeys writes a combination the way the platform names its keys, modifiers in
// order. "" for a key no name covers.
func FormatKeys(platform string, k engine.Key) string {
	name, ok := keyNames[k.Code]
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, m := range modNames {
		if k.Mods&m.mod == 0 {
			continue
		}
		if platform == Mac {
			b.WriteString(m.mac)
		} else {
			b.WriteString(m.other)
		}
		b.WriteByte('+')
	}
	b.WriteString(name)
	return b.String()
}
