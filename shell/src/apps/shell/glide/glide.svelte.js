// Glide's model over Go's state: the Spaces, what the sidebar lists in the Space shown, the
// Favorites and pinned tabs, and the split views. docs/glide.md says why it is shaped so.
//
// - A Space is a workspace, one to one, Main among them. The Space shown is the active
//   tab's: there is no other "current Space" to disagree with Go.
// - Favorites and pinned tabs are kept for the profile (saveProfileData), per Space, by
//   address: they outlive a restart, and every window shows the same. While the app runs,
//   a pinned entry is bound to the tab it opened, in this window's own data.
// - A split is this window's own data: tabs shown side by side, naming tabs by id.

import { activeTab, saveData, saveProfileData, send, state as go } from '@/sdk/svelte.svelte.js';

const MAX_FAVORITES = 12;
const MAX_SPLIT = 4;

// What only this window keeps (saveData): the sidebar's width and whether it shows, the tab
// each Space last showed, the tab each pinned entry is open in, and the splits.
export const win = $state({ width: 240, hidden: false, last: {}, bound: {}, splits: [] });

// What is open over the window: the Command Bar, a menu, a dialog, and a short notice.
// created is how many workspaces there were when New Space asked Go for one.
export const ui = $state({ bar: null, menu: null, dialog: null, toast: '', created: null });

// The tabs most recently shown, newest first, for Add to Split View.
let recent = [];
// What the next new tab is for: binding to a pinned entry, or joining a split beside a tab.
let pending = null;
// Every tab id seen, so a new one is known as new.
let known = new Set();
let saved = '';

// --- Spaces ---------------------------------------------------------------------------

// spaces is every Space: the profile's workspaces, in the user's order.
export function spaces() {
  return go.workspaces;
}

// spaceOf is the id of a tab's Space: its workspace's, which is "" for Main.
export const spaceOf = (tab) => tab.workspace || '';

// spaceId is the Space shown: the active tab's workspace.
export function spaceId() {
  return spaceOf(activeTab(go));
}

export function space() {
  return spaces().find((s) => s.id === spaceId()) || { id: spaceId(), name: '', color: go.windowColor || '#4f9e9e' };
}

// tabsIn is a Space's tabs in Go's order.
export function tabsIn(id) {
  return go.tabs.filter((t) => spaceOf(t) === id);
}

// showSpace shows a Space: the tab it showed last, else its first, else a new tab in it.
export function showSpace(id) {
  if (id === spaceId()) return;
  const tabs = tabsIn(id);
  const last = tabs.find((t) => t.id === win.last[id]) || tabs[0];
  if (last) send({ t: 'activateTab', id: last.id });
  else send({ t: 'newTab', workspace: id });
}

// stepSpace shows the Space by places before or after the one shown, round the list.
export function stepSpace(by) {
  const list = spaces();
  const i = list.findIndex((s) => s.id === spaceId());
  showSpace(list[(i + by + list.length) % list.length].id);
}

// --- What the profile keeps -------------------------------------------------------------

function kept() {
  const p = go.profileData || {};
  return { favorites: p.favorites || {}, pinned: p.pinned || {} };
}

function keep(change) {
  const next = structuredClone($state.snapshot(kept()));
  change(next);
  saveProfileData(next);
}

const entryOf = (tab) => ({ url: tab.url, title: tab.title, favicon: tab.favicon || '' });

const hostOf = (url) => {
  try {
    return new URL(url).host;
  } catch {
    return '';
  }
};

// --- Favorites --------------------------------------------------------------------------

export function favorites(id = spaceId()) {
  return kept().favorites[id] || [];
}

// favoriteTab is the tab a Favorite stands for in its Space: one at its address, else one
// on its site.
export function favoriteTab(fav, id = spaceId()) {
  const tabs = tabsIn(id);
  return tabs.find((t) => t.url === fav.url) || tabs.find((t) => hostOf(t.url) && hostOf(t.url) === hostOf(fav.url));
}

export function isFavorite(tab) {
  return favorites(spaceOf(tab)).some((f) => f.url === tab.url);
}

export function addFavorite(tab) {
  if (!tab.url || isFavorite(tab)) return;
  if (favorites(spaceOf(tab)).length >= MAX_FAVORITES) {
    toast(`Favorites hold ${MAX_FAVORITES}`);
    return;
  }
  keep((k) => (k.favorites[spaceOf(tab)] = [...(k.favorites[spaceOf(tab)] || []), entryOf(tab)]));
}

