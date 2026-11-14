package mockengine

import (
	"strings"

	"projectb/engine"
)

// dialogs is what the mock keeps of pages' dialogs: those waiting, by tab, and the app's
// answers, in order.
type dialogs struct {
	next    engine.DialogID
	waiting map[engine.DialogID]engine.TabID
	answers []DialogAnswer
}

// DialogAnswer is one answer the app gave a page's dialog.
type DialogAnswer struct {
	ID   engine.DialogID `json:"id"`
	OK   bool            `json:"ok"`
	Text string          `json:"text"`
}

// openDialog is a page in tab asking kind with message, as content reports it.
func (e *Engine) openDialog(tab engine.TabID, kind, message string) {
	e.mu.Lock()
	if e.dialogs.waiting == nil {
		e.dialogs.waiting = map[engine.DialogID]engine.TabID{}
	}
	e.dialogs.next++
	id := e.dialogs.next
	e.dialogs.waiting[id] = tab
	e.mu.Unlock()
	d := engine.Dialog{ID: id, Tab: tab, Kind: kind, Message: message, Origin: "https://mock.test"}
	if kind == "signin" {
		d.Origin = "https://secure.mock.test"
		// "http:" before the words asks over a connection that isn't secure.
		if rest, ok := strings.CutPrefix(message, "http:"); ok {
			d.Origin, d.Message = "http://plain.mock.test", rest
		}
	}
	if kind == "prompt" {
		d.DefaultText = "mock"
	}
	e.listener.DialogOpened(d)
}

// dropDialogs gives up on tab's dialogs, as content does when the tab navigates or closes.
func (e *Engine) dropDialogs(tab engine.TabID) {
	var gone []engine.DialogID
	e.mu.Lock()
	for id, t := range e.dialogs.waiting {
		if t == tab {
			gone = append(gone, id)
			delete(e.dialogs.waiting, id)
		}
	}
	e.mu.Unlock()
	for _, id := range gone {
		e.listener.DialogClosed(id)
	}
}

func (e *Engine) AnswerDialog(id engine.DialogID, ok bool, text string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, waiting := e.dialogs.waiting[id]; !waiting {
		return
	}
	delete(e.dialogs.waiting, id)
	e.dialogs.answers = append(e.dialogs.answers, DialogAnswer{ID: id, OK: ok, Text: text})
}

func (e *Engine) AnswerSignIn(id engine.DialogID, ok bool, username, password string) {
	e.AnswerDialog(id, ok, username+":"+password)
}

// DialogAnswers are the app's answers to pages' dialogs, in order.
func (e *Engine) DialogAnswers() []DialogAnswer {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]DialogAnswer(nil), e.dialogs.answers...)
}
