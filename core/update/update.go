// Package update keeps the installed app current. It asks the project's release manifest
// for a newer release, downloads it, checks it, stages it, and when the user says so swaps
// the installed bundle and relaunches into the new one.
//
// amarit does the manifest and the verified download. The bundle is ours: the app is a
// signed folder of two binaries, a library and resources, not one file amarit could rename,
// so the unpacking, the signature check, the swap and the relaunch live here.
package update

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/sudiptadeb/amarit"
)

// CheckInterval is how often the app looks for a release. One minute while the updater is
// new, so a release shows up within the minute; the usual cadence would be hours.
const CheckInterval = time.Minute

// firstCheck is how long after start the first check runs: enough for the window to be up.
const firstCheck = 2 * time.Second

// Phases of an update. Idle is empty, so an idle state has nothing to show.
const (
	Idle        = ""
	Downloading = "downloading"
	Ready       = "ready"
	Failed      = "failed"
)

// What a check asked for by the user found, beside the phase. Nothing is empty.
const (
	Checking    = "checking"
	UpToDate    = "current"
	CheckFailed = "failed"
)

// State is what the shell's bubble and the settings page show.
type State struct {
	Phase   string `json:"phase"`
	Version string `json:"version,omitempty"` // the release being downloaded, ready, or refused
	Error   string `json:"error,omitempty"`   // why it failed, in words a person can read
	// Check is how a check the user asked for went: Checking while it runs, then UpToDate,
	// or CheckFailed with CheckError when the manifest couldn't be read. A check that finds
	// a release moves Phase instead. Only the settings page shows these; the bubble shows
	// Phase alone, so automatic checks never set them.
	Check      string `json:"check,omitempty"`
	CheckError string `json:"checkError,omitempty"`
}

// Config says what this build is and where it lives.
type Config struct {
	Project  string // the manifest's project name
	Version  string // the running version; empty is a development build, which never updates
	Releases string // the manifest URL
	StateDir string // where downloads, the staged release, the previous one and amarit's state live
	App      string // the installed bundle this process runs from
	// Verify checks an unpacked bundle before it is staged: the signature, for the real app.
	// Nil checks only that the bundle says it is the version the manifest promised.
	Verify func(app string) error
}

// Manager owns the update state. Every method runs on the main thread; the check loop
// runs on its own goroutine and comes back through post.
type Manager struct {
	cfg    Config
	post   func(func())
	quit   func()
	client *amarit.Client

	mu     sync.Mutex
	state  State
	staged string // the checked bundle waiting for Relaunch

	checking sync.Mutex // one check at a time, automatic or asked for

	// Main thread only.
	auto    bool          // check every CheckInterval
	started bool          // Start has run
	timer   chan struct{} // closing it ends the running timer; nil when none runs

	onChanged []func(State)
}

// New returns a manager. post runs a function on the main thread; quit ends the app, which
// Relaunch calls once the new bundle is in place.
func New(cfg Config, post func(func()), quit func()) *Manager {
	m := &Manager{cfg: cfg, post: post, quit: quit, auto: true}
	if cfg.Version == "" {
		return m
	}
	c, err := amarit.NewClient(filepath.Join(cfg.StateDir, "amarit"), "stable",
		amarit.Project(cfg.Project), amarit.Version(cfg.Version), amarit.Releases(cfg.Releases))
	if err != nil {
		log.Printf("update: %v; updates are off", err)
		return m
	}
	m.client = c
	return m
}

// State returns what is happening now.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// OnChanged subscribes to the state changing. Called on the main thread.
func (m *Manager) OnChanged(f func(State)) { m.onChanged = append(m.onChanged, f) }

// Running is the version this build is: empty for a development build.
func (m *Manager) Running() string { return m.cfg.Version }

// Start begins checking every CheckInterval, unless automatic checks are off. A
// development build, or one amarit could not be configured for, never checks. Whatever an
// earlier run left half done is cleared first.
func (m *Manager) Start() {
	if m.client == nil {
		return
	}
	os.RemoveAll(m.stageDir())
	os.RemoveAll(m.downloadDir())
	m.started = true
	log.Printf("update: %s %s, %s", m.cfg.Project, m.cfg.Version, m.cfg.Releases)
	if m.auto {
		m.startTimer()
	}
}

