<script setup>
import { computed } from 'vue';
import ContextMenu from '@/ui/ContextMenu.vue';
import { chooseContextItem, closeContextMenu, contextMenuPoint, focusPage, keyLabel, send, state } from '@/sdk/vue.js';

// The right-click menu over a page: Go's actions for what was pressed on, in Go's order,
// a separator between sections, each with its icon and key (docs/daily-driver.md).

// Line icons, 16 by 16, by action id.
const icons = {
  'link.newTab': '<rect x="2.5" y="4" width="11" height="9" rx="1.5"/><path d="M2.5 6.5h11M5 4V2.5h6V4"/>',
  'link.newWindow': '<rect x="2.5" y="3" width="11" height="10" rx="1.5"/><path d="M2.5 6h11"/>',
  'link.workspace': '<circle cx="8" cy="8" r="5.5"/>',
  'link.copy': '<rect x="5.5" y="5.5" width="8" height="8" rx="1.5"/><path d="M10.5 5.5V3.5A1 1 0 0 0 9.5 2.5h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2"/>',
  'image.newTab': '<rect x="2.5" y="3" width="11" height="10" rx="1.5"/><circle cx="6" cy="6.5" r="1"/><path d="M13.5 10.5 10 7l-5.5 6"/>',
  'image.copyAddress': '<path d="M6.5 9.5a2.5 2.5 0 0 0 3.5 0l2-2a2.5 2.5 0 0 0-3.5-3.5l-.5.5M9.5 6.5a2.5 2.5 0 0 0-3.5 0l-2 2a2.5 2.5 0 0 0 3.5 3.5l.5-.5"/>',
  'selection.copy': '<rect x="5.5" y="5.5" width="8" height="8" rx="1.5"/><path d="M10.5 5.5V3.5A1 1 0 0 0 9.5 2.5h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2"/>',
  'selection.search': '<circle cx="7" cy="7" r="4.5"/><path d="m10.5 10.5 3 3"/>',
  'page.back': '<path d="M13 8H3M7 4 3 8l4 4"/>',
  'page.forward': '<path d="M3 8h10M9 4l4 4-4 4"/>',
  'page.reload': '<path d="M13 8a5 5 0 1 1-1.5-3.5M13 2.5V5h-2.5"/>',
};

// The key each action shows: the keymap's action it shares.
const keys = {
  'page.back': 'nav.back',
  'page.forward': 'nav.forward',
  'page.reload': 'nav.reload',
  'selection.copy': 'edit.copy',
};

// Where Standard draws a line: between Go's groups, and inside one between these sections.
const sections = { 'link.copy': 'keep', 'link.saveAs': 'keep', 'edit.undo': 'undo', 'edit.redo': 'undo' };
const sectionOf = (a) => sections[a.id] || a.group;

const menu = computed(() => state.contextMenu);
const point = computed(() => menu.value && contextMenuPoint(menu.value));
const label = computed(() => {
  const m = menu.value;
  if (!m) return '';
  if (m.link) return 'Link';
  if (m.src) return 'Image';
  if (m.editable) return 'Text field';
  if (m.selection) return 'Selection';
  return 'Page';
});

function item(m, a) {
  return {
    label: a.title,
    icon: icons[a.id],
    key: a.id.startsWith('edit.') ? keyLabel(a.id) : keys[a.id] ? keyLabel(keys[a.id]) : '',
    disabled: !a.enabled,
    run: () => {
      chosen = true;
      chooseContextItem(m, a.id);
    },
    items: a.actions?.map((s) => ({
      ...item(m, s),
      dot: state.workspaces.find((w) => 'link.workspace.' + w.id === s.id)?.color,
    })),
  };
}

const items = computed(() => {
  const m = menu.value;
  if (!m) return [];
  const out = [];
  let section = '';
  for (const a of m.actions) {
    if (section && sectionOf(a) !== section) out.push({ separator: true });
    section = sectionOf(a);
    out.push(item(m, a));
  }
  return out;
});

// The menu takes the keyboard while it is open, and gives it back to the page as it goes.
// Go closes it once an action is chosen; anything else closing it says so.
let chosen = false;
function close() {
  const m = menu.value;
  if (!m) return;
  if (!chosen) closeContextMenu(m);
  chosen = false;
  focusPage();
}

function opened(el) {
  if (el) send({ t: 'focusShell' });
}
</script>

<template>
  <ContextMenu
    v-if="menu && items.length"
    :key="menu.id"
    :ref="opened"
    v-shell
    :items="items"
    :x="point.x"
    :y="point.y"
    :label="label"
    @close="close"
  />
</template>
