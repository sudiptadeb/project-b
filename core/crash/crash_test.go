package crash

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// crashIn is set when the test binary runs as the program that crashes.
const crashIn = "PROJECTB_CRASH_IN"

func TestMain(m *testing.M) {
	if dir := os.Getenv(crashIn); dir != "" {
		New(dir)
		go func() {
			panic("a goroutine went wrong")
		}()
		select {}
	}
	os.Exit(m.Run())
}

// crashOnce runs this test binary as a program that panics in a goroutine, with dir as
// its profile.
func crashOnce(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), crashIn+"="+dir)
	if err := cmd.Run(); err == nil {
		t.Fatal("the program didn't crash")
	}
}

func TestACrashIsReportedAtTheNextStartAndKept(t *testing.T) {
	dir := t.TempDir()
	if New(dir).Last() != "" {
		t.Fatal("a first start reports a crash")
	}

	crashOnce(t, dir)
	b, err := os.ReadFile(filepath.Join(dir, "crash.log"))
	if err != nil {
		t.Fatal(err)
	}
	report := string(b)
	if !strings.Contains(report, "panic: a goroutine went wrong") || !strings.Contains(report, "crash_test.go") {
		t.Fatalf("crash.log = %q", report)
	}

	m := New(dir)
	last := m.Last()
	if !strings.HasPrefix(filepath.Base(last), "crash-") {
		t.Fatalf("last = %q", last)
	}
	if b, _ := os.ReadFile(last); string(b) != report {
		t.Fatalf("%s holds %q", last, b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "crash.log")); len(b) != 0 {
		t.Fatalf("crash.log after the start = %q", b)
	}
	m.Dismiss()
	if m.Last() != "" {
		t.Fatal("dismissed, still reported")
	}

	// A start after a clean run reports nothing, and the kept report stays.
	if New(dir).Last() != "" {
		t.Fatal("a clean run reports a crash")
	}
	if _, err := os.Stat(last); err != nil {
		t.Fatal(err)
	}
}
