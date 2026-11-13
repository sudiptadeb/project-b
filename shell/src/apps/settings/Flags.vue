<script setup>
import Field from '@/ui/Field.vue';
import { send, state } from './state.js';

// Flags, as core/flags/registry.json defines them, each drawn by its type: a switch for a
// boolean, a select for an enum, a field for the rest. A change waits for the next launch,
// so Relaunch shows beside each flag whose saved value isn't the one running.

function set(flag, text) {
  send({ t: 'setFlag', id: flag.id, text });
}

// pattern is a flag's pattern as a JavaScript one. Go's syntax and JavaScript's agree on
// the simple patterns the registry has; one they don't is left to Go to check.
function pattern(flag) {
  try {
    return flag.pattern ? new RegExp(flag.pattern) : null;
  } catch {
    return null;
  }
}

// error is what is wrong with a value typed for a flag, as Go's flags.normal checks it,
// or ''. Empty is Off, and always good.
function error(flag, text) {
  if (!text) return '';
  const list = flag.type === 'numbers' || flag.type === 'strings';
  const items = list ? text.split(',').map((s) => s.trim()) : [text];
  if (flag.type === 'number' || flag.type === 'numbers') {
    const bad = items.find((s) => !/^[+-]?\d+$/.test(s) || (flag.min != null && +s < flag.min) || (flag.max != null && +s > flag.max));
    if (bad === undefined) return '';
    const n = list ? 'whole numbers' : 'a whole number';
    if (flag.min != null && flag.max != null) return `Use ${n} from ${flag.min} to ${flag.max}`;
    if (flag.min != null) return `Use ${n} of ${flag.min} or more`;
    if (flag.max != null) return `Use ${n} up to ${flag.max}`;
    return `Use ${n}`;
  }
  const p = pattern(flag);
  const bad = items.find((s) => !s || (p && !p.test(s)));
  if (bad === undefined) return '';
  return bad ? `"${bad}" isn't a value this flag takes` : 'Leave no empty value between commas';
}
</script>

<template>
  <section aria-labelledby="flags">
    <h2 id="flags">Flags</h2>
    <div class="card">
      <div v-for="f in state.flags" :key="f.id" class="row">
        <label class="label" :for="'flag-' + f.id">
          {{ f.name }}
          <span class="note">{{ f.built ? f.description : 'Not in this build' }}</span>
        </label>
        <button v-if="f.pending" type="button" class="text-button primary" @click="send({ t: 'relaunch' })">Relaunch</button>
        <input
          v-if="f.type === 'boolean'"
          :id="'flag-' + f.id"
          type="checkbox"
          role="switch"
          class="switch"
          :disabled="!f.built"
          :checked="f.value === 'on'"
          :aria-checked="String(f.value === 'on')"
          @change="set(f, $event.target.checked ? 'on' : '')"
        />
        <select v-else-if="f.type === 'enum'" :id="'flag-' + f.id" class="value" :disabled="!f.built" :value="f.value" @change="set(f, $event.target.value)">
          <option value="">Off</option>
          <option v-for="c in f.choices" :key="c" :value="c">{{ c }}</option>
        </select>
        <!-- A flag the build left out can't be set, so its field is a plain disabled one. -->
        <input v-else-if="!f.built" :id="'flag-' + f.id" class="field value" placeholder="Off" disabled :value="f.value" />
        <Field
          v-else
          class="value"
          :label="f.name"
          placeholder="Off"
          :value="f.value"
          :validate="(text) => error(f, text)"
          @commit="set(f, $event)"
        />
      </div>
    </div>
  </section>
</template>

<style scoped>
.note {
  display: block;
  font-size: 12px;
  font-weight: 400;
  color: var(--muted);
}

.value {
  flex: none;
  width: 200px;
}

.touch .value {
  width: 100%;
}
</style>
