// The mock extension store: a web server on this machine that serves the folder
// tools/fetch-extensions.sh fills. It is a website to visit, as the Chrome Web Store is:
// its page installs through chrome.webstorePrivate, which the browser gives it when it
// is started with
//
//	--apps-gallery-url=http://127.0.0.1:8766
//	--apps-gallery-download-url=http://127.0.0.1:8766/download/%s
//
//	GET /                 the store's page: the catalogue, with a button for each
//	GET /catalogue.json   what the store has, one entry per extension
//	GET /manifest/<id>    an extension's manifest, which the page hands the browser
//	GET /download/<id>    its package, where the browser fetches it to install
//	GET /crx/<file>       a package, byte for byte as it was fetched
//	GET /icon/<id>.png    its icon
//	GET /update?x=...     gupdate XML, as the Web Store's update endpoint answers
//
// See docs/extensions.md.
package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

// One catalogue entry, as tools/extension_catalogue.py writes it.
type entry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Size    int64  `json:"size"`
	Crx     string `json:"crx"`
}

type gupdate struct {
	XMLName  xml.Name `xml:"gupdate"`
	Xmlns    string   `xml:"xmlns,attr"`
	Protocol string   `xml:"protocol,attr"`
	Apps     []app    `xml:"app"`
}

type app struct {
	ID     string       `xml:"appid,attr"`
	Status string       `xml:"status,attr"`
	Check  *updateCheck `xml:"updatecheck"`
}

type updateCheck struct {
	Codebase string `xml:"codebase,attr,omitempty"`
	Size     int64  `xml:"size,attr,omitempty"`
	Version  string `xml:"version,attr,omitempty"`
	Status   string `xml:"status,attr"`
}

//go:embed store.html
var storePage []byte

// read returns the catalogue, read each time, so a package added while the store runs shows.
func read(dir string) ([]entry, error) {
	data, err := os.ReadFile(filepath.Join(dir, "catalogue.json"))
	if err != nil {
		return nil, err
	}
	var catalogue []entry
	err = json.Unmarshal(data, &catalogue)
	return catalogue, err
}

// find returns the catalogue's entry for an id.
func find(dir, id string) (entry, bool) {
	catalogue, _ := read(dir)
	for _, e := range catalogue {
		if e.ID == id {
			return e, true
		}
	}
	return entry{}, false
}

// manifest reads manifest.json out of a CRX3 file: "Cr24", 3, the header's length, the
// header, then a zip.
func manifest(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	archive := data[12+binary.LittleEndian.Uint32(data[8:]):]
	z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(z, "manifest.json")
}

func main() {
	dir := flag.String("dir", ".tools/extensions", "the folder to serve")
	addr := flag.String("addr", "127.0.0.1:8766", "where to listen")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(*dir)))
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(storePage)
	})
	mux.HandleFunc("/manifest/{id}", func(w http.ResponseWriter, r *http.Request) {
		e, ok := find(*dir, r.PathValue("id"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		m, err := manifest(filepath.Join(*dir, e.Crx))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(m)
	})
	mux.HandleFunc("/download/{id}", func(w http.ResponseWriter, r *http.Request) {
		e, ok := find(*dir, r.PathValue("id"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		// The type the Web Store sends, which is how the browser knows a download is a package.
		w.Header().Set("Content-Type", "application/x-chrome-extension")
		http.ServeFile(w, r, filepath.Join(*dir, e.Crx))
	})
	mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		catalogue, err := read(*dir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		answer := gupdate{Xmlns: "http://www.google.com/update2/response", Protocol: "2.0"}
		// Each x is one extension: "id=<id>&v=<the version the browser has>&uc".
		for _, x := range r.URL.Query()["x"] {
			asked, _ := url.ParseQuery(x)
			answer.Apps = append(answer.Apps, check(catalogue, asked.Get("id"), asked.Get("v"), "http://"+r.Host+"/"))
		}
		w.Header().Set("Content-Type", "text/xml; charset=UTF-8")
		w.Write([]byte(xml.Header))
		xml.NewEncoder(w).Encode(answer)
	})

	log.Printf("the mock extension store serves %s at http://%s/", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func check(catalogue []entry, id, have, base string) app {
	for _, e := range catalogue {
		if e.ID != id {
			continue
		}
		if e.Version == have {
			return app{ID: id, Status: "ok", Check: &updateCheck{Status: "noupdate"}}
		}
		return app{ID: id, Status: "ok", Check: &updateCheck{Codebase: base + e.Crx, Size: e.Size, Version: e.Version, Status: "ok"}}
	}
	return app{ID: id, Status: "error-unknownApplication"}
}
