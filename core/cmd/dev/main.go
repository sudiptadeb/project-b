// Command dev runs the real managers on the mock engine, with no Chromium. The browser
// tab is the OS window: the window page at / holds the mock page, the shell and the
// settings page in frames where the real window holds views (window.html). The shell and
// the settings page are the real app's, unchanged; the server plays the engine's side of
// their channel (webui.js).
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"projectb/app"
	"projectb/engine"
	"projectb/flags"
	"projectb/mockengine"
	"projectb/profiles"
	"projectb/shells"
	"projectb/tabs"
	"projectb/timeline"
	"projectb/update"
)

//go:embed mock_page.html window.html webui.js
var pages embed.FS

// routes are what a build tag adds to the server for its tests.
var routes []func(mux *http.ServeMux, e *mockengine.Engine)

// rootOf is the root -root names, else the profile folder's: the mock's profile is a
// folder named outright, as --user-data-dir names one.
func rootOf(root, profile string) string {
	if root != "" {
		return root
	}
	return filepath.Dir(profile)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "listen address; port 0 takes a free one, which the server prints")
	shellDir := flag.String("shell", "../shell/dist", "the built pages, one folder each")
	home, _ := os.UserHomeDir()
	profile := flag.String("profile", filepath.Join(home, ".project-b", "dev"), "profile directory")
	root := flag.String("root", "", "the folder of the profile list the Profiles menu reads; the profile's own folder's by default")
	id := flag.String("id", "", "the id -profile has in the list in -root, for a listed profile; none for a folder named outright")
	pick := flag.Bool("picker", false, "start as a copy that picks its profile from the list in -root, and becomes the one picked")
	block := flag.String("block", "", "comma-separated blocked hosts")
	// The updater runs for real against a manifest when given a version to be, the bundle
	// it is "installed" as, and the manifest. Without a version it stays off, as it does
	// for a binary running from a build directory. The mock has no signing key, so it
	// checks only the version the bundle says it is.
	version := flag.String("version", "", "the version this build is")
	releases := flag.String("releases", "", "the release manifest URL or path")
	appPath := flag.String("app", "", "the installed bundle to replace on relaunch")
	// A test can add flags of its own, in registry.json's form, to see every type drawn.
	moreFlags := flag.String("flags", "", "a file of flags to add to the registry's")
	use := flag.String("use", "", "the shell to choose at start, by its id, as the settings page would")
	flag.Parse()
	if *use != "" && !shells.Has(*use) {
		log.Fatalf("-use: no shell %q in core/shells/registry.json", *use)
	}
	if *moreFlags != "" {
		b, err := os.ReadFile(*moreFlags)
		if err == nil {
			var rows []flags.Flag
			if rows, err = flags.Parse(b); err == nil {
				flags.All = append(flags.All, rows...)
			}
		}
		if err != nil {
			log.Fatalf("-flags: %v", err)
		}
	}

	e := mockengine.New()
	// The flags saved in the profile reach the engine at launch, as the real engine's
	// command line, so one turned on in settings takes effect on Relaunch, as it does there.
	e.SetSwitches(flags.New(*profile).Switches())
	list := profiles.New(rootOf(*root, *profile))
	// A listed profile's update state is the root's, as in the real app; the mock starts no
	// copies of itself, so a relaunch restarts this one alone.
	start := func(dir, id string) *app.App {
		updates := filepath.Join(dir, "updates")
		if id != "" {
			updates = filepath.Join(list.Root(), "update")
		}
		return app.Start(e, app.Config{
			ProfileDir:   dir,
			Profiles:     list,
			Profile:      id,
			BlockedHosts: strings.Split(*block, ","),
			Update: update.Config{
				Project:  "project-b",
				Version:  *version,
				Releases: *releases,
				StateDir: updates,
				App:      *appPath,
			},
			CrashReports: filepath.Join(home, "Library/Logs/DiagnosticReports"),
		})
	}
	// The app, or the picker in a picker copy, which opens a profile by naming it on
	// /mock/regions, as the mock starts no copies of itself.
	var a *app.App
	var p *app.Picker
	var opened atomic.Value
	opened.Store("")
	started := make(chan struct{})
	e.Post(func() {
		if *pick {
			p = app.StartPicker(e, list, func(id string) { opened.Store(id) })
			e.SetListener(p)
		} else {
			a = start(*profile, *id)
			e.SetListener(a)
			if *use != "" {
				a.Prefs.SetShell(*use)
			}
		}
		close(started)
	})
	go e.Run()
	<-started

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		done := make(chan struct{})
		e.Post(func() {
			if a != nil {
				a.Stop()
			}
			close(done)
		})
		<-done
		os.Exit(0)
	}()

	// Each window is an OS window page of its own, at /?window=<id>, window 1 at plain /.
	// It passes its id on to every page and feed it holds, and every route below takes it
	// the same way, 1 when it names none.
	mux := http.NewServeMux()
	// The OS window, at / and at a path npm run dev's server sends on to this one.
	mux.HandleFunc("GET /{$}", serve("window.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /mock/window", serve("window.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /mock/webui.js", serve("webui.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("POST /mock/window/size", func(w http.ResponseWriter, r *http.Request) {
		width, err1 := strconv.ParseFloat(r.FormValue("w"), 64)
		height, err2 := strconv.ParseFloat(r.FormValue("h"), 64)
		if err1 != nil || err2 != nil {
			http.Error(w, "w and h are numbers", http.StatusBadRequest)
			return
		}
		e.Resize(windowOf(r), width, height)
	})
	// When a page's own clock started, its performance.timeOrigin in ms of wall time, which
	// the real engine gives the timeline as the page's document starts.
	mux.HandleFunc("POST /mock/origin", func(w http.ResponseWriter, r *http.Request) {
		ms, err := strconv.ParseFloat(r.FormValue("t"), 64)
		if err != nil {
			http.Error(w, "t is a number", http.StatusBadRequest)
			return
		}
		timeline.PageOrigin(r.FormValue("page"), timeline.FromWall(time.UnixMicro(int64(ms*1000))))
	})
	// Each shell, at /shell/<id>/, as the real app has it at chrome://shell/<id>/.
	mux.HandleFunc("GET /shell/{id}/", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		http.StripPrefix("/shell/"+id+"/", webUI(filepath.Join(*shellDir, "shell", id))).ServeHTTP(w, r)
	})
	mux.HandleFunc("GET /shell/events", func(w http.ResponseWriter, r *http.Request) { stream(w, r, &e.Feeds(windowOf(r)).Shell) })
	mux.HandleFunc("POST /shell/send", func(w http.ResponseWriter, r *http.Request) {
		msg, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		win := windowOf(r)
		e.Post(func() {
			// As in the real engine, a page whose window closed speaks to no one, and only
			// a window's own page is its shell.
			if e.Open(win) && e.PageOf(win).Shell() != "" {
				a.ShellMessage(win, msg)
			}
		})
	})
	// The picker page, in a picker copy: what the engine sends a window's page reaches it.
	mux.Handle("GET /picker/", http.StripPrefix("/picker/", webUI(filepath.Join(*shellDir, "picker"))))
	mux.HandleFunc("GET /picker/events", func(w http.ResponseWriter, r *http.Request) { stream(w, r, &e.Feeds(windowOf(r)).Shell) })
	mux.HandleFunc("POST /picker/send", func(w http.ResponseWriter, r *http.Request) {
		msg, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		win := windowOf(r)
		e.Post(func() {
			if e.Open(win) && e.PageOf(win) == engine.PagePicker {
				p.Message(win, msg)
			}
		})
	})
	// The settings page, an app of its own as in the real app, with its own channel.
	mux.Handle("GET /settings/", http.StripPrefix("/settings/", webUI(filepath.Join(*shellDir, "settings"))))
	mux.HandleFunc("GET /settings/events", func(w http.ResponseWriter, r *http.Request) { stream(w, r, &e.Feeds(windowOf(r)).Settings) })
	mux.HandleFunc("POST /settings/send", func(w http.ResponseWriter, r *http.Request) {
		msg, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		win := windowOf(r)
		e.Post(func() {
			if e.Open(win) {
				a.SettingsMessage(win, msg)
			}
		})
	})
	mux.HandleFunc("GET /mock/page", serve("mock_page.html", "text/html; charset=utf-8"))
	// Each mock page is one slot of its window's, ?slot=<n>, and hears its slot's state, the
	// last one first.
	mux.HandleFunc("GET /mock/events", func(w http.ResponseWriter, r *http.Request) {
		win := windowOf(r)
		slot, _ := strconv.Atoi(r.URL.Query().Get("slot"))
		stream(w, r, &e.Feeds(win).Page, func() []byte { return e.PageState(win, slot) })
	})
	// What the window page holds and how it hit-tests: the state now, then each change.
	mux.HandleFunc("GET /mock/window/events", func(w http.ResponseWriter, r *http.Request) {
		win := windowOf(r)
		stream(w, r, &e.Feeds(win).Window, func() []byte { return e.WindowState(win) })
	})
	// What a page or the user does: kind, its value, and the tab whose page it is in, or
	// none for the first one the window shows.
	mux.HandleFunc("POST /mock/fire", func(w http.ResponseWriter, r *http.Request) {
		tab, _ := strconv.ParseInt(r.FormValue("tab"), 10, 64)
		e.Fire(windowOf(r), engine.TabID(tab), r.FormValue("kind"), r.FormValue("value"))
	})
	mux.HandleFunc("GET /mock/regions", func(w http.ResponseWriter, r *http.Request) {
		win := windowOf(r)
		l := e.Layout(win)
		shell, drag := []engine.Rect{}, []engine.Rect{}
		for _, g := range l.Regions {
			if g.Area == engine.AreaDrag {
				drag = append(drag, g.Rect)
			} else {
				shell = append(shell, g.Rect)
			}
		}
		// Go's own state of the window: its tabs in order, and the active one.
		goTabs, active := []tabs.Tab{}, engine.TabID(0)
		done := make(chan struct{})
		e.Post(func() {
			if a != nil && e.Open(win) {
				goTabs, active = a.Tabs.AllIn(win), a.Tabs.ActiveIn(win)
			}
			close(done)
		})
		<-done
		out := map[string]any{"tabs": goTabs, "active": active, "regions": shell, "drag": drag, "bounds": mockengine.Page(l), "visible": append([]engine.VisibleTab{}, l.Tabs...), "closed": e.Closed(win), "quit": e.Quit(), "windows": e.Windows(), "hidden": e.Hidden(win), "windowColor": l.FrameColor, "shownColor": e.ShownColor(), "frameWidth": l.FrameWidth, "settingsOpen": e.SettingsOpen(win), "page": e.PageOf(win), "shell": e.PageOf(win).Shell(), "focused": e.Focused(), "unshownFocus": e.UnshownFocus(), "dock": e.DockColor(), "opened": opened.Load(), "menu": e.Menu(), "edits": e.Edits(), "window": e.Reported(win)}
		if q := r.URL.Query(); q.Has("x") {
			x, _ := strconv.ParseFloat(q.Get("x"), 64)
			y, _ := strconv.ParseFloat(q.Get("y"), 64)
			out["hit"] = mockengine.AreaNames[l.At(x, y)]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})
	for _, add := range routes {
		add(mux, e)
	}

	// The address it got, which a test that asked for port 0 reads from this line.
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("mock engine on http://%s", l.Addr())
	log.Fatal(http.Serve(l, mux))
}

// serve answers with one of the server's own pages.
func serve(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, _ := pages.ReadFile(name)
		w.Header().Set("Content-Type", contentType)
		w.Write(b)
	}
}

