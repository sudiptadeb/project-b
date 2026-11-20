<script setup>
import { computed, onMounted, reactive, ref, watch, watchEffect } from 'vue';
import { paintTheme } from '@/theme.js';
import ImportWizard from '@/ui/ImportWizard.vue';
import Segmented from '@/ui/Segmented.vue';
import Swatches from '@/ui/Swatches.vue';
import { framePresets } from '@/ui/colours.js';
import { paint, swatches } from '@/ui/hues.js';
import { useTints } from '@/ui/useTints.js';
import { shown } from '@/keys.js';
import Preview from './Preview.vue';
import { send, setPref, state } from './state.js';

// The welcome (docs/onboarding.md): a few steps, each skippable, whose every choice is a
// pref set at once, so the page, its window's frame and the picture of the browser take it
// together. Over its whole window, as a first start opens it, it starts with a splash.

watchEffect(() => paintTheme(state.tones, state.theme));
document.title = 'Welcome';

// Bring your things over, the import wizard, and How you work, the workspaces and the
// keys, are the desktop's: a phone keeps each app's data from the others, and has neither.
// Extras for any page, the built-in sparks the profile still has, is the phone's.
const steps = computed(() =>
  state.phone
    ? ['hello', 'style', ...(state.sparks.length ? ['extras'] : []), 'protection', 'done']
    : ['hello', 'bring', 'style', 'work', 'protection', 'done'],
);
const step = ref('hello');
const at = computed(() => steps.value.indexOf(step.value));
const go = (by) => {
  if (step.value === 'work' && by > 0) send({ t: 'setWorkspaces', workspaces: starters.filter((w) => w.on).map((w) => ({ name: w.name, color: starterColor(w) })) });
  step.value = steps.value[Math.min(steps.value.length - 1, Math.max(0, at.value + by))];
};
const finish = () => send({ t: 'done' });

// The splash plays once, when the welcome has its window to itself; less motion skips it.
const splash = ref(false);
const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
watch(
  () => state.ready,
  (ready) => {
    if (!ready || !state.whole || reduced) return;
    splash.value = true;
    setTimeout(() => (splash.value = false), 1800);
  },
  { once: true },
);

// The way to work: a shell, and for Standard where its tabs go.
const layout = computed(() => {
  const p = state.prefs;
  return state.layouts.find((l) => l.shell === p.shell && (!l.tabLayout || l.tabLayout === p.tabLayout))?.id || 'side';
});
function chooseLayout(id) {
  const l = state.layouts.find((l) => l.id === id);
  if (!l) return;
  if (l.tabLayout) setPref('tabLayout', l.tabLayout);
  setPref('shell', l.shell);
}

// How you work: the workspaces to start with, Personal and Work ticked, and any the person
// adds; Continue makes those ticked (Go's setWorkspaces). Each takes the next of gulal's
// seven hues (hues.js), by its place in the list. Then the keys: Project B's, which are
// Chrome's, or Firefox's, each shown by a few of its keys.
const starters = reactive([
  { name: 'Personal', note: 'Everyday browsing', on: true },
  { name: 'Work', note: 'Mail, docs and tickets, apart', on: true },
  { name: 'Reading', note: 'Articles for later', on: false },
]);
const tints = useTints();
const starterColor = (w) => (tints.value, swatches()[starters.indexOf(w) % 7]?.color || '');
const adding = ref('');
function addStarter() {
  const name = adding.value.trim();
  if (!name || starters.some((w) => w.name.toLowerCase() === name.toLowerCase())) return;
  starters.push({ name, note: '', on: true });
  adding.value = '';
}
const keymaps = [
  { value: 'project-b', label: 'Project B' },
  { value: 'firefox', label: 'Like Firefox' },
];
const cheats = [
  ['tab.new', 'New tab'],
  ['address.focus', 'Address'],
  ['tab.reopenClosed', 'Reopen tab'],
  ['tab.next', 'Next tab'],
  ['history.show', 'History'],
  ['downloads.show', 'Downloads'],
  ['bookmarks.show', 'Bookmarks'],
  ['dev.viewSource', 'Page source'],
];
// New Tab's key in the picture of the browser, as this platform writes it.
const newTabKey = computed(() => {
  const k = (state.keymaps[state.keymap] || {})['tab.new'];
  return k ? shown(k, state.platform) : '';
});
const cheatSheet = computed(() => {
  const keys = state.keymaps[state.keymap] || {};
  return cheats.filter(([id]) => keys[id]).map(([id, label]) => ({ id, label, keys: shown(keys[id], state.platform) }));
});

