//go:build !extensions

package mockengine

// A build without extensions has no fake extension runtime.
type extensionRuntime struct{}

// openPopup is the extension popup the app has open: never one.
func (e *Engine) openPopup() any { return nil }
