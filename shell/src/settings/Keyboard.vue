<script setup>
import { computed, nextTick, ref } from 'vue';
import { comboOf, shown } from '../keys.js';
import { send, state } from './state.js';

// Every action by menu, with its keys on this platform. Changes go to Go, and the list
// changes only when Go sends it again.
const kb = computed(() => state.keyboard);
const platform = computed(() => kb.value.platform);

const reserves = [
  { value: 'always', label: 'Always' },
  { value: 'windowed', label: 'Outside full screen' },
  { value: 'page', label: 'Page first' },
];

const templateNames = { 'project-b': 'Project B', chrome: 'Chrome' };
const templateLabel = (name) => templateNames[name] || name;

const titleOf = (id) => {
  for (const g of kb.value.groups) {
    const a = g.actions.find((x) => x.id === id);
    if (a) return a.title;
  }
  return id;
};

// Recording a new key for one action: Go runs nothing while it lasts, so every key
// reaches this page.
const recording = ref('');
const recorder = ref(null);

async function record(id) {
  recording.value = id;
  send({ t: 'recording', on: true });
  await nextTick();
  recorder.value?.[0]?.focus();
}

function stopRecording() {
  if (!recording.value) return;
  recording.value = '';
  send({ t: 'recording', on: false });
}

function onRecordKey(e, action) {
  e.preventDefault();
  e.stopPropagation();
  if (e.key === 'Escape') {
    stopRecording();
    return;
  }
  const combo = comboOf(e, platform.value);
  if (!combo) return;
  const reserve = action.keys.find((k) => !k.locked)?.reserve || 'page';
  send({ t: 'addKey', id: action.id, text: combo, reserve });
  stopRecording();
}
</script>

<template>
  <div>
    <section aria-labelledby="keymap">
      <h2 id="keymap">Keyboard shortcuts</h2>
      <div class="card">
        <div class="row">
          <label class="label" for="template">Template</label>
          <select id="template" :value="kb.template" @change="send({ t: 'useTemplate', text: $event.target.value })">
            <option v-for="t in kb.templates" :key="t" :value="t">{{ templateLabel(t) }}</option>
          </select>
          <button type="button" class="text-button" :disabled="kb.following" @click="send({ t: 'useTemplate', text: kb.template })">Reset all</button>
        </div>
      </div>
    </section>

    <section v-if="kb.missing.length || kb.problems.length" class="notice" role="status">
      <div class="card">
        <div v-for="p in kb.problems" :key="p" class="row">
          <span class="label problem">{{ p }}</span>
        </div>
        <div v-for="m in kb.missing" :key="m.id" class="row">
          <span class="label">
            New: {{ titleOf(m.id) }}
            <span class="sub">Its {{ m.taken.map((k) => shown(k, platform)).join(', ') }} is already in use</span>
          </span>
        </div>
        <div v-if="kb.missing.length" class="row">
          <span class="label"></span>
          <button type="button" class="text-button" @click="send({ t: 'dismissMissing' })">Dismiss</button>
        </div>
      </div>
    </section>

    <section v-for="g in kb.groups" :key="g.title" :aria-label="g.title">
      <div class="group-head">
        <h2>{{ g.title }}</h2>
        <span class="column">In menu</span>
      </div>
      <div class="card">
        <div v-for="a in g.actions" :key="a.id" class="row action">
          <span class="label">{{ a.title }}</span>
          <div class="keys">
            <span v-for="k in a.keys" :key="k.index + k.keys" class="key" :class="{ clash: k.clash }">
              <kbd :title="k.clash ? `Also ${k.clash}'s, which wins it` : k.fromOS ? 'Set in macOS System Settings' : ''">{{ shown(k.keys, platform) }}</kbd>
              <span v-if="k.locked" class="tag" title="Locked">
                <svg width="10" height="11" viewBox="0 0 10 11" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="1" y="4.6" width="8" height="5.6" rx="1"/><path d="M3 4.6V3a2 2 0 0 1 4 0v1.6"/></svg>
              </span>
              <span v-else-if="k.fromOS" class="tag">macOS</span>
              <select
                v-if="!k.locked"
                class="reserve"
                :aria-label="`Who wins ${k.keys}`"
                :value="k.reserve"
                @change="send({ t: 'setReserve', id: a.id, index: k.index, reserve: $event.target.value })"
              >
                <option v-for="r in reserves" :key="r.value" :value="r.value">{{ r.label }}</option>
              </select>
              <button
                v-if="!k.locked && !k.fromOS"
                type="button"
                class="icon-button small"
                :aria-label="`Remove ${k.keys}`"
                :title="`Remove ${k.keys}`"
                @click="send({ t: 'removeKey', id: a.id, index: k.index })"
              >
                <svg width="8" height="8" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M1 1l8 8M9 1L1 9"/></svg>
              </button>
            </span>
            <input
              v-if="recording === a.id"
              ref="recorder"
              class="recorder"
              readonly
              :aria-label="`New key for ${a.title}`"
              placeholder="Press keys"
              @keydown="onRecordKey($event, a)"
              @blur="stopRecording"
            />
            <button
              v-else
              type="button"
              class="icon-button small"
              :aria-label="`Add a key for ${a.title}`"
              :title="`Add a key for ${a.title}`"
              @click="record(a.id)"
            >
              <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M5 1v8M1 5h8"/></svg>
            </button>
          </div>
          <input
            type="checkbox"
            role="switch"
            class="switch"
            :aria-label="`${a.title} in the menu`"
            :title="a.locked ? 'Always in the menu' : 'In the menu'"
            :checked="!a.hidden"
            :disabled="a.locked"
            @change="send({ t: 'setHidden', id: a.id, on: !$event.target.checked })"
          />
          <button
            type="button"
            class="icon-button small"
            :aria-label="`Reset ${a.title}`"
            :title="`Reset ${a.title}`"
            :disabled="kb.following"
            @click="send({ t: 'resetAction', id: a.id })"
          >
            <svg width="11" height="11" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><path d="M2 6a4 4 0 1 0 1.2-2.85"/><path d="M2 1.5v2.5h2.5"/></svg>
          </button>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.group-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

/* Over the switches: the row's padding, the reset button and the gap between them. */
.column {
  width: 52px;
  margin-right: 46px;
  color: var(--muted);
  font-size: 12px;
  text-align: center;
}

.action {
  min-height: 44px;
  padding-top: 6px;
  padding-bottom: 6px;
}

.action .label {
  flex: 0 0 200px;
  font-weight: 400;
}

.keys {
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.key {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 28px;
  padding: 0 2px 0 8px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--bar);
}

.key.clash {
  border-color: var(--danger);
}

kbd {
  font: inherit;
  font-weight: 500;
  white-space: nowrap;
}

.tag {
  display: inline-flex;
  align-items: center;
  padding-right: 6px;
  color: var(--muted);
  font-size: 12px;
}

.reserve {
  height: 22px;
  padding: 0 2px;
  border: 0;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
}

.icon-button.small {
  width: 22px;
  height: 22px;
}

.icon-button:disabled {
  opacity: 0.35;
  cursor: default;
}

.recorder {
  width: 110px;
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--accent);
  border-radius: 6px;
  background: var(--surface);
  color: inherit;
  font: inherit;
  box-sizing: border-box;
  outline: none;
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.sub {
  display: block;
  color: var(--muted);
  font-size: 12px;
  font-weight: 400;
}

.problem {
  color: var(--danger);
  font-weight: 400;
}
</style>
