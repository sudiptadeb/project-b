<script setup>
import { computed, nextTick, ref } from 'vue';
import { comboOf, shown } from '@/keys.js';
import { send, state } from './state.js';

// Every action by menu, with its keys on this platform. The list only shows; clicking an
// action opens its editor under it. Changes go to Go, and the list changes only when Go
// sends it again.
const kb = computed(() => state.keyboard);
const platform = computed(() => kb.value.platform);

// Who wins a key. Page first is the default, so the list leaves it unmarked; a key the
// browser wins gets a tag after the keys.
const priorities = [
  { value: 'page', label: 'Page first', short: '' },
  { value: 'windowed', label: 'Browser, except in full screen', short: 'Browser · not full screen' },
  { value: 'always', label: 'Browser, always', short: 'Browser' },
];
const shortOf = (r) => priorities.find((p) => p.value === r)?.short ?? '';

// The tags a row shows: one after all its keys when they agree, else one after each key
// the browser wins.
function shared(a) {
  if (!a.keys.length) return null;
  const first = a.keys[0].reserve;
  return a.keys.every((k) => k.reserve === first) ? first : null;
}
function tagText(k) {
  const t = shortOf(k.reserve);
  return k.locked && t ? 'Locked · ' + t : t;
}

const templateNames = { 'project-b': 'Project B', chrome: 'Chrome' };
const templateLabel = (name) => templateNames[name] || name;

const titleOf = (id) => {
  for (const g of kb.value.groups) {
    const a = g.actions.find((x) => x.id === id);
    if (a) return a.title;
  }
  return id;
};

// The one action being edited.
const open = ref('');
function toggle(id) {
  stopRecording();
  open.value = open.value === id ? '' : id;
}

// Recording a new key: Go runs nothing while it lasts, so every key reaches this page.
const recording = ref(false);
const recorder = ref(null);

async function record() {
  recording.value = true;
  send({ t: 'recording', on: true });
  await nextTick();
  recorder.value?.[0]?.focus();
}

function stopRecording() {
  if (!recording.value) return;
  recording.value = false;
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
      <h2>{{ g.title }}</h2>
      <div class="card">
        <template v-for="a in g.actions" :key="a.id">
          <button type="button" class="action" :aria-expanded="open === a.id" @click="toggle(a.id)">
            <span class="name">
              {{ a.title }}
              <span v-if="a.hidden" class="tag">Hidden</span>
            </span>
            <span class="keys">
              <template v-for="k in a.keys" :key="k.index + k.keys">
                <kbd :class="{ clash: k.clash }" :title="k.clash ? `Also ${k.clash}'s, which wins it` : ''">{{ shown(k.keys, platform) }}</kbd>
                <span v-if="shared(a) === null && tagText(k)" class="tag" :class="{ locked: k.locked }">{{ tagText(k) }}</span>
              </template>
              <span v-if="shared(a) !== null && tagText(a.keys[0])" class="tag" :class="{ locked: a.keys[0].locked }">{{ tagText(a.keys[0]) }}</span>
            </span>
          </button>

          <div v-if="open === a.id" class="editor" :aria-label="`Edit ${a.title}`">
            <div v-for="k in a.keys" :key="'e' + k.index + k.keys" class="key-row">
              <kbd>{{ shown(k.keys, platform) }}</kbd>
              <span v-if="k.locked" class="note">Locked</span>
              <span v-else-if="k.fromOS" class="note">Set in macOS</span>
              <span v-if="k.clash" class="note problem">Also {{ k.clash }}'s, which wins it</span>
              <span class="spacer"></span>
              <select
                v-if="!k.locked"
                :aria-label="`Priority of ${k.keys}`"
                :value="k.reserve"
                @change="send({ t: 'setReserve', id: a.id, index: k.index, reserve: $event.target.value })"
              >
                <option v-for="p in priorities" :key="p.value" :value="p.value">{{ p.label }}</option>
              </select>
              <button
                v-if="!k.locked && !k.fromOS"
                type="button"
                class="text-button quiet"
                :aria-label="`Remove ${k.keys}`"
                @click="send({ t: 'removeKey', id: a.id, index: k.index })"
              >Remove</button>
            </div>
            <div class="key-row">
              <input
                v-if="recording"
                ref="recorder"
                class="recorder"
                readonly
                :aria-label="`New key for ${a.title}`"
                placeholder="Press keys"
                @keydown="onRecordKey($event, a)"
                @blur="stopRecording"
              />
              <button v-else type="button" class="text-button" @click="record">Add key</button>
              <span class="spacer"></span>
              <label class="menu-switch">
                In menu
                <input
                  type="checkbox"
                  role="switch"
                  class="switch"
                  :aria-label="`${a.title} in the menu`"
                  :checked="!a.hidden"
                  :disabled="a.locked"
                  @change="send({ t: 'setHidden', id: a.id, on: !$event.target.checked })"
                />
              </label>
              <button type="button" class="text-button quiet" :disabled="kb.following" @click="send({ t: 'resetAction', id: a.id })">Reset</button>
            </div>
          </div>
        </template>
      </div>
    </section>
  </div>
</template>

<style scoped>
/* One grid for every row: the action, then its keys with any priority tags. */
.action {
  display: grid;
  grid-template-columns: 220px 1fr;
  align-items: center;
  column-gap: 16px;
}

.action {
  width: 100%;
  min-height: 40px;
  padding: 6px 14px;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.action + .action,
.editor + .action {
  border-top: 1px solid var(--line);
}

.action:hover,
.action[aria-expanded='true'] {
  background: var(--accent-tint);
}

.action:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 2px var(--accent-soft);
}

.card > .action:first-child {
  border-radius: 8px 8px 0 0;
}

.card > .action:last-child {
  border-radius: 0 0 8px 8px;
}

.keys {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.keys .tag {
  margin-left: 0;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--text);
  font-size: 12px;
  white-space: nowrap;
}

.keys .tag.locked {
  background: none;
  border: 1px solid var(--line);
  color: var(--muted);
}

kbd {
  display: inline-block;
  padding: 1px 7px;
  border: 1px solid var(--line);
  border-radius: 5px;
  background: var(--bar);
  font: inherit;
  font-size: 13px;
  white-space: nowrap;
}

kbd.clash {
  border-color: var(--danger);
}

.tag {
  margin-left: 6px;
  color: var(--muted);
  font-size: 12px;
}

.editor {
  padding: 4px 14px 10px 14px;
  border-top: 1px solid var(--line);
  background: var(--accent-tint);
}

.key-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 38px;
}

.spacer {
  flex: 1;
}

.note {
  color: var(--muted);
  font-size: 12px;
}

.text-button.quiet {
  border-color: transparent;
  background: none;
  color: var(--muted);
}

.text-button.quiet:hover:not(:disabled) {
  background: var(--accent-tint);
  color: inherit;
}

.menu-switch {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 13px;
}

.recorder {
  width: 130px;
  height: 30px;
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
