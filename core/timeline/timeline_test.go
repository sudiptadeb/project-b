package timeline

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartPutsZeroAtProcessStart(t *testing.T) {
	m := New(nil)
	m.Start(t.TempDir(), 250*time.Millisecond)
	m.Record(KindMark, SrcGo, "main.entered", "", Now)
	entries, _ := m.Since(0)
	if len(entries) != 2 || entries[0].Name != "process.start" || entries[0].T != 0 {
		t.Fatalf("want process.start at 0 first, got %+v", entries)
	}
	if got := entries[1].T; got < int64(250*time.Millisecond) || got > int64(time.Second) {
		t.Fatalf("a mark recorded now is %d ns after a start 250 ms ago", got)
	}
}

func TestSpansPairAndIgnoreStrays(t *testing.T) {
	m := New(nil)
	m.Record(KindEnd, SrcGo, "settings.open", "", 5) // Never begun.
	m.Record(KindBegin, "shell", "settings.open", "", 100)
	m.Record(KindBegin, SrcGo, "settings.open", "", 130) // Go saw the same click later.
	m.Record(KindMark, SrcGo, "settings.open.go", "", 131)
	m.Record(KindEnd, "settings", "settings.open", "", 400)
	m.Record(KindEnd, "settings", "settings.open", "", 500)
	entries, _ := m.Since(0)
	var got []string
	for _, e := range entries {
		got = append(got, string(e.Kind)+" "+e.Name)
	}
	want := "begin settings.open, mark settings.open.go, end settings.open"
	if strings.Join(got, ", ") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
	if end := entries[2]; end.Dur != 300 {
		t.Fatalf("the span is %d ns long, want 300: from the first begin", end.Dur)
	}
}

func TestPageEntriesLandOnThePagesOrigin(t *testing.T) {
	m := New(nil)
	m.origin = m.origin.Add(-10 * time.Second) // The page's entries are in the past.
	m.PageOrigin("shell", 2_000_000_000)
	m.FromPage("shell", []byte(`{"t":"timeline","kind":"mark","name":"shell.mounted","at":41.5}`))
	entries, _ := m.Since(0)
	if entries[0].Name != "shell.navigation_start" || entries[0].T != 2_000_000_000 {
		t.Fatalf("the origin isn't marked: %+v", entries[0])
	}
	if e := entries[1]; e.Src != "shell" || e.Name != "shell.mounted" || e.T != 2_041_500_000 {
		t.Fatalf("41.5 ms after an origin at 2 s is %+v", e)
	}
}

func TestPageEntryIsNoLaterThanItsArrival(t *testing.T) {
	m := New(nil)
	m.PageOrigin("shell", m.Now())
	// An origin a little late: the page's click lands after Go's first mark of the span.
	m.FromPage("shell", []byte(`{"kind":"begin","name":"tab.new","at":5}`))
	m.Record(KindMark, SrcGo, "tab.new.go", "", Now)
	entries, _ := m.Since(0)
	begin, mark := entries[1], entries[2]
	if begin.Kind != KindBegin || mark.Name != "tab.new.go" || begin.T > mark.T {
		t.Fatalf("the begin at %d is after the mark at %d, which it came before", begin.T, mark.T)
	}
}

func TestPageWithoutOriginIsStampedOnArrival(t *testing.T) {
	m := New(nil)
	before := m.Now()
	m.FromPage("settings", []byte(`{"kind":"mark","name":"settings.mounted","at":99999}`))
	entries, _ := m.Since(0)
	if e := entries[0]; e.T < before || e.T > m.Now() {
		t.Fatalf("stamped %d, outside %d to now", e.T, before)
	}
}

func TestPageCannotInventAKind(t *testing.T) {
	m := New(nil)
	m.FromPage("shell", []byte(`{"kind":"fatal","name":"x"}`))
	m.FromPage("shell", []byte(`not json`))
	entries, _ := m.Since(0)
	for _, e := range entries {
		if e.Kind != KindWarn || e.Src != SrcGo {
			t.Fatalf("want two warnings of our own, got %+v", entries)
		}
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries", len(entries))
	}
}

