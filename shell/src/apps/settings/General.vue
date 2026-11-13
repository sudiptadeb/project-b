<script setup>
import { computed } from 'vue';
import { phone } from '@/phone.js';
import Segmented from '@/ui/Segmented.vue';
import { send, state } from './state.js';

// General settings: what Go owns, shown, and the user's changes sent back. It changes only
// when Go says so.

// Frame colours to pick from. The default is a dustier teal; the rest are the frames of the
// Ulaa design's modes: Personal, Work, Kids, Open Season and Incognito. The shell derives
// its other tones from whichever is picked.
const presets = [
  { name: 'Dusty teal', color: '#4f9e9e' },
  { name: 'Teal', color: '#0faaaa' },
  { name: 'Blue', color: '#3470d0' },
  { name: 'Amber', color: '#faa804' },
  { name: 'Red', color: '#d82b1f' },
  { name: 'Slate', color: '#384562' },
];
const custom = computed(() => !presets.some((p) => p.color === state.windowColor));

function setColor(color) {
  send({ t: 'setFrameColor', text: color });
}

const themes = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
];

const layouts = [
  { value: 'vertical', label: 'Vertical' },
  { value: 'horizontal', label: 'Horizontal' },
];

// What the update manager is doing, or what the check the user asked for found, in one
// plain line. A failure's own words go in the line's tooltip.
const update = computed(() => {
  const s = state.update;
  if (s.check === 'checking') return { text: 'Checking…' };
  if (s.phase === 'downloading') return { text: `Downloading ${s.version}… ${percent.value}%` };
  if (s.phase === 'ready') return { text: phone ? `Install to update to ${s.version}` : `Relaunch to update to ${s.version}` };
  if (s.phase === 'failed') return { text: s.version ? `Couldn't update to ${s.version}.` : "Couldn't update.", detail: s.error, failed: true };
  if (s.check === 'current') return { text: 'Up to date' };
  if (s.check === 'failed') return { text: "Couldn't check for updates.", detail: s.checkError, failed: true };
  return { text: '' };
});

// How much of the release has arrived, as a whole percentage, while it downloads.
const percent = computed(() => {
  const u = state.update;
  return u.total ? Math.floor((100 * (u.done || 0)) / u.total) : 0;
});

const busy = computed(() => state.update.check === 'checking' || state.update.phase === 'downloading');
</script>

<template>
  <div>
    <section aria-labelledby="appearance">
      <h2 id="appearance">Appearance</h2>
      <div class="card">
        <div class="row">
          <span class="label">Theme</span>
          <Segmented label="Theme" :options="themes" :model-value="state.theme" @update:model-value="send({ t: 'setTheme', text: $event })" />
        </div>
        <div v-if="!phone" class="row">
          <span class="label">Tab layout</span>
          <Segmented label="Tab layout" :options="layouts" :model-value="state.tabLayout" @update:model-value="send({ t: 'setTabLayout', text: $event })" />
        </div>
      </div>
    </section>

    <section aria-labelledby="colours">
      <h2 id="colours">Colours</h2>
      <div class="card">
        <div class="row" role="radiogroup" aria-labelledby="frame-colour">
          <span id="frame-colour" class="label">Frame colour</span>
          <div class="swatches">
            <button
              v-for="p in presets"
              :key="p.color"
              type="button"
              role="radio"
              class="swatch"
              :aria-label="p.name"
              :title="p.name"
              :aria-checked="String(state.windowColor === p.color)"
              :style="{ background: p.color }"
              @click="setColor(p.color)"
            ></button>
            <label class="swatch custom" :class="{ on: custom }" title="Custom colour" :style="custom ? { background: state.windowColor } : null">
              <input type="color" aria-label="Custom colour" :value="state.windowColor" @input="setColor($event.target.value)" />
            </label>
          </div>
        </div>
      </div>
    </section>

    <section aria-labelledby="updates">
      <h2 id="updates">Updates</h2>
      <div class="card">
        <div class="row update-row">
          <span class="label">
            Version <span class="version">{{ state.running || 'Development build' }}</span>
            <span class="secondary" :class="{ failed: update.failed }" role="status" :title="update.detail">{{ update.text }}</span>
          </span>
          <!-- A ready release is applied from here: a relaunch on a desktop, Android's
               installer on a phone. Both start with the same message to Go. -->
          <button v-if="state.update.phase === 'ready'" type="button" class="text-button" @click="send({ t: 'relaunch' })">
            {{ phone ? 'Install update' : 'Relaunch' }}
          </button>
          <button v-else type="button" class="text-button" :disabled="busy" @click="send({ t: 'checkUpdate' })">
            {{ update.failed ? 'Try again' : 'Check for updates' }}
          </button>
          <span
            v-if="state.update.phase === 'downloading'"
            class="progress"
            role="progressbar"
            aria-label="Update download"
            :aria-valuenow="percent"
            aria-valuemin="0"
            aria-valuemax="100"
            :style="{ width: percent + '%' }"
          ></span>
        </div>
        <label class="row">
          <span class="label">Automatic updates</span>
          <input
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
  </div>
</template>

<style scoped>
.version {
  margin-left: 4px;
  color: var(--muted);
  font-weight: 400;
}

.secondary:empty {
  display: none;
}

.secondary.failed {
  color: var(--danger);
}

/* The download's progress: a line along the row's foot, in the accent, as the shell's
   bubble has. */
.update-row {
  position: relative;
}

.progress {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 2px;
  background: var(--accent);
}

.swatches {
  display: flex;
  gap: 8px;
}

.swatch {
  position: relative;
  flex: none;
  width: 22px;
  height: 22px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.12);
  cursor: pointer;
}

/* The current colour has a ring in its own frame colour, a gap from the surface. */
.swatch[aria-checked='true'],
.swatch.custom.on {
  box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--frame);
}

.swatch:focus-visible,
.swatch.custom:focus-within {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}

/* Custom: a rainbow until it is the colour in use. The colour input fills it, invisible,
   so a click opens the picker. */
.swatch.custom {
  background: conic-gradient(#d82b1f, #faa804, #4f9e9e, #3470d0, #8a4fd0, #d82b1f);
}

.swatch.custom input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  border: 0;
  opacity: 0;
  cursor: pointer;
}

/* On a phone the colours take a line of their own, at a finger's size. */
.touch .swatch {
  width: 32px;
  height: 32px;
}

.touch .swatches {
  flex-basis: 100%;
  flex-wrap: wrap;
  gap: 12px;
}
</style>
