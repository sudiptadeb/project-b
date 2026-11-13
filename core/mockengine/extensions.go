//go:build extensions

package mockengine

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"projectb/engine"
)

// extensionRuntime is the mock engine's fake extension runtime. It reads a package's
// manifest as the real engine does and keeps the list of what is installed, but runs no
// JavaScript: a test calls an API as an extension would, with Call, and reads back the
// events and the popup the app asked for.
type extensionRuntime struct {
	listener engine.ExtensionsListener
	list     []engine.Extension
	nextID   int64
	// prompts are the installs waiting for an answer.
	prompts map[int64]install

	// Written on the main thread, read by a test's goroutines.
	mu     sync.Mutex
	events []Event
	popup  *Popup
	// popupOver is the window the popup is over.
	popupOver engine.WindowID
	choose    string
}

type install struct {
	path string
	ext  engine.Extension
}

// Event is one event the app fired in an extension.
type Event struct {
	Extension string          `json:"extension"`
	Name      string          `json:"name"`
	Args      json.RawMessage `json:"args"`
}

// Popup is the popup the app has open.
type Popup struct {
	Extension string      `json:"extension"`
	URL       string      `json:"url"`
	Anchor    engine.Rect `json:"anchor"`
}

// Answer is what the app answered a call with.
type Answer struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error"`
}

func (e *Engine) SetExtensionsListener(l engine.ExtensionsListener) {
	e.extensions.listener = l
	e.extensions.prompts = map[int64]install{}
}

// InstallExtension reads the package and asks before adding it, as the real engine does.
func (e *Engine) InstallExtension(path string) {
	ext, err := readPackage(path)
	if err != nil {
		e.extensions.listener.ExtensionInstallDone(engine.InstallResult{Path: path, Error: err.Error()})
		return
	}
	e.extensions.nextID++
	e.extensions.prompts[e.extensions.nextID] = install{path, ext}
	e.extensions.listener.ExtensionInstallPrompt(engine.InstallPrompt{
		ID: e.extensions.nextID, Extension: ext.ID, Name: ext.Name, Version: ext.Version, Icon: ext.Icon,
		Permissions: append(slices.Clone(ext.Permissions), ext.Hosts...),
	})
}

func (e *Engine) AnswerInstallPrompt(prompt int64, accept bool) {
	in, ok := e.extensions.prompts[prompt]
	if !ok {
		return
	}
	delete(e.extensions.prompts, prompt)
	if !accept {
		e.extensions.listener.ExtensionInstallDone(engine.InstallResult{Path: in.path, Cancelled: true})
		return
	}
	e.add(in.ext)
	e.extensions.listener.ExtensionInstallDone(engine.InstallResult{Path: in.path, Extension: in.ext.ID})
}

func (e *Engine) LoadUnpackedExtension(dir string) {
	ext, err := readFolder(dir)
	if err != nil {
		e.extensions.listener.ExtensionInstallDone(engine.InstallResult{Path: dir, Error: err.Error()})
		return
	}
	e.add(ext)
	e.extensions.listener.ExtensionInstallDone(engine.InstallResult{Path: dir, Extension: ext.ID})
}

// add installs an extension, in place of an older version of itself.
func (e *Engine) add(ext engine.Extension) {
	e.extensions.list = slices.DeleteFunc(e.extensions.list, func(e engine.Extension) bool { return e.ID == ext.ID })
	e.extensions.list = append(e.extensions.list, ext)
	e.extensions.listener.ExtensionsChanged()
}

func (e *Engine) UninstallExtension(id string) {
	e.extensions.list = slices.DeleteFunc(e.extensions.list, func(e engine.Extension) bool { return e.ID == id })
	e.extensions.listener.ExtensionsChanged()
}

func (e *Engine) SetExtensionEnabled(id string, on bool) {
	for i := range e.extensions.list {
		if e.extensions.list[i].ID == id {
			e.extensions.list[i].Enabled = on
		}
	}
	e.extensions.listener.ExtensionsChanged()
}

func (e *Engine) Extensions() []engine.Extension { return slices.Clone(e.extensions.list) }

// events are the events the real engine can send, as engine/extensions/browser/extensions.cc
// lists them: it stops on any other, and so does the mock.
var events = []string{
	"action.onClicked", "commands.onCommand",
	"tabs.onActivated", "tabs.onCreated", "tabs.onRemoved", "tabs.onUpdated",
}

