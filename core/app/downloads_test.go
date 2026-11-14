package app_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"projectb/app"
	"projectb/profiles"
	"projectb/update"
)

// downloads is the list in the last downloads message that matches.
func (r *rig) downloads(match func(list []any) bool) []any {
	r.t.Helper()
	m := r.waitFor(func(m map[string]any) bool {
		list, ok := m["downloads"].([]any)
		return m["t"] == "downloads" && ok && match(list)
	})
	return m["downloads"].([]any)
}

func first(list []any) map[string]any { return list[0].(map[string]any) }

func startDownloading(t *testing.T, profile, dir string, blocked ...string) *rig {
	return startWith(t, app.Config{ProfileDir: profile, Profiles: profiles.New(t.TempDir()), DownloadsDir: dir, BlockedHosts: blocked,
		Update: update.Config{StateDir: filepath.Join(profile, "updates")}})
}

func TestADownloadGoesToTheFolderAndIsListed(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.e.Fire(1, 0, "download", "https://a.test/files/report.pdf")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	d := first(list)
	if d["name"] != "report.pdf" || d["path"] != filepath.Join(dir, "report.pdf") || d["received"] != float64(1000) {
		t.Fatalf("download = %v", d)
	}
	if _, err := os.Stat(filepath.Join(dir, "report.pdf")); err != nil {
		t.Fatalf("the file should be there: %v", err)
	}

	// The same name again is numbered.
	r.e.Fire(1, 0, "download", "https://a.test/other/report.pdf")
	list = r.downloads(func(l []any) bool { return len(l) == 2 && first(l)["state"] == "done" })
	if first(list)["name"] != "report (1).pdf" {
		t.Fatalf("the second should be numbered: %v", first(list))
	}

	// Open and reveal ask the engine; remove leaves the file.
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"open"}`, first(list)["id"]))
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"reveal"}`, first(list)["id"]))
	r.e.Settle()
	if want := []string{filepath.Join(dir, "report (1).pdf")}; !slices.Equal(r.e.Opened(), want) || !slices.Equal(r.e.Shown(), want) {
		t.Fatalf("opened %v, shown %v", r.e.Opened(), r.e.Shown())
	}
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"remove"}`, first(list)["id"]))
	r.downloads(func(l []any) bool { return len(l) == 1 })
	if _, err := os.Stat(filepath.Join(dir, "report (1).pdf")); err != nil {
		t.Fatalf("removing it from the list keeps the file: %v", err)
	}
}

func TestTheListOutlivesARestart(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/a.zip")
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	r.stop()

	r = startDownloading(t, profile, dir)
	defer r.stop()
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	list, _ := snap["downloads"].([]any)
	if len(list) != 1 || first(list)["name"] != "a.zip" {
		t.Fatalf("snapshot downloads = %v", snap["downloads"])
	}
	r.shell(`{"t":"clearDownloads"}`)
	r.downloads(func(l []any) bool { return len(l) == 0 })
}

func TestARunningDownloadCanBeCancelled(t *testing.T) {
	r := startDownloading(t, t.TempDir(), t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/big.iso")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "progress" })
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"cancel"}`, first(list)["id"]))
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "cancelled" })
}

func TestABlockedAddressDownloadsNothing(t *testing.T) {
	dir := t.TempDir()
	r := startDownloading(t, t.TempDir(), dir, "blocked.test")
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://blocked.test/x.exe")
	r.e.Fire(1, 0, "download", "https://a.test/ok.txt")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	if first(list)["name"] != "ok.txt" {
		t.Fatalf("only the allowed one should be listed: %v", list)
	}
}

func TestSaveLinkAsDownloadsThroughThePanel(t *testing.T) {
	dir := t.TempDir()
	r := startDownloading(t, t.TempDir(), dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.choose(r.rightClick(`{"link":"https://a.test/notes.txt"}`), "link.saveAs")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	if first(list)["path"] != filepath.Join(dir, "notes.txt") {
		t.Fatalf("download = %v", first(list))
	}
}
