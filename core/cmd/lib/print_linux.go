//go:build !android

package main

import "io"

// printer is where the timeline prints. nil is stderr, which a terminal or the journal shows.
func printer() io.Writer { return nil }
