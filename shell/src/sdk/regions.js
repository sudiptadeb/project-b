// Tells the window manager, in one layout message, which rectangles are shell, which
// drag the window and which of the controls in those stay the shell's, and which tabs
// show and where. The manager decides what each point does; this only measures.
//
// A shell marks what is its own with shell, a bar or an empty space in the top row that
// should move the window with drag, and each place a tab's page shows with page. Each
// marker has mark(el) and unmark(el), called as the element comes and goes, and page has
// update(el, tab) for another tab there; a framework's adapter calls them as its view
// draws (sdk/vue.js). One batcher measures once per frame, and only when something may
// have changed, and sends only when the layout differs. ResizeObserver misses moves, so
// a running animation or transition keeps it measuring until it ends, and a drag bar's
// subtree is watched, so a control that comes or goes in it is measured without the
// shell doing anything.

import { send } from './channel.js';
import { controls } from './controls.js';

const marked = new Set();
// Each element marked drag, and what watches its subtree.
const draggable = new Map();
// Each element marked page, and the tab whose page shows there: 0 or none for no tab.
const pages = new Map();
let scheduled = false;
let animations = 0;
let last = '';

function rectOf(el) {
  const r = el.getBoundingClientRect();
  return { x: r.x, y: r.y, w: r.width, h: r.height };
}

const shown = (r) => r.w > 0 && r.h > 0;

// The parts of a drag bar that stay the shell's: each control in it that shows, and
// anything in it marked shell, such as a list that floats out of it.
function claimedIn(bar) {
  const found = [...bar.querySelectorAll(controls)].filter((el) => el.checkVisibility({ visibilityProperty: true }));
  for (const el of marked) {
    if (el !== bar && bar.contains(el)) found.push(el);
  }
  return found.map(rectOf);
}

function schedule() {
  if (!scheduled) {
    scheduled = true;
    requestAnimationFrame(() => measure());
  }
}

// The layout as it is now, sent even if it hasn't changed: the shell sends it before it
// says it shows Go's state, so the manager has the regions before anything is clicked.
export function reportLayout() {
  measure(true);
}

// The pages in the order they are in the document, so the layout lists them as the
// shell shows them.
function tabs() {
  return [...pages]
    .filter(([, tab]) => tab)
    .sort(([a], [b]) => (a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1))
    .map(([el, tab]) => ({ tab, rect: rectOf(el) }))
    .filter((t) => shown(t.rect));
}

function measure(always = false) {
  scheduled = false;

  const layout = {
    t: 'layout',
    shell: [...marked].map(rectOf).filter(shown),
    controls: [...draggable.keys()].flatMap(claimedIn).filter(shown),
    drag: [...draggable.keys()].map(rectOf).filter(shown),
    tabs: tabs(),
    viewW: window.innerWidth,
    viewH: window.innerHeight,
  };
  const json = JSON.stringify(layout);
  if (always || json !== last) {
    last = json;
    send(layout);
  }

  if (animations > 0) {
    schedule();
  }
}

const observer = new ResizeObserver(schedule);

window.addEventListener('resize', schedule);
document.addEventListener('transitionrun', () => { animations++; schedule(); });
document.addEventListener('transitionend', () => { animations = Math.max(0, animations - 1); schedule(); });
document.addEventListener('transitioncancel', () => { animations = Math.max(0, animations - 1); schedule(); });
document.addEventListener('animationstart', () => { animations++; schedule(); });
document.addEventListener('animationend', () => { animations = Math.max(0, animations - 1); schedule(); });
document.addEventListener('animationcancel', () => { animations = Math.max(0, animations - 1); schedule(); });

// shell: this element belongs to the shell, wherever it sits in the page.
export const shell = {
  mark(el) {
    marked.add(el);
    observer.observe(el);
    schedule();
  },
  unmark(el) {
    marked.delete(el);
    observer.unobserve(el);
    schedule();
  },
};

// drag: this element moves the window wherever it has no control: an empty space in the
// top row, or a whole bar of controls, whose gaps then drag. Its controls, and what it
// holds marked shell, stay the shell's.
export const drag = {
  mark(el) {
    const watch = new MutationObserver(schedule);
    watch.observe(el, { subtree: true, childList: true, attributes: true, characterData: true });
    draggable.set(el, watch);
    observer.observe(el);
    schedule();
  },
  unmark(el) {
    draggable.get(el).disconnect();
    draggable.delete(el);
    observer.unobserve(el);
    schedule();
  },
};

// page: tab's page shows in this element; 0 for none. It isn't shell, so a press on it
// reaches the page. update(el, tab) shows another tab there, and that goes in the layout
// at once, so the window shows it before anything asks for its keyboard.
export const page = {
  mark(el, tab) {
    pages.set(el, tab);
    observer.observe(el);
    schedule();
  },
  update(el, tab) {
    if (pages.get(el) !== tab) {
      pages.set(el, tab);
      measure();
    }
  },
  unmark(el) {
    pages.delete(el);
    observer.unobserve(el);
    schedule();
  },
};

// measureNow sends the layout at once, when it changed. Asking for a tab's keyboard goes
// after it, so the tab is already showing when it gets it.
export function measureNow() {
  measure();
}
