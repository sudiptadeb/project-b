<script setup>
import { computed, nextTick, onUnmounted, ref, watch } from 'vue';
import { comboOf, shown } from '@/keys.js';
import Dialog from '@/ui/Dialog.vue';
import { pushEscape } from '@/ui/escape.js';
import { send, state } from './state.js';

// Every action by menu, with its keys on this platform. The list only shows; clicking an
// action opens its editor under it. Changes go to Go, and the list changes only when Go
// sends it again.
const kb = computed(() => state.keyboard);
const platform = computed(() => kb.value.platform);

// Who wins a key. Page first is the default, so the list leaves it unmarked; a key the
// browser wins gets its priority as a tag after the keys, the same words as the select.
const priorities = [
  { value: 'page', label: 'Page first' },
  { value: 'windowed', label: 'Browser first, not in full screen' },
  { value: 'always', label: 'Browser first' },
];
const priorityOf = (k) => (k.reserve === 'page' ? '' : priorities.find((p) => p.value === k.reserve)?.label ?? '');

// The tags after a row's keys: one for all of them when they agree, else one after each
// key that has one. Locked is a tag of its own.
function shared(a) {
  if (!a.keys.length) return null;
  const first = a.keys[0];
  return a.keys.every((k) => k.reserve === first.reserve && k.locked === first.locked) ? first : null;
}

const templateNames = { 'project-b': 'Project B', chrome: 'Chrome' };
const templateLabel = (name) => templateNames[name] || name;

// How many actions differ from the template: what a reset or another template loses.
const changedCount = computed(() => kb.value.groups.reduce((n, g) => n + g.actions.filter((a) => a.changed).length, 0));

// Using a template, or the same one again for Reset all. With changes to lose it asks
// first, and the select keeps showing the template in use until the answer.
const asking = ref('');
const templateSelect = ref(null);
function useTemplate(name) {
  if (changedCount.value === 0) {
    send({ t: 'useTemplate', text: name });
    return;
  }
  if (templateSelect.value) templateSelect.value.value = kb.value.template;
  asking.value = name;
}
// Reset all is disabled once nothing differs, so focus goes to the select, which stays.
async function confirmTemplate() {
  send({ t: 'useTemplate', text: asking.value });
  asking.value = '';
  await nextTick();
  templateSelect.value?.focus();
}
const askTitle = computed(() => {
  const n = changedCount.value;
  return `Discard ${n} changed shortcut${n === 1 ? '' : 's'} and use ${templateLabel(asking.value)}'s?`;
});

// "Its ⌘N is already in use", or "Its ⌘N, ⌘⇧N are" for more than one.
const takenText = (keys) =>
  `Its ${keys.map((k) => shown(k, platform.value)).join(', ')} ${keys.length === 1 ? 'is' : 'are'} already in use`;

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
  taken.value = null;
  open.value = open.value === id ? '' : id;
}

// The filter keeps the actions whose title or one of whose keys has what was typed, in
// any case, the keys as shown here or as the keymap writes them.
const filter = ref('');
const groups = computed(() => {
  const q = filter.value.trim().toLowerCase();
  if (!q) return kb.value.groups;
  const hit = (a) =>
    a.title.toLowerCase().includes(q) ||
    a.keys.some((k) => k.keys.toLowerCase().includes(q) || shown(k.keys, platform.value).toLowerCase().includes(q));
  return kb.value.groups.map((g) => ({ ...g, actions: g.actions.filter(hit) })).filter((g) => g.actions.length);
});
// The app's groups first, then the shell in use's own actions under a heading of their
// own; Go sends only the shell in use's.
const ordered = computed(() => [...groups.value.filter((g) => !g.shell), ...groups.value.filter((g) => g.shell)]);
const firstShell = computed(() => ordered.value.find((g) => g.shell));

// Escape in the filter empties it, or leaves it when it is empty; never the page.
function onFilterEscape(e) {
  e.preventDefault();
  if (filter.value) filter.value = '';
  else e.target.blur();
}

