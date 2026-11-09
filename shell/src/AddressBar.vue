<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { send } from './bridge.js';
import { setHandlers, state } from './state.js';

const input = ref(null);
const focused = ref(false);
const text = ref('');
const typed = ref('');
const items = ref([]);
const highlight = ref(-1);

let latestQuery = 0;

const open = computed(() => focused.value && items.value.length > 0);
const status = computed(() => {
  const t = state.tab;
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
        text.value = state.tab.url;
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
  text.value = state.tab.url;
}

onMounted(() => {
  setHandlers({
    suggestions(id, list) {
      if (id === latestQuery) {
        items.value = list;
        highlight.value = -1;
      }
    },
    focusAddressBar() {
      input.value.focus();
    },
  });
});

watch(
  () => state.tab.url,
  (url) => {
    if (!focused.value) text.value = url;
  },
);
</script>

<template>
  <form class="bar" @submit.prevent="go()">
    <div class="field" :class="{ open }">
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
    </div>
    <button type="submit" @mousedown.prevent>Go</button>
    <div class="progress" :class="{ on: state.tab.loading }"></div>
  </form>
</template>

<style scoped>
.bar {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  height: 59px;
  margin: 0 10px;
  padding: 0 12px;
  background: var(--bar);
  border-radius: 16.8px 16.8px 0 0;
}

.field {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  height: 35px;
  border-radius: 6px;
  background: var(--surface);
}

.field:focus-within {
  box-shadow: 0 0 0 2px var(--accent);
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
  font-weight: 500;
}

input::placeholder {
  color: inherit;
  opacity: 1;
}

.status {
  flex: none;
  margin-right: 10px;
  color: var(--danger);
  font-size: 12px;
  font-weight: 600;
}

button {
  flex: none;
  height: 35px;
  min-width: 64px;
  padding: 0 16px;
  border: 0;
  border-radius: 6px;
  background: var(--accent);
  color: #ffffff;
  font: inherit;
  font-weight: 500;
  cursor: pointer;
}

button:hover {
  filter: brightness(0.92);
}

.list {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  z-index: 10;
  margin: 0;
  padding: 6px;
  list-style: none;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.14);
}

li {
  display: flex;
  flex-direction: column;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
}

li.active {
  background: var(--hover);
}

.title,
.url {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.title {
  font-size: 13px;
  font-weight: 500;
}

.url {
  color: var(--muted);
  font-size: 12px;
}

.progress {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 2px;
  width: 0;
  background: var(--accent);
}

.progress.on {
  width: 100%;
  animation: load 1s ease-in-out infinite;
  transform-origin: left;
}

@keyframes load {
  from { transform: scaleX(0); }
  to { transform: scaleX(1); }
}
</style>