export function removeFavorite(fav, id = spaceId()) {
  keep((k) => (k.favorites[id] = (k.favorites[id] || []).filter((f) => f.url !== fav.url)));
}

// openFavorite switches to the Favorite's tab, or opens it in a new tab in the Space.
export function openFavorite(fav) {
  const tab = favoriteTab(fav);
  if (tab) send({ t: 'activateTab', id: tab.id });
  else send({ t: 'newTab', workspace: spaceId(), text: fav.url });
}

// --- Pinned tabs ------------------------------------------------------------------------

// pins is a Space's pinned entries, each with the tab it is open in, if any.
export function pins(id = spaceId()) {
  const live = new Map(go.tabs.map((t) => [t.id, t]));
  return (kept().pinned[id] || []).map((p) => ({ ...p, tab: live.get(win.bound[p.id]) || null }));
}

// pinnedTabs is the ids of the tabs open for pinned entries in this window.
export function pinnedTabs() {
  return new Set(Object.values(win.bound));
}

export const isPinned = (tab) => pinnedTabs().has(tab.id);

export function pin(tab) {
  if (isPinned(tab)) return;
  const id = crypto.randomUUID();
  keep((k) => (k.pinned[spaceOf(tab)] = [...(k.pinned[spaceOf(tab)] || []), { id, ...entryOf(tab) }]));
  win.bound[id] = tab.id;
  for (const s of win.splits) s.tabs = s.tabs.filter((t) => t !== tab.id);
  fitSplits();
  save();
}

// unpin takes a pinned entry away. Its tab, if open, stays, as a tab.
export function unpin(entryId) {
  keep((k) => {
    for (const id of Object.keys(k.pinned)) k.pinned[id] = k.pinned[id].filter((p) => p.id !== entryId);
  });
  delete win.bound[entryId];
  save();
}

export function unpinTab(tab) {
  const entry = Object.keys(win.bound).find((k) => win.bound[k] === tab.id);
  if (entry) unpin(entry);
}

// openPin opens a pinned entry that has no tab in this window: a new tab in its Space at
// its address, bound to it once Go lists it.
export function openPin(entry) {
  pending = { pin: entry.id };
  send({ t: 'newTab', workspace: spaceId(), text: entry.url });
}

// closeTab closes a tab. A pinned tab that closes is unpinned with it: closing closes.
export function closeTab(tab) {
  unpinTab(tab);
  send({ t: 'closeTab', id: tab.id });
}

// --- Splits -----------------------------------------------------------------------------

export function splitOf(id) {
  return win.splits.find((s) => s.tabs.includes(id));
}

// shown is the tabs whose pages show: the active tab's split, or the active tab alone.
export function shown() {
  return splitOf(go.active)?.tabs || (go.active ? [go.active] : []);
}

// addToSplit puts tab beside the active tab, in its split or a new one of the two.
export function addToSplit(tab) {
  const active = activeTab(go);
  if (!active.id || tab.id === active.id || spaceOf(tab) !== spaceOf(active) || isPinned(tab) || isPinned(active)) return;
  for (const s of win.splits) s.tabs = s.tabs.filter((t) => t !== tab.id);
  const split = splitOf(active.id);
  if (split) {
    if (split.tabs.length >= MAX_SPLIT) {
      toast(`A split holds ${MAX_SPLIT}`);
      return;
    }
    split.tabs.push(tab.id);
    split.sizes = split.tabs.map(() => 1 / split.tabs.length);
  } else {
    win.splits.push({ tabs: [active.id, tab.id], sizes: [0.5, 0.5] });
  }
  fitSplits();
  save();
}

// splitWithRecent puts the tab most recently shown in this Space, not in the split, beside
// the active one.
export function splitWithRecent() {
  const here = new Set(tabsIn(spaceId()).map((t) => t.id));
  const taken = new Set([...shown(), ...pinnedTabs()]);
  const id = recent.find((r) => here.has(r) && !taken.has(r));
  const tab = go.tabs.find((t) => t.id === id);
  if (tab) addToSplit(tab);
}

// openInSplit opens an address in a new tab beside the active one.
export function openInSplit(text) {
  pending = { split: go.active };
  send({ t: 'newTab', workspace: spaceId(), text });
}

// separate ends the active tab's split: its tabs stay, each alone.
export function separate(id = go.active) {
  win.splits = win.splits.filter((s) => !s.tabs.includes(id));
  save();
}