// Stop ends the checking.
func (m *Manager) Stop() {
	m.started = false
	m.stopTimer()
}

// SetAuto turns the check every CheckInterval on or off, at once. Turned on again, the
// timer starts as it does at launch. A check the user asks for runs either way.
func (m *Manager) SetAuto(on bool) {
	if on == m.auto {
		return
	}
	m.auto = on
	if !m.started {
		return
	}
	if on {
		m.startTimer()
	} else {
		log.Printf("update: automatic checks are off")
		m.stopTimer()
	}
}

func (m *Manager) startTimer() {
	log.Printf("update: checking every %s", CheckInterval)
	m.timer = make(chan struct{})
	go m.loop(m.timer)
}

func (m *Manager) stopTimer() {
	if m.timer != nil {
		close(m.timer)
		m.timer = nil
	}
}

func (m *Manager) loop(stop chan struct{}) {
	wait := firstCheck
	for {
		select {
		case <-stop:
			return
		case <-time.After(wait):
		}
		m.check(context.Background())
		wait = CheckInterval
	}
}

// CheckNow checks at once, whether or not automatic checks are on, and says how it went in
// State.Check. A newer release goes the way an automatic check's does: download, checks,
// stage, and the bubble offers relaunch.
func (m *Manager) CheckNow() {
	if m.client == nil {
		m.setCheck(CheckFailed, "a development build doesn't update")
		return
	}
	m.setCheck(Checking, "")
	go func() {
		current, err := m.check(context.Background())
		switch {
		case err != nil:
			m.setCheck(CheckFailed, err.Error())
		case current:
			m.setCheck(UpToDate, "")
		default:
			m.setCheck("", "")
		}
	}()
}

// check is one look at the manifest, and the download, unpack and checks when it names a
// newer release. current says nothing newer was found and nothing is on show; err says the
// manifest couldn't be read. An automatic check only logs that error: the next minute
// tries again. A download that fails is shown.
func (m *Manager) check(ctx context.Context) (current bool, err error) {
	m.checking.Lock()
	defer m.checking.Unlock()
	if err := writable(filepath.Dir(m.cfg.App)); err != nil {
		m.set(State{Phase: Failed, Error: err.Error()})
		return false, nil
	}
	d, err := m.client.Check(ctx)
	if err != nil {
		log.Printf("update: check: %v", err)
		return false, err
	}
	log.Printf("update: %s", d.Reason)
	m.mu.Lock()
	s := m.state
	m.mu.Unlock()
	if !d.Update {
		// A failure stays on show while the channel still points at the release that
		// failed; once it points elsewhere, or at nothing newer, there is nothing to say.
		if s.Phase == Failed && s.Version != d.Version {
			m.set(State{})
		}
		return m.State().Phase == Idle, nil
	}
	if s.Phase == Ready && s.Version == d.Version {
		return false, nil
	}
	m.set(State{Phase: Downloading, Version: d.Version})
	app, err := m.fetch(ctx, d)
	if err != nil {
		log.Printf("update: %s: %v", d.Version, err)
		m.set(State{Phase: Failed, Version: d.Version, Error: err.Error()})
		return false, nil
	}
	m.mu.Lock()
	m.staged = app
	m.mu.Unlock()
	log.Printf("update: %s is staged at %s; ready to relaunch", d.Version, app)
	m.set(State{Phase: Ready, Version: d.Version})
	return false, nil
}

