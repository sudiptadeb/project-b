<script setup>
import { computed } from 'vue';
import { keyLabel, send, state, tabLabel } from '@/sdk/vue.js';
import { close, split } from './panes.js';

// One pane: a title bar over the place its tab's page shows. A press on the bar makes the
// pane active.
const props = defineProps({ id: { type: Number, required: true } });

const tab = computed(() => state.tabs.find((t) => t.id === props.id));
const active = computed(() => props.id === state.active);
const host = computed(() => (tab.value.title && tab.value.url ? new URL(tab.value.url).host : ''));
const status = computed(() => {
  const t = tab.value;
  if (t.blocked) return 'Blocked';
  if (t.crashed) return 'Crashed';
  if (t.error) return 'Failed';
  return '';
});

function press() {
  if (!active.value) send({ t: 'activateTab', id: props.id });
}

// A control's tooltip: its name, and its key when it has one.
function hint(name, action) {
  const key = keyLabel(action);
  return key ? `${name} (${key})` : name;
}
</script>

<template>
  <div class="pane">
    <div v-shell class="title" :class="{ active }" :data-pane="id" @pointerdown="press">
      <span class="label">{{ tabLabel(tab) }}</span>
      <span v-if="host" class="host">{{ host }}</span>
      <span v-if="status" class="status">{{ status }}</span>
      <span class="gap"></span>
      <button class="control" type="button" aria-label="Split right" :title="hint('Split right', 'terminal.splitRight')" @pointerdown.stop @click="split(id, 'row')">
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2"><rect x=".6" y=".6" width="10.8" height="10.8"/><path d="M6 .6v10.8"/></svg>
      </button>
      <button class="control" type="button" aria-label="Split down" :title="hint('Split down', 'terminal.splitDown')" @pointerdown.stop @click="split(id, 'column')">
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2"><rect x=".6" y=".6" width="10.8" height="10.8"/><path d="M.6 6h10.8"/></svg>
      </button>
      <button class="control" type="button" :aria-label="`Close ${tabLabel(tab)}`" title="Close" @pointerdown.stop @click="close(id)">
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"><path d="M9.5 2.5l-7 7M2.5 2.5l7 7"/></svg>
      </button>
      <span class="progress" :class="{ on: tab.loading }" :style="{ width: (tab.progress || 0) * 100 + '%' }"></span>
    </div>
    <div v-page="id" class="page"></div>
  </div>
</template>

<style scoped>
.pane {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
}

/* Quiet on the bar; the active pane's is in the accent. */
.title {
  position: relative;
  display: flex;
  align-items: center;
  flex: none;
  gap: 8px;
  height: 20px;
  padding: 0 4px 0 8px;
  box-sizing: border-box;
  background: var(--bar);
  color: var(--muted);
  cursor: default;
  user-select: none;
}

.title.active {
  background: var(--accent);
  color: var(--on-accent);
}

.label,
.host {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.label {
  min-width: 0;
}

.host {
  flex: none;
  max-width: 40%;
}

.status {
  flex: none;
  padding: 0 4px;
  background: var(--danger);
  color: var(--on-danger);
}

.gap {
  flex: 1;
}

.control {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 16px;
  height: 16px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: inherit;
  cursor: pointer;
  opacity: 0.8;
}

.control:hover {
  background: var(--hover);
  opacity: 1;
}

/* On the accent bar the toolbar's hover would wash the icon out, so it darkens instead,
   and the focus ring, in the accent itself, is drawn in the bar's ink. */
.title.active .control:hover {
  background: var(--press);
}

.control:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
  opacity: 1;
}

.title.active .control:focus-visible {
  box-shadow: inset 0 0 0 1px var(--on-accent);
}

/* The page's load progress, along the foot of the bar. */
.progress {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 2px;
  background: var(--accent);
  opacity: 0;
  transition: width 0.15s ease-out;
}

.title.active .progress {
  background: var(--on-accent);
}

.progress.on {
  opacity: 1;
}

.page {
  flex: 1;
  min-height: 0;
}
</style>
