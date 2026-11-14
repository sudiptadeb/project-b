// The menus Glide's rows open, each a list of items: {label, run, action} where action is
// the id whose key the item shows, or {separator: true}. Glide's Menu draws them.

import { activeTab, send, state } from '@/sdk/svelte.svelte.js';
import * as g from './glide.svelte.js';

// The colours a Space or a folder can take: the workspace colours Go picks from.
export const colors = ['#3470d0', '#d03434', '#4f9e9e', '#e8a317', '#8a4fd0', '#2e9d5b', '#34404f'];

const sep = { separator: true };

export function tabMenu(tab) {
  const items = [];
  const pinned = g.isPinned(tab);
  items.push(pinned ? { label: 'Unpin Tab', run: () => g.unpinTab(tab), action: 'glide.pin' } : { label: 'Pin Tab', run: () => g.pin(tab), action: 'glide.pin' });
  if (tab.url) {
    items.push(
      g.isFavorite(tab)
        ? { label: 'Remove from Favorites', run: () => g.removeFavorite(g.favorites(g.spaceOf(tab)).find((f) => f.url === tab.url), g.spaceOf(tab)) }
        : { label: 'Add to Favorites', run: () => g.addFavorite(tab), action: 'glide.favorite' },
    );
  }
  if (!pinned) {
    const split = g.splitOf(tab.id);
    const active = activeTab(state);
    if (split) {
      items.push({ label: 'Take Out of Split View', run: () => g.takeOut(tab.id) });
      items.push({ label: 'Separate Split View', run: () => g.separate(tab.id), action: 'glide.splitRemove' });
    } else if (active.id && tab.id !== active.id && !g.isPinned(active)) {
      items.push({ label: 'Open in Split View', run: () => g.addToSplit(tab) });
    }
    items.push(sep);
    items.push({ label: 'New Tab in Folder', run: () => send({ t: 'newTab', groupWith: tab.id }) });
    const others = (state.groups || []).filter((gr) => gr.id !== tab.group && gr.tabs.some((id) => g.tabsIn(g.spaceOf(tab)).some((t) => t.id === id)));
    for (const gr of others) items.push({ label: `Move to ${gr.name || 'Folder'}`, run: () => send({ t: 'addToGroup', id: tab.id, group: gr.id }) });
    if (tab.group) items.push({ label: 'Remove from Folder', run: () => send({ t: 'removeFromGroup', id: tab.id }) });
    else items.push({ label: 'Add to New Folder', run: () => send({ t: 'createGroup', ids: [tab.id] }) });
  }
  items.push(sep);
  if (tab.url) items.push({ label: 'Copy Address', run: () => copy(tab.url), action: 'glide.copyAddress' });
  items.push({ label: 'Close Tab', run: () => g.closeTab(tab), action: 'tab.close' });
  return items;
}

export function folderMenu(group) {
  return [
    { label: 'New Tab in Folder', run: () => send({ t: 'newTab', groupWith: group.tabs[0] }) },
    {
      label: 'Rename Folder…',
      run: () =>
        (g.ui.dialog = {
          title: 'Folder',
          name: group.name,
          color: group.color,
          save: (name, color) => send({ t: 'editGroup', group: group.id, name, color }),
        }),
    },
    sep,
    { label: 'Ungroup', run: () => send({ t: 'ungroup', group: group.id }) },
    { label: 'Close Folder', run: () => send({ t: 'closeGroup', group: group.id }) },
  ];
}

// A Space's menu. Deleting one, which erases its storage, is in Settings, Workspaces.
export function spaceMenu(space) {
  return [
    {
      label: 'Edit Space…',
      run: () =>
        (g.ui.dialog = {
          title: 'Space',
          name: space.name,
          color: space.color,
          save: (name, color) => send({ t: 'editWorkspace', workspace: space.id, text: name, color }),
        }),
    },
    { label: 'New Space…', run: newSpace, action: 'glide.newSpace' },
    sep,
    { label: 'Settings', run: () => send({ t: 'run', text: 'app.settings' }), action: 'app.settings' },
  ];
}

export function newSpace() {
  const used = new Set(g.spaces().map((s) => s.color));
  g.ui.dialog = {
    title: 'New Space',
    name: '',
    color: colors.find((c) => !used.has(c)) || colors[0],
    create: true,
    // Go lists the new workspace last; the shell shows it once it hears of it.
    save: (name, color) => {
      g.ui.created = state.workspaces.length;
      send({ t: 'createWorkspace', text: name, color });
    },
  };
}

export function favoriteMenu(fav) {
  return [
    { label: 'Open', run: () => g.openFavorite(fav) },
    { label: 'Copy Address', run: () => copy(fav.url) },
    sep,
    { label: 'Remove from Favorites', run: () => g.removeFavorite(fav) },
  ];
}

export function pinMenu(entry) {
  return [
    { label: 'Open', run: () => g.openPin(entry) },
    { label: 'Copy Address', run: () => copy(entry.url) },
    sep,
    { label: 'Unpin', run: () => g.unpin(entry.id) },
  ];
}

function copy(url) {
  navigator.clipboard.writeText(url).then(
    () => g.toast('Copied'),
    () => g.toast('Could not copy'),
  );
}
