<script setup>
import { computed } from 'vue';
import PageView from './PageView.vue';
import Sidebar from './Sidebar.vue';
import TabStrip from './TabStrip.vue';
import TopBar from './TopBar.vue';
import { state } from './state.js';

// Both layouts show the same tabs from the same state; only where they sit differs.
const horizontal = computed(() => state.tabLayout === 'horizontal');
</script>

<template>
  <div class="app">
    <TabStrip v-if="horizontal" />
    <TopBar />
    <div class="body">
      <Sidebar v-if="!horizontal" />
      <PageView :class="{ 'no-sidebar': horizontal || !state.sidebar }" />
    </div>
  </div>
</template>

<style>
/* Colours from the Ulaa Browser design, Personal mode. Spacing, radii, type sizes and the
   flat look follow the Deb Design System: a 4px grid, borders instead of shadows. */
:root {
  --frame: #0faaaa;
  --bar: #e2f2f2;
  --accent: #0faaaa;
  --hover: rgba(255, 255, 255, 0.6);
  --surface: #ffffff;
  --text: #1f1f1f;
  --muted: #5c6270;
  --line: rgba(15, 20, 25, 0.1);
  --accent-soft: rgba(15, 170, 170, 0.18);
  --danger: #ea3323;
  font: 13.5px/1.4 'Zoho Puvi', 'Zoho_Puvi', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif;
  color: var(--text);
  -webkit-font-smoothing: antialiased;
}

/* The root stays transparent: the page shows through wherever nothing is painted. */
.app {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.body {
  display: flex;
  flex: 1;
  min-height: 0;
}

/* Every control in the shell is a flat 24px square with a 6px radius. */
.icon-button {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--accent);
  cursor: pointer;
}

.icon-button:hover:not(:disabled) {
  background: rgba(0, 0, 0, 0.06);
}

.icon-button:disabled {
  opacity: 0.5;
  cursor: default;
}

.icon-button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1px var(--accent), 0 0 0 3px var(--accent-soft);
}
</style>
