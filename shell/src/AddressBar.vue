<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { send } from './bridge.js';
import { activeTab, setHandlers, state } from './state.js';

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
        send({ t: 'focusPage' });
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
  send({ t: 'focusPage' });
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
  setHandlers({
    suggestions(id, list) {
      if (id === latestQuery) {
        items.value = list;
        highlight.value = -1;
      }
    },
    focusAddressBar: () => input.value.focus(),
  });
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
      send({ t: 'focusPage' });
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
        <span v-if="item.title" class="url">{{ item.url }}</span>
      </li>
    </ul>
  </form>
</template>

<style scoped>
/* The same height as the icon buttons beside it, so one line of controls runs through
   the row with 4px of bar above and below. The Ulaa design's omnibox has no border: the
   white fill on the tinted bar is the edge. The focus ring stays inside the bar. */
.field {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  height: 24px;
  margin: 0;
  border-radius: 6px;
  background: var(--surface);
  box-sizing: border-box;
}

.field:focus-within {
  box-shadow: 0 0 0 1px var(--accent), 0 0 0 3px var(--accent-soft);
}

input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0 10px;
  border: 0;
  border-radius: 6px;
  outline: none;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 12.5px;
}

input::placeholder {
  color: var(--muted);
  opacity: 1;
}

.status {
  flex: none;
  margin-right: 8px;
  color: var(--danger);
  font-size: 12.5px;
  font-weight: 500;
}

.list {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  z-index: 10;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);
}

li {
  display: flex;
  flex-direction: column;
  padding: 4px 8px;
  border-radius: 4px;
  cursor: pointer;
}

li.active {
  background: rgba(15, 170, 170, 0.1);
}

.title,
.url {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.url {
  color: var(--muted);
  font-size: 12.5px;
}
</style>