const themes = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
];
const positions = [
  { value: 'top', label: 'Top' },
  { value: 'bottom', label: 'Bottom' },
];

// The colours the hello's window turns through while it waits: the presets with a colour.
const cycle = framePresets.filter((p) => p.color).map((p) => p.color);
const turn = ref(0);
if (!reduced) setInterval(() => (turn.value = (turn.value + 1) % cycle.length), 2400);

// Focus goes to each step's heading, so a screen reader reads where it is.
const heading = ref(null);
watch(step, () => setTimeout(() => heading.value?.focus(), 0));
onMounted(() => heading.value?.focus());
</script>

<template>
  <div class="welcome" :class="{ whole: state.whole, phone: state.phone }">
    <div v-if="splash" class="splash" aria-hidden="true">
      <span class="mark"><svg viewBox="0 0 32 32"><rect x="7" y="9" width="18" height="14" rx="3" /><path d="M16 9v14" /></svg></span>
      <span class="name">Project B</span>
    </div>

    <main v-if="state.ready" class="content">
      <!-- Hello: the mark, the name, one line, and the way in or past. -->
      <section v-if="step === 'hello'" class="hello" aria-labelledby="hello">
        <span class="mark" aria-hidden="true"><svg viewBox="0 0 32 32"><rect x="7" y="9" width="18" height="14" rx="3" /><path d="M16 9v14" /></svg></span>
        <h1 id="hello" ref="heading" tabindex="-1">Welcome to Project B</h1>
        <p class="line">A browser you shape to fit.</p>
        <div class="actions">
          <button type="button" class="text-button primary" @click="go(1)">Get started</button>
          <button type="button" class="text-button quiet" @click="finish">Skip setup</button>
        </div>
        <div class="waiting" aria-hidden="true" :style="{ '--frame': cycle[turn], '--accent-soft': 'var(--line)' }">
          <Preview :platform="state.platform" :new-tab="newTabKey" :layout="state.phone ? 'phone' : 'side'" />
        </div>
      </section>

      <!-- Bring your things over: the import wizard, Settings' Import's too. -->
      <section v-else-if="step === 'bring'" class="bring">
        <ImportWizard :send="send" heading="h1" />
      </section>

      <!-- Style: the picture beside the choices, each taken at once. -->
      <section v-else-if="step === 'style'" class="style" aria-labelledby="style">
        <h1 id="style" ref="heading" tabindex="-1">Pick your style</h1>
        <div class="split">
          <div class="picture">
            <Preview :platform="state.platform" :new-tab="newTabKey" :layout="state.phone ? 'phone' : layout" :top="state.prefs.addressBarPosition === 'top'" />
          </div>
          <div class="choices">
            <div v-if="!state.phone" class="choice">
              <span class="label" id="layout-label">Tabs</span>
              <div class="layouts" role="radiogroup" aria-labelledby="layout-label">
                <button
                  v-for="l in state.layouts"
                  :key="l.id"
                  type="button"
                  role="radio"
                  class="layout"
                  :aria-checked="layout === l.id"
                  @click="chooseLayout(l.id)"
                >
                  <Preview :platform="state.platform" :new-tab="newTabKey" :layout="l.id" />
                  <span>{{ l.name }}</span>
                </button>
              </div>
            </div>
            <div v-else class="choice">
              <span class="label">Address bar</span>
              <Segmented label="Address bar" :options="positions" :model-value="state.prefs.addressBarPosition" @update:model-value="setPref('addressBarPosition', $event)" />
            </div>
            <div class="choice">
              <span class="label">Colour</span>
              <Swatches label="Colour" custom :colors="framePresets" :model-value="state.prefs.frameColor" @update:model-value="setPref('frameColor', $event)" />
            </div>
            <div class="choice">
              <span class="label">Theme</span>
              <Segmented label="Theme" :options="themes" :model-value="state.prefs.theme" @update:model-value="setPref('theme', $event)" />
            </div>
          </div>
        </div>
      </section>

      <!-- Protection: what keeps the web in check, and the search engine. -->
      <!-- How you work: the workspaces to start with, as the switcher shows them, and the keys. -->
      <section v-else-if="step === 'work'" class="work" aria-labelledby="work">
        <h1 id="work" ref="heading" tabindex="-1">How you work</h1>
        <p class="sub">Workspaces keep sets of tabs apart, each with its own sign-ins. Start with these, or add your own.</p>
        <div class="split">
          <div class="picture switcher" aria-hidden="true">
            <Preview :platform="state.platform" :new-tab="newTabKey" layout="side" />
            <div class="menu">
              <span class="item"><i class="tick">✓</i>All workspaces</span>
              <span v-for="w in starters.filter((x) => x.on)" :key="w.name" class="item"><i class="dot" :style="{ background: paint(starterColor(w)) }"></i>{{ w.name }}</span>
            </div>
          </div>
          <div class="choices">
            <div class="choice">
              <span class="label" id="workspaces-label">Workspaces</span>
              <div class="card" role="group" aria-labelledby="workspaces-label">
                <label v-for="w in starters" :key="w.name" class="row starter">
                  <i class="dot" :style="{ background: paint(starterColor(w)) }"></i>
                  <span class="what">
                    <span class="label">{{ w.name }}</span>
                    <span v-if="w.note" class="state">{{ w.note }}</span>
                  </span>
                  <input v-model="w.on" type="checkbox" :aria-label="w.name" />
                </label>
                <form class="row starter" @submit.prevent="addStarter">
                  <i class="dot add">+</i>
                  <input v-model="adding" class="add-name" type="text" placeholder="Add one…" aria-label="Add a workspace" maxlength="40" />
                </form>
              </div>
            </div>
            <div class="choice">
              <span class="label">Keys</span>
              <Segmented label="Keys" :options="keymaps" :model-value="state.keymap" @update:model-value="send({ t: 'useKeymap', value: $event })" />
              <ul class="card cheats" aria-label="Keys">
                <li v-for="c in cheatSheet" :key="c.id" class="row"><span>{{ c.label }}</span><kbd>{{ c.keys }}</kbd></li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      <section v-else-if="step === 'extras'" class="extras" aria-labelledby="extras">
        <h1 id="extras" ref="heading" tabindex="-1">Extras for any page</h1>
        <p class="sub">Small scripts that add to the pages you visit. Turn on any you like, and change or remove them later in Settings&nbsp;›&nbsp;Sparks.</p>
        <div class="card">
          <label v-for="s in state.sparks" :key="s.id" class="row">
            <span class="spark">
              <span class="label">{{ s.name }}</span>
              <span :id="'spark-' + s.id + '-line'" class="state">{{ s.description }}</span>
            </span>
            <input
              :aria-label="s.name"
              :aria-describedby="'spark-' + s.id + '-line'"
              type="checkbox"
              role="switch"
              class="switch"
              :checked="s.on"
              :aria-checked="String(s.on)"
              @change="send({ t: 'setSparkOn', id: s.id, on: $event.target.checked })"
            />
          </label>
        </div>
      </section>

      <section v-else-if="step === 'protection'" class="protection" aria-labelledby="protection">
        <h1 id="protection" ref="heading" tabindex="-1">Protection</h1>
        <div class="card">
          <div class="row">
            <span class="label">Ad blocker</span>
            <span class="state">On</span>
          </div>
          <label class="row">
            <span class="label">Search engine</span>
            <select :value="state.prefs.searchEngine" @change="send({ t: 'setSearchEngine', value: $event.target.value })">
              <option v-for="e in state.searchEngines" :key="e.id" :value="e.id">{{ e.name }}</option>
            </select>
          </label>
          <label class="row">
            <span class="label">Automatic updates</span>
            <input
              aria-label="Automatic updates"
              type="checkbox"
              role="switch"
              class="switch"
              :checked="state.autoUpdate"
              :aria-checked="String(state.autoUpdate)"
              @change="send({ t: 'setAutoUpdate', on: $event.target.checked })"
            />
          </label>
        </div>
      </section>

      <!-- Done: three things worth knowing, and the way in. -->
      <section v-else class="done" aria-labelledby="done">
        <h1 id="done" ref="heading" tabindex="-1">You're set</h1>
        <ul class="tips">
          <li><b>Type anywhere</b><span>An address or a search, in the box a new tab opens.</span></li>
          <li><b>Each site's own settings</b><span>From the address: what the site may use, and its ads.</span></li>
          <li><b>Workspaces</b><span>Sets of tabs, each with its own sign-ins.</span></li>
        </ul>
        <div class="card default">
          <div class="row">
            <span class="label">Default browser</span>
            <span v-if="state.isDefault" class="state">This is your default browser</span>
            <button v-else type="button" class="text-button" @click="send({ t: 'makeDefault' })">Make default browser</button>
          </div>
          <div v-if="state.phone" class="row">
            <span class="label">Download notifications</span>
            <span v-if="state.notifications" class="state">On</span>
            <button v-else type="button" class="text-button" @click="send({ t: 'askNotifications' })">Allow</button>
          </div>
        </div>
        <div class="actions">
          <button type="button" class="text-button primary" @click="finish">Start browsing</button>
        </div>
      </section>
    </main>

    <!-- The way through: where it is, back, and on. -->
    <footer v-if="state.ready && step !== 'hello' && step !== 'done'" class="nav">
      <button type="button" class="text-button quiet" @click="go(-1)">Back</button>
      <ol class="dots" aria-label="Steps">
        <li v-for="(s, i) in steps" :key="s" :class="{ on: i === at }" :aria-current="i === at ? 'step' : undefined"></li>
      </ol>
      <div class="ahead">
        <button type="button" class="text-button quiet" @click="finish">Skip setup</button>
        <button type="button" class="text-button primary" @click="go(1)">Continue</button>
      </div>
    </footer>
  </div>
