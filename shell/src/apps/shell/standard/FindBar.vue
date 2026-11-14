<script setup>
import { computed, nextTick, onUnmounted, ref, watch } from 'vue';
import { pushEscape } from '@/ui/escape.js';
import { closeFind, findNext, findText, focusPage, keyLabel, send, state } from '@/sdk/vue.js';
import { find } from './state.js';

// Find in page (docs/daily-driver.md): a bar hung from the page's top right while the active
// tab's search is open, each tab keeping its own. What is typed searches as it changes, with
// the count at the field's end; Enter shows the next match and Shift+Enter the one before;
// Escape closes it and gives the page the keyboard back.
const field = ref(null);
const text = ref('');
let pop = null;

const open = computed(() => state.find && state.find.tab === state.active);
const tab = () => state.find?.tab;
const missed = computed(() => !!state.find?.text && !state.find.matches);
const count = computed(() => {
  const f = state.find;
  if (!f || !f.text) return '';
  return f.matches ? `${f.active} of ${f.matches}` : 'No results';
});
// A button's tooltip, with the key the keymap gives its action, when it has one.
const tip = (title, action) => (keyLabel(action) ? `${title} (${keyLabel(action)})` : title);

// The field follows Go's text when another tab's search shows, not while typing.
watch(
  () => [state.find?.tab, open.value],
  () => {
    if (state.find) text.value = state.find.text;
    pop?.();
    pop = open.value ? pushEscape(close) : null;
  },
  { immediate: true },
);
onUnmounted(() => pop?.());

async function focus() {
  send({ t: 'focusShell' });
  await nextTick();
  field.value?.focus();
  field.value?.select();
}
find.focus = focus;

function close() {
  closeFind(tab());
  focusPage();
}
</script>

<template>
  <div v-if="open" v-shell class="find-bar" role="search" aria-label="Find in page">
    <div class="box" :class="{ missed }">
      <input
        ref="field"
        v-model="text"
        class="text-field"
        type="text"
        aria-label="Find in page"
        placeholder="Find in page"
        spellcheck="false"
        @input="findText(tab(), text)"
        @keydown.enter.exact.prevent="findNext(tab(), true)"
        @keydown.shift.enter.prevent="findNext(tab(), false)"
      />
      <span class="count" role="status">{{ count }}</span>
    </div>
    <button type="button" class="icon-button" aria-label="Previous match" :title="tip('Previous', 'find.previous')" :disabled="!state.find.matches" @click="findNext(tab(), false)">
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M4 10l4-4 4 4" /></svg>
    </button>
    <button type="button" class="icon-button" aria-label="Next match" :title="tip('Next', 'find.next')" :disabled="!state.find.matches" @click="findNext(tab(), true)">
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6l4 4 4-4" /></svg>
    </button>
    <button type="button" class="icon-button" aria-label="Close find bar" title="Close (Esc)" @click="close">
      <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="m2 2 6 6M8 2 2 8" /></svg>
    </button>
  </div>
</template>

<style scoped>
/* The design system's floating surface, hung from the page's top right edge. */
.find-bar {
  position: absolute;
  top: 0;
  right: 16px;
  z-index: 4;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 6px 6px 6px 8px;
  border: 1px solid var(--line);
  border-top: 0;
  border-radius: 0 0 8px 8px;
  background: var(--surface);
  box-shadow: var(--shadow-float);
  color: var(--text);
}

/* The count sits at the field's own right end. */
.box {
  position: relative;
  margin-right: 4px;
}

.text-field {
  width: 260px;
  height: 28px;
  padding-right: 84px;
  box-sizing: border-box;
}

.count {
  position: absolute;
  top: 50%;
  right: 10px;
  transform: translateY(-50%);
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
  pointer-events: none;
}

/* No match: the design system's error state, so a miss shows without reading. */
.missed .text-field {
  border-color: var(--danger);
}

.missed .count {
  color: var(--danger);
}
</style>
