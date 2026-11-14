<script setup>
import { computed, ref } from 'vue';
import { state } from './state.js';

// The web-features registry (core/webfeatures/registry.json), as Go sends it for this
// platform: each feature whose browser side is Chrome's, what a page gets of it here, and
// why. It only shows; the registry changes with the code.

const statusText = {
  works: 'Works',
  off: 'Turned off',
  quiet: 'Fails quietly',
  absent: 'Absent',
  partly: 'Partly',
  kills: 'Crashes the page',
  unknown: 'Not checked',
};

// The filter keeps the features whose name, API, status or reason has what was typed.
const filter = ref('');
const rows = computed(() => {
  const q = filter.value.trim().toLowerCase();
  if (!q) return state.webFeatures;
  return state.webFeatures.filter((f) => [f.name, f.api, statusText[f.status], f.why].some((s) => s.toLowerCase().includes(q)));
});

// Escape in the filter empties it, or leaves it when it is empty; never the page.
function onFilterEscape(e) {
  e.preventDefault();
  if (filter.value) filter.value = '';
  else e.target.blur();
}
</script>

<template>
  <div>
    <section class="filter-section" aria-label="Filter">
      <input
        v-model="filter"
        class="text-field filter"
        type="text"
        spellcheck="false"
        aria-label="Filter web features"
        placeholder="Filter by name or status"
        @keydown.esc="onFilterEscape"
      />
    </section>
    <section aria-label="Web features">
      <div class="card">
        <div v-if="!rows.length" class="row empty">No features match</div>
        <div v-for="f in rows" :key="f.id" class="row">
          <span class="label">
            <span class="name" :title="f.api">{{ f.name }}</span>
            <span class="secondary">{{ f.why }}</span>
          </span>
          <span class="tag">{{ statusText[f.status] }}</span>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.filter-section {
  margin-top: 8px;
}

.filter {
  width: 100%;
}

.empty {
  color: var(--muted);
}

.name {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
