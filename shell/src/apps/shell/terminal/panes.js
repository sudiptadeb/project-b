// The terminal shell's own model: Go's tabs are panes, arranged in groups, one group shown
// at a time. A group is a tree. A node is a pane, {tab}, or a split, {dir, parts, sizes}:
// a row or a column of parts, each with its share of the split, the shares adding up to 1.
// The groups are the shell's own data; Go's tab list is the truth they follow.

import { computed, reactive, ref } from 'vue';
import { saveData, send, state } from '@/sdk/vue.js';

export const groups = reactive([]);

// The pane a split control asked a new tab to land beside, and which way, until the next
// new tab arrives.
let asked = null;
// The groups as last saved, so an unchanged tree isn't sent again.
let saved = '';
// The tab that was active when Go's list last came, to tell when the active pane closed.
let was = 0;
// The pane the shell asked Go to make active in place of the one that closed, shown at once
// so the body never shows the group Go chose for a moment; 0 once Go agrees.
export const heirAsked = ref(0);

export function tabsOf(node) {
  return node.parts ? node.parts.flatMap(tabsOf) : [node.tab];
}

// The groups in the order of Go's list, each by its first tab there, with its tabs in the
// order they lie on screen, left to right and top to bottom. The shell moves Go's tabs to
// that order (order, below), so the strip, Cmd+1 to 9 and the next and previous tab keys
// all agree.
export const chips = computed(() => {
  const index = new Map(state.tabs.map((t, i) => [t.id, i]));
  const byId = new Map(state.tabs.map((t) => [t.id, t]));
  const first = (ids) => Math.min(...ids.map((id) => index.get(id)));
  return groups
    .map((root) => ({ root, ids: tabsOf(root).filter((id) => byId.has(id)) }))
    .filter((c) => c.ids.length)
    .sort((a, b) => first(a.ids) - first(b.ids))
    .map(({ root, ids }) => ({ root, tabs: ids.map((id) => byId.get(id)) }));
});

// The pane the body shows the group of: the active one, or its heir while Go makes it so.
export const focused = computed(() => heirAsked.value || state.active);

// The group that holds that pane, the one the body shows.
export const shown = computed(() => groups.find((root) => tabsOf(root).includes(focused.value)));

// load takes the groups from the data Go handed back, none for a new window or a new
// shell, and fits them to Go's tabs.
export function load(data) {
  groups.splice(0, groups.length, ...(data ? data.groups : []));
  saved = JSON.stringify(groups);
  was = 0;
  follow();
}

// follow fits the groups to Go's tabs: a tab Go no longer lists leaves its tree, and a tab
// the groups lack goes beside the pane a split asked for, or into a group of its own.
//
// When the active pane closed, by its button or by Go's own Close Tab, the pane that takes
// its room becomes active, whatever Go chose from its own list: Go picks a neighbour in
// its order, which can be in another group.
export function follow() {
  const live = new Set(state.tabs.map((t) => t.id));
  const next = was && !live.has(was) ? heir(was, live) : 0;
  for (let i = groups.length - 1; i >= 0; i--) {
    const kept = prune(groups[i], live);
    if (!kept) groups.splice(i, 1);
    else if (kept !== groups[i]) groups[i] = kept;
  }
  const placed = new Set(groups.flatMap(tabsOf));
  for (const t of state.tabs) {
    if (placed.has(t.id)) continue;
    if (asked && placed.has(asked.beside)) insert(asked.beside, t.id, asked.dir);
    else groups.push({ tab: t.id });
    asked = null;
    placed.add(t.id);
  }
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
  const i = groups.findIndex((root) => tabsOf(root).includes(beside));
  if (groups[i].tab === beside) {
    groups[i] = { dir, parts: [groups[i], { tab }], sizes: [0.5, 0.5] };
    return;
  }
  const parent = parentOf(groups[i], beside);
  const j = parent.parts.findIndex((p) => p.tab === beside);
  const half = parent.sizes[j] / 2;
  if (parent.dir === dir) {
    parent.parts.splice(j + 1, 0, { tab });
    parent.sizes.splice(j, 1, half, half);
  } else {
    parent.parts[j] = { dir, parts: [parent.parts[j], { tab }], sizes: [0.5, 0.5] };
  }
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
// any live pane of its group. 0 for a pane alone in its group.
function heir(tab, live) {
  const root = groups.find((r) => tabsOf(r).includes(tab));
  if (!root) return 0;
  const parent = parentOf(root, tab);
  if (parent) {
    const j = parent.parts.findIndex((p) => p.tab === tab);
    const ids = (j > 0 ? tabsOf(parent.parts[j - 1]).reverse() : tabsOf(parent.parts[1])).filter((id) => live.has(id));
    if (ids.length) return ids[0];
  }
  return tabsOf(root).find((id) => live.has(id)) || 0;
}

// split asks Go for a new tab, to land beside the pane of tab: to its right for 'row',
// under it for 'column'.
export function split(tab, dir) {
  asked = { beside: tab, dir };
  send({ t: 'newTab' });
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

// save hands Go the groups when they changed.
export function save() {
  const json = JSON.stringify(groups);
  if (json === saved) return;
  saved = json;
  saveData({ groups: JSON.parse(json) });
}