</template>

<style>
body {
  background: var(--bar);
  transition: background var(--motion) var(--ease);
}
</style>

<style scoped>
.welcome {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 100%;
  box-sizing: border-box;
  padding: 24px;
}

/* Over the whole window the top is the window's: its buttons, and the strip it moves by. */
.welcome.whole {
  padding-top: 48px;
}

.content {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  width: 100%;
  max-width: 880px;
  margin: 0 auto;
}

h1 {
  margin: 0 0 24px;
  font-size: 24px;
  font-weight: 600;
  outline: none;
}

.mark {
  display: grid;
  place-items: center;
  width: 64px;
  height: 64px;
  border-radius: 16px;
  background: var(--accent);
  transition: background var(--motion) var(--ease);
}

.mark svg {
  width: 32px;
  height: 32px;
  fill: none;
  stroke: var(--on-accent);
  stroke-width: 1.5;
  stroke-linejoin: round;
}

/* The splash: the mark draws itself and the name fades in, then the welcome comes. */
.splash {
  position: fixed;
  inset: 0;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  background: var(--bar);
  animation: splash-out 300ms var(--ease) 1500ms forwards;
}

.splash .mark {
  width: 88px;
  height: 88px;
  border-radius: 22px;
  animation: grow 500ms var(--ease) both;
}

