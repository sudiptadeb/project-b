<script setup>
import { ref } from 'vue';
import TabIcon from './TabIcon.vue';
import { closesOutside } from './dismiss.js';
import { send, state, tabLabel } from '@/kit/shell.js';

// Every tab of the window in one floating menu, the active one marked; choosing one makes
// it active. It reaches the tabs the strip has scrolled out of sight.
const root = ref(null);
const open = ref(false);
closesOutside(open, root);

function choose(tab) {
  open.value = false;
  send({ t: 'activateTab', id: tab.id });
}
</script>

<template>
  <div ref="root" class="all">
    <button class="icon-button" :class="{ open }" type="button" aria-label="List all tabs" title="List all tabs" :aria-expanded="open" @click="open = !open">
      <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m2 4.5 4 4 4-4" /></svg>
    </button>
    <div v-if="open" v-shell class="menu" role="group" aria-label="All tabs">
      <button
        v-for="tab in state.tabs"
        :key="tab.id"
        class="item"
        :class="{ current: tab.id === state.active }"
        type="button"
        :aria-current="tab.id === state.active"
        @click="choose(tab)"
      >
        <TabIcon :tab="tab" />
        <span class="title">{{ tabLabel(tab) }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.all {
  position: relative;
  flex: none;
}

.icon-button {
  color: inherit;
}

.icon-button:hover,
.icon-button.open {
  background: color-mix(in srgb, var(--on-accent) 16%, transparent);
}

/* Floating, under its button, over the bar and the page. A long list scrolls. */
.menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 10;
  display: flex;
  flex-direction: column;
  width: 280px;
  max-height: calc(100vh - 96px);
  overflow-y: auto;
  padding: 4px;
  box-sizing: border-box;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: var(--shadow-float);
  color: var(--text);
}

.item {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: none;
  height: 32px;
  padding: 0 8px;
  border: 0;
  border-radius: 4px;
  background: none;
  color: inherit;
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.item:hover {
  background: var(--accent-tint);
}

.item.current {
  background: var(--accent-soft);
  font-weight: 500;
}

.item:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 2px var(--accent-soft);
}

.title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
</style>
