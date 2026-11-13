<script setup>
import { ref } from 'vue';
import { closesOutside } from './dismiss.js';
import { keyOf } from './hints.js';
import { send } from '@/kit/shell.js';

// The menu button's panel: commands, each an action Go has, run as its menu item would
// run it, with the key that action shows.
const groups = [
  [
    { action: 'tab.new', label: 'New Tab' },
    { action: 'window.new', label: 'New Window' },
  ],
  [
    { action: 'view.zoomIn', label: 'Zoom In' },
    { action: 'view.zoomOut', label: 'Zoom Out' },
    { action: 'view.zoomReset', label: 'Actual Size' },
    { action: 'view.fullScreen', label: 'Full Screen' },
  ],
  [{ action: 'app.settings', label: 'Settings' }],
];

const root = ref(null);
const open = ref(false);
closesOutside(open, root);

function run(action) {
  open.value = false;
  send({ t: 'run', text: action });
}
</script>

<template>
  <div ref="root" class="app-menu">
    <button class="icon-button" :class="{ open }" type="button" aria-label="Menu" title="Menu" :aria-expanded="open" @click="open = !open">
      <svg width="14" height="12" viewBox="0 0 14 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M1 1.5h12M1 6h12M1 10.5h12" /></svg>
    </button>
    <div v-if="open" v-shell class="menu" role="menu" aria-label="Menu">
      <template v-for="(group, i) in groups" :key="i">
        <div v-if="i > 0" class="line" role="separator"></div>
        <button v-for="item in group" :key="item.action" class="item" type="button" role="menuitem" :data-action="item.action" @click="run(item.action)">
          <span class="label">{{ item.label }}</span>
          <span v-if="keyOf(item.action)" class="key">{{ keyOf(item.action) }}</span>
        </button>
      </template>
    </div>
  </div>
</template>

<style scoped>
.app-menu {
  position: relative;
  flex: none;
}

.icon-button.open,
.icon-button.open:hover {
  background: var(--accent-soft);
}

/* Floating, under the button, against the window's right. */
.menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 10;
  width: 280px;
  padding: 4px;
  box-sizing: border-box;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: var(--shadow-float);
}

.item {
  display: flex;
  align-items: center;
  width: 100%;
  height: 32px;
  padding: 0 8px;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.item:hover {
  background: var(--accent-tint);
}

.item:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 2px var(--accent-soft);
}

.label {
  flex: 1;
}

.key {
  flex: none;
  color: var(--muted);
}

.line {
  height: 1px;
  margin: 4px 0;
  background: var(--line);
}
</style>