// Escape closes the open editor before it closes the page, and focus goes back to its row.
// Escape in a filter with text empties it first, as it would with no editor open.
async function closeEditor() {
  if (filter.value && document.activeElement?.classList.contains('filter')) {
    filter.value = '';
    return;
  }
  const id = open.value;
  toggle(id);
  await nextTick();
  document.getElementById('action-' + id)?.focus();
}
let popEditor = null;
watch(open, (id) => {
  if (id && !popEditor) popEditor = pushEscape(closeEditor);
  if (!id && popEditor) {
    popEditor();
    popEditor = null;
  }
});

// Going to another action, from a clash or the notice: its editor opens, and its row
// scrolls into view and takes focus.
async function goTo(id) {
  filter.value = '';
  stopRecording();
  taken.value = null;
  open.value = id;
  await nextTick();
  const row = document.getElementById('action-' + id);
  row?.scrollIntoView({ block: 'center' });
  row?.focus({ preventScroll: true });
}

// Recording a new key: Go runs nothing while it lasts, so every key reaches this page.
// A key the action has, or one with no modifier, is refused in the row and recording
// goes on; a key another action has asks there whether to take it from that action.
const recording = ref(false);
const recordError = ref('');
const taken = ref(null);
let recorder = null;
let addButton = null;

async function record() {
  recordError.value = '';
  recording.value = true;
  send({ t: 'recording', on: true });
  await nextTick();
  recorder?.focus();
}

function stopRecording() {
  if (!recording.value) return;
  recording.value = false;
  recordError.value = '';
  send({ t: 'recording', on: false });
}

// Escape, Cancel and a key added all end at Add key, so the keyboard stays in the editor.
async function backToAdd() {
  stopRecording();
  taken.value = null;
  await nextTick();
  addButton?.focus();
}

// While a key is being recorded or a taken key asks its question, Escape ends that and
// leaves the editor open.
let popRecording = null;
watch(
  () => recording.value || !!taken.value,
  (on) => {
    if (on && !popRecording) popRecording = pushEscape(backToAdd);
    if (!on && popRecording) {
      popRecording();
      popRecording = null;
    }
  },
);

// The modifiers in the one order, and the key, so keys written two ways compare equal.
const modOrder = ['Ctrl', 'Alt', 'Shift', 'Cmd'];
const modAlias = { Option: 'Alt', Win: 'Cmd' };
function same(keys) {
  const parts = keys.split('+');
  const key = parts.pop().toUpperCase();
  const mods = parts.map((p) => modAlias[p] || p);
  return modOrder.filter((m) => mods.includes(m)).concat(key).join('+');
}

// The action that has a key and the binding, preferring the one that wins it.
function ownerOf(text) {
  let found = null;
  for (const g of kb.value.groups) {
    for (const a of g.actions) {
      for (const k of a.keys) {
        if (same(k.keys) !== text) continue;
        if (!found || (found.key.clash && !k.clash)) found = { action: a, key: k };
      }
    }
  }
  return found;
}

const modifierHint = computed(() => (platform.value === 'mac' ? 'Hold ⌘, ⌃ or ⌥ with it' : 'Hold Ctrl or Alt with it'));

async function onRecordKey(e, action) {
  // Tab still moves on, which ends recording.
  if (e.key === 'Tab' && !e.ctrlKey && !e.altKey && !e.metaKey) return;
  e.preventDefault();
  e.stopPropagation();
  const combo = comboOf(e, platform.value);
  if (!combo) return;
  // F keys are fine alone, as the templates' F5, F6 and F11 are; anything else needs a
  // modifier other than Shift, or it would only type.
  if (!e.ctrlKey && !e.altKey && !e.metaKey && !/^F[0-9]+$/.test(combo.split('+').pop())) {
    recordError.value = modifierHint.value;
    return;
  }
  const text = same(combo);
  if (action.keys.some((k) => same(k.keys) === text)) {
    recordError.value = `${action.title} already has ${shown(combo, platform.value)}`;
    return;
  }
  const owner = ownerOf(text);
  if (owner) {
    stopRecording();
    taken.value = { combo, action: owner.action, key: owner.key };
    await nextTick();
    document.getElementById('taken-answer')?.focus();
    return;
  }
  addKey(action, combo);
  backToAdd();
}