func (e *Engine) DispatchExtensionEvent(id, event, args string) {
	if !slices.Contains(events, event) {
		panic("the engine has no event " + event)
	}
	e.extensions.mu.Lock()
	defer e.extensions.mu.Unlock()
	e.extensions.events = append(e.extensions.events, Event{id, event, json.RawMessage(args)})
}

func (e *Engine) OpenExtensionPopup(w engine.WindowID, id, url string, anchor engine.Rect) {
	e.extensions.mu.Lock()
	e.extensions.popup = &Popup{id, url, anchor}
	e.extensions.popupOver = w
	e.extensions.mu.Unlock()
	e.Feeds(w).Window.Publish(e.WindowState(w))
}

func (e *Engine) CloseExtensionPopup() {
	e.extensions.mu.Lock()
	open, w := e.extensions.popup != nil, e.extensions.popupOver
	e.extensions.popup = nil
	e.extensions.mu.Unlock()
	if open {
		e.Feeds(w).Window.Publish(e.WindowState(w))
		e.extensions.listener.ExtensionPopupClosed()
	}
}

// openPopup is the extension popup the app has open over window w, or nil. Any goroutine.
func (e *Engine) openPopup(w engine.WindowID) any {
	e.extensions.mu.Lock()
	defer e.extensions.mu.Unlock()
	if e.extensions.popup == nil || e.extensions.popupOver != w {
		return nil
	}
	p := *e.extensions.popup
	return p
}

// ChoosePath answers with what the test said the user would pick.
func (e *Engine) ChoosePath(engine.WindowID, engine.PathKind) {
	e.extensions.mu.Lock()
	path := e.extensions.choose
	e.extensions.choose = ""
	e.extensions.mu.Unlock()
	e.extensions.listener.PathChosen(path)
}

// Call calls one of the browser's APIs as code in an extension would, from a tab's page
// or, with tab zero, from its popup or background script, and returns the app's answer.
// Any goroutine but the main thread.
func (e *Engine) Call(extension, name, args string, tab engine.TabID) Answer {
	answer := make(chan Answer, 1)
	e.Post(func() {
		result, err := e.extensions.listener.ExtensionCall(engine.ExtensionCall{Extension: extension, Name: name, Args: args, Tab: tab})
		answer <- Answer{Result: json.RawMessage(result), Error: err}
	})
	return <-answer
}

// FireExtension makes the runtime do what happens outside the app. Any goroutine.
//
//	choose      the next ChoosePath picks value
//	closePopup  the user clicked outside the popup
//	blocked     extension's rules have blocked value requests in the tab window 1 shows
func (e *Engine) FireExtension(kind, extension, value string) {
	switch kind {
	case "choose":
		e.extensions.mu.Lock()
		e.extensions.choose = value
		e.extensions.mu.Unlock()
	case "closePopup":
		e.Post(e.CloseExtensionPopup)
	case "blocked":
		e.Post(func() {
			var n int
			fmt.Sscan(value, &n)
			e.extensions.listener.ExtensionBlockedCount(extension, e.windows[1].shown, n)
		})
	}
}

// State is what a test reads back: the events fired so far and the open popup.
type State struct {
	Events []Event `json:"events"`
	Popup  *Popup  `json:"popup"`
}

// ExtensionState returns the events fired so far and the open popup. Any goroutine.
func (e *Engine) ExtensionState() State {
	e.extensions.mu.Lock()
	defer e.extensions.mu.Unlock()
	return State{Events: slices.Clone(e.extensions.events), Popup: e.extensions.popup}
}

// readPackage reads a CRX3 file: "Cr24", 3, the header's length, the header, a zip. The
// id is in the header's signed data.
func readPackage(path string) (engine.Extension, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return engine.Extension{}, err
	}
	if len(data) < 12 || string(data[:4]) != "Cr24" || binary.LittleEndian.Uint32(data[4:]) != 3 {
		return engine.Extension{}, errors.New("not a CRX3 package")
	}
	header := data[12 : 12+binary.LittleEndian.Uint32(data[8:])]
	archive := data[12+len(header):]
	// Field 10000 of the header is the signed data, whose field 1 is the id's 16 bytes.
	id := field(field(header, 10000), 1)
	z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return engine.Extension{}, err
	}
	ext, err := read(z, letters(id))
	ext.Path = path
	return ext, err
}

