<script setup>
import { computed } from 'vue';
import { send, state, tabLabel, workspaceOf } from '@/sdk/vue.js';

// One tab, in the tab bar or the side list: its workspace's colour when it has one, its
// label, a loading mark while it loads, and a close button, always on the active tab
// and while pointed at on the others.
const props = defineProps({ tab: { type: Object, required: true } });
const active = computed(() => props.tab.id === state.active);
const workspace = computed(() => workspaceOf(props.tab));
</script>

<template>
  <div
    class="tab"
    :class="{ active }"
    role="tab"
    :data-tab="tab.id"
    :aria-selected="active"
    @click="send({ t: 'activateTab', id: tab.id })"
  >
    <span v-if="workspace" class="dot" :style="{ background: workspace.color }"></span>
    <span class="t">{{ tabLabel(tab) }}</span>
    <!-- SVG's own animation turns it, so the shell's layout isn't measured every frame
         while a page loads, as it is during a CSS animation. -->
    <svg v-if="tab.loading" class="loading" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" role="img" aria-label="Loading">
      <path d="M6 1.25A4.75 4.75 0 1 1 1.25 6">
        <animateTransform attributeName="transform" type="rotate" from="0 6 6" to="360 6 6" dur="0.9s" repeatCount="indefinite" />
      </path>
    </svg>
    <button class="x" :aria-label="`Close ${tabLabel(tab)}`" @click.stop="send({ t: 'closeTab', id: tab.id })">
      <svg viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.25"><path d="m2 2 6 6M8 2 2 8" /></svg>
    </button>
  </div>
</template>

<style scoped>
.tab {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 10px;
  box-sizing: border-box;
  overflow: hidden;
  white-space: nowrap;
  color: var(--muted);
}

.tab:hover:not(.active) {
  background: var(--hover);
}

.tab.active {
  background: var(--bar);
  color: var(--text);
  font-weight: 500;
}

.dot {
  flex: none;
  width: 6px;
  height: 6px;
}

.t {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
}

.loading {
  flex: none;
  width: 12px;
  height: 12px;
  color: var(--accent);
}

.x {
  flex: none;
  display: grid;
  place-items: center;
  width: 16px;
  height: 16px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--muted);
}

.x svg {
  width: 10px;
  height: 10px;
}

.x:hover {
  background: var(--hover);
}

.tab:not(.active, :hover) .x {
  display: none;
}
</style>
