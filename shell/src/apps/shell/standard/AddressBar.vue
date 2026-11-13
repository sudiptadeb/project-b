<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { activeTab, focusPage, send, state } from '@/kit/shell.js';
import { addressBar } from './state.js';

const input = ref(null);
const focused = ref(false);
const text = ref('');
const typed = ref('');
const items = ref([]);
const highlight = ref(-1);

let latestQuery = 0;

const open = computed(() => focused.value && items.value.length > 0);
const status = computed(() => {
  const t = activeTab.value;
  if (t.blocked) return 'Blocked';
  if (t.crashed) return 'Crashed';
  if (t.error) return 'Failed';
  return '';
});

function close() {
  items.value = [];
  highlight.value = -1;
  latestQuery++;
}

function onInput() {
  typed.value = text.value;
  highlight.value = -1;
  if (text.value.trim() === '') {
    close();
    return;
  }
  send({ t: 'suggest', id: ++latestQuery, text: text.value });
}

// Moving through the list shows that item in the bar. Up from the first item, or down
// from the last, comes back to what the user typed.
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
  selectOnMouseUp = false;
  switch (e.key) {
    case 'ArrowDown':
      if (open.value) {
        e.preventDefault();
        move(1);
      }
      break;
    case 'ArrowUp':
      if (open.value) {
        e.preventDefault();
        move(-1);
      }
      break;
    case 'Escape':
      if (open.value) {
        e.preventDefault();
        text.value = typed.value;
        close();
      } else {
        text.value = activeTab.value.url;
        input.value.blur();
        focusPage();
      }
      break;
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

// Focusing the bar selects everything. When a click did the focusing, its mouse up
// would put the caret back down and clear the selection, so that one mouse up selects
// again. The first key clears this, so later clicks place the caret as usual.
let selectOnMouseUp = false;

function onFocus() {
  focused.value = true;
  typed.value = text.value;
  selectOnMouseUp = true;
  nextTick(() => input.value.select());
}

function onMouseUp(e) {
  if (selectOnMouseUp) {
    e.preventDefault();
    input.value.select();
    selectOnMouseUp = false;
  }
}

function onBlur() {
  focused.value = false;
  close();
  text.value = activeTab.value.url;
}

// The shell page has to hold the keyboard before the input can.
function focusBar() {
  send({ t: 'focusShell' });
  input.value.focus();
}

onMounted(() => {
  addressBar.suggestions = (id, list) => {
    if (id === latestQuery) {
      items.value = list;
      highlight.value = -1;
    }
  };
  addressBar.focus = () => input.value.focus();
});

watch(
  () => activeTab.value.url,
  (url) => {
    if (!focused.value) text.value = url;
  },
);

// A tab that becomes active with nothing in it wants the address bar; one with a page
// wants the page.
watch(
  () => state.active,
  () => {
    if (state.active === 0) return;
    close();
    text.value = activeTab.value.url;
    if (activeTab.value.url === '') {
      focusBar();
    } else {
      input.value.blur();
      focusPage();
    }
  },
);
</script>

<template>
  <form class="field" :class="{ open }" @submit.prevent="go()">
    <input
      ref="input"
      v-model="text"
      aria-label="Address"
      placeholder="Search DuckDuckGo or type a URL"
      role="combobox"
      :aria-expanded="open"
      aria-controls="suggestions"
      :aria-activedescendant="highlight >= 0 ? `suggestion-${highlight}` : undefined"
      autocomplete="off"
      autocapitalize="off"
      spellcheck="false"
      @input="onInput"
      @keydown="onKeydown"
      @mouseup="onMouseUp"
      @focus="onFocus"
      @blur="onBlur"
    />
    <span v-if="status" class="status">{{ status }}</span>
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
        <span v-if="item.title" class="dash" aria-hidden="true"></span>
        <span v-if="item.title" class="url">{{ item.url }}</span>
      </li>
    </ul>
  </form>
</template>

<style scoped>
/* Taller than the icon buttons beside it, as Ulaa's is, with 6px of bar above and below. The Ulaa design's omnibox has no border: the
   white fill on the tinted bar is the edge. The focus ring stays inside the bar. */
.field {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  height: 32px;
  margin: 0;
  border-radius: 6px;
  background: var(--surface);
  box-sizing: border-box;
}

.field:focus-within {
  box-shadow: var(--focus-ring);
}

input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0 12px;
  border: 0;
  border-radius: 6px;
  outline: none;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 13px;
}

input::placeholder {
  color: var(--muted);
  opacity: 1;
}

.status {
  flex: none;
  margin-right: 8px;
  color: var(--danger);
  font-size: 13px;
  font-weight: 500;
}

/* A floating surface, with the shadow the Ulaa design gives its menus. */
.list {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  z-index: 10;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: var(--shadow-float);
}

/* One line per item, the title then the address, the way a tab row in the design reads
   "title – detail". The title keeps its words; the address gives way first. */
li {
  display: flex;
  align-items: center;
  height: 32px;
  padding: 0 10px;
  border-radius: 4px;
  font-size: 13px;
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

.dash {
  flex: none;
  width: 5px;
  height: 1px;
  margin: 0 8px;
  background: var(--muted);
}

/* Fingers need bigger targets than a mouse does. */
.touch .field {
  height: 40px;
}

.touch input {
  font-size: 15px;
}

.touch li {
  height: 44px;
  font-size: 15px;
}
</style>
