<script setup>
import { state } from './state.js';
</script>

<template>
  <div class="tabstrip">
    <div class="tab">
      <svg class="shape" width="178" height="32.5" viewBox="0 0 179 33" aria-hidden="true">
        <path d="M0.03 31.67C1.08 31.57 2.06 31.11 2.8 30.37C3.54 29.62 3.99 28.64 4.08 27.6V9.6C4.06 7.07 5.05 4.64 6.82 2.84C8.6 1.04 11.01.01 13.54 0H164.46C166.98.01 169.4 1.04 171.18 2.84C172.95 4.64 173.94 7.07 173.92 9.6V27.6C174.03 28.63 174.5 29.59 175.25 30.3C176.01 31.02 176.99 31.44 178.03 31.5C178.01 31.47 153.92 32.49 76.23 32.49C1.57 32.49-.38 31.42.03 31.67Z" />
      </svg>
      <span class="title">{{ state.tab.title || 'New Tab' }}</span>
    </div>
  </div>
</template>

<style scoped>
/* The window buttons sit in this strip, to the left of the tab. */
.tabstrip {
  position: relative;
  height: 42px;
}

.tab {
  position: absolute;
  left: 92px;
  top: 10.8px;
  width: 178px;
  height: 32.5px;
  color: var(--text);
}

.shape {
  position: absolute;
  inset: 0;
  fill: var(--bar);
}

.title {
  position: absolute;
  left: 16px;
  right: 16px;
  top: 8px;
  overflow: hidden;
  font-size: 12px;
  font-weight: 500;
  line-height: 15px;
  white-space: nowrap;
  text-overflow: ellipsis;
}
</style>
