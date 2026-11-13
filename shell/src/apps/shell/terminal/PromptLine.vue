<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { activeTab, focusPage, send, state } from '@/kit/shell.js';
import { prompt } from './prompt.js';

// The foot of the window: back, forward and reload for the active pane, then its address
// after a prompt mark. Typing asks Go for suggestions, listed above the line.
const input = ref(null);
const focused = ref(false);
const text = ref('');
const typed = ref('');
const items = ref([]);
const highlight = ref(-1);
let latest = 0;

const open = computed(() => focused.value && items.value.length > 0);

function act(t) {
  send({ t });
  focusPage();
}

function close() {
  items.value = [];
  highlight.value = -1;
  latest++;
}

function onInput() {
  typed.value = text.value;
  highlight.value = -1;
  if (text.value.trim() === '') {
    close();
    return;
  }
  send({ t: 'suggest', id: ++latest, text: text.value });
}

// Up and Down show each suggestion in the line in turn; past either end is what was typed.
function move(delta) {
  const next = highlight.value + delta;
  if (next < 0 || next >= items.value.length) {
    highlight.value = -1;
    text.value = typed.value;
    return;
  }
  highlight.value = next;
  text.value = items.value[next].url;
}

function onKeydown(e) {
  if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && open.value) {
    e.preventDefault();
    move(e.key === 'ArrowDown' ? 1 : -1);
  } else if (e.key === 'Escape') {
    e.preventDefault();
    text.value = activeTab.value.url;
    input.value.blur();
    focusPage();
  }
}

function go(target) {
  const value = (target ?? text.value).trim();
  if (value === '') return;
  text.value = value;
  close();
  send({ t: 'navigate', text: value });
  input.value.blur();
  focusPage();
}

function onFocus() {
  focused.value = true;
  typed.value = text.value;
  nextTick(() => input.value.select());
}

function onBlur() {
  focused.value = false;
  close();
  text.value = activeTab.value.url;
}

onMounted(() => {
  prompt.suggestions = (id, list) => {
    if (id === latest) {
      items.value = list;
      highlight.value = -1;
    }
  };
  prompt.focus = () => input.value.focus();
  // The shell has to hold the keyboard before its field can.
  prompt.activated = () => {
    close();
    text.value = activeTab.value.url;
    if (activeTab.value.url === '') {
      send({ t: 'focusShell' });
      input.value.focus();
    } else {
      input.value.blur();
      focusPage();
    }
  };
});

watch(
  () => activeTab.value.url,
  (url) => {
    if (!focused.value) text.value = url;
  },
);
</script>

<template>
  <footer v-shell class="line">
    <button class="tool" type="button" aria-label="Back" :disabled="!activeTab.canGoBack" @click="act('back')">
      <svg width="12" height="10" viewBox="0 0 14 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M5.93.81.81 5.93l5.12 5.13M1.23 5.93H13.2"/></svg>
    </button>
    <button class="tool" type="button" aria-label="Forward" :disabled="!activeTab.canGoForward" @click="act('forward')">
      <svg width="12" height="10" viewBox="0 0 14 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m8.07 11.06 5.12-5.13L8.07.81m4.7 5.13H.8"/></svg>
    </button>
    <button class="tool" type="button" aria-label="Reload" :disabled="!activeTab.url" @click="act('reload')">
      <svg width="12" height="12" viewBox="0 0 15 15" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="m12.35 1.4.24 4.28-3.78.22"/><path d="M11.68 9.55a5.15 5.15 0 1 1 .19-4.39"/></svg>
    </button>
    <form class="field" @submit.prevent="go()">
      <span class="mark" aria-hidden="true">&gt;</span>
      <input
        ref="input"
        v-model="text"
        aria-label="Address"
        role="combobox"
        :aria-expanded="open"
        aria-controls="suggestions"
        :aria-activedescendant="highlight >= 0 ? `suggestion-${highlight}` : undefined"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        @input="onInput"
        @keydown="onKeydown"
        @focus="onFocus"
        @blur="onBlur"
      />
      <ul v-if="open" id="suggestions" v-shell class="list" role="listbox" @mousedown.prevent>
        <li
          v-for="(item, i) in items"
          :id="`suggestion-${i}`"
          :key="item.url"
          role="option"
          :aria-selected="i === highlight"
          :class="{ active: i === highlight }"
          @mouseenter="highlight = i"
          @click="go(item.url)"
        >
          <span class="title">{{ item.title || item.url }}</span>
          <span v-if="item.title" class="url">{{ item.url }}</span>
        </li>
      </ul>
    </form>
    <span v-if="state.update.phase === 'ready'" class="status">
      {{ state.update.version }} ready
      <button class="relaunch" type="button" @click="send({ t: 'relaunch' })">Relaunch</button>
    </span>
  </footer>
</template>

<style scoped>
/* One row on the bar, under a hairline. */
.line {
  display: flex;
  align-items: center;
  flex: none;
  gap: 4px;
  height: 28px;
  padding: 0 4px;
  box-sizing: border-box;
  border-top: 1px solid var(--line);
  background: var(--bar);
  color: var(--text);
}

.tool {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--icon);
  cursor: pointer;
}

.tool:hover:not(:disabled) {
  background: var(--accent-tint);
}

.tool:disabled {
  opacity: 0.4;
  cursor: default;
}

.tool:focus-visible,
.relaunch:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.field {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  height: 20px;
  margin: 0 0 0 4px;
}

.mark {
  flex: none;
  width: 16px;
  color: var(--icon);
  font-weight: 700;
}

input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0;
  border: 0;
  outline: none;
  background: transparent;
  color: inherit;
  font: inherit;
  caret-color: var(--accent);
}

/* Floating, so it has the shadow, square as the rest of this shell. */
.list {
  position: absolute;
  bottom: calc(100% + 8px);
  left: 0;
  right: 0;
  z-index: 10;
  margin: 0;
  padding: 4px 0;
  list-style: none;
  background: var(--surface);
  border: 1px solid var(--line);
  box-shadow: var(--shadow-float);
}

li {
  display: flex;
  align-items: center;
  gap: 16px;
  height: 24px;
  padding: 0 12px 0 16px;
  cursor: pointer;
}

li.active {
  background: var(--accent-tint);
}

.title,
.url {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.title {
  flex: none;
  max-width: 60%;
}

.url {
  flex: 1;
  min-width: 0;
  color: var(--muted);
}

.status {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 4px;
  color: var(--muted);
}

.relaunch {
  height: 20px;
  padding: 0 8px;
  border: 1px solid var(--line);
  border-radius: 4px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  cursor: pointer;
}

.relaunch:hover {
  background: var(--accent-tint);
}
</style>
