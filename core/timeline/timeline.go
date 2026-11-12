// Package timeline is the logging manager: one record of what the app did and when, for
// the engine's C++, Go and our pages' JavaScript alike. docs/logging.md is the guide.
//
// Everything is an entry with one kind of time: nanoseconds since the process started,
// on the system's monotonic clock. An entry is a mark (a moment), the begin or end of a
// span (an interaction, such as opening settings), or a line of log at info, warn or
// error. Each entry is kept in memory for scripts to fetch, and handed to one goroutine
// that prints it and writes it to timeline.log in the profile, so recording never waits
// for a disk or a terminal.
//
// Any goroutine or thread may record.
package timeline

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Kind is what an entry is.
type Kind string

const (
	KindMark  Kind = "mark"
	KindBegin Kind = "begin"
	KindEnd   Kind = "end"
	KindInfo  Kind = "info"
	KindWarn  Kind = "warn"
	KindError Kind = "error"
)

// Who recorded an entry. Our pages are named by their channel: "shell", "settings".
const (
	SrcGo     = "go"
	SrcEngine = "engine"
)

// Now, as an entry's time, means the moment it is recorded.
const Now int64 = -1

// Entry is one line of the timeline.
type Entry struct {
	Seq  uint64 `json:"seq"` // Counts entries in the order they were recorded.
	T    int64  `json:"t"`   // Nanoseconds since process start.
	Kind Kind   `json:"kind"`
	Src  string `json:"src"`
	Name string `json:"name,omitempty"` // A mark's or a span's name.
	Text string `json:"text,omitempty"` // A log line, or a mark's detail.
	Dur  int64  `json:"dur,omitempty"`  // On an end: nanoseconds since its begin.
}

const (
	ringSize  = 16384   // Entries kept in memory.
	queueSize = 4096    // Entries waiting for the writer; more than this are not written.
	fileLimit = 8 << 20 // timeline.log starts again past this size.
	fileName  = "timeline.log"
	oldName   = "timeline.previous.log"
)

// Manager is one timeline. The app has one, behind the package's functions.
type Manager struct {
	mu      sync.Mutex
	origin  time.Time
	seq     uint64
	ring    []Entry
	spans   map[string]int64 // The spans that are open, with when each began.
	origins map[string]int64 // Each page's time origin: what its performance.now() counts from.

	queue   chan Entry
	control chan func()
	// The writer's own.
	print   io.Writer
	file    *os.File
	size    int64
	dir     string
	written uint64
}

// New returns a manager whose clock starts now and which prints to print.
func New(print io.Writer) *Manager {
	m := &Manager{
		origin:  time.Now(),
		spans:   map[string]int64{},
		origins: map[string]int64{},
		queue:   make(chan Entry, queueSize),
		control: make(chan func()),
		print:   print,
	}
	go m.write()
	return m
}

// Start says how old the process already is, so that time zero is its start, and opens
// timeline.log in dir, keeping the last run's as timeline.previous.log. Call it before
// anything is recorded.
func (m *Manager) Start(dir string, age time.Duration) {
	m.mu.Lock()
	m.origin = time.Now().Add(-age)
	m.mu.Unlock()
	m.do(func() {
		m.dir = dir
		m.openFile()
	})
	m.Record(KindMark, SrcGo, "process.start", "", 0)
}

// Close writes what is waiting and closes the file.
func (m *Manager) Close() {
	m.do(func() {
		if m.file != nil {
			m.file.Close()
			m.file = nil
		}
	})
}

// Now is the time on the timeline: nanoseconds since process start.
func (m *Manager) Now() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(time.Since(m.origin))
}

