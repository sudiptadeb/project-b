<script setup>
import { computed } from 'vue';
import { phone } from '@/phone.js';
import Segmented from '@/ui/Segmented.vue';
import Swatches from '@/ui/Swatches.vue';
import { send, state } from './state.js';

// General settings: what Go owns, shown, and the user's changes sent back. It changes only
// when Go says so.

// Frame colours to pick from. The default is a dustier teal; the rest are the frames of the
// Ulaa design's modes: Personal, Work, Kids, Open Season and Incognito, with Work's blue
// the one the Work profile template sets, so a profile made from it shows a preset. The
// shell derives its other tones from whichever is picked.
const presets = [
  { name: 'Dusty teal', color: '#4f9e9e' },
  { name: 'Teal', color: '#0faaaa' },
  { name: 'Blue', color: '#5b7fd6' },
  { name: 'Amber', color: '#faa804' },
  { name: 'Red', color: '#d82b1f' },
  { name: 'Slate', color: '#384562' },
];

const themes = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
];

// A phone's address bar: at the foot, by the thumb, or on top.
const positions = [
  { value: 'top', label: 'Top' },
  { value: 'bottom', label: 'Bottom' },
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
        <div v-if="phone" class="row">
          <span class="label">Address bar</span>
          <Segmented label="Address bar" :options="positions" :model-value="state.addressBarPosition" @update:model-value="send({ t: 'setAddressBarPosition', text: $event })" />
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
        <div class="row">
          <span class="label">Frame colour</span>
          <Swatches
            label="Frame colour"
            custom
            :colors="presets"
            :model-value="state.windowColor"
            @update:model-value="send({ t: 'setFrameColor', text: $event })"
          />
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

/* On a phone the colours take a line of their own. */
.touch .ui-swatches {
  flex-basis: 100%;
  flex-wrap: wrap;
}
</style>
