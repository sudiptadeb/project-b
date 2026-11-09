<script setup>
import { onMounted, ref } from 'vue';
import { setPageElement } from './regions.js';
import { state } from './state.js';

const el = ref(null);
onMounted(() => setPageElement(el.value));
</script>

<template>
  <div ref="el" class="page-view">
    <iframe v-if="state.engine === 'mock'" src="/mock/page" title="Mock page"></iframe>
  </div>
</template>

<style scoped>
.page-view {
  flex: 1;
  min-height: 0;
}

iframe {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
}
</style>