// Record adds one entry. t is its time, or Now. Beginning a span that is open, and
// ending one that isn't, do nothing: whichever layer saw the interaction first began it.
func (m *Manager) Record(kind Kind, src, name, text string, t int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t < 0 {
		t = int64(time.Since(m.origin))
	}
	e := Entry{T: t, Kind: kind, Src: src, Name: name, Text: text}
	switch kind {
	case KindBegin:
		if _, open := m.spans[name]; open {
			return
		}
		m.spans[name] = t
	case KindEnd:
		began, open := m.spans[name]
		if !open {
			return
		}
		delete(m.spans, name)
		e.Dur = t - began
	}
	m.seq++
	e.Seq = m.seq
	if len(m.ring) < ringSize {
		m.ring = append(m.ring, e)
	} else {
		m.ring[(e.Seq-1)%ringSize] = e
	}
	// Never waits: with the queue full the entry stays in memory only, and the writer
	// says how many it missed.
	select {
	case m.queue <- e:
	default:
	}
}

// PageOrigin says when a page's own clock started: the navigation start that its
// performance.now() counts from, which the engine knows on our clock.
func (m *Manager) PageOrigin(page string, t int64) {
	m.mu.Lock()
	m.origins[page] = t
	m.mu.Unlock()
	m.Record(KindMark, page, page+".navigation_start", "", t)
}

// pageEntry is what a page sends: {"t":"timeline","kind":"mark","name":"shell.mounted","at":41.5}.
// at is the page's performance.now(), in milliseconds.
type pageEntry struct {
	Kind Kind    `json:"kind"`
	Name string  `json:"name"`
	Text string  `json:"text"`
	At   float64 `json:"at"`
}

// FromPage records an entry one of our pages sent. Its time is the page's own, placed on
// the timeline by the page's origin. A page whose origin nobody gave, as on the mock
// engine, is stamped as it arrives. No entry is later than its arrival: the origin is a few
// microseconds off at times, and a span the shell began at a click then began after Go's
// first mark of it, which was then left out of the span's stages.
func (m *Manager) FromPage(page string, msg []byte) {
	var p pageEntry
	if err := json.Unmarshal(msg, &p); err != nil {
		m.Record(KindWarn, SrcGo, "", fmt.Sprintf("timeline: bad entry from %s: %v", page, err), Now)
		return
	}
	switch p.Kind {
	case KindMark, KindBegin, KindEnd, KindInfo, KindWarn, KindError:
	default:
		m.Record(KindWarn, SrcGo, "", fmt.Sprintf("timeline: %s sent an entry of kind %q", page, p.Kind), Now)
		return
	}
	m.mu.Lock()
	origin, known := m.origins[page]
	m.mu.Unlock()
	t := Now
	if known {
		t = min(origin+int64(p.At*1e6), m.Now())
	}
	m.Record(p.Kind, page, p.Name, p.Text, t)
}

// Since returns the entries kept in memory that were recorded after entry seq, oldest
// first, and the last one's seq to ask from next time. They are in the order they were
// recorded, which for a page's entries is later than their times.
func (m *Manager) Since(seq uint64) ([]Entry, uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	first := uint64(1)
	if m.seq > ringSize {
		first = m.seq - ringSize + 1
	}
	if seq+1 > first {
		first = seq + 1
	}
	if first > m.seq {
		return []Entry{}, m.seq
	}
	out := make([]Entry, 0, m.seq+1-first)
	for s := first; s <= m.seq; s++ {
		out = append(out, m.ring[(s-1)%ringSize])
	}
	return out, m.seq
}

// do runs f on the writer once everything recorded so far is written, and waits for it.
func (m *Manager) do(f func()) {
	done := make(chan struct{})
	m.control <- func() {
		for {
			select {
			case e := <-m.queue:
				m.writeEntry(e)
				continue
			default:
			}
			break
		}
		f()
		close(done)
	}
	<-done
}

func (m *Manager) write() {
	for {
		select {
		case e := <-m.queue:
			m.writeEntry(e)
		case f := <-m.control:
			f()
		}
	}
}

func (m *Manager) writeEntry(e Entry) {
	if missed := e.Seq - m.written - 1; m.written != 0 && missed > 0 {
		m.writeLine(fmt.Sprintf("%13s %-5s %-8s timeline: %d entries came too fast to write\n", "", KindWarn, SrcGo, missed))
	}
	m.written = e.Seq
	m.writeLine(e.Line())
}

