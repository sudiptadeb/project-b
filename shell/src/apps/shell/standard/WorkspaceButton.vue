<script setup>
import { computed } from 'vue';
import { openMenu } from './menu.js';
import { view, workspaces } from './state.js';

// The workspace the window shows, and the menu that chooses another: at the sidebar's foot,
// or, compact, at the start of the strip on top. There only while there are two or more.
defineProps({ compact: Boolean });
const current = computed(() => workspaces.value.find((w) => w.id === view.workspace) || null);
</script>

<template>
  <button
    type="button"
    class="ws-button"
    :class="{ compact }"
    aria-haspopup="menu"
    :aria-label="`Workspace: ${current ? current.name : 'All workspaces'}`"
    @click="openMenu('workspaces', null, $event, true)"
    @keydown.shift.f10="openMenu('workspaces', null, $event, true)"
  >
    <span v-if="current" class="dot" :style="{ background: current.color }"></span>
    <span v-else class="dots" aria-hidden="true">
      <span v-for="w in workspaces.slice(0, 3)" :key="w.id" class="dot" :style="{ background: w.color }"></span>
    </span>
    <span class="name">{{ current ? current.name : 'All workspaces' }}</span>
    <span class="chevron" aria-hidden="true"></span>
  </button>
</template>

<style scoped>
.ws-button {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
  height: 32px;
  padding: 0 10px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}

.ws-button:hover {
  background: var(--accent-tint);
}

.ws-button:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

.compact {
  height: 28px;
  margin-right: 6px;
  padding: 0 8px;
  font-size: 12px;
}

.dot {
  flex: none;
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.dots {
  display: flex;
}

.dots .dot + .dot {
  margin-left: -4px;
  box-shadow: 0 0 0 1.5px var(--bar);
}

.name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.compact .name {
  max-width: 110px;
}

.chevron {
  flex: none;
  width: 6px;
  height: 6px;
  margin: 0 2px 3px 2px;
  border-right: 1.5px solid var(--icon);
  border-bottom: 1.5px solid var(--icon);
  transform: rotate(45deg);
}
</style>
