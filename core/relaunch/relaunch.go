// Package relaunch starts copies of the app: this one again once this process has gone,
// since starting it any earlier would run two copies on one profile, or another copy now,
// on another profile.
package relaunch

import (
	"os"
	"os/exec"
	"projectb/timeline"
	"strconv"
	"syscall"
)

// After starts executable with args from a shell that outlives this process and waits
// for it to exit. The caller then ends the app.
func After(executable string, args []string) {
	script := `pid=$1; shift; while kill -0 "$pid" 2>/dev/null; do sleep 0.2; done; exec "$@"`
	cmd := exec.Command("/bin/sh", append([]string{"-c", script, "relaunch", strconv.Itoa(os.Getpid()), executable}, args...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		timeline.Errorf("relaunch: %v", err)
	}
}

// Copy starts another copy of this program now, with args, apart from this process: it
// goes on when this one ends.
func Copy(args ...string) {
	exe, err := os.Executable()
	if err != nil {
		timeline.Errorf("relaunch: %v", err)
		return
	}
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		timeline.Errorf("relaunch: %v", err)
		return
	}
	cmd.Process.Release()
}

// Self starts this program again, with the arguments it was started with.
func Self() {
	exe, err := os.Executable()
	if err != nil {
		timeline.Errorf("relaunch: %v", err)
		return
	}
	After(exe, os.Args[1:])
}
