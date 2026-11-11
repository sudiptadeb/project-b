//go:build extensions

package extensions

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// The store is a web server the browser knows by one setting, its URL. It lists what it
// has in catalogue.json and serves each package as a file. See core/cmd/extstore.

// SetStoreURL sets where the store is and reads its catalogue.
func (m *Manager) SetStoreURL(u string) {
	m.saved.StoreURL = strings.TrimRight(strings.TrimSpace(u), "/")
	m.save()
	m.RefreshCatalogue()
}

// RefreshCatalogue reads the store's catalogue again, off the main thread.
func (m *Manager) RefreshCatalogue() {
	store := m.saved.StoreURL
	m.catalogue, m.catalogueErr = nil, ""
	m.changed()
	if store == "" {
		return
	}
	go func() {
		items, err := fetchCatalogue(store)
		m.post(func() {
			if store != m.saved.StoreURL {
				return
			}
			m.catalogue = items
			if err != nil {
				m.catalogueErr = err.Error()
			}
			m.changed()
		})
	}()
}

// fetchCatalogue reads the catalogue, and each icon into a data: URL, so the settings
// page needs nothing from the store itself.
func fetchCatalogue(store string) ([]Item, error) {
	body, err := get(store + "/catalogue.json")
	if err != nil {
		return nil, err
	}
	var items []Item
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("%s/catalogue.json: %v", store, err)
	}
	for i, it := range items {
		if it.Icon == "" {
			continue
		}
		icon, err := get(store + "/" + it.Icon)
		if err != nil {
			return nil, err
		}
		items[i].Icon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(icon)
	}
	return items, nil
}

func get(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// Install downloads a store item's package into the profile and hands it to the engine.
func (m *Manager) Install(id string) {
	var item *Item
	for i := range m.catalogue {
		if m.catalogue[i].ID == id {
			item = &m.catalogue[i]
		}
	}
	if item == nil || m.phases[id] != "" {
		return
	}
	m.failure = ""
	m.phases[id] = "downloading"
	m.changed()
	url := m.saved.StoreURL + "/" + item.Crx
	file := filepath.Join(m.dir, "extension-downloads", path.Base(item.Crx))
	go func() {
		err := download(url, file)
		m.post(func() {
			if err != nil {
				delete(m.phases, id)
				m.failure = err.Error()
				m.changed()
				return
			}
			m.phases[id] = "installing"
			m.downloads[file] = id
			m.changed()
			m.eng.InstallExtension(file)
		})
	}()
}

func download(url, file string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
