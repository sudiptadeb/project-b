<script setup>
import { ref } from 'vue';
import { send, state } from '@/sdk/vue.js';
import Tab from './Tab.vue';
import { keepActiveInView } from './inView.js';

// The tabs down the side: New Tab pinned at the top, and the list under it, which
// scrolls with the active tab kept in view.
const list = ref(null);
keepActiveInView(list, false);
</script>

<template>
  <div v-shell class="side">
    <button class="new" @click="send({ t: 'newTab' })">
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2v12M2 8h12" /></svg>
      New Tab
    </button>
    <div ref="list" class="list" role="tablist" aria-orientation="vertical">
      <Tab v-for="tab in state.tabs" :key="tab.id" :tab="tab" />
    </div>
  </div>
</template>

<style scoped>
.side {
  display: flex;
  flex-direction: column;
  width: 220px;
  background: var(--tabbar);
  border-right: 1px solid var(--line);
}

.new {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  box-sizing: content-box;
  padding: 0 12px;
  border: 0;
  border-bottom: 1px solid var(--line);
  background: none;
  color: var(--muted);
}

.new svg {
  width: 12px;
  height: 12px;
  color: var(--icon);
}

.new:hover {
  background: var(--hover);
}

.list {
  position: relative;
  flex: 1;
  overflow-y: auto;
  scrollbar-width: none;
}

.tab {
  height: 32px;
  border-bottom: 1px solid var(--line);
}
</style>
