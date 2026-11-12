//go:build extensions

package engine

// Extension is one installed extension, as the engine reads it from its package.
type Extension struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	// Unpacked says it was loaded from a folder, which Path names.
	Unpacked bool   `json:"unpacked"`
	Path     string `json:"path"`
	// Icon is a data: URL, or "" when the package has none.
	Icon string `json:"icon"`
	// Permissions and Hosts are the names and host patterns its manifest asks for.
	Permissions []string `json:"permissions"`
	Hosts       []string `json:"hosts"`
	// What its toolbar button starts with. Popup and OptionsPage are URLs, or "".
	HasAction   bool               `json:"hasAction"`
	ActionTitle string             `json:"actionTitle"`
	ActionPopup string             `json:"actionPopup"`
	ActionIcon  string             `json:"actionIcon"`
	OptionsPage string             `json:"optionsPage"`
	Commands    []ExtensionCommand `json:"commands"`
}

// ExtensionCommand is a command an extension's manifest declares.
type ExtensionCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// InstallPrompt is the engine asking whether to install a package it has unpacked, or,
// with Remove, whether to remove an extension a store's page asked to remove.
type InstallPrompt struct {
	ID          int64    `json:"-"`
	Extension   string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Icon        string   `json:"icon"`
	Permissions []string `json:"permissions"` // Names and host patterns, as the manifest has them.
	Remove      bool     `json:"remove"`
}

// InstallResult is how an install or a load ended. A person who declines the prompt gets
// Cancelled; anything else that stops it is in Error.
type InstallResult struct {
	Path      string // The package or folder it was asked with.
	Extension string // The id, once installed.
	Error     string
	Cancelled bool
}

// ExtensionCall is an extension calling one of the browser's own APIs, such as
// "tabs.query". Args is the JSON list of its arguments. Tab is the tab the caller runs
// in, or zero for a popup or a background script.
type ExtensionCall struct {
	Extension string
	Name      string
	Args      string
	Tab       TabID
}

// PathKind is what ChoosePath lets the user pick.
type PathKind int

const (
	PathFolder PathKind = iota + 1
	PathPackage
)

// Extensions is what the app can ask an engine that runs extensions. The engine keeps
// what is installed in its own storage and brings it back at the next start.
type Extensions interface {
	SetExtensionsListener(l ExtensionsListener)

	// InstallExtension installs a package file. The engine asks with an InstallPrompt
	// first, and says how it ended with ExtensionInstallDone.
	InstallExtension(path string)
	AnswerInstallPrompt(prompt int64, accept bool)
	// LoadUnpackedExtension loads an extension from its folder, with no prompt.
	LoadUnpackedExtension(dir string)
	UninstallExtension(id string)
	SetExtensionEnabled(id string, on bool)
	Extensions() []Extension

	// DispatchExtensionEvent fires an event such as "tabs.onUpdated" in one extension.
	// args is the JSON list of the listener's arguments.
	DispatchExtensionEvent(id, event, args string)

	// OpenExtensionPopup shows an extension's page in a view of its own over a window,
	// under the anchor, a rectangle in that window's coordinates. There is one popup at a
	// time. It closes when the user clicks elsewhere or it closes itself, and the engine
	// says so.
	OpenExtensionPopup(w WindowID, id, url string, anchor Rect)
	CloseExtensionPopup()

	// ChoosePath asks the user for a folder or a package file, on a window. PathChosen
	// answers.
	ChoosePath(w WindowID, kind PathKind)
}

// ExtensionsListener is what such an engine tells the app. There is exactly one.
type ExtensionsListener interface {
	// ExtensionsChanged says the installed extensions, or what the engine says about
	// one of them, changed.
	ExtensionsChanged()
	ExtensionInstallPrompt(p InstallPrompt)
	ExtensionInstallDone(r InstallResult)
	// ExtensionCall answers a call: its result as JSON, "" for none, or an error.
	ExtensionCall(c ExtensionCall) (result, err string)
	// ExtensionBlockedCount says how many requests an extension's rules have blocked in
	// a tab's page, for an extension that asked to show that on its button.
	ExtensionBlockedCount(id string, tab TabID, count int)
	ExtensionPopupClosed()
	// PathChosen carries what the user picked, or "" when they picked nothing.
	PathChosen(path string)
}
