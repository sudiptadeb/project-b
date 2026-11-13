<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import AllTabs from './AllTabs.vue';
import TabIcon from './TabIcon.vue';
import { withKey } from './hints.js';
import { controlSpace, send, state, tabLabel, workspaceOf } from '@/sdk/vue.js';

// The top row is the title bar: the native window buttons' space, the tabs, New Tab right
// after the last one, then empty space that drags the window, and the list of all tabs.
// The tabs share their row between a minimum and a maximum width each. Once they no
// longer fit at the minimum the row scrolls sideways, with arrows at its ends, New Tab
// stays at its end, and the active tab is always scrolled into view.
const scroller = ref(null);
const overflows = ref(false);
const atStart = ref(true);
const atEnd = ref(true);
const tabEls = new Map();

function measure() {
  const s = scroller.value;
  overflows.value = s.scrollWidth - s.clientWidth > 1;
  atStart.value = s.scrollLeft <= 0;
  atEnd.value = s.scrollLeft + s.clientWidth >= s.scrollWidth - 1;
}

function reveal() {
  const s = scroller.value;
  const el = tabEls.get(state.active);
  // Before the snapshot there is no tab to bring into view.
  if (!el) return;
  const left = el.offsetLeft;
  const right = left + el.offsetWidth;
  if (left < s.scrollLeft) s.scrollLeft = left;
  else if (right > s.scrollLeft + s.clientWidth) s.scrollLeft = right - s.clientWidth;
}

function update() {
  measure();
  reveal();
}

// An arrow moves the row by half of what it shows.
function scrollBy(direction) {
  const s = scroller.value;
  s.scrollBy({ left: (direction * s.clientWidth) / 2, behavior: 'smooth' });
}

// A mouse wheel turns the row sideways; a trackpad's sideways swipe already does.
function onWheel(e) {
  if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
    e.preventDefault();
    scroller.value.scrollLeft += e.deltaY;
  }
}

const observer = new ResizeObserver(update);
onMounted(() => observer.observe(scroller.value));
onBeforeUnmount(() => observer.disconnect());
watch(() => [state.active, state.tabs.length], update, { flush: 'post' });
</script>

<template>
  <div v-shell class="strip">
    <div v-drag class="space" :style="{ width: controlSpace.left }"></div>
    <button v-if="overflows" class="icon-button arrow" type="button" aria-label="Scroll tabs left" :disabled="atStart" @click="scrollBy(-1)">
      <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M7.5 2 3.5 6l4 4" /></svg>
    </button>
    <div ref="scroller" class="tabs" role="tablist" aria-label="Open tabs" @scroll="measure" @wheel="onWheel">
      <div
        v-for="tab in state.tabs"
        :key="tab.id"
        :ref="(el) => (el ? tabEls.set(tab.id, el) : tabEls.delete(tab.id))"
        class="tab"
        :class="{ active: tab.id === state.active }"
        role="tab"
        :data-tab="tab.id"
        tabindex="0"
        :aria-selected="tab.id === state.active"
        :title="tabLabel(tab)"
        @click="send({ t: 'activateTab', id: tab.id })"
        @keydown.enter="send({ t: 'activateTab', id: tab.id })"
        @mousedown.middle.prevent
        @auxclick.middle="send({ t: 'closeTab', id: tab.id })"
      >
        <span v-if="workspaceOf(tab)" class="workspace" role="img" :aria-label="`In ${workspaceOf(tab).name}`" :style="{ background: workspaceOf(tab).color }"></span>
        <TabIcon :tab="tab" />
        <span class="title">{{ tabLabel(tab) }}</span>
        <button class="close" type="button" :aria-label="`Close ${tabLabel(tab)}`" @click.stop="send({ t: 'closeTab', id: tab.id })">
          <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68" /></svg>
        </button>
      </div>
    </div>
    <button v-if="overflows" class="icon-button arrow" type="button" aria-label="Scroll tabs right" :disabled="atEnd" @click="scrollBy(1)">
      <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m4.5 2 4 4-4 4" /></svg>
    </button>
    <button class="icon-button new-tab" type="button" aria-label="New Tab" :title="withKey('New Tab', 'tab.new')" @click="send({ t: 'newTab' })">
      <svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M6.5.75v11.5M.75 6.5h11.5" /></svg>
    </button>
    <div v-drag class="rest"></div>
    <AllTabs />
    <div v-drag class="space" :style="{ width: controlSpace.right }"></div>
  </div>
</template>

<style scoped>
/* On the frame colour, inside the frame's top and side bands. Inactive tabs and the
   row's buttons take the colour that reads on the accent, which the frame is in light
   and near in dark. */
.strip {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  flex: none;
  height: 44px;
  padding: var(--frame-width) var(--frame-width) 0;
  box-sizing: border-box;
  background: var(--frame);
  color: var(--on-accent);
}

.space {
  flex: none;
  align-self: stretch;
}

.rest {
  flex: 1 0 24px;
  align-self: stretch;
}

.tabs {
  position: relative;
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 0 1 auto;
  min-width: 0;
  height: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;
}

.tab {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 1 224px;
  min-width: 76px;
  height: 32px;
  padding: 0 6px 0 10px;
  box-sizing: border-box;
  border-radius: 6px;
  font-size: 13px;
  cursor: default;
}

.tab:hover:not(.active) {
  background: color-mix(in srgb, var(--on-accent) 16%, transparent);
}

.tab.active {
  background: var(--bar);
  color: var(--text);
  box-shadow: var(--shadow-float);
}

.tab:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 2px var(--accent-soft);
}

.tab.active:focus-visible {
  box-shadow: var(--shadow-float), inset 0 0 0 2px var(--accent-soft);
}

/* The workspace the tab is in, as a line in its colour along the top edge. */
.workspace {
  position: absolute;
  top: 0;
  left: 8px;
  right: 8px;
  height: 2px;
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
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: inherit;
  cursor: pointer;
}

/* An inactive tab shows its close control only while pointed at, and gives its room to
   the title otherwise: a narrow tab has little to spare. */
.tab:not(.active, :hover, :focus-within) .close {
  display: none;
}

.tab.active .close {
  color: var(--muted);
}

.close:hover {
  background: color-mix(in srgb, currentColor 16%, transparent);
}

.close:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

/* The row's own buttons sit on the frame colour, not the bar. */
.strip > .icon-button {
  color: inherit;
}

.strip > .icon-button:hover:not(:disabled) {
  background: color-mix(in srgb, var(--on-accent) 16%, transparent);
}

.arrow {
  width: 20px;
  margin: 0 4px;
}

.new-tab {
  margin-left: 4px;
}
</style>
