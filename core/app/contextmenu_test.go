package app_test

import (
	"fmt"
	"slices"
	"testing"

	"projectb/engine"
)

// rightClick fires a right-click on what spec says, as JSON, and returns the menu sent.
func (r *rig) rightClick(spec string) map[string]any {
	r.t.Helper()
	r.e.Fire(1, 0, "contextMenu", spec)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "contextMenu" && m["contextMenu"] != nil })
	return m["contextMenu"].(map[string]any)
}

// ids are a menu's action ids, a submenu's after its own.
func ids(menu map[string]any) []string {
	var out []string
	var walk func([]any)
	walk = func(list []any) {
		for _, a := range list {
			a := a.(map[string]any)
			out = append(out, a["id"].(string))
			if sub, ok := a["actions"].([]any); ok {
				walk(sub)
			}
		}
	}
	walk(menu["actions"].([]any))
	return out
}

func (r *rig) choose(menu map[string]any, item string) {
	r.shell(fmt.Sprintf(`{"t":"contextMenuChoose","id":%v,"item":%q}`, menu["id"], item))
	r.waitFor(func(m map[string]any) bool { return m["t"] == "contextMenu" && m["contextMenu"] == nil })
}

func TestARightClickSendsWhatCanRunThere(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	link := r.rightClick(`{"link":"https://b.test/","linkText":"b"}`)
	if got := ids(link); !slices.Equal(got, []string{"link.newTab", "link.newWindow", "link.copy", "link.saveAs"}) {
		t.Fatalf("link menu = %v", got)
	}
	if link["link"] != "https://b.test/" || link["tab"] != float64(1) {
		t.Fatalf("the context should come too: %v", link)
	}
	if got := ids(r.rightClick(`{"link":"mailto:a@b.test"}`)); !slices.Equal(got, []string{"link.copy"}) {
		t.Fatalf("a mail link opens no tab: %v", got)
	}
	if got := ids(r.rightClick(`{"src":"https://a.test/i.png","media":"image"}`)); !slices.Equal(got, []string{"image.newTab", "image.saveAs", "image.copyAddress"}) {
		t.Fatalf("image menu = %v", got)
	}
	sel := r.rightClick(`{"selection":"set up your workspace and open the dashboard"}`)
	if got := ids(sel); !slices.Equal(got, []string{"selection.copy", "selection.search"}) {
		t.Fatalf("selection menu = %v", got)
	}
	if title := sel["actions"].([]any)[1].(map[string]any)["title"]; title != "Search for “set up your workspace an…”" {
		t.Fatalf("search title = %q", title)
	}
	field := r.rightClick(`{"editable":true}`)
	if got := ids(field); !slices.Equal(got, []string{"edit.undo", "edit.redo", "edit.cut", "edit.copy", "edit.paste", "edit.selectAll"}) {
		t.Fatalf("field menu = %v", got)
	}
	if cut := field["actions"].([]any)[2].(map[string]any); cut["enabled"] != false {
		t.Fatalf("cut needs a selection: %v", cut)
	}
	page := r.rightClick(`{}`)
	if got := ids(page); !slices.Equal(got, []string{"page.back", "page.forward", "page.reload"}) {
		t.Fatalf("page menu = %v", got)
	}
	if back := page["actions"].([]any)[0].(map[string]any); back["enabled"] != false {
		t.Fatalf("nowhere to go back to: %v", back)
	}
}

func TestAChosenActionRuns(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.choose(r.rightClick(`{"link":"https://b.test/"}`), "link.copy")
	r.e.Settle()
	if got := r.e.Clipboard(); got != "https://b.test/" {
		t.Fatalf("clipboard = %q", got)
	}

	// A new tab opens behind, in the same window.
	r.choose(r.rightClick(`{"link":"https://b.test/"}`), "link.newTab")
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	if m["active"] != float64(1) {
		t.Fatalf("the link's tab should open behind: %v", m)
	}
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "https://b.test/" && tab["loading"] == false && tab["title"] != ""
	})

	r.choose(r.rightClick(`{"selection":"two words"}`), "selection.copy")
	r.e.Settle()
	if got := r.e.Clipboard(); got != "two words" {
		t.Fatalf("clipboard = %q", got)
	}

	// An editing action gives the page the keyboard first.
	r.choose(r.rightClick(`{"editable":true}`), "edit.paste")
	r.e.Settle()
	if got := r.e.Edits(); !slices.Equal(got, []engine.EditCommand{engine.EditPaste}) {
		t.Fatalf("edits = %v", got)
	}
}

func TestAMenuGoesWithoutRunningAnything(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	// Closed: a late choice does nothing.
	menu := r.rightClick(`{"link":"https://b.test/"}`)
	r.shell(fmt.Sprintf(`{"t":"contextMenuClose","id":%v}`, menu["id"]))
	r.waitFor(func(m map[string]any) bool { return m["t"] == "contextMenu" && m["contextMenu"] == nil })
	r.shell(fmt.Sprintf(`{"t":"contextMenuChoose","id":%v,"item":"link.copy"}`, menu["id"]))

	// Replaced: the older menu's choice does nothing.
	older := r.rightClick(`{"link":"https://c.test/"}`)
	r.rightClick(`{"link":"https://d.test/"}`)
	r.shell(fmt.Sprintf(`{"t":"contextMenuChoose","id":%v,"item":"link.copy"}`, older["id"]))

	// An action the menu didn't offer does nothing.
	r.shell(fmt.Sprintf(`{"t":"contextMenuChoose","id":%v,"item":"page.reload"}`, r.rightClick(`{"link":"https://e.test/"}`)["id"]))

	// The page navigating drops the menu.
	r.rightClick(`{"link":"https://f.test/"}`)
	r.shell(`{"t":"navigate","text":"https://g.test/"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "contextMenu" && m["contextMenu"] == nil })
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "https://g.test/" && tab["loading"] == false && tab["title"] != ""
	})

	r.e.Settle()
	if got := r.e.Clipboard(); got != "" {
		t.Fatalf("nothing should have been copied: %q", got)
	}
}