.splash .mark svg {
  width: 44px;
  height: 44px;
}

.splash .mark rect,
.splash .mark path {
  stroke-dasharray: 64;
  stroke-dashoffset: 64;
  animation: draw 700ms var(--ease) 300ms forwards;
}

.splash .mark path {
  animation-delay: 700ms;
}

.splash .name {
  font-size: 20px;
  font-weight: 600;
  opacity: 0;
  animation: fade-in 400ms var(--ease) 800ms forwards;
}

@keyframes grow {
  from {
    transform: scale(0.6);
    opacity: 0;
  }
}

@keyframes draw {
  to {
    stroke-dashoffset: 0;
  }
}

@keyframes fade-in {
  to {
    opacity: 1;
  }
}

@keyframes splash-out {
  to {
    opacity: 0;
    visibility: hidden;
  }
}

/* Hello: the words high up, and a window rising from the foot of the page, its frame
   turning through the colours as it waits. */
.content:has(> .hello) {
  justify-content: flex-start;
}

.hello {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding-top: 72px;
}

.hello .mark {
  margin-bottom: 24px;
}

.hello h1 {
  margin-bottom: 8px;
  font-size: 32px;
}

.line {
  margin: 0 0 24px;
  color: var(--muted);
  font-size: 15px;
}

