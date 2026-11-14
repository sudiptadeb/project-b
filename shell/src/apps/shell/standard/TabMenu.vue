<script setup>
import { computed } from 'vue';
import ContextMenu from '@/ui/ContextMenu.vue';
import { keyLabel, send, state } from '@/sdk/vue.js';
import { groupLabel } from './groups.js';
import { closeMenu, menu } from './menu.js';
import { view } from './state.js';

// A tab's own menu, by a right-click on it or Shift+F10: the design system's context menu
// (docs/design-system.md, Context menus). What it does is Go's: actions that act on the
// active tab run once the tab is made active.

const tab = computed(() => (menu.kind === 'tab' ? state.tabs.find((t) => t.id === menu.id) || null : null));

const icons = {
  duplicate: '<rect x="5.5" y="5.5" width="8" height="8" rx="1.5"/><path d="M10.5 5.5V3.5A1 1 0 0 0 9.5 2.5h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2"/>',
  group: '<rect x="2" y="4" width="8" height="8" rx="1.5"/><path d="M12 5v6M14 6.5v3"/>',
  workspace: '<circle cx="8" cy="8" r="5.5"/>',
  window: '<rect x="2.5" y="3" width="11" height="10" rx="1.5"/><path d="M2.5 6h11"/>',
  close: '<path d="m4.5 4.5 7 7M11.5 4.5l-7 7"/>',
};

// run makes the tab active when an action needs it to be, then sends msg.
const onTab = (t, msg) => () => {
  if (t.id !== state.active) send({ t: 'activateTab', id: t.id });
  send(msg);
};

const items = computed(() => {
  const t = tab.value;
  if (!t) return [];
  // The groups a tab can join: its window's, in its workspace, but its own.
  const groups = (state.groups || []).filter((g) => g.id !== t.group && state.tabs.find((x) => x.id === g.tabs[0])?.workspace === t.workspace);
  const elsewhere = (state.workspaces || []).filter((w) => w.id !== t.workspace);
  // The other tabs this window lists: the workspace shown's, or all.
  const others = state.tabs.filter((x) => x.id !== t.id && (view.workspace === null || x.workspace === view.workspace));
  const out = [
    { label: 'Duplicate Tab', icon: icons.duplicate, key: keyLabel('tab.duplicate'), run: onTab(t, { t: 'run', text: 'tab.duplicate' }) },
    {
      label: 'Add Tab to Group',
      icon: icons.group,
      items: [
        { label: 'New Group', run: () => send({ t: 'createGroup', ids: [t.id] }) },
        ...groups.map((g) => ({ label: groupLabel(g), dot: g.color, run: () => send({ t: 'addToGroup', id: t.id, group: g.id }) })),
      ],
    },
  ];
  if (t.group) out.push({ label: 'Remove from Group', run: () => send({ t: 'removeFromGroup', id: t.id }) });
  // A tab's workspace is fixed (docs/workspaces.md): another workspace gets the same address
  // in a tab of its own.
  if (elsewhere.length && t.url) {
    out.push({
      label: 'Open in Workspace',
      icon: icons.workspace,
      items: elsewhere.map((w) => ({ label: w.name, dot: w.color, run: () => send({ t: 'newTab', workspace: w.id, text: t.url }) })),
    });
  }
  out.push(
    { label: 'Move Tab to New Window', icon: icons.window, key: keyLabel('tab.moveToNewWindow'), disabled: state.tabs.length < 2, run: onTab(t, { t: 'run', text: 'tab.moveToNewWindow' }) },
    { separator: true },
    { label: 'Close Tab', icon: icons.close, key: t.id === state.active ? keyLabel('tab.close') : '', run: () => send({ t: 'closeTab', id: t.id }) },
    { label: 'Close Other Tabs', disabled: !others.length, run: () => others.forEach((x) => send({ t: 'closeTab', id: x.id })) },
  );
  return out;
});

const label = computed(() => (tab.value ? `Tab ${tab.value.title || tab.value.url || 'New Tab'}` : 'Tab'));
</script>

<template>
  <ContextMenu
    v-if="tab"
    :key="tab.id"
    v-shell
    :items="items"
    :x="menu.x"
    :y="menu.y"
    :label="label"
    :keyboard="menu.keyboard"
    @close="closeMenu"
  />
</template>
