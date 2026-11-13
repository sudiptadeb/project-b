<script setup>
import { computed, ref } from 'vue';
import { activeTab, connect, focusPage, send, state } from '@/sdk/vue.js';
import CrashBar from './CrashBar.vue';
import SideList from './SideList.vue';
import TabBar from './TabBar.vue';
import Toolbar from './Toolbar.vue';

// The minimal shell, in Vue: a tab bar, a toolbar under it, any crash notice, then the
// page, to the window's edges: it asks for no band. With the tabs down the side the tab
// bar goes, the toolbar is the top row, and the tabs are a list left of the page. The
// same shell is built in Svelte and in plain JS, so the three compare only in how they
// are built.
const toolbar = ref(null);
const vertical = computed(() => state.tabLayout === 'vertical');
// The tab last made active, so each change of it is seen once.
let active = 0;

// A tab that becomes active with nothing in it wants the address; one with a page wants
// the page. The shell has to hold the keyboard before its field can.
function activated() {
  if (state.active === active) return;
  active = state.active;
  if (activeTab.value.url === '') {
    send({ t: 'focusShell' });
    toolbar.value.address.focus();
  } else {
    toolbar.value.address.blur();
    focusPage();
  }
}

connect({
  snapshot: activated,
  tabs: activated,
  focusAddressBar: () => toolbar.value.address.focus(),
  suggestions: (m) => toolbar.value.address.suggestions(m),
});
</script>

<template>
  <div class="window" :class="{ vertical }">
    <TabBar v-if="!vertical" />
    <Toolbar ref="toolbar" :vertical="vertical" />
    <CrashBar />
    <SideList v-if="vertical" />
    <main v-page="state.active" class="page"></main>
  </div>
</template>

<style>
/* The Deb Design System's type, kept in this folder: Space Grotesk, and Space Mono for
   the address. Both are under the SIL Open Font License. */
@font-face {
  font-family: 'Space Grotesk';
  src: url('./fonts/space-grotesk-var.woff2') format('woff2');
  font-weight: 300 700;
}

@font-face {
  font-family: 'Space Mono';
  src: url('./fonts/space-mono-400.woff2') format('woff2');
  font-weight: 400;
}

button,
input {
  font: inherit;
  color: inherit;
}

button:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

/* The toolbar's square buttons, an icon each, also the extensions'. */
.btn {
  flex: none;
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--icon);
}

.btn svg {
  width: 14px;
  height: 14px;
}

.btn:hover:not(:disabled) {
  background: var(--hover);
}

.btn:disabled {
  opacity: 0.4;
}
</style>

<style scoped>
/* Nothing is painted where the page shows: it shows through. */
.window {
  --tabbar: color-mix(in srgb, var(--frame) 12%, var(--bar));
  display: grid;
  grid-template-columns: auto 1fr;
  grid-template-rows: auto auto auto minmax(0, 1fr);
  height: 100%;
  overflow: hidden;
  font: 13px/1 'Space Grotesk', system-ui, sans-serif;
  color: var(--text);
}

.tabbar,
.toolbar,
.crash {
  grid-column: 1 / -1;
}

.side {
  grid-row: 4;
  grid-column: 1;
}

.page {
  grid-row: 4;
  grid-column: 1 / -1;
}

.vertical .page {
  grid-column: 2;
}
</style>
