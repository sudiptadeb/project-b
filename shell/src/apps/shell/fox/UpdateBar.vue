<script setup>
import { computed } from 'vue';
import { send, state } from '@/kit/shell.js';

// What the updater is doing, in a bar under the navigation bar while something is
// happening: a download and how far it is, a release ready to relaunch into, or why one
// failed.
const percent = computed(() => {
  const u = state.update;
  return u.total ? Math.floor((100 * (u.done || 0)) / u.total) : 0;
});
</script>

<template>
  <div v-if="state.update.phase" v-shell class="notice">
    <div class="bar" :class="state.update.phase">
      <template v-if="state.update.phase === 'downloading'">
        <span class="text" role="status">Downloading Project B {{ state.update.version }}… {{ percent }}%</span>
        <span
          class="progress"
          role="progressbar"
          aria-label="Update download"
          :aria-valuenow="percent"
          aria-valuemin="0"
          aria-valuemax="100"
          :style="{ width: percent + '%' }"
        ></span>
      </template>
      <template v-else-if="state.update.phase === 'ready'">
        <span class="text" role="status">Project B {{ state.update.version }} is ready.</span>
        <button class="text-button" type="button" @click="send({ t: 'relaunch' })">Relaunch to update</button>
      </template>
      <span v-else class="text" role="status" :title="state.update.error">Update{{ state.update.version ? ' to ' + state.update.version : '' }} failed: {{ state.update.error }}</span>
    </div>
  </div>
</template>

<style scoped>
.notice {
  flex: none;
  padding: 0 var(--frame-width);
  background: var(--frame);
}

.bar {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 40px;
  padding: 4px 8px 4px 12px;
  box-sizing: border-box;
  background: var(--bar);
  border-bottom: 1px solid var(--line);
  font-size: 13px;
}

.text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.bar.failed {
  color: var(--danger);
}

/* The download's progress: a line along the bar's foot, in the accent. */
.progress {
  position: absolute;
  left: 0;
  bottom: -1px;
  height: 2px;
  background: var(--accent);
}
</style>
