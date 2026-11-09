// Tells the main process which rectangles are shell, which empty shell drags the
// window, and where the page goes.
//
// A component marks itself with v-shell, and empty shell that should move the window
// with v-drag. One batcher measures once per frame, and only when something may have
// changed, and sends only what differs. ResizeObserver misses moves, so a running
// animation or transition keeps it measuring until it ends.

import { send } from './bridge.js';

const debug = new URLSearchParams(location.search).get('debug') === 'regions';

const marked = new Set();
const draggable = new Set();
let pageEl = null;
let scheduled = false;
let animations = 0;
let lastRegions = '';
let lastDrag = '';
let lastBounds = '';

function rectOf(el) {
  const r = el.getBoundingClientRect();
  return { x: r.x, y: r.y, w: r.width, h: r.height };
}

function schedule() {
  if (!scheduled) {
    scheduled = true;
    requestAnimationFrame(measure);
  }
}

function measure() {
  scheduled = false;

  const rects = [...marked].map(rectOf).filter((r) => r.w > 0 && r.h > 0);
  const regions = JSON.stringify(rects);
  if (regions !== lastRegions) {
    lastRegions = regions;
    send({ t: 'regions', rects });
  }

  const dragRects = [...draggable].map(rectOf).filter((r) => r.w > 0 && r.h > 0);
  const drag = JSON.stringify(dragRects);
  if (drag !== lastDrag) {
    lastDrag = drag;
    send({ t: 'dragRegions', rects: dragRects });
  }

  if (pageEl) {
    const rect = rectOf(pageEl);
    const bounds = JSON.stringify([rect, window.innerWidth, window.innerHeight]);
    if (bounds !== lastBounds) {
      lastBounds = bounds;
      send({ t: 'pageBounds', rect, viewW: window.innerWidth, viewH: window.innerHeight });
    }
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

// v-drag: this empty shell element moves the window. Nothing inside it gets clicks.
export const vDrag = directive(draggable, '2px dashed rgba(38, 99, 220, 0.8)');

// setPageElement says where the page goes. It isn't shell, so it isn't marked.
export function setPageElement(el) {
  pageEl = el;
  observer.observe(el);
  schedule();
}
