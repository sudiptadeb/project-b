// Package crash keeps what Go prints when it crashes. The runtime writes its own crash
// report, the panic or fatal error and every goroutine's stack, to crash.log in the
// profile, as well as to stderr, whichever goroutine crashed. At the next start a report
// there is renamed after the time it was written, so no crash overwrites another, and
// the shell is told where it is.
package crash

import (
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
)

// Manager knows whether the last run crashed.
type Manager struct {
	last string
}

// New sends this run's crash report to crash.log in dir, after keeping the last run's,
// if it crashed, as crash-<time>.log.
func New(dir string) *Manager {
	m := &Manager{}
	current := filepath.Join(dir, "crash.log")
	if info, err := os.Stat(current); err == nil && info.Size() > 0 {
		last := filepath.Join(dir, "crash-"+info.ModTime().Format("2006-01-02-150405")+".log")
		if err := os.Rename(current, last); err != nil {
			log.Printf("crash: %v", err)
		} else {
			m.last = last
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf("crash: %v", err)
		return m
	}
	f, err := os.OpenFile(current, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Printf("crash: %v", err)
		return m
	}
	// The runtime keeps its own copy of the descriptor.
	defer f.Close()
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		log.Printf("crash: %v", err)
	}
	return m
}

// Last is the report of the last run's crash, or "" if it didn't crash or the notice
// was closed.
func (m *Manager) Last() string { return m.last }

// Dismiss forgets the last crash: the user has seen where its report is.
func (m *Manager) Dismiss() { m.last = "" }
