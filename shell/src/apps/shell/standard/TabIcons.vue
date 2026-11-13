<script setup>
import { send, state, tabLabel } from '@/kit/shell.js';

// A phone's tabs: one favicon each, in a row under the toolbar. A tap shows that tab; the
// sidebar button opens the full list. A tab with no favicon yet shows its label's first
// letter.
function initial(tab) {
  return [...tabLabel(tab)][0].toUpperCase();
}
</script>

<template>
  <div v-shell class="icons" role="tablist" aria-label="Open tabs">
    <div class="band"></div>
    <div class="row">
      <button
        v-for="tab in state.tabs"
        :key="tab.id"
        class="tab"
        :class="{ active: tab.id === state.active }"
        type="button"
        role="tab"
        :data-tab="tab.id"
        :aria-selected="tab.id === state.active"
        :aria-label="tabLabel(tab)"
        @click="send({ t: 'activateTab', id: tab.id })"
      >
        <img v-if="tab.favicon" :src="tab.favicon" alt="" width="16" height="16" />
        <span v-else class="initial" aria-hidden="true">{{ initial(tab) }}</span>
      </button>
    </div>
    <div class="band"></div>
  </div>
</template>

<style scoped>
/* The toolbar's bar carried on below it, between the frame's side bands. */
.icons {
  display: flex;
  flex: none;
  background: var(--frame);
}

.band {
  flex: none;
  width: var(--frame-width);
}

.row {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  gap: 4px;
  height: 48px;
  padding: 0 8px;
  box-sizing: border-box;
  overflow-x: auto;
  scrollbar-width: none;
  background: var(--bar);
}

/* The shown tab is marked as in the sidebar: a white card with a line, no shadow. */
.tab {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 40px;
  height: 40px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 6px;
  box-sizing: border-box;
  background: none;
  cursor: pointer;
}

.tab.active {
  background: var(--surface);
  border-color: var(--line);
}

.tab:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.tab img {
  display: block;
  width: 16px;
  height: 16px;
}

.initial {
  color: var(--muted);
  font-size: 13px;
  font-weight: 500;
}
</style>
