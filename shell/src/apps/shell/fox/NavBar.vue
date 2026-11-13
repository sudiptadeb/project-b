<script setup>
import AddressField from './AddressField.vue';
import AppMenu from './AppMenu.vue';
import { withKey } from './hints.js';
import { ExtensionsToolbar } from '@extensions';
import { activeTab, focusPage, send, state } from '@/sdk/vue.js';

// Back, forward and reload on the left, the address field centred in the row, the
// extensions and the menu on the right. Reload is Stop while the page loads.
function act(msg) {
  send(msg);
  focusPage();
}
</script>

<template>
  <div v-shell class="nav">
    <div class="bar">
      <div class="side start">
        <button class="icon-button" type="button" aria-label="Back" :title="withKey('Back', 'nav.back')" :disabled="!activeTab.canGoBack" @click="act({ t: 'back' })">
          <svg width="14" height="12" viewBox="0 0 14 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M5.93.81.81 5.93l5.12 5.13M1.23 5.93H13.2" /></svg>
        </button>
        <button class="icon-button" type="button" aria-label="Forward" :title="withKey('Forward', 'nav.forward')" :disabled="!activeTab.canGoForward" @click="act({ t: 'forward' })">
          <svg width="14" height="12" viewBox="0 0 14 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m8.07 11.06 5.12-5.13L8.07.81m4.7 5.13H.8" /></svg>
        </button>
        <button v-if="activeTab.loading" class="icon-button" type="button" aria-label="Stop" :title="withKey('Stop', 'nav.stop')" @click="act({ t: 'run', text: 'nav.stop' })">
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M10.5 1.5l-9 9M1.5 1.5l9 9" /></svg>
        </button>
        <button v-else class="icon-button" type="button" aria-label="Reload" :title="withKey('Reload', 'nav.reload')" :disabled="!activeTab.url" @click="act({ t: 'reload' })">
          <svg width="15" height="15" viewBox="0 0 15 15" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m12.35 1.4.24 4.28-3.78.22" /><path d="M11.68 9.55a5.15 5.15 0 1 1 .19-4.39" /></svg>
        </button>
      </div>
      <AddressField />
      <div class="side end">
        <component :is="ExtensionsToolbar" v-if="ExtensionsToolbar" :extensions="state.extensions" />
        <AppMenu />
      </div>
      <!-- The active page's load progress, along the bar's foot while it loads. -->
      <div class="progress" :class="{ on: activeTab.loading }" :style="{ width: (activeTab.progress || 0) * 100 + '%' }"></div>
    </div>
  </div>
</template>

<style scoped>
/* The bar tone between the frame's side bands, flat. Above the page and its notices, so
   the floating menus it opens lie over them. */
.nav {
  position: relative;
  z-index: 1;
  flex: none;
  padding: 0 var(--frame-width);
  background: var(--frame);
}

/* Three columns: the outer two share what the field leaves, so the field sits in the
   row's middle, as wide as it may be. */
.bar {
  position: relative;
  display: grid;
  grid-template-columns: 1fr minmax(0, 800px) 1fr;
  align-items: center;
  column-gap: 12px;
  height: 40px;
  padding: 0 8px;
  background: var(--bar);
  border-bottom: 1px solid var(--line);
  box-sizing: border-box;
}

.side {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.side.end {
  justify-content: flex-end;
}

.progress {
  position: absolute;
  left: 0;
  bottom: -1px;
  height: 2px;
  background: var(--accent);
  opacity: 0;
  transition: width 0.15s ease-out;
}

.progress.on {
  opacity: 1;
}
</style>
