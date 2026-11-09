<script setup>
import { send } from './bridge.js';
import { controlSpace as controls, state, tabLabel } from './state.js';

// The strip is the title bar when the tabs are on top: it sits on the frame colour,
// leaves the native window controls their space, and its empty parts drag the window.
</script>

<template>
  <div v-shell class="strip" role="tablist" aria-label="Open tabs">
    <div v-drag class="space" :style="{ width: controls.left }"></div>
    <div
      v-for="tab in state.tabs"
      :key="tab.id"
      class="tab"
      :class="{ active: tab.id === state.active }"
      role="tab"
      tabindex="0"
      :aria-selected="tab.id === state.active"
      @click="send({ t: 'activateTab', id: tab.id })"
      @keydown.enter="send({ t: 'activateTab', id: tab.id })"
      @auxclick.middle="send({ t: 'closeTab', id: tab.id })"
    >
      <!-- The active tab's shape flares into the bar below it, from the Ulaa design. -->
      <svg v-if="tab.id === state.active" class="shape" viewBox="0 0 179 33" preserveAspectRatio="none" aria-hidden="true">
        <path d="M0.03 31.67C1.08 31.57 2.06 31.11 2.8 30.37C3.54 29.62 3.99 28.64 4.08 27.6V9.6C4.06 7.07 5.05 4.64 6.82 2.84C8.6 1.04 11.01.01 13.54 0H164.46C166.98.01 169.4 1.04 171.18 2.84C172.95 4.64 173.94 7.07 173.92 9.6V27.6C174.03 28.63 174.5 29.59 175.25 30.3C176.01 31.02 176.99 31.44 178.03 31.5C178.01 31.47 153.92 32.49 76.23 32.49C1.57 32.49-.38 31.42.03 31.67Z" />
      </svg>
      <img v-if="tab.favicon" class="icon" :src="tab.favicon" alt="" width="16" height="16" />
      <span class="title">{{ tabLabel(tab) }}</span>
      <button class="close" type="button" :aria-label="`Close ${tabLabel(tab)}`" @click.stop="send({ t: 'closeTab', id: tab.id })">
        <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68"/></svg>
      </button>
    </div>
    <button class="new-tab" type="button" aria-label="New Tab" title="New Tab (⌘T)" @click="send({ t: 'newTab' })">
      <svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M6.44.75v11.39M.75 6.19h11.39"/></svg>
    </button>
    <div v-drag class="rest"></div>
    <div v-drag class="space" :style="{ width: controls.right }"></div>
  </div>
</template>

<style scoped>
/* Frame colour through and through: the top band and the sides are its padding, and the
   bar below cuts its own corners into it. */
.strip {
  display: flex;
  align-items: flex-end;
  flex: none;
  height: 48px;
  padding: var(--frame-width) var(--frame-width) 0;
  box-sizing: border-box;
  background: var(--frame);
  color: rgba(255, 255, 255, 0.9);
}

.space {
  flex: none;
  height: 100%;
}

.rest {
  flex: 1;
  height: 100%;
}

.tab {
  position: relative;
  display: flex;
  align-items: center;
  flex: 0 1 168px;
  min-width: 56px;
  height: 32px;
  padding: 0 8px 0 12px;
  border-radius: 6px 6px 0 0;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  box-sizing: border-box;
}

.tab:hover:not(.active) {
  background: rgba(255, 255, 255, 0.14);
}

.tab:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.8);
}

.tab.active {
  color: var(--text);
}

/* The design's path ends half a unit above its bottom edge, which left a hairline of
   frame colour between the tab and the bar. Drawn 2px taller, that edge lies under the
   bar, which paints over it. */
.shape {
  position: absolute;
  inset: 0 -4px;
  width: calc(100% + 8px);
  height: calc(100% + 2px);
  fill: var(--bar);
}

.icon,
.title,
.close {
  position: relative;
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

.tab.active .close {
  color: var(--muted);
}

.close:hover {
  background: rgba(0, 0, 0, 0.08);
}

.new-tab {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 28px;
  height: 28px;
  margin: 0 0 2px 8px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: none;
  color: inherit;
  cursor: pointer;
}

.new-tab:hover {
  background: rgba(255, 255, 255, 0.14);
}
</style>
