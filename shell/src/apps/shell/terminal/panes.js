// The terminal shell's own model: Go's tabs are panes, arranged in windows, as tmux calls
// them, one shown at a time. A window is one of Go's tab groups, or a tab in none; how its
// panes lie is a tree, the shell's own data. A node is a pane, {tab}, or a split, {dir,
// parts, sizes}: a row or a column of parts, each with its share of the split, the shares
// adding up to 1. Go's tabs and groups are the truth the trees follow.

import { computed, nextTick, reactive, ref } from 'vue';
import { saveData, send, state } from '@/sdk/vue.js';

export const windows = reactive([]);

// The pane a split control asked a new tab to land beside, and which way, until the next
// new tab arrives.
let asked = null;
// The windows as last saved, so an unchanged tree isn't sent again.
let saved = '';
// The tab that was active when Go's list last came, to tell when the active pane closed.
let was = 0;
// The pane the shell asked Go to make active in place of the one that closed, shown at once
// so the body never shows the window Go chose for a moment; 0 once Go agrees.
export const heirAsked = ref(0);
// The pane zoomed to fill the body, 0 for none. Another pane becoming active, a split, or
// the pane closing ends it.
export const zoomed = ref(0);

// No pane is made narrower or shorter than these, in pixels, by a drag or a key.
export const smallest = { row: 80, column: 80 };
// How far a resize key moves a divider, in pixels.
const step = 32;

export function tabsOf(node) {
  return node.parts ? node.parts.flatMap(tabsOf) : [node.tab];
}

// The windows in the order of Go's list, each by its first tab there, with its tabs in the
// order they lie on screen, left to right and top to bottom. The shell moves Go's tabs to
// that order (order, below), so the strip, Cmd+1 to 9 and the next and previous tab keys
// all agree.
export const chips = computed(() => {
  const index = new Map(state.tabs.map((t, i) => [t.id, i]));
  const byId = new Map(state.tabs.map((t) => [t.id, t]));
  const first = (ids) => Math.min(...ids.map((id) => index.get(id)));
  return windows
    .map((root) => ({ root, ids: tabsOf(root).filter((id) => byId.has(id)) }))
    .filter((c) => c.ids.length)
    .sort((a, b) => first(a.ids) - first(b.ids))
    .map(({ root, ids }) => ({ root, tabs: ids.map((id) => byId.get(id)) }));
});

// The pane the body shows the window of: the active one, or its heir while Go makes it so.
export const focused = computed(() => heirAsked.value || state.active);

// The window that holds that pane, the one the body shows.
export const shown = computed(() => windows.find((root) => tabsOf(root).includes(focused.value)));

// load takes the windows from the data Go handed back, none for a new window or a new
// shell, and fits them to Go's tabs.
export function load(data) {
  windows.splice(0, windows.length, ...(data ? data.windows : []));
  zoomed.value = data?.zoomed || 0;
  saved = JSON.stringify({ windows, zoomed: zoomed.value });
  was = 0;
  follow();
}

// follow fits the windows to Go's tabs and tab windows. Go says which tabs make a window: a
// tab group's tabs are one, and a tab in no group is one alone. The trees only lay them
// out. A tab Go no longer lists, or that left its window's group, leaves its tree; a tab a
// tree lacks goes into its window's tree beside the pane a split asked for, or beside the
// tab before it in Go's list, or makes a window of its own. A tree holding the tabs of
// two windows keeps the window most of them are in, and two trees of one window become
// one.
//
// When the active pane closed, by its button or by Go's own Close Tab, the pane that takes
// its room becomes active, whatever Go chose from its own list: Go picks a neighbour in
// its order, which can be in another window.
export function follow() {
  const live = new Set(state.tabs.map((t) => t.id));
  const next = was && !live.has(was) ? heir(was, live) : 0;
  const home = new Map(state.tabs.map((t) => [t.id, t.group ? `group ${t.group}` : `tab ${t.id}`]));
  const kept = [];
  const taken = new Set();
  for (const root of windows) {
    const ids = tabsOf(root).filter((id) => live.has(id));
    const counts = new Map();
    for (const id of ids) if (!taken.has(home.get(id))) counts.set(home.get(id), (counts.get(home.get(id)) || 0) + 1);
    let which = null;
    for (const [k, n] of counts) if (which === null || n > counts.get(which)) which = k;
    if (which === null) continue;
    taken.add(which);
    kept.push(prune(root, new Set(ids.filter((id) => home.get(id) === which))));
  }
  if (kept.length !== windows.length || kept.some((root, i) => root !== windows[i])) windows.splice(0, windows.length, ...kept);
  const placed = new Set(windows.flatMap(tabsOf));
  state.tabs.forEach((t, k) => {
    if (placed.has(t.id)) return;
    const root = windows.find((r) => home.get(tabsOf(r)[0]) === home.get(t.id));
    if (!root) {
      windows.push({ tab: t.id });
    } else if (asked && tabsOf(root).includes(asked.beside)) {
      insert(asked.beside, t.id, asked.dir);
      asked = null;
    } else {
      const ids = tabsOf(root);
      const before = state.tabs.slice(0, k).reverse().find((b) => ids.includes(b.id));
      const beside = before ? before.id : ids[ids.length - 1];
      insert(beside, t.id, fit(beside));
    }
    placed.add(t.id);
  });
  if (zoomed.value && zoomed.value !== (next || state.active)) zoomed.value = 0;
  save();
  order();
  if (next && next !== state.active) {
    heirAsked.value = next;
    send({ t: 'activateTab', id: next });
  } else if (!live.has(heirAsked.value) || heirAsked.value === state.active) {
    heirAsked.value = 0;
  }
  was = heirAsked.value || state.active;
}

