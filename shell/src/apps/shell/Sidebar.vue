<script setup>
import { send } from '@/bridge.js';
import { shown } from '@/keys.js';
import { state, tabLabel } from './state.js';
</script>

<template>
  <nav v-shell class="sidebar" :class="{ closed: !state.sidebar }" aria-label="Tabs">
    <div class="inner">
      <button class="row new-tab" type="button" @click="send({ t: 'newTab' })">
        <span class="icon">
          <svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M6.44.75v11.39M.75 6.19h11.39"/></svg>
        </span>
        <span class="title">New Tab</span>
        <span v-if="state.keys['tab.new']" class="hint" aria-hidden="true">{{ shown(state.keys['tab.new'], state.platform) }}</span>
      </button>
      <div class="tabs" role="tablist" aria-label="Open tabs">
        <div
          v-for="tab in state.tabs"
          :key="tab.id"
          class="row tab"
          :class="{ active: tab.id === state.active }"
          role="tab"
          tabindex="0"
          :aria-selected="tab.id === state.active"
          @click="send({ t: 'activateTab', id: tab.id })"
          @keydown.enter="send({ t: 'activateTab', id: tab.id })"
          @auxclick.middle="send({ t: 'closeTab', id: tab.id })"
        >
          <span class="icon">
            <img v-if="tab.favicon" :src="tab.favicon" alt="" width="16" height="16" />
          </span>
          <span class="title" :class="{ muted: !tab.title }">{{ tabLabel(tab) }}</span>
          <button class="close" type="button" :aria-label="`Close ${tabLabel(tab)}`" @click.stop="send({ t: 'closeTab', id: tab.id })">
            <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68"/></svg>
          </button>
        </div>
      </div>
      <div class="rest"></div>
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
  background: var(--hover);
}

.row:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1px var(--accent), 0 0 0 3px var(--accent-soft);
}

/* Borders carry separation: the active tab is a white card with a line, no shadow. */
.row.active {
  background: var(--surface);
  border-color: var(--line);
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

/* Fingers need bigger targets than a mouse does, a touch screen has no hover, and a phone
   has no Command key. */
.touch .row {
  height: 44px;
  padding-right: 4px;
}

.touch .close {
  width: 32px;
  height: 32px;
}

.touch .row:hover:not(.active),
.touch .close:hover {
  background: none;
}

.touch .hint {
  display: none;
}
</style>
