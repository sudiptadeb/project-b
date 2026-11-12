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
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"projectb/app"
	"projectb/engine"
	"projectb/flags"
	"projectb/mockengine"
	"projectb/timeline"
	"projectb/update"
)

//go:embed mock_page.html window.html webui.js
var pages embed.FS

// routes are what a build tag adds to the server for its tests.
var routes []func(mux *http.ServeMux, e *mockengine.Engine)

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "listen address")
	shellDir := flag.String("shell", "../shell/dist", "the built pages, one folder each")
	home, _ := os.UserHomeDir()
	profile := flag.String("profile", filepath.Join(home, ".project-b", "dev"), "profile directory")
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
	flag.Parse()
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
	var a *app.App
	started := make(chan struct{})
	e.Post(func() {
		a = app.Start(e, app.Config{
			ProfileDir:   *profile,
			BlockedHosts: strings.Split(*block, ","),
			Update: update.Config{
				Project:  "project-b",
				Version:  *version,
				Releases: *releases,
				StateDir: filepath.Join(*profile, "updates"),
				App:      *appPath,
			},
			CrashReports: filepath.Join(home, "Library/Logs/DiagnosticReports"),
		})
		e.SetListener(a)
		close(started)
	})
	go e.Run()
	<-started

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		done := make(chan struct{})
		e.Post(func() { a.Stop(); close(done) })
		<-done
		os.Exit(0)
	}()

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
		e.Resize(width, height)
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
	mux.Handle("GET /shell/", http.StripPrefix("/shell/", webUI(filepath.Join(*shellDir, "shell"))))
	mux.HandleFunc("GET /shell/events", func(w http.ResponseWriter, r *http.Request) { stream(w, r, &e.Shell) })
	mux.HandleFunc("POST /shell/send", func(w http.ResponseWriter, r *http.Request) {
		msg, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		e.Post(func() { a.ShellMessage(msg) })
	})
	// The settings page, an app of its own as in the real app, with its own channel.
	mux.Handle("GET /settings/", http.StripPrefix("/settings/", webUI(filepath.Join(*shellDir, "settings"))))
	mux.HandleFunc("GET /settings/events", func(w http.ResponseWriter, r *http.Request) { stream(w, r, &e.Settings) })
	mux.HandleFunc("POST /settings/send", func(w http.ResponseWriter, r *http.Request) {
		msg, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		e.Post(func() { a.SettingsMessage(msg) })
	})
	mux.HandleFunc("GET /mock/page", serve("mock_page.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /mock/events", func(w http.ResponseWriter, r *http.Request) { stream(w, r, &e.Page) })
	// What the window page holds and how it hit-tests: the state now, then each change.
	mux.HandleFunc("GET /mock/window/events", func(w http.ResponseWriter, r *http.Request) {
		stream(w, r, &e.Window, e.WindowState)
	})
	mux.HandleFunc("POST /mock/fire", func(w http.ResponseWriter, r *http.Request) {
		e.Fire(r.FormValue("kind"), r.FormValue("value"))
	})
	mux.HandleFunc("GET /mock/regions", func(w http.ResponseWriter, r *http.Request) {
		l := e.Layout()
		shell, drag := []engine.Rect{}, []engine.Rect{}
		for _, g := range l.Regions {
			if g.Area == engine.AreaDrag {
				drag = append(drag, g.Rect)
			} else {
				shell = append(shell, g.Rect)
			}
		}
		out := map[string]any{"regions": shell, "drag": drag, "bounds": l.Page, "closed": e.Closed(), "hidden": e.Hidden(), "windowColor": l.FrameColor, "shownColor": e.ShownColor(), "frameWidth": l.FrameWidth, "settingsOpen": e.SettingsOpen(), "menu": e.Menu(), "edits": e.Edits(), "window": e.Reported()}
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

	log.Printf("mock engine on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
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

// stream sends a feed to the browser as server-sent events, after now's message if it is
// given: it is read once subscribed, so no change falls between it and the feed.
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
		fmt.Fprintf(w, "data: %s\n\n", msg())
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
