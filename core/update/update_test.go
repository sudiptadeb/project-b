package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeApp writes a bundle the way tools/package.sh lays one out: the plist and an
// executable, here a script that records that it ran.
func fakeApp(t *testing.T, dir, version string) string {
	t.Helper()
	app := filepath.Join(dir, "Project B.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key><string>project_b</string>
  <key>CFBundleShortVersionString</key><string>%s</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
`, version)
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\necho %s > \"$(dirname \"$0\")/ran\"\n", version)
	if err := os.WriteFile(filepath.Join(app, "Contents", "MacOS", "project_b"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return app
}

// archive zips a bundle as tools/release.sh does.
func archive(t *testing.T, app string) []byte {
	t.Helper()
	zip := filepath.Join(t.TempDir(), "app.zip")
	if out, err := exec.Command("/usr/bin/ditto", "-c", "-k", "--keepParent", app, zip).CombinedOutput(); err != nil {
		t.Fatalf("ditto: %v %s", err, out)
	}
	b, err := os.ReadFile(zip)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// releaseServer serves a manifest pointing stable at Version, and the asset. The manifest's
// digest is of Good; Served is what the client gets. Both can change between checks.
type releaseServer struct {
	mu        sync.Mutex
	Version   string
	Good      []byte
	Served    []byte
	downloads atomic.Int32
	manifests atomic.Int32
	url       string
	serve     func(w http.ResponseWriter) // serves the asset instead of Served, when set
}

func release(t *testing.T, version string, good, served []byte) *releaseServer {
	t.Helper()
	rs := &releaseServer{Version: version, Good: good, Served: served}
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/releases.json", func(w http.ResponseWriter, r *http.Request) {
		rs.manifests.Add(1)
		rs.mu.Lock()
		defer rs.mu.Unlock()
		sum := sha256.Sum256(rs.Good)
		fmt.Fprintf(w, `{"spec":1,"project":"project-b","published":"2026-11-09T00:00:00Z","channels":{"stable":"%s"},
		  "releases":{"%s":{"assets":{"%s/%s":{"url":"%s/asset.zip","sha256":"%s","size":%d}}}}}`,
			rs.Version, rs.Version, runtime.GOOS, runtime.GOARCH, srv.URL, hex.EncodeToString(sum[:]), len(rs.Good))
	})
	mux.HandleFunc("/asset.zip", func(w http.ResponseWriter, r *http.Request) {
		rs.downloads.Add(1)
		if rs.serve != nil {
			rs.serve(w)
			return
		}
		rs.mu.Lock()
		defer rs.mu.Unlock()
		w.Write(rs.Served)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	rs.url = srv.URL + "/releases.json"
	return rs
}

func (rs *releaseServer) set(version string, good, served []byte) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.Version, rs.Good, rs.Served = version, good, served
}

type rig struct {
	m      *Manager
	mu     sync.Mutex // the download's progress is told from its own goroutine
	states []State
	quit   bool
	app    string
}

func start(t *testing.T, running, releases string, verify func(string) error) *rig {
	t.Helper()
	installed := t.TempDir()
	r := &rig{app: fakeApp(t, installed, running)}
	cfg := Config{Project: "project-b", Version: running, Releases: releases, StateDir: filepath.Join(t.TempDir(), "updates"), App: r.app, Verify: verify}
	r.m = New(cfg, func(f func()) { f() }, func() { r.quit = true })
	r.m.OnChanged(func(s State) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.states = append(r.states, s)
	})
	if r.m.client == nil {
		t.Fatal("no client")
	}
	return r
}

func (r *rig) check() { r.m.check(context.Background()) }

// phases lists the phases the shell was shown, once each in a row: a download's progress
// is more news in the same phase.
func (r *rig) phases() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var p []string
	for _, s := range r.states {
		if len(p) == 0 || p[len(p)-1] != s.Phase {
			p = append(p, s.Phase)
		}
	}
	return strings.Join(p, " ")
}

func TestDevelopmentBuildNeverChecks(t *testing.T) {
	m := New(Config{}, func(f func()) { f() }, func() {})
	m.Start()
	defer m.Stop()
	if m.client != nil || m.State().Phase != Idle {
		t.Fatalf("a build with no version must stay idle: %+v", m.State())
	}
}

func TestSameVersionShowsNothing(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	r := start(t, "0.2.0", rs.url, nil)
	r.check()
	if len(r.states) != 0 || rs.downloads.Load() != 0 {
		t.Fatalf("states %v downloads %d", r.states, rs.downloads.Load())
	}
}

func TestNewerReleaseIsStagedAndRelaunchSwapsTheBundle(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	verified := ""
	r := start(t, "0.1.0", rs.url, func(app string) error { verified = app; return nil })
	r.check()
	if r.phases() != "downloading ready" || r.states[1].Version != "0.2.0" {
		t.Fatalf("phases %q states %+v", r.phases(), r.states)
	}
	if !strings.HasPrefix(verified, r.m.stageDir()) {
		t.Fatalf("verify saw %q", verified)
	}
	if v, _ := Version(r.app); v != "0.1.0" {
		t.Fatalf("installed app changed before relaunch: %s", v)
	}
	r.check()
	if rs.downloads.Load() != 1 {
		t.Fatalf("a release already ready was downloaded again: %d", rs.downloads.Load())
	}

	r.m.Relaunch()
	if !r.quit {
		t.Fatal("the app was not asked to quit")
	}
	if v, _ := Version(r.app); v != "0.2.0" {
		t.Fatalf("installed app is %s after relaunch", v)
	}
	if v, _ := Version(filepath.Join(r.m.cfg.StateDir, "previous", "Project B.app")); v != "0.1.0" {
		t.Fatalf("previous release is %s", v)
	}
	if _, err := os.Stat(r.m.stageDir()); !os.IsNotExist(err) {
		if entries, _ := os.ReadDir(r.m.stageDir()); len(entries) != 0 {
			t.Fatalf("stage still holds %v", entries)
		}
	}
}

func TestCorruptedDownloadIsRefusedAndTriedAgain(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	tampered := append([]byte(nil), good...)
	tampered[len(tampered)/2] ^= 0xff
	rs := release(t, "0.2.0", good, tampered)
	r := start(t, "0.1.0", rs.url, nil)
	r.check()
	if r.phases() != "downloading failed" || !strings.Contains(r.states[1].Error, "sha256") {
		t.Fatalf("phases %q states %+v", r.phases(), r.states)
	}
	if v, _ := Version(r.app); v != "0.1.0" {
		t.Fatalf("installed app changed: %s", v)
	}
	if _, err := os.Stat(r.m.stageDir()); err == nil {
		t.Fatal("a refused download left a stage")
	}
	r.m.Relaunch()
	if r.quit {
		t.Fatal("relaunch with nothing ready must do nothing")
	}
	r.check()
	if rs.downloads.Load() != 2 {
		t.Fatalf("a failed download should be tried again next check: %d", rs.downloads.Load())
	}

	// The publisher pulls the release: there is nothing to say any more.
	rs.set("0.1.0", good, good)
	r.check()
	if r.m.State().Phase != Idle {
		t.Fatalf("a withdrawn release should clear the failure: %+v", r.m.State())
	}
}

func TestBundleOfAnotherVersionIsRefusedOnce(t *testing.T) {
	wrong := archive(t, fakeApp(t, t.TempDir(), "0.1.5"))
	rs := release(t, "0.2.0", wrong, wrong)
	r := start(t, "0.1.0", rs.url, nil)
	r.check()
	if r.phases() != "downloading failed" || !strings.Contains(r.states[1].Error, "says it is 0.1.5") {
		t.Fatalf("phases %q states %+v", r.phases(), r.states)
	}
	r.check()
	if rs.downloads.Load() != 1 || r.m.State().Phase != Failed {
		t.Fatalf("a refused bundle must not be downloaded again, and stays on show: %d %+v", rs.downloads.Load(), r.m.State())
	}
	// A newer release is tried.
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.1"))
	rs.set("0.2.1", good, good)
	r.check()
	if s := r.m.State(); s.Phase != Ready || s.Version != "0.2.1" {
		t.Fatalf("the next release should be taken: %+v", s)
	}
}

func TestBundleFailingVerifyIsRefused(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	r := start(t, "0.1.0", rs.url, func(string) error { return errors.New("not signed with Project B's key") })
	r.check()
	if r.phases() != "downloading failed" || !strings.Contains(r.states[1].Error, "not signed") {
		t.Fatalf("phases %q states %+v", r.phases(), r.states)
	}
	if _, err := os.Stat(r.m.stageDir()); err == nil {
		t.Fatal("a refused bundle left a stage")
	}
}

func TestUnwritableInstallLocationIsReportedWithoutDownloading(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	r := start(t, "0.1.0", rs.url, nil)
	dir := filepath.Dir(r.app)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	r.check()
	if r.phases() != "failed" || !strings.Contains(r.states[0].Error, "move Project B to Applications") || rs.downloads.Load() != 0 {
		t.Fatalf("phases %q states %+v downloads %d", r.phases(), r.states, rs.downloads.Load())
	}
	// Moved somewhere writable: the next check goes ahead.
	os.Chmod(dir, 0o755)
	r.check()
	if r.m.State().Phase != Ready {
		t.Fatalf("after the folder became writable: %+v", r.m.State())
	}
}

func TestBundleOfFindsTheApp(t *testing.T) {
	if got := BundleOf("/Applications/Project B.app/Contents/MacOS/project_b"); got != "/Applications/Project B.app" {
		t.Fatal(got)
	}
	if got := BundleOf("/Users/me/project-b/chromium/src/out/pb/project_b"); got != "" {
		t.Fatal(got)
	}
}

// eventually waits for the manager's state to match, as a check asked for runs on its own
// goroutine.
func eventually(t *testing.T, m *Manager, match func(State) bool) State {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if s := m.State(); match(s) {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("state stayed %+v", m.State())
	return State{}
}

func TestCheckNowSaysUpToDate(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	r := start(t, "0.2.0", rs.url, nil)
	r.m.CheckNow()
	eventually(t, r.m, func(s State) bool { return s.Check == UpToDate && s.Phase == Idle })
	if rs.manifests.Load() != 1 || rs.downloads.Load() != 0 {
		t.Fatalf("manifests %d downloads %d", rs.manifests.Load(), rs.downloads.Load())
	}
}

func TestCheckNowSaysWhyTheManifestCouldNotBeRead(t *testing.T) {
	r := start(t, "0.2.0", "http://127.0.0.1:1/releases.json", nil)
	r.m.CheckNow()
	s := eventually(t, r.m, func(s State) bool { return s.Check == CheckFailed })
	if s.CheckError == "" || s.Phase != Idle {
		t.Fatalf("state %+v", s)
	}
}

func TestCheckNowTakesANewerReleaseTheWayAnAutomaticCheckDoes(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	r := start(t, "0.1.0", rs.url, nil)
	r.m.CheckNow()
	s := eventually(t, r.m, func(s State) bool { return s.Phase == Ready })
	if s.Version != "0.2.0" || s.Check != "" {
		t.Fatalf("state %+v", s)
	}
}

func TestDevelopmentBuildSaysItDoesNotUpdate(t *testing.T) {
	m := New(Config{}, func(f func()) { f() }, func() {})
	m.CheckNow()
	if s := m.State(); s.Check != CheckFailed || !strings.Contains(s.CheckError, "development build") {
		t.Fatalf("state %+v", s)
	}
}

// Off, the timer doesn't check; a check asked for still does. On again, the timer starts
// as it does at launch.
func TestAutomaticChecksTurnOffAndOnAtOnce(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	installed := t.TempDir()
	cfg := Config{Project: "project-b", Version: "0.2.0", Releases: rs.url, StateDir: filepath.Join(t.TempDir(), "updates"), App: fakeApp(t, installed, "0.2.0")}
	m := New(cfg, func(f func()) { f() }, func() {})
	m.SetAuto(false)
	m.Start()
	defer m.Stop()
	time.Sleep(firstCheck + time.Second)
	if n := rs.manifests.Load(); n != 0 {
		t.Fatalf("off, the timer still checked %d times", n)
	}
	m.CheckNow()
	eventually(t, m, func(s State) bool { return s.Check == UpToDate })
	if n := rs.manifests.Load(); n != 1 {
		t.Fatalf("a check asked for made %d requests", n)
	}
	m.SetAuto(true)
	deadline := time.Now().Add(firstCheck + 3*time.Second)
	for rs.manifests.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if n := rs.manifests.Load(); n != 2 {
		t.Fatalf("on again, the timer made %d requests in all", n)
	}
}

func TestInstallTakesTheDownloadAsItIs(t *testing.T) {
	apk := []byte("an APK, as far as the updater cares")
	rs := release(t, "0.2.0", apk, apk)
	var installed []string
	cfg := Config{Project: "project-b", Version: "0.1.0", Releases: rs.url, StateDir: filepath.Join(t.TempDir(), "updates"),
		Install: func(file string) { installed = append(installed, file) }}
	quit := false
	m := New(cfg, func(f func()) { f() }, func() { quit = true })
	m.check(context.Background())
	if s := m.State(); s.Phase != Ready || s.Version != "0.2.0" {
		t.Fatalf("state %+v", s)
	}
	m.Relaunch()
	m.Relaunch()
	if len(installed) != 2 || installed[0] != installed[1] || !strings.HasPrefix(installed[0], m.stageDir()) {
		t.Fatalf("installed %v", installed)
	}
	if b, err := os.ReadFile(installed[0]); err != nil || string(b) != string(apk) {
		t.Fatalf("staged %q %v", b, err)
	}
	if quit {
		t.Fatal("the app quit; the installer replaces it")
	}
}

func TestDownloadShowsHowMuchHasArrived(t *testing.T) {
	good := archive(t, fakeApp(t, t.TempDir(), "0.2.0"))
	rs := release(t, "0.2.0", good, good)
	half := make(chan struct{})
	rest := make(chan struct{})
	rs.serve = func(w http.ResponseWriter) {
		w.Write(good[:len(good)/2])
		w.(http.Flusher).Flush()
		close(half)
		<-rest
		w.Write(good[len(good)/2:])
	}
	r := start(t, "0.1.0", rs.url, nil)
	done := make(chan struct{})
	go func() { r.check(); close(done) }()
	<-half
	s := eventually(t, r.m, func(s State) bool { return s.Done > 0 })
	close(rest)
	<-done
	if s.Phase != Downloading || s.Done != int64(len(good)/2) || s.Total != int64(len(good)) {
		t.Fatalf("mid-download state %+v, want %d of %d", s, len(good)/2, len(good))
	}
	if r.phases() != "downloading ready" {
		t.Fatalf("phases %q", r.phases())
	}
}
