<script setup>
import { computed } from 'vue';
import { send, state, tabLabel, workspaceOf } from '@/sdk/vue.js';
import { shown } from '@/keys.js';
import { entries, groupLabel } from './groups.js';
import { openMenu } from './menu.js';
import { newTab, view, workspaces } from './state.js';
import WorkspaceButton from './WorkspaceButton.vue';

// The list, flat: a group's header, then its tabs, which carry the group for their rule.
const rows = computed(() =>
  entries(state, view.workspace).flatMap((e) => (e.group ? [{ header: e.group }, ...e.tabs.map((tab) => ({ tab, group: e.group }))] : [e])),
);
</script>

<template>
  <nav v-shell class="sidebar" :class="{ closed: !view.sidebar }" aria-label="Tabs">
    <div class="inner">
      <button class="row new-tab" type="button" @click="newTab">
        <span class="icon">
          <svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M6.44.75v11.39M.75 6.19h11.39"/></svg>
        </span>
        <span class="title">New Tab</span>
        <span v-if="state.keys['tab.new']" class="hint" aria-hidden="true">{{ shown(state.keys['tab.new'], state.platform) }}</span>
      </button>
      <div class="tabs" role="tablist" aria-label="Open tabs">
        <template v-for="r in rows" :key="r.header ? 'g' + r.header.id : r.tab.id">
        <button
          v-if="r.header"
          type="button"
          class="row group"
          :data-group="r.header.id"
          :aria-expanded="!r.header.collapsed"
          :title="groupLabel(r.header)"
          @click="send({ t: 'editGroup', group: r.header.id, collapsed: !r.header.collapsed })"
          @contextmenu="openMenu('group', r.header.id, $event)"
          @keydown.shift.f10="openMenu('group', r.header.id, $event)"
        >
          <span class="icon"><span class="swatch" :style="{ background: r.header.color }"></span></span>
          <span class="title" :class="{ muted: !r.header.name }">{{ groupLabel(r.header) }}</span>
          <span class="chevron" :class="{ collapsed: r.header.collapsed }" aria-hidden="true"></span>
        </button>
        <div
          v-else
          class="row tab"
          :class="{ active: r.tab.id === state.active, grouped: r.group }"
          :style="r.group ? { '--group': r.group.color } : null"
          role="tab"
          :data-tab="r.tab.id"
          tabindex="0"
          :aria-selected="r.tab.id === state.active"
          @click="send({ t: 'activateTab', id: r.tab.id })"
          @keydown.enter="send({ t: 'activateTab', id: r.tab.id })"
          @keydown.shift.f10="openMenu('tab', r.tab.id, $event)"
          @auxclick.middle="send({ t: 'closeTab', id: r.tab.id })"
          @contextmenu="openMenu('tab', r.tab.id, $event)"
        >
          <span class="icon">
            <img v-if="r.tab.favicon" :src="r.tab.favicon" alt="" width="16" height="16" />
          </span>
          <span class="title" :class="{ muted: !r.tab.title }">{{ tabLabel(r.tab) }}</span>
          <span v-if="workspaceOf(r.tab)" class="workspace" role="img" :aria-label="`In ${workspaceOf(r.tab).name}`" :title="workspaceOf(r.tab).name" :style="{ background: workspaceOf(r.tab).color }"></span>
          <button class="close" type="button" :aria-label="`Close ${tabLabel(r.tab)}`" @click.stop="send({ t: 'closeTab', id: r.tab.id })">
            <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68"/></svg>
          </button>
        </div>
        </template>
      </div>
      <div class="rest"></div>
      <WorkspaceButton v-if="workspaces.length" class="foot" />
    </div>
  </nav>
</template>

<style scoped>
/* The sidebar collapses to nothing; the page follows. Frame colour underneath, as for
   every tracked component, with its surface drawn on top. */
.sidebar {
  flex: none;
  width: 220px;
  overflow: hidden;
  background: var(--frame);
  transition: width 160ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.sidebar.closed {
  width: 0;
}

.inner {
  display: flex;
  flex-direction: column;
  width: 220px;
  height: 100%;
  padding: 8px;
  box-sizing: border-box;
  background: var(--bar);
}

.tabs {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 4px;
  overflow-y: auto;
  scrollbar-width: none;
}

.rest {
  flex: 1;
}

.row {
  display: flex;
  align-items: center;
  flex: none;
  height: 32px;
  padding: 0 6px 0 10px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
  box-sizing: border-box;
}

.row:hover {
  background: var(--accent-tint);
}

.row:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

/* The selected tab as Settings marks its selected section: --accent-soft. */
.row.active {
  background: var(--accent-soft);
}

.new-tab {
  color: var(--icon);
}

.icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 16px;
  height: 16px;
}

.icon img {
  display: block;
  width: 16px;
  height: 16px;
}

.title {
  flex: 1;
  min-width: 0;
  margin-left: 10px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.title.muted {
  color: var(--muted);
}

/* A group's header: its colour, its name, and a chevron that folds it. */
.group .swatch {
  width: 10px;
  height: 10px;
  border-radius: 3px;
}

.chevron {
  flex: none;
  width: 6px;
  height: 6px;
  margin: 0 6px 2px;
  border-right: 1.5px solid var(--icon);
  border-bottom: 1.5px solid var(--icon);
  transform: rotate(45deg);
}

.chevron.collapsed {
  margin-bottom: 0;
  transform: rotate(-45deg);
}

/* A grouped tab sits in from the edge, on a rule in its group's colour. */
.tab.grouped {
  position: relative;
  margin-left: 12px;
}

.tab.grouped::before {
  content: '';
  position: absolute;
  left: -8px;
  top: -2px;
  bottom: -2px;
  width: 2px;
  border-radius: 1px;
  background: var(--group);
}

/* The workspace a tab is in, as a dot in its colour. */
.workspace {
  flex: none;
  width: 6px;
  height: 6px;
  margin: 0 6px;
  border-radius: 50%;
}

.hint {
  flex: none;
  margin-right: 4px;
  color: var(--muted);
  font-size: 13px;
  opacity: 0.6;
}

.close {
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
  color: var(--muted);
  cursor: pointer;
  visibility: hidden;
}

.row:hover .close,
.row.active .close,
.row:focus-within .close {
  visibility: visible;
}

.close:hover {
  background: var(--press);
  color: var(--text);
}
</style>
