<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { send, state } from '@/sdk/vue.js';
import { pushEscape } from '@/ui/escape.js';
import Swatches from '@/ui/Swatches.vue';
import { colors, groupLabel } from './groups.js';
import { closeMenu, menu } from './menu.js';
import { chooseWorkspace, newTab, view, workspaces } from './state.js';

// The menu open in the shell, the design system's floating menu (docs/design-system.md,
// Context menus): a group's, which names it and colours it, or a tab's. It floats over
// the page, so it is the shell's while open, and stays inside the window.

const el = ref(null);
const pos = ref({ left: 0, top: 0 });
const name = ref('');

const group = computed(() => (menu.kind === 'group' ? (state.groups || []).find((g) => g.id === menu.id) : null));
const tab = computed(() => (menu.kind === 'tab' ? state.tabs.find((t) => t.id === menu.id) : null));
const open = computed(() => !!(group.value || tab.value || menu.kind === 'workspaces'));
// The groups a tab can join: its window's, in its workspace, but its own.
const others = computed(() => {
  const t = tab.value;
  if (!t) return [];
  return (state.groups || []).filter((g) => g.id !== t.group && state.tabs.find((x) => x.id === g.tabs[0])?.workspace === t.workspace);
});

let pop = null;
watch(open, async (on) => {
  pop?.();
  pop = null;
  if (!on) return;
  pop = pushEscape(close);
  name.value = group.value?.name || '';
  pos.value = { left: menu.x, top: menu.y };
  await nextTick();
  place();
  if (menu.keyboard) el.value?.querySelector('input, [role^="menuitem"]:not(:disabled)')?.focus();
});

// A menu whose group or tab went away closes.
watch(() => [menu.kind, group.value, tab.value], () => {
  if (menu.kind && !open.value) closeMenu();
});

function place() {
  const r = el.value?.getBoundingClientRect();
  if (!r) return;
  const left = Math.max(4, Math.min(menu.x, window.innerWidth - r.width - 4));
  const top = menu.y + r.height + 4 > window.innerHeight ? Math.max(4, menu.above - r.height) : menu.y;
  pos.value = { left, top };
}

function close() {
  saveName();
  closeMenu();
}

function saveName() {
  const g = group.value;
  if (g && name.value.trim() !== g.name) send({ t: 'editGroup', group: g.id, name: name.value.trim() });
}

// run closes the menu, then sends msg, or calls it when it's a function.
function run(msg) {
  saveName();
  closeMenu();
  if (typeof msg === 'function') msg();
  else send(msg);
}

function onOutside(e) {
  if (open.value && el.value && !el.value.contains(e.target)) close();
}
window.addEventListener('pointerdown', onOutside, true);
window.addEventListener('blur', close);
onBeforeUnmount(() => {
  window.removeEventListener('pointerdown', onOutside, true);
  window.removeEventListener('blur', close);
  pop?.();
});

function onKey(e) {
  const items = [...el.value.querySelectorAll('[role^="menuitem"]:not(:disabled)')];
  const i = items.indexOf(document.activeElement);
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    if (e.target.tagName === 'INPUT' && e.key === 'ArrowUp') return;
    e.preventDefault();
    const step = e.key === 'ArrowDown' ? 1 : -1;
    items[(i + step + items.length) % items.length]?.focus();
  } else if (e.key === 'Home' || e.key === 'End') {
    if (e.target.tagName === 'INPUT') return;
    e.preventDefault();
    items[e.key === 'Home' ? 0 : items.length - 1]?.focus();
  } else if (e.key === 'Tab' && !el.value.contains(e.target.closest('.ui-swatches, input'))) {
    close();
  }
}
</script>