.welcome:has(.hello) {
  height: 100%;
  overflow: hidden;
}

.waiting {
  width: min(900px, 100%);
  margin-top: 56px;
}

.waiting :deep(.window) {
  box-shadow: 0 16px 48px rgba(16, 24, 40, 0.12);
}

.waiting :deep(.window),
.waiting :deep(.phone) {
  transition: border-color 600ms var(--ease), background-color 600ms var(--ease);
}

.actions {
  display: flex;
  gap: 8px;
  justify-content: center;
}

.split {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr);
  gap: 32px;
  align-items: start;
}

.phone .split {
  grid-template-columns: 1fr;
}

.choices {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.choice {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.label {
  font-weight: 500;
}

.layouts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.layout {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  color: inherit;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  transition: border-color var(--motion) var(--ease);
}

.layout :deep(.window) {
  border-width: 2px;
  border-radius: 6px;
}

.layout[aria-checked='true'] {
  border-color: var(--accent);
  box-shadow: inset 0 0 0 1px var(--accent);
}

.layout:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.card {
  display: flex;
  flex-direction: column;
  max-width: 560px;
  border-radius: 8px;
  background: var(--surface);
  box-shadow: inset 0 0 0 1px var(--line);
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 48px;
  padding: 0 16px;
}

.row + .row {
  border-top: 1px solid var(--line);
}

.state {
  color: var(--muted);
}

.spark {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 12px 0;
}

.spark .state {
  font-size: 13px;
}

.tips {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin: 0 0 32px;
  padding: 0;
  list-style: none;
}

.tips li {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tips span {
  color: var(--muted);
}

.done .actions {
  justify-content: flex-start;
}

.card.default {
  margin-bottom: 32px;
}

/* How you work: the switcher's menu as the standard shell draws it, at the sidebar's foot. */
/* How you work: the standard shell, as Pick your style draws it, with its workspace menu
   open above the switcher at the sidebar's foot. */
.switcher {
  position: relative;
}

.switcher .menu {
  position: absolute;
  left: 3%;
  bottom: 9%;
  display: flex;
  flex-direction: column;
  width: 220px;
  padding: 4px;
  border-radius: 8px;
  background: var(--surface);
  box-shadow: 0 8px 24px rgb(0 0 0 / 0.14), 0 0 0 1px var(--line);
}

.switcher .item {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 6px;
}

.switcher .tick {
  width: 10px;
  font-style: normal;
}

.dot {
  flex: none;
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.starter {
  justify-content: flex-start;
}

.starter input[type='checkbox'] {
  accent-color: var(--accent);
}

.starter .what {
  display: flex;
  flex-direction: column;
  flex: 1;
  padding: 6px 0;
}

.dot.add {
  display: grid;
  place-items: center;
  font-style: normal;
  color: var(--muted);
}

.add-name {
  flex: 1;
  border: 0;
  background: none;
  font: inherit;
  color: var(--text);
}

.add-name:focus {
  outline: none;
}

/* The keys, two columns of them. */
.cheats {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin: 0;
  padding: 4px 0;
  list-style: none;
}

.cheats .row {
  min-height: 32px;
  padding: 0 16px;
}

.cheats .row + .row {
  border-top: 0;
}

.cheats kbd {
  font: inherit;
  color: var(--muted);
}

.nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  max-width: 880px;
  margin: 24px auto 0;
}

.dots {
  display: flex;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.dots li {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--line);
  transition: background var(--motion) var(--ease);
}

.dots li.on {
  background: var(--accent);
}

.ahead {
  display: flex;
  gap: 8px;
}

.sub {
  margin: -16px 0 24px;
  color: var(--muted);
}

.what {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.what .state {
  font-size: 12px;
}

/* A phone: one column, the way through at the thumb. */
.phone .content {
  justify-content: flex-start;
}
</style>
