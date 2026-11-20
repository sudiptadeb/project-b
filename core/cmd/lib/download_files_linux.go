//go:build !android

package main

// Downloads on Linux are the user's own files in their own folder, as on the Mac: the app
// doesn't delete them.
func (e *nativeEngine) DeleteDownload(string) bool { return false }