func TestSinceReturnsOnlyWhatIsNewAndTheRingForgetsTheOldest(t *testing.T) {
	m := New(nil)
	m.Record(KindMark, SrcGo, "a", "", 1)
	m.Record(KindMark, SrcGo, "b", "", 2)
	_, last := m.Since(0)
	m.Record(KindMark, SrcGo, "c", "", 3)
	entries, next := m.Since(last)
	if len(entries) != 1 || entries[0].Name != "c" || next != 3 {
		t.Fatalf("after seq %d: %+v, next %d", last, entries, next)
	}
	if entries, _ := m.Since(next); len(entries) != 0 {
		t.Fatalf("nothing is new, got %+v", entries)
	}
	for i := 0; i < ringSize; i++ {
		m.Record(KindMark, SrcGo, "fill", "", Now)
	}
	entries, _ = m.Since(0)
	if len(entries) != ringSize || entries[0].Seq != 4 {
		t.Fatalf("the ring holds %d entries from seq %d", len(entries), entries[0].Seq)
	}
}

func TestFileAndPrintGetEveryLineAndTheLastRunIsKept(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, fileName), []byte("the run before\n"), 0o644)
	var printed bytes.Buffer
	m := New(&printed)
	m.Start(dir, 0)
	m.Record(KindMark, SrcEngine, "shell.created", "", 412_006_291)
	m.Record(KindBegin, "shell", "settings.open", "", 1_000_000_000)
	m.Record(KindEnd, "settings", "settings.open", "", 1_084_200_000)
	m.Record(KindWarn, SrcGo, "", "prefs: two\nlines", 2_000_000_000)
	m.Close()

	file, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	want := "     0.000000 mark  go       process.start\n" +
		"   412.006291 mark  engine   shell.created\n" +
		"  1000.000000 begin shell    settings.open\n" +
		"  1084.200000 end   settings settings.open 84.200000ms\n" +
		"  2000.000000 warn  go       prefs: two lines\n"
	if string(file) != want {
		t.Fatalf("the file is\n%s\nwant\n%s", file, want)
	}
	if printed.String() != want {
		t.Fatalf("printed\n%s", printed.String())
	}
	old, _ := os.ReadFile(filepath.Join(dir, oldName))
	if string(old) != "the run before\n" {
		t.Fatalf("the last run's file holds %q", old)
	}
}

// slow holds the writer up, as a full disk or a stuck terminal would.
type slow struct{ release chan struct{} }

func (s slow) Write(p []byte) (int, error) {
	<-s.release
	return len(p), nil
}

func TestRecordingNeverWaitsForTheWriter(t *testing.T) {
	stuck := slow{release: make(chan struct{})}
	m := New(stuck)
	done := make(chan struct{})
	go func() {
		for i := 0; i < queueSize*2; i++ {
			m.Record(KindInfo, SrcGo, "", "line", Now)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("recording waited for a writer that was stuck")
	}
	if entries, _ := m.Since(0); len(entries) != queueSize*2 {
		t.Fatalf("memory holds %d of %d entries", len(entries), queueSize*2)
	}
	close(stuck.release)
}

func TestManyThreadsRecordAtOnce(t *testing.T) {
	m := New(nil)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 500 {
				m.Record(KindMark, SrcEngine, "x", "", Now)
			}
		}()
	}
	wg.Wait()
	entries, last := m.Since(0)
	if len(entries) != 4000 || last != 4000 {
		t.Fatalf("%d entries, last seq %d", len(entries), last)
	}
	for i, e := range entries {
		if e.Seq != uint64(i+1) {
			t.Fatalf("entry %d has seq %d", i, e.Seq)
		}
		if i > 0 && e.T < entries[i-1].T {
			t.Fatalf("entry %d is earlier than the one before it", i)
		}
	}
}
