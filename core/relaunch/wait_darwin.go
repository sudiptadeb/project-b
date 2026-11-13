package relaunch

import (
	"errors"
	"syscall"
)

// waitEnded returns once every process of pids has ended. The kernel says so: a process
// that ended already is reported at once, and the rest as they end.
func waitEnded(pids []int) error {
	if len(pids) == 0 {
		return nil
	}
	kq, err := syscall.Kqueue()
	if err != nil {
		return err
	}
	defer syscall.Close(kq)
	changes := make([]syscall.Kevent_t, len(pids))
	for i, pid := range pids {
		syscall.SetKevent(&changes[i], pid, syscall.EVFILT_PROC, syscall.EV_ADD|syscall.EV_ONESHOT)
		changes[i].Fflags = syscall.NOTE_EXIT
	}
	events := make([]syscall.Kevent_t, len(pids))
	for left := len(pids); left > 0; {
		n, err := syscall.Kevent(kq, changes, events, nil)
		// The changes are in before the wait, which a signal can cut short. A process
		// already gone comes back as an error for its pid.
		changes = nil
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		left -= n
	}
	return nil
}
