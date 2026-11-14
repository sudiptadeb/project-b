"""The web-features registry, core/webfeatures/registry.json, for the probes that call its
APIs in a page (tools/drive_first_app.py on the Mac, tools/drive_android.py on Android) and
for tools/web_features_bump.py. docs/web-features.md.

A probe's call gives one of:

    absent    it threw at once: the API isn't there
    resolved  its value, or the promise it gave, came back
    rejected  the promise it gave was rejected
    pending   no answer within two seconds
    killed    the page's renderer died

and each status allows some of them. A check fails on any other, naming the row.
"""

import json
import os

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
REGISTRY = os.path.join(ROOT, "core/webfeatures/registry.json")

ANSWERED = {"resolved", "rejected", "pending"}
ALLOWED = {
    "works": {"resolved"},
    # A quiet API is there and fails without saying so: it rejects, answers empty or
    # "denied", or never answers. Many of those resolve.
    "quiet": ANSWERED,
    "partly": ANSWERED,
    "off": {"absent"},
    "absent": {"absent"},
    "kills": {"killed"},
    "unknown": ANSWERED | {"absent"},
}

# A JavaScript function that runs a call, given as a function, and resolves to what it gave.
OUTCOME = ("(call => { let r; try { r = call() } catch (e) { return Promise.resolve('absent') }"
           " return Promise.race([Promise.resolve(r).then(() => 'resolved', () => 'rejected'),"
           " new Promise(done => setTimeout(() => done('pending'), 2000))]) })")


def load():
    with open(REGISTRY) as f:
        return json.load(f)


def feature(id):
    return next(f for f in load() if f["id"] == id)


def probes():
    """Every call the probes make, as (row, probe), in the registry's order."""
    return [(f, p) for f in load() for p in f.get("probe", [])]


def mismatch(row, probe, platform, seen):
    """What is wrong with what a call gave on platform, naming its row, or ''. A call that
    has no answer a page can see is only held to its renderer living."""
    status = row["status"][platform]
    allowed = ALLOWED[status]
    if probe.get("sees_nothing") and status != "kills":
        allowed = ANSWERED | {"absent"}
    if seen in allowed:
        return ""
    return "%s gave %s, but %s is %s on %s in the registry" % (probe["name"], seen, row["id"], status, platform)
