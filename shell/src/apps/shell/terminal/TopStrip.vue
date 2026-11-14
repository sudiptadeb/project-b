<script setup>
import { shown as keyShown } from '@/keys.js';
import { controlSpace, send, state, tabLabel, workspaceOf } from '@/sdk/vue.js';
import { ExtensionsToolbar } from '@extensions';
import { chips, shown } from './panes.js';

// The title bar: one chip per window, numbered, named for its tab group or the host it
// shows, with a square per pane. A square is the pane's tab; pressing it makes that tab
// active, which shows its window. Pressing the name shows the window at the pane it
// showed last.
</script>

<template>
  <div v-shell class="strip">
    <div v-drag class="space" :style="{ width: controlSpace.left }"></div>
    <div class="chips" role="tablist" aria-label="Windows">
      <div v-for="(chip, n) in chips" :key="chip.tabs[0].id" class="chip" :class="{ shown: chip.root === shown }">
        <button class="name" type="button" :title="chip.name" @click="send({ t: 'activateTab', id: chip.current.id })">
          <span v-if="workspaceOf(chip.current)" class="workspace" role="img" :aria-label="`In ${workspaceOf(chip.current).name}`" :style="{ background: workspaceOf(chip.current).color }"></span>
          <span class="number">{{ n + 1 }}</span>
          <span class="label">{{ chip.name }}</span>
        </button>
        <button
          v-for="tab in chip.tabs"
          :key="tab.id"
          class="pane"
          :class="{ current: tab.id === chip.current.id, loading: tab.loading }"
          type="button"
          role="tab"
          :data-tab="tab.id"
          :aria-selected="tab.id === state.active"
          :aria-label="tabLabel(tab)"
          :title="tabLabel(tab)"
          @click="send({ t: 'activateTab', id: tab.id })"
        ><span class="square"></span></button>
      </div>
    </div>
    <button
      class="new-tab"
      type="button"
      aria-label="New Tab"
      :title="state.keys['tab.new'] ? `New Tab (${keyShown(state.keys['tab.new'], state.platform)})` : 'New Tab'"
      @click="send({ t: 'newTab' })"
    >
      <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"><path d="M6 1v10M1 6h10"/></svg>
    </button>
    <div v-drag class="rest"></div>
    <component :is="ExtensionsToolbar" v-if="ExtensionsToolbar" :extensions="state.extensions" />
    <!-- The Ulaa design's gear, as the standard shell draws it. -->
    <button class="settings" type="button" aria-label="Settings" title="Settings" @click="send({ t: 'run', text: 'app.settings' })">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
    </button>
    <div v-drag class="space" :style="{ width: controlSpace.right }"></div>
  </div>
</template>

<style scoped>
.strip {
  display: flex;
  align-items: center;
  flex: none;
  height: 32px;
  border-bottom: 1px solid var(--line);
  box-sizing: border-box;
  background: var(--bar);
  color: var(--text);
}

.space {
  flex: none;
  align-self: stretch;
}

.rest {
  flex: 1;
  min-width: 8px;
  align-self: stretch;
}

/* The chips scroll sideways when they don't fit. */
.chips {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: none;
}

/* A window: its number and name, then a square per pane, the one it shows filled. The
   shown window is the surface, with the accent along its foot. */
.chip {
  display: flex;
  align-items: center;
  flex: none;
  height: 20px;
  padding-right: 4px;
  border-radius: 4px;
  box-sizing: border-box;
}

.chip.shown {
  background: var(--surface);
  box-shadow: inset 0 -2px 0 var(--accent);
}

.chip:hover:not(.shown) {
  background: var(--accent-tint);
}

.name {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 100%;
  max-width: 200px;
  padding: 0 4px 0 8px;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--muted);
  font: inherit;
  cursor: pointer;
}

.shown .name {
  color: var(--text);
}

.number {
  flex: none;
  color: var(--muted);
}

.label {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

/* A pane: an 8px square in a 12px by 20px button, outlined, filled for the pane the window
   shows; while its page loads, the square is in the accent. */
.pane {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 12px;
  height: 20px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--muted);
  cursor: pointer;
}

.square {
  width: 8px;
  height: 8px;
  box-sizing: border-box;
  border: 1px solid currentColor;
}

.pane.current {
  color: var(--text);
}

.pane.current .square {
  background: currentColor;
}

.pane.loading {
  color: var(--accent);
}

.name:focus-visible,
.pane:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

/* A workspace's colour, as a small square. */
.workspace {
  flex: none;
  width: 8px;
  height: 8px;
}

/* New Tab and the gear: small square buttons, the strip's own. */
.new-tab,
.settings {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 20px;
  height: 20px;
  margin-left: 4px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--icon);
  cursor: pointer;
}

.new-tab:hover,
.settings:hover {
  background: var(--accent-tint);
}

.new-tab:focus-visible,
.settings:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}
</style>