// webUI serves one of our pages' folders as the real app's WebUI does, with what the
// engine gives the page, chrome.send and the call back, in its index.html before the
// page's own scripts.
func webUI(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "" && r.URL.Path != "index.html" {
			files.ServeHTTP(w, r)
			return
		}
		b, err := os.ReadFile(filepath.Join(dir, "index.html"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		b = bytes.Replace(b, []byte("</head>"), []byte("<script src=\"/mock/webui.js\"></script>\n</head>"), 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	})
}

// windowOf is the window a request is for: its window parameter, or 1.
func windowOf(r *http.Request) engine.WindowID {
	if w, err := strconv.ParseInt(r.URL.Query().Get("window"), 10, 64); err == nil {
		return engine.WindowID(w)
	}
	return 1
}

// stream sends a feed to the browser as server-sent events, after now's message if it is
// given and there is one: it is read once subscribed, so no change falls between it and
// the feed.
func stream(w http.ResponseWriter, r *http.Request, f *mockengine.Feed, now ...func() []byte) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	msgs, stop := f.Subscribe()
	defer stop()
	fmt.Fprint(w, ": connected\n\n")
	for _, msg := range now {
		if b := msg(); b != nil {
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
	}
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case m := <-msgs:
			fmt.Fprintf(w, "data: %s\n\n", m)
			flusher.Flush()
		}
	}
}
