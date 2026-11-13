<script setup>
import { ref } from 'vue';
import { connect, controlSpace, send, setFrame, state, tabLabel } from '@/sdk/vue.js';
import { ExtensionsToolbar } from '@extensions';

setFrame(4);
const address = ref(null);
connect({ focusAddressBar: () => address.value.focus() });
</script>

<template>
  <header v-shell v-drag :style="{ paddingLeft: controlSpace.left }">
    <button
      v-for="tab in state.tabs" :key="tab.id"
      role="tab" :data-tab="tab.id" :aria-selected="tab.id === state.active"
      @click="send({ t: 'activateTab', id: tab.id })"
    >{{ tabLabel(tab) }}</button>
    <button aria-label="New Tab" @click="send({ t: 'newTab' })">+</button>
    <input ref="address" aria-label="Address"
      @keydown.enter="send({ t: 'navigate', text: $event.target.value })">
    <component :is="ExtensionsToolbar" v-if="ExtensionsToolbar" :extensions="state.extensions" />
    <button @click="send({ t: 'run', text: 'app.settings' })">Settings</button>
  </header>
  <main v-page="state.active"></main>
</template>

<style>
#app { display: flex; flex-direction: column; }
header { display: flex; align-items: center; gap: 4px; height: 44px; padding-right: 8px; background: var(--bar); color: var(--text); }
input { flex: 1; }
main { flex: 1; margin: 0 var(--frame-width) var(--frame-width); }
</style>
