<script setup>
import { nextTick, onUnmounted, ref } from 'vue';
import { pushEscape } from './escape.js';

// A row's actions: a ⋯ button that opens a menu of them. items is a list of
// { label, run, danger, disabled } and { separator: true }. The row opens the same menu on
// a right-click by calling open(). The arrow keys move through it, Enter runs an item,
// and Escape, Tab or a press outside closes it, focus going back to the button.
const props = defineProps({ items: Array, label: String });

const shown = ref(false);
const button = ref(null);
const menu = ref(null);
let pop = null;

function entries() {
  return [...(menu.value?.querySelectorAll('[role="menuitem"]:not(:disabled)') || [])];
}

async function open() {
  if (shown.value) return;
  shown.value = true;
  pop = pushEscape(() => close(true));
  document.addEventListener('pointerdown', outside, true);
  await nextTick();
  entries()[0]?.focus();
}

function close(refocus) {
  if (!shown.value) return;
  shown.value = false;
  pop();
  document.removeEventListener('pointerdown', outside, true);
  if (refocus) button.value?.focus();
}

function outside(e) {
  if (!menu.value?.contains(e.target) && !button.value?.contains(e.target)) close(false);
}

function run(item) {
  close(true);
  item.run();
}

function onKey(e) {
  const list = entries();
  const i = list.indexOf(document.activeElement);
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault();
    const step = e.key === 'ArrowDown' ? 1 : -1;
    list[(i + step + list.length) % list.length]?.focus();
  } else if (e.key === 'Home' || e.key === 'End') {
    e.preventDefault();
    list[e.key === 'Home' ? 0 : list.length - 1]?.focus();
  } else if (e.key === 'Tab') {
    close(false);
  }
}

onUnmounted(() => close(false));
defineExpose({ open });
</script>

<template>
  <div class="ui-row-menu">
    <button
      ref="button"
      type="button"
      class="icon-button"
      :aria-label="label"
      :title="label"
      aria-haspopup="menu"
      :aria-expanded="shown"
      @click="shown ? close(true) : open()"
    >
      <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
        <circle cx="3.5" cy="8" r="1.25" /><circle cx="8" cy="8" r="1.25" /><circle cx="12.5" cy="8" r="1.25" />
      </svg>
    </button>
    <div v-if="shown" ref="menu" class="ui-menu" role="menu" :aria-label="label" @keydown="onKey">
      <template v-for="(item, i) in props.items" :key="i">
        <div v-if="item.separator" class="ui-menu-separator" role="separator"></div>
        <button
          v-else
          type="button"
          role="menuitem"
          class="ui-menu-item"
          :class="{ danger: item.danger }"
          :disabled="item.disabled"
          @click="run(item)"
        >{{ item.label }}</button>
      </template>
    </div>
  </div>
</template>

<style>
.ui-row-menu {
  position: relative;
  flex: none;
}

.ui-row-menu > .icon-button[aria-expanded='true'] {
  background: var(--accent-soft);
}

/* The menu, as the design system's floating menu: right-aligned under its button. */
.ui-menu {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  z-index: 10;
  min-width: 180px;
  padding: 4px;
  box-sizing: border-box;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  box-shadow: var(--shadow-float);
}

.ui-menu-item {
  display: flex;
  align-items: center;
  width: 100%;
  height: 32px;
  padding: 0 12px;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  text-align: left;
  white-space: nowrap;
  cursor: pointer;
}

.ui-menu-item:hover:not(:disabled) {
  background: var(--accent-tint);
}

.ui-menu-item:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

.ui-menu-item:disabled {
  opacity: 0.5;
  cursor: default;
}

.ui-menu-item.danger {
  color: var(--danger);
}

.ui-menu-separator {
  margin: 4px 0;
  border-top: 1px solid var(--line);
}

.touch .ui-menu-item {
  height: 40px;
}
</style>
