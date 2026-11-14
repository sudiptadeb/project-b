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
// The groups, worst first; a status not named here comes after these.
const order = ['quiet', 'partly', 'off', 'absent', 'works'];
const rank = (s) => (order.includes(s) ? order.indexOf(s) : order.length);

// The filter keeps the features whose name, reason or status has what was typed.
const filter = ref('');
const groups = computed(() => {
  const q = filter.value.trim().toLowerCase();
  const hit = (f) => !q || [f.name, f.why, statusText[f.status]].some((s) => s.toLowerCase().includes(q));
  const byStatus = new Map();
  for (const f of state.webFeatures.filter(hit)) {
    if (!byStatus.has(f.status)) byStatus.set(f.status, []);
    byStatus.get(f.status).push(f);
  }
  return [...byStatus.entries()]
    .sort(([a], [b]) => rank(a) - rank(b))
    .map(([status, rows]) => ({ status, title: `${statusText[status]} (${rows.length})`, rows }));
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
    <p class="intro">How web features behave in Project B, and why. Read-only.</p>
    <section class="filter-section" aria-label="Filter">
      <input
        v-model="filter"
        class="text-field filter"
        type="text"
        spellcheck="false"
        aria-label="Filter web features"
        placeholder="Filter by name, reason or status"
        @keydown.esc="onFilterEscape"
      />
    </section>
    <section v-if="!groups.length" aria-label="No matches">
      <div class="card">
        <div class="row empty">No features match</div>
      </div>
    </section>
    <section v-for="g in groups" :key="g.status" :aria-label="g.title">
      <h2>{{ g.title }}</h2>
      <div class="card">
        <div v-for="f in g.rows" :key="f.id" class="row">
          <span class="label">
            <span class="name" :title="f.api">{{ f.name }}</span>
            <span class="secondary why">{{ f.why }}</span>
          </span>
          <span class="tag" :class="{ quiet: f.status === 'quiet' }">{{ statusText[f.status] }}</span>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.intro {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 13px;
}

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

.why {
  overflow-wrap: anywhere;
}

/* Only a feature that fails without saying so is marked; every other tag stays neutral. */
.tag.quiet {
  box-shadow: inset 0 0 0 1px var(--danger);
  color: var(--danger);
}
</style>
