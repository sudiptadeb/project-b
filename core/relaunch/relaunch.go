// Package relaunch starts the app again once this process has gone. Starting it any
// earlier would run two copies on one profile.
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

// Self starts this program again, with the arguments it was started with.
func Self() {
	exe, err := os.Executable()
	if err != nil {
		timeline.Errorf("relaunch: %v", err)
		return
	}
	After(exe, os.Args[1:])
}
