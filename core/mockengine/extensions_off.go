//go:build !extensions

package mockengine

// A build without extensions has no fake extension runtime.
type extensionRuntime struct{}