// readFolder reads an unpacked extension. Its id is its folder's path, hashed.
func readFolder(dir string) (engine.Extension, error) {
	sum := sha256.Sum256([]byte(dir))
	ext, err := read(os.DirFS(dir), letters(sum[:16]))
	ext.Path, ext.Unpacked = dir, true
	return ext, err
}

// letters is an extension id: each half byte as a letter from a to p.
func letters(b []byte) string {
	var s strings.Builder
	for _, c := range b {
		s.WriteByte('a' + c>>4)
		s.WriteByte('a' + c&15)
	}
	return s.String()
}

// field returns the first length-delimited protobuf field with this number.
func field(msg []byte, number uint64) []byte {
	for len(msg) > 0 {
		key, n := binary.Uvarint(msg)
		msg = msg[n:]
		size, n := binary.Uvarint(msg)
		msg = msg[n:]
		if key>>3 == number {
			return msg[:size]
		}
		msg = msg[size:]
	}
	return nil
}

// manifest is the part of manifest.json the browser shows or acts on.
type manifest struct {
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Description   string            `json:"description"`
	DefaultLocale string            `json:"default_locale"`
	Icons         map[string]string `json:"icons"`
	Permissions   []string          `json:"permissions"`
	Hosts         []string          `json:"host_permissions"`
	OptionsPage   string            `json:"options_page"`
	Action        *struct {
		Title string `json:"default_title"`
		Popup string `json:"default_popup"`
		Icon  any    `json:"default_icon"`
	} `json:"action"`
	Commands map[string]struct {
		Description string `json:"description"`
	} `json:"commands"`
}

func read(files fs.FS, id string) (engine.Extension, error) {
	raw, err := fs.ReadFile(files, "manifest.json")
	if err != nil {
		return engine.Extension{}, err
	}
	var m manifest
	if err := json.Unmarshal(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")), &m); err != nil {
		return engine.Extension{}, fmt.Errorf("manifest.json: %v", err)
	}
	text := func(s string) string { return message(files, m.DefaultLocale, s) }
	// A page inside the extension, as the URL the engine serves it at.
	page := func(path string) string {
		if path == "" {
			return ""
		}
		return "chrome-extension://" + id + "/" + strings.TrimPrefix(path, "/")
	}
	ext := engine.Extension{
		ID: id, Name: text(m.Name), Version: m.Version, Description: text(m.Description), Enabled: true,
		Icon: image(files, biggest(m.Icons)), Permissions: m.Permissions, Hosts: m.Hosts,
		OptionsPage: page(m.OptionsPage),
	}
	if a := m.Action; a != nil {
		ext.HasAction, ext.ActionTitle, ext.ActionPopup = true, text(a.Title), page(a.Popup)
		switch icon := a.Icon.(type) {
		case string:
			ext.ActionIcon = image(files, icon)
		case map[string]any:
			bySize := map[string]string{}
			for size, path := range icon {
				bySize[size], _ = path.(string)
			}
			ext.ActionIcon = image(files, biggest(bySize))
		}
	}
	for name, c := range m.Commands {
		ext.Commands = append(ext.Commands, engine.ExtensionCommand{Name: name, Description: text(c.Description)})
	}
	slices.SortFunc(ext.Commands, func(a, b engine.ExtensionCommand) int { return strings.Compare(a.Name, b.Name) })
	return ext, nil
}

// biggest is the path of the largest icon in a manifest's size-to-path table.
func biggest(bySize map[string]string) string {
	best, path := 0, ""
	for size, p := range bySize {
		var n int
		fmt.Sscan(size, &n)
		if n > best {
			best, path = n, p
		}
	}
	return path
}

// image reads a PNG from the package into a data: URL.
func image(files fs.FS, path string) string {
	if path == "" {
		return ""
	}
	f, err := files.Open(strings.TrimPrefix(path, "/"))
	if err != nil {
		return ""
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
}

// message looks "__MSG_name__" up in the package's default locale. Names aren't case
// sensitive.
func message(files fs.FS, locale, s string) string {
	if !strings.HasPrefix(s, "__MSG_") || !strings.HasSuffix(s, "__") {
		return s
	}
	raw, err := fs.ReadFile(files, filepath.ToSlash(filepath.Join("_locales", locale, "messages.json")))
	if err != nil {
		return s
	}
	var messages map[string]struct {
		Message string `json:"message"`
	}
	json.Unmarshal(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")), &messages)
	for name, m := range messages {
		if strings.EqualFold(name, s[6:len(s)-2]) {
			return m.Message
		}
	}
	return s
}