// Remove and Reset take away the button that had focus once Go sends the new list. Focus
// then goes to the Remove now in the removed one's place, or else to Add key.
let refocus = null;
function removeKey(a, k, e) {
  refocus = [...document.querySelectorAll('.editor .remove')].indexOf(e.currentTarget);
  send({ t: 'removeKey', id: a.id, index: k.index });
}
function resetAction(a) {
  refocus = -1;
  send({ t: 'resetAction', id: a.id });
}
watch(kb, async () => {
  if (refocus === null) return;
  const at = refocus;
  refocus = null;
  await nextTick();
  const next = at >= 0 ? document.querySelectorAll('.editor .remove')[at] : null;
  (next || addButton)?.focus();
});

// Leaving the section ends a recording and takes this page's Escape layers off.
onUnmounted(() => {
  stopRecording();
  popRecording?.();
  popEditor?.();
});

function addKey(action, combo) {
  const reserve = action.keys.find((k) => !k.locked)?.reserve || 'page';
  send({ t: 'addKey', id: action.id, text: combo, reserve });
}

// Replace takes the key from the action that has it and gives it to this one.
function replace(action) {
  const t = taken.value;
  send({ t: 'removeKey', id: t.action.id, index: t.key.index });
  addKey(action, t.combo);
  backToAdd();
}

// A key another action has can be taken from it unless it is locked or set in macOS.
const replaceable = (t) => !t.key.locked && !t.key.fromOS;
</script>

