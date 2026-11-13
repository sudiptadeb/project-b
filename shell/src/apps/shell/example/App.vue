<script setup>
import { ref } from 'vue';
import { connect, controlSpace, send, state, tabLabel } from '@/kit/shell.js';
import { ExtensionsToolbar } from '@extensions';
import { useTheme } from '@/theme.js';

useTheme(state);
const address = ref(null);
connect({ focusAddressBar: () => address.value.focus() });
</script>

<template>
  <header v-shell>
    <div v-drag class="space" :style="{ width: controlSpace.left }"></div>
    <button
      v-for="tab in state.tabs" :key="tab.id"
      role="tab" :data-tab="tab.id" :aria-selected="tab.id === state.active"
      @click="send({ t: 'activateTab', id: tab.id })"
    >{{ tabLabel(tab) }}</button>
    <button aria-label="New Tab" @click="send({ t: 'newTab' })">+</button>
    <input ref="address" aria-label="Address"
      @keydown.enter="send({ t: 'navigate', text: $event.target.value })">
    <div v-drag class="space rest"></div>
    <component :is="ExtensionsToolbar" v-if="ExtensionsToolbar" />
  </header>
  <main v-page="state.active" :style="{ margin: `0 ${state.frameWidth}px ${state.frameWidth}px` }"></main>
</template>

<style>
#app { display: flex; flex-direction: column; }
header { display: flex; align-items: center; gap: 4px; height: 44px; padding-right: 8px; background: var(--bar); color: var(--text); }
.space { flex: none; align-self: stretch; }
.rest { flex: 1; }
main { flex: 1; }
</style>
