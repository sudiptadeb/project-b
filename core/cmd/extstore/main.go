// The mock extension store: a web server on this machine that serves the folder
// tools/fetch-extensions.sh fills. The browser is its client and knows it only by URL.
//
//	GET /catalogue.json   what the store has, one entry per extension
//	GET /update?x=...     gupdate XML, as the Web Store's update endpoint answers
//	GET /crx/<file>       a package, byte for byte as it was fetched
//	GET /icon/<id>.png    its icon
//
// See docs/extensions.md.
package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
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

func main() {
	dir := flag.String("dir", ".tools/extensions", "the folder to serve")
	addr := flag.String("addr", "127.0.0.1:8766", "where to listen")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(*dir)))
	mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		// The catalogue is read each time, so a package added while the store runs shows.
		data, err := os.ReadFile(filepath.Join(*dir, "catalogue.json"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var catalogue []entry
		if err := json.Unmarshal(data, &catalogue); err != nil {
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
