//go:build extensions

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"projectb/engine"
	"projectb/mockengine"
)

// The mock engine's fake extension runtime, and what a test does with it over HTTP:
//
//	POST /mock/extensions/call?extension=&name=&tab=   the body is the JSON list of
//	                                                   arguments; answers {result, error}
//	POST /mock/extensions/fire                         kind, extension, value: see
//	                                                   mockengine.FireExtension
//	GET  /mock/extensions                              the events fired and the open popup
func init() {
	routes = append(routes, func(mux *http.ServeMux, fake *mockengine.Engine) {
		mux.HandleFunc("POST /mock/extensions/call", func(w http.ResponseWriter, r *http.Request) {
			args, _ := io.ReadAll(r.Body)
			q := r.URL.Query()
			tab, _ := strconv.ParseInt(q.Get("tab"), 10, 64)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fake.Call(q.Get("extension"), q.Get("name"), string(args), engine.TabID(tab)))
		})
		mux.HandleFunc("POST /mock/extensions/fire", func(w http.ResponseWriter, r *http.Request) {
			fake.FireExtension(r.FormValue("kind"), r.FormValue("extension"), r.FormValue("value"))
		})
		mux.HandleFunc("GET /mock/extensions", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fake.ExtensionState())
		})
	})
}