// prune is node without the tabs that are gone, or null when none is left. A part that
// goes gives its share to the part before it, or after it when it was first, the same
// part heir names. A split left with one part becomes that part, and a split inside one
// the same way round becomes its parts.
function prune(node, live) {
  if (!node.parts) return live.has(node.tab) ? node : null;
  const parts = [];
  const sizes = [];
  let freed = 0;
  let changed = false;
  node.parts.forEach((part, i) => {
    const kept = prune(part, live);
    if (kept !== part) changed = true;
    if (!kept) {
      if (sizes.length) sizes[sizes.length - 1] += node.sizes[i];
      else freed += node.sizes[i];
      return;
    }
    const size = node.sizes[i] + freed;
    freed = 0;
    if (kept.dir === node.dir) {
      parts.push(...kept.parts);
      sizes.push(...kept.sizes.map((s) => s * size));
    } else {
      parts.push(kept);
      sizes.push(size);
    }
  });
  if (!changed) return node;
  if (parts.length === 0) return null;
  if (parts.length === 1) return parts[0];
  return { dir: node.dir, parts, sizes };
}

// insert puts tab beside the pane of beside, after it in a row or under it in a column,
// the two sharing what beside had.
function insert(beside, tab, dir) {
  const i = windows.findIndex((root) => tabsOf(root).includes(beside));
  if (windows[i].tab === beside) {
    windows[i] = { dir, parts: [windows[i], { tab }], sizes: [0.5, 0.5] };
    return;
  }
  const parent = parentOf(windows[i], beside);
  const j = parent.parts.findIndex((p) => p.tab === beside);
  const half = parent.sizes[j] / 2;
  if (parent.dir === dir) {
    parent.parts.splice(j + 1, 0, { tab });
    parent.sizes.splice(j, 1, half, half);
  } else {
    parent.parts[j] = { dir, parts: [parent.parts[j], { tab }], sizes: [0.5, 0.5] };
  }
}

// fit is the way a pane is split when nothing asked: beside it when it is wider than tall,
// else under it; beside it when it isn't on screen.
function fit(tab) {
  const r = paneOf(tab)?.getBoundingClientRect();
  return r && r.height > r.width ? 'column' : 'row';
}

// parentOf is the split whose parts hold the pane of tab.
function parentOf(node, tab) {
  if (!node.parts) return null;
  if (node.parts.some((p) => p.tab === tab)) return node;
  for (const p of node.parts) {
    const found = parentOf(p, tab);
    if (found) return found;
  }
  return null;
}

// heir is the pane that takes the room of tab's when it closes: the nearest pane of the
// part before it, or after it when it is first, among the tabs still live; failing that,
// any live pane of its window. 0 for a pane alone in its window.
function heir(tab, live) {
  const root = windows.find((r) => tabsOf(r).includes(tab));
  if (!root) return 0;
  const parent = parentOf(root, tab);
  if (parent) {
    const j = parent.parts.findIndex((p) => p.tab === tab);
    const ids = (j > 0 ? tabsOf(parent.parts[j - 1]).reverse() : tabsOf(parent.parts[1])).filter((id) => live.has(id));
    if (ids.length) return ids[0];
  }
  return tabsOf(root).find((id) => live.has(id)) || 0;
}

// split asks Go for a new tab in the tab group of tab, starting one of the two when it is
// in none, to land beside the pane of tab: to its right for 'row', under it for 'column'.
export function split(tab, dir) {
  zoomed.value = 0;
  asked = { beside: tab, dir };
  send({ t: 'newTab', groupWith: tab });
}

// The move last asked of Go, with its list as it was, so one Go doesn't make isn't asked
// again for the same list.
let moved = '';

