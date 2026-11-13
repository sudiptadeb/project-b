package relaunch

import (
	"projectb/instance"
	"projectb/profiles"
	"projectb/timeline"
)

// Copies are the copies of the app running on the profiles of one root, and a picker
// copy there, seen from the copy on the listed profile Profile, which an update relaunches
// with all of them. A running copy holds its folder's lock, the root's for a picker copy,
// and answers on its socket (core/instance).
type Copies struct {
	List    *profiles.Manager
	Profile string
}

// QuitOthers asks each other running copy to quit, and returns once their processes have
// ended, with this copy's profile and each other one that was running. A picker copy quits
// too, and isn't started again.
func (c Copies) QuitOthers() []string {
	started := []string{c.Profile}
	var pids []int
	list, err := c.List.List()
	if err != nil {
		timeline.Errorf("relaunch: %v", err)
	}
	for _, p := range list {
		if p.ID == c.Profile {
			continue
		}
		pid, err := instance.Quit(c.List.Dir(p.ID))
		if err != nil {
			timeline.Errorf("relaunch: the copy on %s: %v", p.ID, err)
			continue
		}
		if pid != 0 {
			started = append(started, p.ID)
			pids = append(pids, pid)
		}
	}
	if pid, err := instance.Quit(c.List.Root()); err != nil {
		timeline.Errorf("relaunch: the picker copy: %v", err)
	} else if pid != 0 {
		pids = append(pids, pid)
	}
	timeline.Infof("relaunch: waiting for %d other copies to end", len(pids))
	if err := waitEnded(pids); err != nil {
		timeline.Errorf("relaunch: %v", err)
	}
	return started
}

// StartAfter starts executable on each listed profile of the root once this process has
// ended, each a copy of its own.
func (c Copies) StartAfter(executable string, profiles []string) {
	for _, id := range profiles {
		After(executable, []string{"--root=" + c.List.Root(), "--profile=" + id})
	}
}
