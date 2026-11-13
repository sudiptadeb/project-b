// Tells the window manager, in one layout message, which rectangles are shell, which
// empty shell drags the window, and which tabs show and where. The manager decides what
// each point does; this only measures.
//
// A component marks itself with v-shell, and an empty space in the top row that should
// move the window with v-drag. One batcher measures once per frame, and only when
// something may have changed, and sends only when the layout differs. ResizeObserver misses moves, so a running
// animation or transition keeps it measuring until it ends.

import { send } from '@/bridge.js';

const debug = new URLSearchParams(location.search).get('debug') === 'regions';

const marked = new Set();
const draggable = new Set();
let pageEl = null;
let pageTab = () => 0;
let scheduled = false;
let animations = 0;
let last = '';

function rectOf(el) {
  const r = el.getBoundingClientRect();
  return { x: r.x, y: r.y, w: r.width, h: r.height };
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

function measure(always = false) {
  scheduled = false;

  const tab = pageTab();
  const layout = {
    t: 'layout',
    shell: [...marked].map(rectOf).filter((r) => r.w > 0 && r.h > 0),
    drag: [...draggable].map(rectOf).filter((r) => r.w > 0 && r.h > 0),
    tabs: tab ? [{ tab, rect: rectOf(pageEl) }] : [],
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

function directive(set, outline) {
  return {
    mounted(el) {
      set.add(el);
      observer.observe(el);
      if (debug) {
        el.style.outline = outline;
      }
      schedule();
    },
    unmounted(el) {
      set.delete(el);
      observer.unobserve(el);
      schedule();
    },
  };
}

// v-shell: this element belongs to the shell, wherever it sits in the page.
export const vShell = directive(marked, '2px solid rgba(220, 38, 38, 0.8)');

// v-drag: this empty space in the top row moves the window. Nothing inside it gets clicks.
export const vDrag = directive(draggable, '2px dashed rgba(38, 99, 220, 0.8)');

// setPageElement says where a tab's page goes, and tab says which tab that is now. It
// isn't shell, so it isn't marked.
export function setPageElement(el, tab) {
  pageEl = el;
  pageTab = tab;
  observer.observe(el);
  schedule();
}

// measureNow sends the layout at once, when it changed. Asking for a tab's keyboard goes
// after it, so the tab is already showing when it gets it.
export function measureNow() {
  measure();
}
