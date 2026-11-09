<script setup>
import { computed, watchEffect } from 'vue';
import PageView from './PageView.vue';
import Sidebar from './Sidebar.vue';
import TabStrip from './TabStrip.vue';
import TopBar from './TopBar.vue';
import { state } from './state.js';

// Both layouts show the same tabs from the same state; only where they sit differs.
const horizontal = computed(() => state.tabLayout === 'horizontal');

// The window's own corner radius, as the engine reports it, becomes one CSS variable on
// the root. Whatever meets the window's curve derives its radius from it.
watchEffect(() => {
  document.documentElement.style.setProperty('--window-radius', state.cornerRadius + 'px');
});
</script>

<!-- Every component the shell tracks starts as frame colour and draws its own surface on
     top; the frame is what shows where nothing was drawn. The page is a plain rectangle
     between them, with the left, right and bottom edges as plain strips of their own. -->
<template>
  <div class="app">
    <TabStrip v-if="horizontal" />
    <TopBar />
    <div class="body">
      <div v-drag class="edge left"></div>
      <Sidebar v-if="!horizontal" />
      <PageView />
      <div v-drag class="edge right"></div>
    </div>
    <div v-drag class="edge bottom"></div>
    <!-- macOS rounds the window, and the page underneath is square: this border, with the
         window's own radius, is the frame's curve over the corners. Nothing else. -->
    <div class="rounding" aria-hidden="true"></div>
  </div>
</template>

<style>
/* Colours from the Ulaa Browser design, Personal mode. Spacing, radii, type sizes and the
   flat look follow the Deb Design System: a 4px grid, borders instead of shadows. */
:root {
  --frame: #0faaaa;
  --frame-width: 4px;
  --bar: #e2f2f2;
  --accent: #0faaaa;
  /* Icons and the sidebar's New Tab take the design's deeper ink for this mode, so the
     frame is the one thing in the full accent. */
  --icon: #0b8f8f;
  --hover: rgba(255, 255, 255, 0.6);
  --surface: #ffffff;
  --text: #1f1f1f;
  --muted: #5c6270;
  --line: rgba(15, 20, 25, 0.1);
  --accent-soft: rgba(15, 170, 170, 0.18);
  --accent-tint: rgba(15, 170, 170, 0.1);
  --danger: #ea3323;
  font: 14px/1.4 'Zoho Puvi', 'Zoho_Puvi', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif;
  color: var(--text);
  -webkit-font-smoothing: antialiased;
}

/* The root has no background: the page shows through wherever nothing is painted. */
.app {
  position: relative;
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

.edge {
  flex: none;
  background: var(--frame);
}

.edge.left,
.edge.right {
  width: var(--frame-width);
}

.edge.bottom {
  height: var(--frame-width);
}

.rounding {
  position: absolute;
  inset: 0;
  border: var(--frame-width) solid var(--frame);
  border-radius: var(--window-radius, 0px);
  pointer-events: none;
}

/* Every control in the shell is a flat 28px square with a 6px radius. */
.icon-button {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--icon);
  cursor: pointer;
}

.icon-button:hover:not(:disabled) {
  background: var(--accent-tint);
}

.icon-button:disabled {
  opacity: 0.4;
  cursor: default;
}

.icon-button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1px var(--accent), 0 0 0 3px var(--accent-soft);
}
</style>
