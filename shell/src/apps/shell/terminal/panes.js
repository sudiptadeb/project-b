// The terminal shell's own model: Go's tabs are panes, arranged in groups, one group shown
// at a time. A group is a tree. A node is a pane, {tab}, or a split, {dir, parts, sizes}:
// a row or a column of parts, each with its share of the split, the shares adding up to 1.
// The groups are the shell's own data; Go's tab list is the truth they follow.

import { computed, reactive } from 'vue';
import { saveData, send, state } from '@/sdk/vue.js';

export const groups = reactive([]);

// The pane a split control asked a new tab to land beside, and which way, until the next
// new tab arrives.
let asked = null;
// The groups as last saved, so an unchanged tree isn't sent again.
let saved = '';

export function tabsOf(node) {
  return node.parts ? node.parts.flatMap(tabsOf) : [node.tab];
}

// The groups in the order of Go's list, each by its first tab there, with its tabs in that
// order too: the strip follows Go's order.
export const chips = computed(() => {
  const index = new Map(state.tabs.map((t, i) => [t.id, i]));
  return groups
    .map((root) => {
      const ids = new Set(tabsOf(root));
      return { root, tabs: state.tabs.filter((t) => ids.has(t.id)) };
    })
    .sort((a, b) => index.get(a.tabs[0].id) - index.get(b.tabs[0].id));
});

// The group that holds the active tab, the one the body shows.
export const shown = computed(() => groups.find((root) => tabsOf(root).includes(state.active)));

// load takes the groups from the data Go handed back, none for a new window or a new
// shell, and fits them to Go's tabs.
export function load(data) {
  groups.splice(0, groups.length, ...(data ? data.groups : []));
  saved = JSON.stringify(groups);
  follow();
}

// follow fits the groups to Go's tabs: a tab Go no longer lists leaves its tree, and a tab
// the groups lack goes beside the pane a split asked for, or into a group of its own.
export function follow() {
  const live = new Set(state.tabs.map((t) => t.id));
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
// part before it, or after it when it is first. 0 for a pane alone in its group.
function heir(tab) {
  const root = groups.find((r) => tabsOf(r).includes(tab));
  const parent = parentOf(root, tab);
  if (!parent) return 0;
  const j = parent.parts.findIndex((p) => p.tab === tab);
  const ids = j > 0 ? tabsOf(parent.parts[j - 1]) : tabsOf(parent.parts[1]);
  return j > 0 ? ids[ids.length - 1] : ids[0];
}

// split asks Go for a new tab, to land beside the pane of tab: to its right for 'row',
// under it for 'column'.
export function split(tab, dir) {
  asked = { beside: tab, dir };
  send({ t: 'newTab' });
}

// close closes the pane of tab. Closing the active pane first makes its heir active, so
// the group stays shown rather than Go choosing a neighbour in its own list.
export function close(tab) {
  const next = heir(tab);
  if (tab === state.active && next) send({ t: 'activateTab', id: next });
  send({ t: 'closeTab', id: tab });
}

// save hands Go the groups when they changed.
export function save() {
  const json = JSON.stringify(groups);
  if (json === saved) return;
  saved = json;
  saveData({ groups: JSON.parse(json) });
}