<template>
  <div
    v-if="open"
    ref="el"
    v-shell
    class="ui-menu float"
    role="menu"
    :aria-label="group ? `Group ${groupLabel(group)}` : tab ? 'Tab' : 'Workspaces'"
    :style="{ left: pos.left + 'px', top: pos.top + 'px' }"
    @keydown="onKey"
    @contextmenu.prevent
  >
    <template v-if="group">
      <div class="head">
        <input
          v-model="name"
          class="text-field"
          type="text"
          aria-label="Group name"
          placeholder="Name this group"
          spellcheck="false"
          @keydown.enter.prevent="close"
        />
        <Swatches
          :model-value="group.color"
          :colors="colors"
          label="Group colour"
          @update:model-value="(c) => send({ t: 'editGroup', group: group.id, color: c })"
        />
      </div>
      <div class="ui-menu-separator" role="separator"></div>
      <button type="button" role="menuitem" class="ui-menu-item" @click="run({ t: 'newTab', groupWith: group.tabs[group.tabs.length - 1] })">New tab in group</button>
      <button type="button" role="menuitem" class="ui-menu-item" @click="run({ t: 'editGroup', group: group.id, collapsed: !group.collapsed })">{{ group.collapsed ? 'Expand group' : 'Collapse group' }}</button>
      <div class="ui-menu-separator" role="separator"></div>
      <button type="button" role="menuitem" class="ui-menu-item" @click="run({ t: 'ungroup', group: group.id })">Ungroup</button>
      <button type="button" role="menuitem" class="ui-menu-item danger" @click="run({ t: 'closeGroup', group: group.id })">Close group</button>
    </template>
    <template v-else-if="menu.kind === 'workspaces'">
      <button
        type="button"
        role="menuitemradio"
        class="ui-menu-item"
        :aria-checked="view.workspace === null"
        @click="run(() => chooseWorkspace(null))"
      >
        <span class="dot all"></span>All workspaces<svg v-if="view.workspace === null" class="tick" width="12" height="12" viewBox="0 0 12 12" aria-hidden="true"><path d="M2.5 6.5l2.5 2.5 4.5-5" /></svg>
      </button>
      <button
        v-for="w in workspaces"
        :key="w.id"
        type="button"
        role="menuitemradio"
        class="ui-menu-item"
        :aria-checked="view.workspace === w.id"
        @click="run(() => chooseWorkspace(w.id))"
      >
        <span class="dot" :style="{ background: w.color }"></span>{{ w.name }}<svg v-if="view.workspace === w.id" class="tick" width="12" height="12" viewBox="0 0 12 12" aria-hidden="true"><path d="M2.5 6.5l2.5 2.5 4.5-5" /></svg>
      </button>
      <div class="ui-menu-separator" role="separator"></div>
      <button v-if="view.workspace !== null" type="button" role="menuitem" class="ui-menu-item" @click="run(newTab)">New tab in {{ workspaces.find((w) => w.id === view.workspace)?.name }}</button>
      <button type="button" role="menuitem" class="ui-menu-item" @click="run({ t: 'run', text: 'app.settings' })">Manage Workspaces…</button>
    </template>
    <template v-else-if="tab">
      <button type="button" role="menuitem" class="ui-menu-item" @click="run({ t: 'createGroup', ids: [tab.id] })">Add to new group</button>
      <button
        v-for="g in others"
        :key="g.id"
        type="button"
        role="menuitem"
        class="ui-menu-item"
        @click="run({ t: 'addToGroup', id: tab.id, group: g.id })"
      >
        <span class="dot" :style="{ background: g.color }"></span>{{ g.name ? `Add to ${g.name}` : `Add to group of ${groupLabel(g)}` }}
      </button>
      <button v-if="tab.group" type="button" role="menuitem" class="ui-menu-item" @click="run({ t: 'removeFromGroup', id: tab.id })">Remove from group</button>
      <div class="ui-menu-separator" role="separator"></div>
      <button type="button" role="menuitem" class="ui-menu-item" @click="run({ t: 'closeTab', id: tab.id })">Close Tab</button>
    </template>
  </div>
</template>

<style scoped>
.float {
  position: fixed;
  right: auto;
  z-index: 20;
  min-width: 220px;
  max-width: 320px;
}

.ui-menu-item {
  overflow: hidden;
  text-overflow: ellipsis;
}

.head {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 6px 8px 8px;
}

.head .text-field {
  width: 100%;
  box-sizing: border-box;
}

.dot.all {
  background: conic-gradient(#3470d0 0 33%, #d03434 0 66%, #2e9d5b 0);
}

.tick {
  flex: none;
  margin-left: auto;
  padding-left: 12px;
  fill: none;
  stroke: var(--accent);
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
  box-sizing: content-box;
}

.dot {
  flex: none;
  width: 10px;
  height: 10px;
  margin-right: 10px;
  border-radius: 50%;
}
</style>
