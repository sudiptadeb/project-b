# deskctl: see and drive this Mac's real desktop

Agent and ssh sessions on a Mac run in a Background session with no display, so `screencapture` fails with
"could not create image from display". Once someone is logged in to the desktop, `desk` runs a small tool
inside that session.

```
.tools/desk/desk status                     # logged in? locked? which permissions?
.tools/desk/desk windows                    # app, x, y, width, height, title
.tools/desk/desk shot out.png "Project B"   # one app's window, nothing else
.tools/desk/desk shot out.png 4242          # the window of one process, when an app is open more than once
.tools/desk/desk shot out.png               # the whole display
.tools/desk/desk raise "Project B"          # bring an app to the front
.tools/desk/desk click 100 200
.tools/desk/desk drag 100 200 300 200
.tools/desk/desk scroll 600 400 20 10         # a trackpad scroll at a point: 10 steps of 20px, swiping down (content moves up), with momentum
.tools/desk/desk scroll 600 400 20 10 1500    # the same, holding the fingers down 1.5s before letting go, to capture a rubber band
.tools/desk/desk key 36                     # a key code; 36 is Return
.tools/desk/desk key 13 cmd                 # Cmd+W
.tools/desk/desk type "hello"
```

Coordinates are points from the top left of the main display. Input lands on whatever is on top at that
point, so `raise` the target first and check with `windows`.

## What it needs

- Someone logged in with the screen unlocked. A reboot undoes this. Logging in works over Screen Sharing.
- `.tools/desk/deskctl` switched on in System Settings, under Privacy & Security, in both
  **Screen & System Audio Recording** and **Accessibility**. Nothing else.

## Care

- It drives someone's real desktop. Don't click outside the window under test.
- Prefer `shot out.png "<app>"`: a whole-display capture shows whatever else is open.

## Changing it

Run `tools/deskctl/install.sh`. It signs with `.tools/desk/signing.pem`, a local key made on
first install and never committed, so a rebuild keeps both permissions. `DESK_DIR` installs
into another folder that already holds a key.