// fetch downloads the release and unpacks it into the stage directory, and checks that the
// bundle inside says it is that version and passes Verify. A download that fails is tried
// again next check; a bundle that fails its checks is refused for good, so a bad release is
// downloaded once, not every minute.
func (m *Manager) fetch(ctx context.Context, d *amarit.Decision) (string, error) {
	stage, downloads := m.stageDir(), m.downloadDir()
	os.RemoveAll(stage)
	os.RemoveAll(downloads)
	if err := os.MkdirAll(downloads, 0o700); err != nil {
		return "", err
	}
	defer os.RemoveAll(downloads)
	archive, err := m.client.Download(ctx, d, downloads)
	if err != nil {
		return "", err
	}
	app, err := unpack(archive, stage)
	if err == nil {
		err = m.checkBundle(app, d.Version)
	}
	if err != nil {
		os.RemoveAll(stage)
		if rerr := m.client.Reject(d.Version); rerr != nil {
			log.Printf("update: %v", rerr)
		}
		return "", fmt.Errorf("%s was refused: %w", d.Version, err)
	}
	return app, nil
}

func (m *Manager) checkBundle(app, version string) error {
	v, _, err := bundleInfo(app)
	if err != nil {
		return err
	}
	if v != version {
		return fmt.Errorf("the download says it is %s", v)
	}
	if m.cfg.Verify != nil {
		return m.cfg.Verify(app)
	}
	return nil
}

// Relaunch puts the staged bundle where the running one is, keeps the running one as the
// previous release, and ends the app; a helper left behind starts the new one once this
// process has gone. Nothing happens unless a release is ready.
func (m *Manager) Relaunch() {
	m.mu.Lock()
	staged := m.staged
	m.mu.Unlock()
	if staged == "" {
		return
	}
	_, executable, err := bundleInfo(staged)
	if err != nil {
		m.set(State{Phase: Failed, Version: m.State().Version, Error: err.Error()})
		return
	}
	previous := filepath.Join(m.cfg.StateDir, "previous", filepath.Base(m.cfg.App))
	os.RemoveAll(filepath.Dir(previous))
	if err := os.MkdirAll(filepath.Dir(previous), 0o700); err == nil {
		err = os.Rename(m.cfg.App, previous)
	}
	if err != nil {
		m.set(State{Phase: Failed, Version: m.State().Version, Error: "couldn't move the installed app aside: " + err.Error()})
		return
	}
	if err := os.Rename(staged, m.cfg.App); err != nil {
		os.Rename(previous, m.cfg.App)
		m.set(State{Phase: Failed, Version: m.State().Version, Error: "couldn't put the new app in place: " + err.Error()})
		return
	}
	m.mu.Lock()
	m.staged = ""
	m.mu.Unlock()
	log.Printf("update: %s is in place, previous release kept at %s; relaunching", m.cfg.App, previous)
	relaunch(filepath.Join(m.cfg.App, "Contents", "MacOS", executable))
	m.quit()
}

// relaunch starts the new app after this process has exited, from a shell that outlives
// it. Starting it any earlier would run two copies on one profile.
func relaunch(executable string) {
	cmd := exec.Command("/bin/sh", "-c", `while kill -0 "$1" 2>/dev/null; do sleep 0.2; done; exec "$2"`,
		"relaunch", strconv.Itoa(os.Getpid()), executable)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		log.Printf("update: relaunch: %v", err)
	}
}

// set stores the state and tells the listeners, on the main thread.
func (m *Manager) set(s State) {
	m.mu.Lock()
	changed := m.state != s
	m.state = s
	m.mu.Unlock()
	if changed {
		m.notify(s)
	}
}

// setCheck changes only how the asked-for check went.
func (m *Manager) setCheck(check, err string) {
	m.mu.Lock()
	s := m.state
	s.Check, s.CheckError = check, err
	changed := m.state != s
	m.state = s
	m.mu.Unlock()
	if changed {
		m.notify(s)
	}
}

func (m *Manager) notify(s State) {
	m.post(func() {
		for _, f := range m.onChanged {
			f(s)
		}
	})
}

func (m *Manager) stageDir() string    { return filepath.Join(m.cfg.StateDir, "stage") }
func (m *Manager) downloadDir() string { return filepath.Join(m.cfg.StateDir, "download") }

// writable says whether the app can be replaced where it is installed, in words for the
// overlay when it can't: a copy still on its DMG, or in a folder the user doesn't own.
func writable(dir string) error {
	if err := syscall.Access(dir, 2); err != nil {
		return fmt.Errorf("can't update in %s: move Project B to Applications", dir)
	}
	return nil
}
