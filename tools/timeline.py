#!/usr/bin/env python3
"""Reads the app's timeline (docs/logging.md) for the tests and tools/measure.py.

An entry is a dict as ProjectB.getTimeline gives it: seq, t (nanoseconds since process
start), kind, src, name, text, and dur on an end. The same entries come from the lines
of timeline.log or of Android's logcat, through parse().

Run on a file, it prints every span with its stages, in milliseconds since the span
began, and with a second file writes a trace that ui.perfetto.dev and chrome://tracing
open:

    tools/timeline.py ~/.project-b/default/timeline.log
    tools/timeline.py ~/.project-b/default/timeline.log trace.json
"""

import json
import re
import statistics
import sys

LINE = re.compile(r"\s*(\d+\.\d{6}) (mark|begin|end|info|warn|error)\s+(\S+)\s*(.*)$")
KINDS_WITH_NAMES = ("mark", "begin", "end")


def parse(lines):
    """Entries from printed lines. Anything that isn't a line of the timeline is skipped,
    so logcat's own prefix and other programs' lines do no harm."""
    entries = []
    for line in lines:
        m = LINE.search(line.rstrip("\n"))
        if not m:
            continue
        e = {"seq": len(entries) + 1, "t": round(float(m.group(1)) * 1e6), "kind": m.group(2), "src": m.group(3)}
        rest = m.group(4)
        if e["kind"] in KINDS_WITH_NAMES:
            e["name"], _, rest = rest.partition(" ")
            if e["kind"] == "end":
                dur, _, rest = rest.partition(" ")
                e["dur"] = round(float(dur.removesuffix("ms")) * 1e6)
        if rest:
            e["text"] = rest
        entries.append(e)
    return entries


def marks(entries):
    """Each mark's first time, by name, in nanoseconds."""
    out = {}
    for e in sorted(entries, key=lambda e: e["t"]):
        if e["kind"] == "mark":
            out.setdefault(e["name"], e["t"])
    return out


def spans(entries):
    """Every finished span: its name, begin, end and dur in nanoseconds, and stages, the
    marks between its two ends as name to nanoseconds after the begin."""
    by_time = sorted(entries, key=lambda e: e["t"])
    out = []
    for end in (e for e in by_time if e["kind"] == "end"):
        begin = end["t"] - end["dur"]
        stages = {}
        for e in by_time:
            if e["kind"] == "mark" and begin <= e["t"] <= end["t"]:
                stages.setdefault(e["name"], e["t"] - begin)
        out.append({"name": end["name"], "begin": begin, "end": end["t"], "dur": end["dur"], "stages": stages})
    return out


def summary(samples):
    """Median, least and most of some nanosecond samples, in milliseconds."""
    ms = [s / 1e6 for s in samples]
    return {"median": round(statistics.median(ms), 3), "min": round(min(ms), 3), "max": round(max(ms), 3), "n": len(ms)}


def trace(entries):
    """The entries as Chrome trace events: marks and log lines as instants, spans as
    async slices, one row per source."""
    rows = {}
    events = []
    for e in sorted(entries, key=lambda e: e["t"]):
        tid = rows.setdefault(e["src"], len(rows) + 1)
        base = {"pid": 1, "tid": tid, "ts": e["t"] / 1000, "cat": e["kind"]}
        if e["kind"] == "mark":
            events.append({**base, "ph": "i", "s": "t", "name": e["name"], "args": {"text": e.get("text", "")}})
        elif e["kind"] == "begin":
            events.append({**base, "ph": "b", "id": e["name"], "name": e["name"], "cat": "span"})
        elif e["kind"] == "end":
            events.append({**base, "ph": "e", "id": e["name"], "name": e["name"], "cat": "span"})
        else:
            events.append({**base, "ph": "i", "s": "t", "name": e["kind"] + ": " + e.get("text", "")})
    for src, tid in rows.items():
        events.append({"ph": "M", "pid": 1, "tid": tid, "name": "thread_name", "args": {"name": src}})
    return {"traceEvents": events, "displayTimeUnit": "ms"}


if __name__ == "__main__":
    if len(sys.argv) not in (2, 3):
        sys.exit(__doc__)
    with open(sys.argv[1], errors="replace") as f:
        found = parse(f)
    if len(sys.argv) == 3:
        with open(sys.argv[2], "w") as f:
            json.dump(trace(found), f)
        print(f"{len(found)} entries -> {sys.argv[2]}")
    else:
        for span in spans(found):
            stages = sorted(span["stages"].items(), key=lambda kv: kv[1])
            print("%12.3f %-14s %9.3f ms  %s" % (span["begin"] / 1e6, span["name"], span["dur"] / 1e6,
                                                 "  ".join("%s %.3f" % (n.removeprefix(span["name"]), t / 1e6) for n, t in stages)))
