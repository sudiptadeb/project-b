<script setup>
import { activeTab, focusPage, send } from '@/sdk/vue.js';

function act(t) {
  send({ t });
  focusPage();
}
</script>

<!-- The glyphs are the Ulaa design's: one stroke weight across the toolbar. -->
<template>
  <div class="nav">
    <button class="icon-button" type="button" aria-label="Back" :disabled="!activeTab.canGoBack" @click="act('back')">
      <svg width="14" height="12" viewBox="0 0 14 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M5.93.81.81 5.93l5.12 5.13M1.23 5.93H13.2"/></svg>
    </button>
    <button class="icon-button" type="button" aria-label="Forward" :disabled="!activeTab.canGoForward" @click="act('forward')">
      <svg width="14" height="12" viewBox="0 0 14 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m8.07 11.06 5.12-5.13L8.07.81m4.7 5.13H.8"/></svg>
    </button>
    <button class="icon-button" type="button" aria-label="Reload" :disabled="!activeTab.url" @click="act('reload')">
      <svg width="15" height="15" viewBox="0 0 15 15" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m12.35 1.4.24 4.28-3.78.22"/><path d="M11.68 9.55a5.15 5.15 0 1 1 .19-4.39"/></svg>
    </button>
  </div>
</template>

<style scoped>
.nav {
  display: flex;
  flex: none;
  gap: 4px;
  margin: 0 12px 0 8px;
}
</style>
