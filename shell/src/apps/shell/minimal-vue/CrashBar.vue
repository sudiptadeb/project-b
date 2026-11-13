<script setup>
import { send, state } from '@/sdk/vue.js';

// Says the last run crashed and where its log is, in one line, until it is dismissed.
</script>

<template>
  <div v-if="state.crashLog" v-shell class="crash" role="status">
    <span class="text">Project B crashed last time. The log is at {{ state.crashLog }}</span>
    <button @click="send({ t: 'dismissCrash' })">Dismiss</button>
  </div>
</template>

<style scoped>
.crash {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  padding: 0 6px 0 12px;
  background: var(--bar);
  border-bottom: 1px solid var(--line);
}

.text {
  flex: 1;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

button {
  flex: none;
  height: 24px;
  padding: 0 12px;
  border: 1px solid var(--line);
  background: none;
}

button:hover {
  background: var(--hover);
}
</style>
