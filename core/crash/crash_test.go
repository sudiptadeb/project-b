package crash

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"projectb/engine"
)

// crashIn is set when the test binary runs as the program that crashes.
const crashIn = "PROJECTB_CRASH_IN"

func TestMain(m *testing.M) {
	if dir := os.Getenv(crashIn); dir != "" {
		New(dir, "")
		go func() {
			panic("a goroutine went wrong")
		}()
		select {}
	}
	os.Exit(m.Run())
}

// crashOnce runs this test binary as a program that panics in a goroutine, with dir as
// its profile, and returns its pid.
func crashOnce(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), crashIn+"="+dir)
	if err := cmd.Run(); err == nil {
		t.Fatal("the program didn't crash")
	}
	return cmd.ProcessState.Pid()
}

func TestAGoCrashIsReportedAtTheNextStartAndKept(t *testing.T) {
	dir, reports := t.TempDir(), t.TempDir()
	if New(dir, reports).Last() != "" {
		t.Fatal("a first start reports a crash")
	}

	pid := crashOnce(t, dir)
	b, err := os.ReadFile(filepath.Join(dir, "crash.log"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(b)
	if !strings.HasPrefix(log, fmt.Sprintf("pid %d, started ", pid)) || !strings.Contains(log, "panic: a goroutine went wrong") || !strings.Contains(log, "crash_test.go") {
		t.Fatalf("crash.log = %q", log)
	}

	m := New(dir, reports)
	last := m.Last()
	if !strings.HasPrefix(filepath.Base(last), "crash-") {
		t.Fatalf("last = %q", last)
	}
	if b, _ := os.ReadFile(last); string(b) != log {
		t.Fatalf("%s holds %q", last, b)
	}
	if m.Report() != reports {
		t.Fatalf("with no report from macOS, report = %q", m.Report())
	}
	m.Dismiss()
	if m.Last() != "" || m.Report() != "" {
		t.Fatal("dismissed, still reported")
	}

	// A start after a clean stop reports nothing, and the kept log stays.
	m.Stop()
	if New(dir, reports).Last() != "" {
		t.Fatal("a clean stop reports a crash")
	}
	if _, err := os.Stat(last); err != nil {
		t.Fatal(err)
	}
}

func TestARunThatDidNotStopIsACrashWithMacOSReport(t *testing.T) {
	dir, reports := t.TempDir(), t.TempDir()
	New(dir, reports)
	// macOS names its report after the process and puts the pid in the report itself,
	// after one line about the report.
	exe, _ := os.Executable()
	write := func(name string, pid int) string {
		path := filepath.Join(reports, filepath.Base(exe)+"-"+name+".ips")
		body := fmt.Sprintf("{\"app_name\":\"x\"}\n{\n  \"pid\" : %d,\n  \"procName\" : \"x\"\n}\n", pid)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write("2026-11-10-100000", os.Getpid()+1)
	ours := write("2026-11-10-100001", os.Getpid())

	m := New(dir, reports)
	if m.Last() == "" {
		t.Fatal("a run that didn't stop isn't reported")
	}
	if m.Report() != ours {
		t.Fatalf("report = %q, want %q", m.Report(), ours)
	}
}

func TestAChildProcessCrashIsALineAndNoNotice(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, "")
	m.ProcessCrashed(engine.ProcessCrash{Type: "renderer", Status: "crashed", ExitCode: 11, PID: 42, URL: "https://p.test/"})
	m.ProcessCrashed(engine.ProcessCrash{Type: "GPU", Status: "crashed", ExitCode: 11, PID: 43})
	m.Stop()
	if New(dir, "").Last() != "" {
		t.Fatal("a child's crash brings up the notice")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "crashes.log"))
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], " renderer crashed, exit code 11, pid 42, https://p.test/") || !strings.HasSuffix(lines[1], " GPU crashed, exit code 11, pid 43") {
		t.Fatalf("crashes.log = %q", b)
	}
}
