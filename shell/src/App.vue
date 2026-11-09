<script setup>
import AddressBar from './AddressBar.vue';
import PageView from './PageView.vue';
</script>

<template>
  <div class="app">
    <AddressBar />
    <PageView />
  </div>
</template>

<style>
:root {
  --bg: #ffffff;
  --bg-hover: #eff6ff;
  --text: #1e293b;
  --muted: #64748b;
  --line: #e2e8f0;
  --accent: #2563eb;
  --danger: #dc2626;
  font: 14px/1.4 system-ui, -apple-system, 'Segoe UI', sans-serif;
  color: var(--text);
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg: #1e293b;
    --bg-hover: #334155;
    --text: #f1f5f9;
    --muted: #94a3b8;
    --line: #334155;
    --accent: #60a5fa;
    --danger: #f87171;
  }
}

.app {
  display: flex;
  flex-direction: column;
  height: 100%;
}
</style>
