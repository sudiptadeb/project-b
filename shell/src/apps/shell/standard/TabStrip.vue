<script setup>
import { shown } from '@/keys.js';
import { controlSpace as controls, send, state, tabLabel, workspaceOf } from '@/sdk/vue.js';

// The strip is the title bar when the tabs are on top: it sits on the bar, leaves the
// native window controls their space, and moves the window wherever it has no tab or
// button. Each tab is a row of the design system laid sideways: the selected one in
// --accent-soft, as Settings marks its section.
</script>

<template>
  <div v-shell v-drag class="strip" role="tablist" aria-label="Open tabs">
    <div class="space" :style="{ width: controls.left }"></div>
    <div
      v-for="tab in state.tabs"
      :key="tab.id"
      class="tab"
      :class="{ active: tab.id === state.active }"
      role="tab"
      :data-tab="tab.id"
      tabindex="0"
      :aria-selected="tab.id === state.active"
      @click="send({ t: 'activateTab', id: tab.id })"
      @keydown.enter="send({ t: 'activateTab', id: tab.id })"
      @auxclick.middle="send({ t: 'closeTab', id: tab.id })"
    >
      <img v-if="tab.favicon" class="icon" :src="tab.favicon" alt="" width="16" height="16" />
      <span class="title">{{ tabLabel(tab) }}</span>
      <span v-if="workspaceOf(tab)" class="workspace" role="img" :aria-label="`In ${workspaceOf(tab).name}`" :title="workspaceOf(tab).name" :style="{ background: workspaceOf(tab).color }"></span>
      <button class="close" type="button" :aria-label="`Close ${tabLabel(tab)}`" @click.stop="send({ t: 'closeTab', id: tab.id })">
        <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68"/></svg>
      </button>
    </div>
    <button class="new-tab" type="button" aria-label="New Tab" :title="state.keys['tab.new'] ? `New Tab (${shown(state.keys['tab.new'], state.platform)})` : 'New Tab'" @click="send({ t: 'newTab' })">
      <svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M6.44.75v11.39M.75 6.19h11.39"/></svg>
    </button>
    <div class="space end" :style="{ width: controls.right }"></div>
  </div>
</template>

<style scoped>
/* The bar's colour inside the window's band: the band is the strip's padding, in the
   frame colour. */
.strip {
  display: flex;
  align-items: center;
  flex: none;
  height: 48px;
  padding: var(--frame-width) var(--frame-width) 0;
  box-sizing: border-box;
  background: linear-gradient(var(--bar), var(--bar)) content-box, var(--frame);
  color: var(--text);
}

.space {
  flex: none;
  height: 100%;
}

/* The room the window controls need when they are on the right, at the strip's end. */
.end {
  margin-left: auto;
}

.tab {
  position: relative;
  display: flex;
  align-items: center;
  flex: 0 1 168px;
  min-width: 56px;
  height: 30px;
  padding: 0 8px 0 12px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  box-sizing: border-box;
}

.tab + .tab {
  margin-left: 2px;
}

.tab:hover:not(.active) {
  background: var(--accent-tint);
}

.tab:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

.tab.active {
  background: var(--accent-soft);
}

/* The workspace a tab is in, as a dot in its colour. */
.workspace {
  flex: none;
  width: 6px;
  height: 6px;
  margin: 0 6px;
  border-radius: 50%;
}

.icon {
  display: block;
  flex: none;
  width: 16px;
  height: 16px;
  margin-right: 8px;
}

.title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.close {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 16px;
  height: 16px;
  margin-left: 4px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: inherit;
  cursor: pointer;
  visibility: hidden;
}

.tab:hover .close,
.tab.active .close,
.tab:focus-within .close {
  visibility: visible;
}

.close {
  color: var(--muted);
}

.close:hover {
  background: var(--press);
}

.new-tab {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 28px;
  height: 28px;
  margin-left: 6px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--icon);
  cursor: pointer;
}

.new-tab:hover {
  background: var(--accent-tint);
}

</style>
