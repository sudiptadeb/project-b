// Command dev runs the real managers on the mock engine, with no Chromium. The shell
// runs in an ordinary browser tab and reaches them over HTTP.
package main

import (
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

	"projectb/app"
	"projectb/mockengine"
	"projectb/update"
)

//go:embed mock_page.html
var mockPage embed.FS

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "listen address")
	shellDir := flag.String("shell", "../shell/dist", "built shell")
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
	flag.Parse()

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
	mux.Handle("/", http.FileServer(http.Dir(*shellDir)))
	mux.HandleFunc("GET /shell/events", func(w http.ResponseWriter, r *http.Request) { stream(w, r, &e.Shell) })
	mux.HandleFunc("POST /shell/send", func(w http.ResponseWriter, r *http.Request) {
		msg, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		e.Post(func() { a.ShellMessage(msg) })
	})
	mux.HandleFunc("GET /mock/page", func(w http.ResponseWriter, r *http.Request) {
		b, _ := mockPage.ReadFile("mock_page.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	})
	mux.HandleFunc("GET /mock/events", func(w http.ResponseWriter, r *http.Request) { stream(w, r, &e.Page) })
	mux.HandleFunc("POST /mock/fire", func(w http.ResponseWriter, r *http.Request) {
		e.Fire(r.FormValue("kind"), r.FormValue("value"))
	})
	mux.HandleFunc("GET /mock/regions", func(w http.ResponseWriter, r *http.Request) {
		regions, drag, bounds := e.Regions()
		out := map[string]any{"regions": regions, "drag": drag, "bounds": bounds, "closed": e.Closed(), "hidden": e.Hidden()}
		if q := r.URL.Query(); q.Has("x") {
			x, _ := strconv.ParseFloat(q.Get("x"), 64)
			y, _ := strconv.ParseFloat(q.Get("y"), 64)
			out["hit"] = e.Hit(x, y)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})

	log.Printf("mock engine on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// stream sends a feed to the browser as server-sent events.
func stream(w http.ResponseWriter, r *http.Request, f *mockengine.Feed) {
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
