package relaunch

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"projectb/instance"
	"projectb/profiles"
)

// A test binary started with FAKE_COPY_OF is a fake copy of the app: it owns that folder,
// says so, and when asked to quit writes "ended" there and ends.
func TestMain(m *testing.M) {
	if dir := os.Getenv("FAKE_COPY_OF"); dir != "" {
		fakeCopy(dir)
		return
	}
	os.Exit(m.Run())
}

func fakeCopy(dir string) {
	o, err := instance.Claim(dir, nil)
	if err != nil || o == nil {
		os.Exit(1)
	}
	quit := make(chan struct{})
	o.Serve(func([]string) {}, func() { close(quit) })
	os.Stdout.WriteString("owned\n")
	<-quit
	o.Close()
	os.WriteFile(filepath.Join(dir, "ended"), nil, 0o644)
}

// startFake starts a fake copy owning dir, and returns once it does.
func startFake(t *testing.T, dir string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "FAKE_COPY_OF="+dir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if line, _ := bufio.NewReader(out).ReadString('\n'); line != "owned\n" {
		t.Fatalf("the fake copy on %s said %q", dir, line)
	}
	return cmd
}

// Relaunching from one profile's copy: the other running copies, a picker among them, are
// asked to quit, and QuitOthers returns only once each has ended, with the profiles to
// start again, this copy's first. A listed profile that isn't running, and the picker,
// aren't started again.
func TestQuitOthersWaitsForEachCopyToEnd(t *testing.T) {
	list := profiles.New(t.TempDir())
	self, err := list.Choose("", false)
	if err != nil {
		t.Fatal(err)
	}
	work, _ := list.Create("Work", "Work")
	idle, _ := list.Create("Idle", "Personal")
	running := startFake(t, list.Dir(work.ID))
	picker := startFake(t, list.Root())

	got := Copies{List: list, Profile: self.ID}.QuitOthers()
	if want := []string{self.ID, work.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("to start again: %v, want %v (%s isn't running)", got, want, idle.ID)
	}
	for _, dir := range []string{list.Dir(work.ID), list.Root()} {
		if _, err := os.Stat(filepath.Join(dir, "ended")); err != nil {
			t.Fatalf("QuitOthers returned before the copy on %s ended", dir)
		}
	}
	running.Wait()
	picker.Wait()
}