func (m *Manager) writeLine(line string) {
	if m.print != nil {
		io.WriteString(m.print, line)
	}
	if m.file == nil {
		return
	}
	if m.size > fileLimit {
		m.file.Close()
		m.openFile()
		if m.file == nil {
			return
		}
	}
	n, _ := m.file.WriteString(line)
	m.size += int64(n)
}

// openFile starts timeline.log afresh, keeping the one before it.
func (m *Manager) openFile() {
	os.MkdirAll(m.dir, 0o755)
	path := filepath.Join(m.dir, fileName)
	os.Rename(path, filepath.Join(m.dir, oldName))
	f, err := os.Create(path)
	m.file, m.size = f, 0
	if err != nil {
		m.file = nil
		fmt.Fprintf(os.Stderr, "timeline: %v\n", err)
	}
}

// Line is the entry as it is printed and written: milliseconds since process start to
// the nanosecond, the kind, who recorded it, then the name and the text.
//
//	 412.006291 mark  engine   shell.created
//	1630.212000 end   settings settings.open 84.200000ms
//	 655.100417 warn  go       prefs: open prefs.json: permission denied
func (e Entry) Line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%13.6f %-5s %-8s", float64(e.T)/1e6, e.Kind, e.Src)
	if e.Name != "" {
		b.WriteString(" " + e.Name)
	}
	if e.Kind == KindEnd {
		fmt.Fprintf(&b, " %.6fms", float64(e.Dur)/1e6)
	}
	if e.Text != "" {
		b.WriteString(" " + strings.ReplaceAll(e.Text, "\n", " "))
	}
	b.WriteString("\n")
	return b.String()
}

// The app's timeline. It prints to stderr from the first line; Start gives it its file.
var std = New(os.Stderr)

// Start starts the app's timeline: see Manager.Start. print, when not nil, is where
// entries are printed instead of stderr. Go's standard log package writes here too, at
// info, so a library's lines land in the same place.
func Start(dir string, age time.Duration, print io.Writer) {
	if print != nil {
		std.do(func() { std.print = print })
	}
	std.Start(dir, age)
	log.SetFlags(0)
	log.SetOutput(logWriter{})
}

type logWriter struct{}

func (logWriter) Write(p []byte) (int, error) {
	std.Record(KindInfo, SrcGo, "", strings.TrimRight(string(p), "\n"), Now)
	return len(p), nil
}

// Close writes what is waiting. Call it on the way out.
func Close() { std.Close() }

// Mark records a moment by name, such as "go.started".
func Mark(name string) { std.Record(KindMark, SrcGo, name, "", Now) }

// Begin and End record the two ends of a span, such as "settings.open". Marks between
// them are its stages.
func Begin(name string) { std.Record(KindBegin, SrcGo, name, "", Now) }
func End(name string)   { std.Record(KindEnd, SrcGo, name, "", Now) }

// Infof, Warnf and Errorf record a line of log. Start it with the package's name:
// "prefs: ...".
func Infof(format string, args ...any) {
	std.Record(KindInfo, SrcGo, "", fmt.Sprintf(format, args...), Now)
}

func Warnf(format string, args ...any) {
	std.Record(KindWarn, SrcGo, "", fmt.Sprintf(format, args...), Now)
}

func Errorf(format string, args ...any) {
	std.Record(KindError, SrcGo, "", fmt.Sprintf(format, args...), Now)
}

// Record, PageOrigin, FromPage, Since and NowNanos are the app's timeline's: see Manager.
func Record(kind Kind, src, name, text string, t int64) { std.Record(kind, src, name, text, t) }
func PageOrigin(page string, t int64)                   { std.PageOrigin(page, t) }
func FromPage(page string, msg []byte)                  { std.FromPage(page, msg) }
func Since(seq uint64) ([]Entry, uint64)                { return std.Since(seq) }
func NowNanos() int64                                   { return std.Now() }
