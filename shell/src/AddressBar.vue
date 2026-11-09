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

function onFocus() {
  focused.value = true;
  typed.value = text.value;
  nextTick(() => input.value.select());
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
  <form v-shell class="bar" @submit.prevent="go()">
    <div class="field" :class="{ open }">
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
  gap: 8px;
  padding: 8px;
  background: var(--bg);
  border-bottom: 1px solid var(--line);
}

.field {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  height: 36px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--bg);
}

.field:focus-within {
  border-color: var(--accent);
}

input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  outline: none;
  background: transparent;
  color: inherit;
  font: inherit;
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
  height: 38px;
  padding: 0 16px;
  border: 0;
  border-radius: 8px;
  background: var(--accent);
  color: #ffffff;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}

.list {
  position: absolute;
  top: calc(100% + 4px);
  left: -1px;
  right: -1px;
  z-index: 10;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.18);
}

li {
  display: flex;
  flex-direction: column;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
}

li.active {
  background: var(--bg-hover);
}

.title,
.url {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.url {
  color: var(--muted);
  font-size: 12px;
}

.progress {
  position: absolute;
  left: 0;
  bottom: -1px;
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
