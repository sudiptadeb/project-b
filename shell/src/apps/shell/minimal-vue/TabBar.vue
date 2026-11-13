<script setup>
import { ref } from 'vue';
import { controlSpace, send, state } from '@/sdk/vue.js';
import Tab from './Tab.vue';
import { keepActiveInView } from './inView.js';

// The tabs on top: the window buttons' space, the tabs, New Tab, then empty space that
// drags the window. A tab is 200px wide and shrinks to 96px when many are open; past that
// the tabs scroll sideways, the active one kept in view, while New Tab and the drag space
// stay put. The tabs scroll, so the bar is the shell's and its two spaces drag.
const tabs = ref(null);
keepActiveInView(tabs, true);

// A mouse wheel turns the tabs sideways; a trackpad's sideways swipe already does.
function wheel(e) {
  if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
    e.preventDefault();
    tabs.value.scrollLeft += e.deltaY;
  }
}
</script>

<template>
  <div v-shell class="tabbar">
    <div v-drag class="lights" :style="{ width: controlSpace.left }"></div>
    <div ref="tabs" class="tabs" role="tablist" @wheel="wheel">
      <Tab v-for="tab in state.tabs" :key="tab.id" :tab="tab" />
    </div>
    <button class="new" aria-label="New Tab" @click="send({ t: 'newTab' })">
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2v12M2 8h12" /></svg>
    </button>
    <div v-drag class="drag"></div>
  </div>
</template>

<style scoped>
.tabbar {
  display: flex;
  height: 34px;
  background: var(--tabbar);
  border-bottom: 1px solid var(--line);
}

.lights {
  flex: none;
}

/* The tabs reach over the bar's hairline, so the active one joins the toolbar. */
.tabs {
  position: relative;
  display: flex;
  min-width: 0;
  margin-bottom: -1px;
  overflow-x: auto;
  scrollbar-width: none;
  border-left: 1px solid var(--line);
}

/* A width, not a flex basis, so the tabs ask for 200px each and shrink only when they
   don't fit. */
.tab {
  width: 200px;
  min-width: 96px;
  border-right: 1px solid var(--line);
}

.new {
  flex: none;
  display: grid;
  place-items: center;
  width: 34px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--icon);
}

.new svg {
  width: 12px;
  height: 12px;
}

.new:hover {
  background: var(--hover);
}

.drag {
  flex: 1;
  min-width: 48px;
}
</style>
