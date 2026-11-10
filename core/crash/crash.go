// Package crash keeps what is known about crashes. While the app runs, crash.log in the
// profile says so: it starts with the process id, and if Go crashes the runtime writes
// its own report there, the panic or fatal error and every goroutine's stack, as well as
// to stderr, whichever goroutine crashed. A clean stop deletes it. So a crash.log found
// at the next start means the last run crashed, from Go or not: it is renamed after the
// time it was last written, so no crash overwrites another, and the shell is told where
// it is and where macOS put its own report of that process.
package crash

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

// Manager knows whether the last run crashed.
type Manager struct {
	current string
	last    string
	report  string
}

// New starts this run's crash.log in dir, after keeping the last run's, if there is one,
// as crash-<time>.log. reports is the folder macOS writes its crash reports to.
func New(dir, reports string) *Manager {
	m := &Manager{current: filepath.Join(dir, "crash.log")}
	if info, err := os.Stat(m.current); err == nil {
		last := filepath.Join(dir, "crash-"+info.ModTime().Format("2006-01-02-150405")+".log")
		if err := os.Rename(m.current, last); err != nil {
			log.Printf("crash: %v", err)
		} else {
			m.last = last
			m.report = osReport(last, info.ModTime(), reports)
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf("crash: %v", err)
		return m
	}
	f, err := os.OpenFile(m.current, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Printf("crash: %v", err)
		return m
	}
	// The runtime keeps its own copy of the descriptor.
	defer f.Close()
	if _, err := fmt.Fprintf(f, "pid %d, started %s\n", os.Getpid(), time.Now().Format(time.DateTime)); err != nil {
		log.Printf("crash: %v", err)
	}
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		log.Printf("crash: %v", err)
	}
	return m
}

// osReport is macOS's report of the process that wrote the crash log at path, or the
// folder of reports if it hasn't written one there.
func osReport(path string, since time.Time, reports string) string {
	var pid int
	if f, err := os.Open(path); err == nil {
		fmt.Fscanf(f, "pid %d,", &pid)
		f.Close()
	}
	exe, err := os.Executable()
	if pid == 0 || err != nil {
		return reports
	}
	found, _ := filepath.Glob(filepath.Join(reports, filepath.Base(exe)+"-*.ips"))
	for _, name := range found {
		if info, err := os.Stat(name); err == nil && !info.ModTime().Before(since) && reportPID(name) == pid {
			return name
		}
	}
	return reports
}

// reportPID reads the process id from a macOS crash report: a line of JSON about the
// report, then the report, one JSON object.
func reportPID(name string) int {
	f, err := os.Open(name)
	if err != nil {
		return 0
	}
	defer f.Close()
	r := bufio.NewReader(f)
	if _, err := r.ReadString('\n'); err != nil {
		return 0
	}
	var body struct {
		PID int `json:"pid"`
	}
	json.NewDecoder(r).Decode(&body)
	return body.PID
}

// Stop says this run ends cleanly: there is no crash to report next time.
func (m *Manager) Stop() {
	if err := os.Remove(m.current); err != nil {
		log.Printf("crash: %v", err)
	}
}

// Last is the log of the last run's crash, or "" if it didn't crash or the notice was
// closed.
func (m *Manager) Last() string { return m.last }

// Report is macOS's report of the last run's crash, or the folder macOS keeps its
// reports in when it has none for that run yet; "" when Last is.
func (m *Manager) Report() string { return m.report }

// Dismiss forgets the last crash: the user has seen where its report is.
func (m *Manager) Dismiss() { m.last, m.report = "", "" }
