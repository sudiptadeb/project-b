package app_test

import (
	"fmt"
	"strings"
	"testing"

	"projectb/mockengine"
	"projectb/timeline"
)

func (r *rig) dialogs(match func(list []any) bool) []any {
	r.t.Helper()
	m := r.waitFor(func(m map[string]any) bool {
		list, ok := m["dialogs"].([]any)
		return m["t"] == "dialogs" && (ok || m["dialogs"] == nil) && match(list)
	})
	list, _ := m["dialogs"].([]any)
	return list
}

func TestAPageDialogWaitsForTheShellsAnswer(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.e.Fire(1, 0, "prompt", "Your name?")
	d := r.dialogs(func(l []any) bool { return len(l) == 1 })[0].(map[string]any)
	if d["kind"] != "prompt" || d["title"] != "mock.test says" || d["message"] != "Your name?" || d["defaultText"] != "mock" || d["ok"] != "OK" || d["cancel"] != "Cancel" {
		t.Fatalf("dialog = %v", d)
	}
	r.shell(fmt.Sprintf(`{"t":"answerDialog","id":%v,"ok":true,"text":"Ada"}`, d["id"]))
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	// A second answer to the same one does nothing.
	r.shell(fmt.Sprintf(`{"t":"answerDialog","id":%v,"ok":false}`, d["id"]))
	r.e.Settle()
	if got := r.e.DialogAnswers(); len(got) != 1 || got[0] != (mockengine.DialogAnswer{ID: 1, OK: true, Text: "Ada"}) {
		t.Fatalf("answers = %v", got)
	}

	r.e.Fire(1, 0, "alert", "Hello")
	a := r.dialogs(func(l []any) bool { return len(l) == 1 })[0].(map[string]any)
	if a["cancel"] != "" {
		t.Fatalf("an alert has only OK: %v", a)
	}
	r.e.Fire(1, 0, "beforeunload", "")
	l := r.dialogs(func(l []any) bool { return len(l) == 2 })
	if b := l[1].(map[string]any); b["title"] != "Leave site?" || b["ok"] != "Leave" {
		t.Fatalf("leave-page = %v", b)
	}

	// The page navigating drops its dialogs, unanswered.
	r.shell(`{"t":"navigate","text":"https://b.test/"}`)
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "https://b.test/" && tab["loading"] == false && tab["title"] != ""
	})
	if got := r.e.DialogAnswers(); len(got) != 1 {
		t.Fatalf("dropped dialogs get no answer: %v", got)
	}
}

func TestASiteSignInIsADialogAnsweredWithAPassword(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.e.Fire(1, 0, "signin", "Staff only")
	d := r.dialogs(func(l []any) bool { return len(l) == 1 })[0].(map[string]any)
	if d["kind"] != "signin" || d["title"] != "Sign in" || d["message"] != "secure.mock.test asks for a username and password." || d["realm"] != "Staff only" || d["host"] != "secure.mock.test" || d["insecure"] != nil || d["ok"] != "Sign in" {
		t.Fatalf("sign-in = %v", d)
	}
	r.shell(fmt.Sprintf(`{"t":"answerDialog","id":%v,"ok":true,"text":"ada","password":"secret"}`, d["id"]))
	r.dialogs(func(l []any) bool { return len(l) == 0 })
	r.e.Settle()
	if got := r.e.DialogAnswers(); len(got) != 1 || got[0].Text != "ada:secret" || !got[0].OK {
		t.Fatalf("answers = %v", got)
	}
}

// A sign-in's password crosses from the shell to the engine and is written down nowhere:
// not in the timeline, and not in what the shells are sent.
func TestASignInPasswordLeavesNoTrace(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	_, from := timeline.Since(0)

	const password = "pw-7f3a9c-never-logged"
	r.e.Fire(1, 0, "signin", "Staff only")
	d := r.dialogs(func(l []any) bool { return len(l) == 1 })[0].(map[string]any)
	r.shell(fmt.Sprintf(`{"t":"answerDialog","id":%v,"ok":true,"text":"ada","password":%q}`, d["id"], password))
	// A malformed answer, which the shell drops with a warning, carries none of it either.
	r.shell(`{"t":"answerDialog","id":"` + password + `"}`)
	sent := r.dialogs(func(l []any) bool { return len(l) == 0 })
	r.e.Settle()
	if fmt.Sprint(sent) != "[]" {
		t.Fatalf("dialogs = %v", sent)
	}
	entries, _ := timeline.Since(from)
	// The timeline is being kept: the malformed answer's warning is in it.
	if !strings.Contains(fmt.Sprintf("%+v", entries), "bad message") {
		t.Fatalf("the warning should be in the timeline: %+v", entries)
	}
	for _, e := range entries {
		if strings.Contains(fmt.Sprintf("%+v", e), password) {
			t.Fatalf("the timeline holds the password: %+v", e)
		}
	}
}