<template>
  <div>
    <section aria-label="Template">
      <div class="card">
        <div class="row">
          <label class="label" for="template">Template</label>
          <select id="template" ref="templateSelect" :value="kb.template" @change="useTemplate($event.target.value)">
            <option v-for="t in kb.templates" :key="t" :value="t">{{ templateLabel(t) }}</option>
          </select>
          <button type="button" class="text-button" :disabled="!changedCount" @click="useTemplate(kb.template)">Reset all…</button>
        </div>
        <div v-for="p in kb.problems" :key="p" class="row">
          <span class="label problem" :title="p">{{ p }}</span>
        </div>
      </div>
    </section>

    <section v-if="kb.missing.length" class="notice" role="status" aria-labelledby="notice">
      <div class="heading-row">
        <h2 id="notice">New actions</h2>
        <button type="button" class="text-button quiet" @click="send({ t: 'dismissMissing' })">Dismiss</button>
      </div>
      <div class="card">
        <div v-for="m in kb.missing" :key="m.id" class="row">
          <span class="label">
            <button type="button" class="link" @click="goTo(m.id)">{{ titleOf(m.id) }}</button>
            <span class="secondary">{{ takenText(m.taken) }}</span>
          </span>
        </div>
      </div>
    </section>

    <section class="filter-section" aria-label="Filter">
      <input
        v-model="filter"
        class="text-field filter"
        type="text"
        spellcheck="false"
        aria-label="Filter shortcuts"
        placeholder="Filter by name or key"
        @keydown.esc="onFilterEscape"
      />
    </section>

    <section v-if="!groups.length" aria-label="No matches">
      <div class="card">
        <div class="row empty">No shortcuts match</div>
      </div>
    </section>

    <template v-for="g in ordered" :key="g.title">
    <section v-if="g === firstShell" class="shells" aria-label="Shells">
      <h2 class="shells-title">Shells</h2>
      <p class="shells-note">Actions a shell adds. They work while that shell is the one in use.</p>
    </section>
    <section :aria-label="g.shell || g.title">
      <h2 v-if="g.shell">{{ g.shell }} <span class="tag">In use</span></h2>
      <h2 v-else>{{ g.title }}</h2>
      <div class="card">
        <template v-for="a in g.actions" :key="a.id">
          <button :id="'action-' + a.id" type="button" class="action" :aria-expanded="open === a.id" @click="toggle(a.id)">
            <span class="name">
              <span class="title" :title="a.title">{{ a.title }}</span>
              <span v-if="a.hidden" class="tag">Hidden</span>
            </span>
            <span class="keys">
              <template v-for="k in a.keys" :key="k.index + k.keys">
                <kbd :class="{ clash: k.clash }" :title="k.clash ? `Also ${k.clash}'s, which wins it` : ''">{{ shown(k.keys, platform) }}</kbd>
                <template v-if="!shared(a)">
                  <span v-if="k.locked" class="tag">Locked</span>
                  <span v-if="priorityOf(k)" class="tag">{{ priorityOf(k) }}</span>
                </template>
              </template>
              <template v-if="shared(a)">
                <span v-if="shared(a).locked" class="tag">Locked</span>
                <span v-if="priorityOf(shared(a))" class="tag">{{ priorityOf(shared(a)) }}</span>
              </template>
            </span>
            <svg class="chevron" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 4l4 4-4 4" /></svg>
          </button>

          <div v-if="open === a.id" class="editor" :aria-label="`Edit ${a.title}`">
            <div v-for="k in a.keys" :key="'e' + k.index + k.keys" class="key-row">
              <kbd>{{ shown(k.keys, platform) }}</kbd>
              <span v-if="k.locked" class="tag">Locked</span>
              <span v-else-if="k.fromOS" class="tag">Set in macOS</span>
              <span v-if="k.clash" class="note problem">
                Also <button type="button" class="link" @click="goTo(k.clashID)">{{ k.clash }}</button>'s, which wins it
              </span>
              <span class="spacer"></span>
              <select
                v-if="!k.locked"
                :aria-label="`Priority of ${shown(k.keys, platform)}`"
                :value="k.reserve"
                @change="send({ t: 'setReserve', id: a.id, index: k.index, reserve: $event.target.value })"
              >
                <option v-for="p in priorities" :key="p.value" :value="p.value">{{ p.label }}</option>
              </select>
              <button
                v-if="!k.locked && !k.fromOS"
                type="button"
                class="text-button quiet remove"
                :aria-label="`Remove ${shown(k.keys, platform)}`"
                @click="removeKey(a, k, $event)"
              >Remove</button>
            </div>
            <div v-if="taken" class="key-row">
              <span class="taken">
                {{ shown(taken.combo, platform) }} is
                <button type="button" class="link" @click="goTo(taken.action.id)">{{ taken.action.title }}</button>'s
              </span>
              <span class="spacer"></span>
              <button v-if="replaceable(taken)" id="taken-answer" type="button" class="text-button" @click="replace(a)">Replace</button>
              <button :id="replaceable(taken) ? undefined : 'taken-answer'" type="button" class="text-button" @click="backToAdd">Cancel</button>
            </div>
            <div v-else class="key-row add">
              <template v-if="recording">
                <input
                  :ref="(el) => (recorder = el)"
                  class="text-field recorder"
                  readonly
                  :aria-label="`New key for ${a.title}`"
                  :aria-invalid="recordError ? 'true' : undefined"
                  :aria-describedby="recordError ? 'record-error' : undefined"
                  placeholder="Press keys"
                  @keydown="onRecordKey($event, a)"
                  @blur="stopRecording"
                />
                <span v-if="recordError" id="record-error" class="text-field-error" role="alert">{{ recordError }}</span>
              </template>
              <button v-else :ref="(el) => (addButton = el)" type="button" class="text-button" @click="record">Add key</button>
              <span class="spacer"></span>
              <span v-if="a.locked" class="tag">Always in menu</span>
              <label v-else class="menu-switch">
                In menu
                <input
                  type="checkbox"
                  role="switch"
                  class="switch"
                  :aria-label="`${a.title} in the menu`"
                  :checked="!a.hidden"
                  @change="send({ t: 'setHidden', id: a.id, on: !$event.target.checked })"
                />
              </label>
              <button type="button" class="text-button quiet" :disabled="!a.changed" @click="resetAction(a)">Reset</button>
            </div>
          </div>
        </template>
      </div>
    </section>
    </template>

    <Dialog v-if="asking" :title="askTitle" @close="asking = ''">
      <template #buttons>
        <button type="button" class="text-button" @click="asking = ''">Cancel</button>
        <button type="button" class="text-button danger" @click="confirmTemplate">Discard</button>
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.shells {
  margin-top: 12px;
}

