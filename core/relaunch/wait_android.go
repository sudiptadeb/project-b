package relaunch

// waitEnded has nothing to wait for: Android runs one copy of the app.
func waitEnded(pids []int) error { return nil }
