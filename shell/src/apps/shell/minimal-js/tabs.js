// The tabs: a bar on top, or a list down the side, as the tabLayout pref says. Each of Go's
// tabs is one element, kept by its id, which moves whole from the one to the other. Many
// tabs scroll, the bar sideways and the list down, and the active one is kept in view.

import { send, state, tabLabel, workspaceOf } from '@/sdk/shell.js';
import { drag, shell } from '@/sdk/regions.js';
import { h, icon, keyed, set, svg } from './dom.js';

const plus = () => icon(12, 16, ['M8 2v12M2 8h12']);

// The loading mark: an arc that SVG's own animation turns, so the SDK doesn't measure
// the layout every frame while a page loads, as it would during a CSS animation.
const loadingMark = () =>
  svg(
    'svg',
    { class: 'loading', width: 12, height: 12, viewBox: '0 0 16 16', fill: 'none', stroke: 'currentColor', 'stroke-width': 2, role: 'img', 'aria-label': 'Loading' },
    svg('path', { d: 'M8 2a6 6 0 1 1-6 6' }, svg('animateTransform', { attributeName: 'transform', type: 'rotate', from: '0 8 8', to: '360 8 8', dur: '0.9s', repeatCount: 'indefinite' })),
  );

// One tab: its workspace's colour when it has one, its label, the loading mark while it
// loads, and its close button, which shows on the active tab and on a hovered one.
function tabView(tab) {
  const dot = h('span', { class: 'dot', role: 'img' });
  const label = h('span', { class: 't' });
  const loading = loadingMark();
  const close = h('button', {
    class: 'x',
    type: 'button',
    onclick: (e) => {
      e.stopPropagation();
      send({ t: 'closeTab', id: tab.id });
    },
  }, icon(10, 16, ['M3 3l10 10M13 3 3 13']));
  const activate = () => send({ t: 'activateTab', id: tab.id });
  const el = h('div', {
    class: 'tab',
    role: 'tab',
    tabindex: '0',
    'data-tab': tab.id,
    'aria-selected': 'false',
    onclick: activate,
    onkeydown: (e) => e.key === 'Enter' && e.target === el && activate(),
  }, label, close);
  return {
    el,
    fill(tab) {
      const text = tabLabel(tab);
      label.textContent = text;
      set(el, 'title', text);
      set(close, 'aria-label', `Close ${text}`);
      const workspace = workspaceOf(state, tab);
      if (workspace) {
        dot.style.background = workspace.color;
        set(dot, 'aria-label', `In ${workspace.name}`);
        el.prepend(dot);
      } else {
        dot.remove();
      }
      if (tab.loading) label.after(loading);
      else loading.remove();
    },
  };
}

const views = keyed((tab) => tab.id, tabView);
// What holds the tab elements now, the bar's .tabs or the side's .list, and whether it
// scrolls down.
let box;
let down = false;
// The tab drawn as the active one.
let shown = 0;

// reveal scrolls the active tab into view in its box.
export function reveal() {
  const el = views.el(state.active);
  const [start, size, pos, room] = down
    ? [el.offsetTop, el.offsetHeight, 'scrollTop', box.clientHeight]
    : [el.offsetLeft, el.offsetWidth, 'scrollLeft', box.clientWidth];
  if (start < box[pos]) box[pos] = start;
  else if (start + size > box[pos] + room) box[pos] = start + size - room;
}

const resized = new ResizeObserver(reveal);

// drawTabs draws Go's tabs into the box there is now, every one again with all (the
// workspaces changed), and marks the active one.
export function drawTabs(all) {
  views.draw(box, state.tabs, all);
  if (shown !== state.active) {
    const before = views.el(shown);
    if (before) {
      before.classList.remove('active');
      set(before, 'aria-selected', 'false');
    }
    const now = views.el(state.active);
    now.classList.add('active');
    set(now, 'aria-selected', 'true');
    shown = state.active;
  }
}

// The two layouts. Each is its element, the box its tabs go in, and what it marks, each
// [marker, element], to unmark when the other takes its place.
function layout(el, tabs, vertical, marks) {
  resized.disconnect();
  box = tabs;
  down = vertical;
  resized.observe(box);
  return { el, marks };
}

const newTab = () => send({ t: 'newTab' });

// The bar on top: the native window buttons' space, the tabs, New Tab, then space that
// drags the window. The tabs scroll, so the bar is the shell's and only its spaces drag.
export function tabBar() {
  const start = h('div', { class: 'space start' });
  const end = h('div', { class: 'space end' });
  const rest = h('div', { class: 'drag' });
  const tabs = h('div', {
    class: 'tabs',
    role: 'tablist',
    'aria-label': 'Tabs',
    // A mouse wheel turns the bar sideways; a trackpad's sideways swipe already does.
    onwheel: (e) => {
      if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
        e.preventDefault();
        tabs.scrollLeft += e.deltaY;
      }
    },
  });
  const el = h('div', { class: 'tabbar' }, start, tabs, h('button', { class: 'new', type: 'button', 'aria-label': 'New Tab', title: 'New Tab', onclick: newTab }, plus()), rest, end);
  return layout(el, tabs, false, [[shell, el], [drag, start], [drag, rest], [drag, end]]);
}

// The list down the side: New Tab at its top, the tabs under it, scrolling.
export function sideList() {
  const list = h('div', { class: 'list', role: 'tablist', 'aria-label': 'Tabs', 'aria-orientation': 'vertical' });
  const el = h('div', { class: 'side' }, h('button', { class: 'new', type: 'button', onclick: newTab }, plus(), 'New Tab'), list);
  return layout(el, list, true, [[shell, el]]);
}
