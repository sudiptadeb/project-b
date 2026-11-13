<script setup>
import { shown as keyShown } from '@/keys.js';
import { controlSpace, send, state, tabLabel, workspaceOf } from '@/kit/shell.js';
import { ExtensionsToolbar } from '@extensions';
import { chips, shown } from './panes.js';

// The title bar: one chip per group, a segment per pane in it. A segment is the pane's tab;
// pressing it makes that tab active, which shows its group.
</script>

<template>
  <div v-shell class="strip">
    <div v-drag class="space" :style="{ width: controlSpace.left }"></div>
    <div class="chips" role="tablist" aria-label="Groups">
      <div v-for="chip in chips" :key="chip.tabs[0].id" class="chip" :class="{ shown: chip.root === shown }">
        <button
          v-for="tab in chip.tabs"
          :key="tab.id"
          class="segment"
          :class="{ active: tab.id === state.active }"
          type="button"
          role="tab"
          :data-tab="tab.id"
          :aria-selected="tab.id === state.active"
          :title="tabLabel(tab)"
          @click="send({ t: 'activateTab', id: tab.id })"
        >
          <span v-if="workspaceOf(tab)" class="workspace" role="img" :aria-label="`In ${workspaceOf(tab).name}`" :style="{ background: workspaceOf(tab).color }"></span>
          <img v-if="tab.favicon" class="icon" :src="tab.favicon" alt="" width="12" height="12" />
          <span class="label">{{ tabLabel(tab) }}</span>
          <span v-if="tab.loading" class="loading" role="img" aria-label="Loading"></span>
        </button>
      </div>
    </div>
    <button
      class="new-tab"
      type="button"
      aria-label="New Tab"
      :title="state.keys['tab.new'] ? `New Tab (${keyShown(state.keys['tab.new'], state.platform)})` : 'New Tab'"
      @click="send({ t: 'newTab' })"
    >
      <svg width="10" height="10" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M6.44.75v11.39M.75 6.19h11.39"/></svg>
    </button>
    <div v-drag class="rest"></div>
    <component :is="ExtensionsToolbar" v-if="ExtensionsToolbar" />
    <!-- The Ulaa design's gear, as the standard shell draws it. -->
    <button class="settings" type="button" aria-label="Settings" title="Settings" @click="send({ t: 'run', text: 'app.settings' })">
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
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

/* A group: its panes side by side, in a hairline box. The shown one is the surface. */
.chip {
  display: flex;
  flex: none;
  height: 22px;
  border: 1px solid var(--line);
  border-radius: 4px;
  box-sizing: border-box;
  overflow: hidden;
}

.chip.shown {
  background: var(--surface);
  border-color: var(--accent);
}

.segment {
  display: flex;
  align-items: center;
  gap: 6px;
  max-width: 160px;
  padding: 0 8px;
  border: 0;
  background: none;
  color: var(--muted);
  font: inherit;
  cursor: pointer;
}

.segment + .segment {
  border-left: 1px solid var(--line);
}

.segment:hover {
  background: var(--accent-tint);
}

.segment.active {
  color: var(--text);
  font-weight: 600;
}

.segment:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 1px var(--accent);
}

.label {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.icon {
  display: block;
  flex: none;
  width: 12px;
  height: 12px;
}

/* A workspace's colour, as a small square. */
.workspace {
  flex: none;
  width: 6px;
  height: 6px;
}

/* Loading, as a dot in the accent after the label. */
.loading {
  flex: none;
  width: 4px;
  height: 4px;
  background: var(--accent);
}

/* New Tab and the gear: small square buttons, the strip's own. */
.new-tab,
.settings {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 22px;
  height: 22px;
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