// order asks Go to move the first tab that is out of the strip's order to its place. Go
// answers with its list, which comes back here, until the two agree.
function order() {
  const want = chips.value.flatMap((c) => c.tabs.map((t) => t.id));
  const have = state.tabs.map((t) => t.id);
  const i = want.findIndex((id, k) => have[k] !== id);
  if (i < 0) return;
  const ask = JSON.stringify([have, want[i], i]);
  if (ask === moved) return;
  moved = ask;
  send({ t: 'moveTab', id: want[i], index: i });
}

// close closes the pane of tab; follow makes its heir active when it was the active one.
export function close(tab) {
  send({ t: 'closeTab', id: tab });
}

// save hands Go the windows when they changed.
export function save() {
  const json = JSON.stringify({ windows, zoomed: zoomed.value });
  if (json === saved) return;
  saved = json;
  saveData(JSON.parse(json));
}

// zoom fills the body with the pane of tab, the active one unless a pane's button says
// which, or ends the zoom. A pane alone in its window has nothing to zoom over.
export function zoom(tab = state.active) {
  if (zoomed.value) zoomed.value = 0;
  else if (shown.value?.parts) zoomed.value = tab;
  save();
}

// The element a pane's page and title bar are in.
function paneOf(tab) {
  return document.querySelector(`.title[data-pane="${tab}"]`)?.parentElement;
}

// focusToward makes the pane next to the active one, in the direction of an arrow, active:
// the nearest that lies that way and shares some of its height (left, right) or width (up,
// down), the one sharing most when two are as near. A zoom ends first.
export async function focusToward(dir) {
  if (!shown.value) return;
  if (zoomed.value) {
    zoomed.value = 0;
    save();
    await nextTick();
  }
  const from = paneOf(state.active)?.getBoundingClientRect();
  if (!from) return;
  const across = dir === 'left' || dir === 'right';
  let best = null;
  for (const id of tabsOf(shown.value)) {
    if (id === state.active) continue;
    const r = paneOf(id)?.getBoundingClientRect();
    if (!r) continue;
    const gap = { left: from.left - r.right, right: r.left - from.right, up: from.top - r.bottom, down: r.top - from.bottom }[dir];
    const shared = across
      ? Math.min(from.bottom, r.bottom) - Math.max(from.top, r.top)
      : Math.min(from.right, r.right) - Math.max(from.left, r.left);
    if (gap < -2 || shared <= 0) continue;
    if (!best || gap < best.gap - 2 || (Math.abs(gap - best.gap) <= 2 && shared > best.shared)) best = { id, gap, shared };
  }
  if (best) send({ t: 'activateTab', id: best.id });
}

// pathTo is the splits from root down to the pane of tab, each with the place of the part
// that leads to it.
function pathTo(node, tab) {
  if (!node.parts) return node.tab === tab ? [] : null;
  for (let i = 0; i < node.parts.length; i++) {
    const rest = pathTo(node.parts[i], tab);
    if (rest) return [{ node, i }, ...rest];
  }
  return null;
}

// resizeToward moves a divider of the active pane in the direction of an arrow: the one on
// that side of it, in the innermost split that has one, else the one on its other side. So
// Right widens a pane with room to its right, and narrows one at the right edge.
export function resizeToward(dir) {
  if (!shown.value || zoomed.value) return;
  const path = pathTo(shown.value, state.active);
  if (!path) return;
  const axis = dir === 'left' || dir === 'right' ? 'row' : 'column';
  const ahead = dir === 'right' || dir === 'down';
  const find = (wantAhead) => {
    for (let k = path.length - 1; k >= 0; k--) {
      const { node, i } = path[k];
      if (node.dir !== axis) continue;
      if (wantAhead && i < node.parts.length - 1) return { k, divider: i };
      if (!wantAhead && i > 0) return { k, divider: i - 1 };
    }
    return null;
  };
  const pick = find(ahead) || find(!ahead);
  if (!pick) return;
  // The split's element is the pick's place among the pane's enclosing splits, innermost
  // first.
  const splits = [];
  for (let el = paneOf(state.active)?.parentElement; el; el = el.parentElement) {
    if (el.classList.contains('split')) splits.push(el);
  }
  const box = splits[path.length - 1 - pick.k];
  if (!box) return;
  const r = box.getBoundingClientRect();
  const node = path[pick.k].node;
  shift(node, pick.divider, ahead ? step : -step, (axis === 'row' ? r.width : r.height) - (node.parts.length - 1));
  save();
}

// shift moves the divider after part i of a split by px, out of room pixels the parts
// share, keeping both parts either side of it at their smallest or more.
export function shift(node, i, px, room) {
  const least = smallest[node.dir] / room;
  const a = node.sizes[i];
  const b = node.sizes[i + 1];
  const delta = Math.min(Math.max(px / room, Math.min(0, least - a)), Math.max(0, b - least));
  node.sizes[i] = a + delta;
  node.sizes[i + 1] = b - delta;
}
