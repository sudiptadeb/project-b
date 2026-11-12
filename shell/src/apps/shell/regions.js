// Tells the window manager, in one layout message, which rectangles are shell, which
// empty shell drags the window, and where the page goes. The manager decides what each
// point does; this only measures.
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
    requestAnimationFrame(measure);
  }
}

function measure() {
  scheduled = false;

  const layout = {
    t: 'layout',
    shell: [...marked].map(rectOf).filter((r) => r.w > 0 && r.h > 0),
    drag: [...draggable].map(rectOf).filter((r) => r.w > 0 && r.h > 0),
    page: rectOf(pageEl),
    viewW: window.innerWidth,
    viewH: window.innerHeight,
  };
  const json = JSON.stringify(layout);
  if (json !== last) {
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

// Each marked element also says so in data-region, which the mock's dev window reads to
// send the page's clicks past the shell, as the real window does (mockwindow.js).
function directive(set, outline, region) {
  return {
    mounted(el) {
      set.add(el);
      el.dataset.region = region;
      observer.observe(el);
      if (debug) {
        el.style.outline = outline;
      }
      schedule();
    },
    unmounted(el) {
      set.delete(el);
      delete el.dataset.region;
      observer.unobserve(el);
      schedule();
    },
  };
}

// v-shell: this element belongs to the shell, wherever it sits in the page.
export const vShell = directive(marked, '2px solid rgba(220, 38, 38, 0.8)', 'shell');

// v-drag: this empty space in the top row moves the window. Nothing inside it gets clicks.
export const vDrag = directive(draggable, '2px dashed rgba(38, 99, 220, 0.8)', 'drag');

// setPageElement says where the page goes. It isn't shell, so it isn't marked.
export function setPageElement(el) {
  pageEl = el;
  observer.observe(el);
  schedule();
}