.shells-title {
  color: var(--text);
  font-size: 14px;
}

.shells-note {
  margin: -4px 0 0;
  color: var(--muted);
  font-size: 13px;
}

h2 .tag {
  margin-left: 6px;
  vertical-align: 1px;
}

/* The card clips its rows' hover and open colour to its corners, and its width decides
   the columns below. */
.card {
  overflow: hidden;
  container-type: inline-size;
}

/* One grid for every row: the action, its keys with any tags, and the chevron. The name
   column gives way first, so a narrow page wraps the tags rather than overflowing. */
.action {
  display: grid;
  grid-template-columns: minmax(0, 220px) minmax(0, 1fr) 16px;
  align-items: center;
  column-gap: 16px;
  width: 100%;
  min-height: 40px;
  padding: 8px 16px;
  box-sizing: border-box;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

/* On a narrow card the keys take the larger share, so a priority tag fits on its line. */
@container (max-width: 560px) {
  .action {
    grid-template-columns: minmax(0, 2fr) minmax(0, 3fr) 16px;
  }
}

.action + .action,
.editor + .action {
  border-top: 1px solid var(--line);
}

.action:hover,
.action[aria-expanded='true'] {
  background: var(--accent-tint);
}

/* --focus-ring drawn inward, so the card's edge can't clip it. */
.action:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

.chevron {
  color: var(--icon);
  transition: transform 120ms;
}

.action[aria-expanded='true'] .chevron {
  transform: rotate(90deg);
}

.name {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

/* A long name is one line with an ellipsis, the whole of it in the tooltip. */
.title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

kbd {
  display: inline-flex;
  align-items: center;
  height: 24px;
  padding: 0 8px;
  border: 1px solid var(--line);
  border-radius: 4px;
  box-sizing: border-box;
  background: var(--bar);
  font: inherit;
  font-size: 13px;
  white-space: nowrap;
}

kbd.clash {
  border-color: var(--danger);
}

.editor {
  padding: 4px 16px 8px;
  border-top: 1px solid var(--line);
  background: var(--accent-tint);
}

.key-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 40px;
}

/* On a narrow page the priority select gives way, so Remove stays on its key's line; the
   last row wraps instead, its error under the recorder. */
.key-row select {
  flex: 0 1 auto;
  min-width: 0;
  text-overflow: ellipsis;
}

.key-row.add {
  flex-wrap: wrap;
}

.spacer {
  flex: 1;
}

.note {
  color: var(--muted);
  font-size: 12px;
}

.menu-switch {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 13px;
}

/* The recorder is a field that shows it is listening: always ringed while it lasts. */
.recorder {
  width: 128px;
  box-shadow: var(--focus-ring);
  caret-color: transparent;
}

.key-row .text-field-error {
  margin-top: 0;
}

.taken {
  min-width: 0;
}

/* A link to another action, in running text. */
.filter-section {
  margin-top: 16px;
}

.filter {
  width: 100%;
}

.empty {
  color: var(--muted);
}

/* A heading with one action on its right, 30px high. */
.heading-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  height: 30px;
}

.problem {
  color: var(--danger);
  font-weight: 400;
}
</style>