// takeOut takes one tab out of its split.
export function takeOut(id) {
  const s = splitOf(id);
  if (!s) return;
  s.tabs = s.tabs.filter((t) => t !== id);
  fitSplits();
  save();
}

// fitSplits keeps the splits to Go's tabs: a tab gone leaves its split, a split of one ends,
// and the shares are made to add up again.
function fitSplits() {
  const live = new Set(go.tabs.map((t) => t.id));
  for (const s of win.splits) {
    const kept = s.tabs.map((t, i) => [t, s.sizes?.[i] || 0]).filter(([t]) => live.has(t));
    const total = kept.reduce((a, [, size]) => a + size, 0) || 1;
    s.tabs = kept.map(([t]) => t);
    s.sizes = kept.map(([, size]) => (size > 0 ? size / total : 1 / kept.length));
  }
  win.splits = win.splits.filter((s) => s.tabs.length > 1);
}

// --- Following Go -----------------------------------------------------------------------

// load takes this window's own data, none for a new window, and fits it to Go's tabs.
export function load(data) {
  Object.assign(win, { width: 240, hidden: false, last: {}, bound: {}, splits: [] }, data || {});
  known = new Set(go.tabs.map((t) => t.id));
  saved = JSON.stringify(data || null);
  follow();
}

// follow fits what this window keeps to Go's tabs, after each change of them.
export function follow() {
  const live = new Set(go.tabs.map((t) => t.id));
  // A pinned tab that closed is unpinned: closing closes, from a key or from the page too.
  for (const [entry, tab] of Object.entries(win.bound)) {
    if (!live.has(tab)) {
      delete win.bound[entry];
      if (known.has(tab)) unpin(entry);
    }
  }
  const fresh = go.tabs.filter((t) => !known.has(t.id));
  known = live;
  if (pending && fresh.length) {
    const tab = fresh[fresh.length - 1];
    if (pending.pin) win.bound[pending.pin] = tab.id;
    // The new tab is active now, so it goes beside the tab the split was asked from.
    const beside = pending.split;
    if (beside && live.has(beside)) {
      const split = splitOf(beside);
      if (!split) win.splits.push({ tabs: [beside, tab.id], sizes: [0.5, 0.5] });
      else if (split.tabs.length < MAX_SPLIT) {
        split.tabs.push(tab.id);
        split.sizes = split.tabs.map(() => 1 / split.tabs.length);
      }
    }
    pending = null;
  }
  fitSplits();
  if (go.active) {
    win.last[spaceId()] = go.active;
    recent = [go.active, ...recent.filter((r) => r !== go.active && live.has(r))];
  }
  for (const id of Object.keys(win.last)) if (!live.has(win.last[id])) delete win.last[id];
  save();
}

// save hands Go this window's data when it changed.
export function save() {
  const json = JSON.stringify($state.snapshot(win));
  if (json === saved) return;
  saved = json;
  saveData(JSON.parse(json));
}

// --- Notices ----------------------------------------------------------------------------

let toastTimer = 0;
export function toast(text) {
  ui.toast = text;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (ui.toast = ''), 1600);
}

export { hostOf };

// --- The sidebar's list ---------------------------------------------------------------

// listing is what the sidebar lists under the pinned tabs, in Go's order: a tab, a split
// (one row for its tabs, where its first is), or a folder, a tab group, where its first
// tab is, holding its own tabs and splits.
export function listing() {
  const pinned = pinnedTabs();
  const tabs = tabsIn(spaceId()).filter((t) => !pinned.has(t.id));
  const byId = new Map(go.tabs.map((t) => [t.id, t]));
  const groups = new Map((go.groups || []).map((g) => [g.id, g]));
  const splitsDone = new Set();
  const groupsDone = new Set();
  const entry = (t) => {
    const s = splitOf(t.id);
    if (!s) return { kind: 'tab', key: 't' + t.id, tab: t };
    if (splitsDone.has(s)) return null;
    splitsDone.add(s);
    return { kind: 'split', key: 's' + s.tabs[0], split: s, tabs: s.tabs.map((id) => byId.get(id)).filter(Boolean) };
  };
  const out = [];
  for (const t of tabs) {
    const g = t.group ? groups.get(t.group) : null;
    if (!g) {
      const e = entry(t);
      if (e) out.push(e);
    } else if (!groupsDone.has(g.id)) {
      groupsDone.add(g.id);
      const children = tabs.filter((x) => x.group === g.id).map(entry).filter(Boolean);
      out.push({ kind: 'folder', key: 'g' + g.id, group: g, children });
    }
  }
  return out;
}
