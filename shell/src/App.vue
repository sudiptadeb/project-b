<script setup>
import AddressBar from './AddressBar.vue';
import PageView from './PageView.vue';
import TabStrip from './TabStrip.vue';
</script>

<template>
  <div class="app">
    <header v-shell class="top">
      <TabStrip />
      <AddressBar />
    </header>
    <PageView />
  </div>
</template>

<style>
/* Colours from the Ulaa Browser design, Personal mode. Spacing, radii, type sizes and the
   flat look follow the Deb Design System: a 4px grid, borders instead of shadows. */
:root {
  --frame: #0faaaa;
  --bar: #e2f2f2;
  --accent: #0faaaa;
  --hover: #e2f2f2;
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

.app {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.top {
  position: relative;
  z-index: 1;
  flex: none;
}
</style>
