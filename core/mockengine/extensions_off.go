//go:build !extensions

package mockengine

import "projectb/engine"

// A build without extensions has no fake extension runtime.
type extensionRuntime struct{}

// openPopup is the extension popup the app has open: never one.
func (e *Engine) openPopup(engine.WindowID) any { return nil }
