<script setup>
import { computed, ref, watch } from 'vue';
import { send } from '@/apps/settings/state.js';
import ExtensionIcon from './ExtensionIcon.vue';
import InstallPrompt from './InstallPrompt.vue';
import PinIcon from './PinIcon.vue';
import { extensions } from './state.js';

// The extension manager: what is installed, and what the store offers. Go owns all of it;
// the page shows it and sends the user's changes back.
const view = computed(() => extensions.value);
const installed = computed(() => view.value.installed || []);
const pinned = (id) => (view.value.pinned || []).includes(id);

// The one extension whose details are open.
const open = ref('');

// The store's address as the user types it, until it is set.
const storeText = ref('');
watch(() => view.value.storeUrl, (url) => (storeText.value = url), { immediate: true });

// What a store item's button says: an install under way, or how the item compares with
// what is installed.
function installState(item) {
  if (item.phase === 'downloading') return { label: 'Downloading…', busy: true };
  if (item.phase === 'installing') return { label: 'Installing…', busy: true };
  const have = installed.value.find((e) => e.id === item.id);
  if (!have) return { label: 'Install', busy: false };
  if (have.version === item.version) return { label: 'Installed', busy: true };
  return { label: 'Update', busy: false };
}

// A package under a megabyte keeps a second decimal, so it doesn't read as nothing.
// A package's size as people read it: megabytes, or kilobytes for a small one.
const packageSize = (size) => (size < 1e6 ? Math.ceil(size / 1e3) + ' KB' : (size / 1e6).toFixed(1) + ' MB');
</script>

<template>
  <div>
    <section aria-labelledby="pbx-installed">
      <div class="pbx-heading">
        <h2 id="pbx-installed">Installed</h2>
        <button type="button" class="text-button" @click="send({ t: 'extensionLoadUnpacked' })">Load unpacked</button>
        <button type="button" class="text-button" @click="send({ t: 'extensionInstallPackage' })">Install package…</button>
      </div>
      <p v-if="view.error" class="pbx-message" role="alert">{{ view.error }}</p>
      <div class="card">
        <div v-if="!installed.length" class="row">
          <span class="label pbx-muted">No extensions installed</span>
        </div>
        <template v-for="e in installed" :key="e.id">
          <div class="row pbx-extension" :aria-label="e.name" role="group">
            <ExtensionIcon :src="e.icon" :name="e.name" :size="24" />
            <span class="label">
              {{ e.name }} <span class="pbx-version">{{ e.version }}</span>
              <span class="pbx-description">{{ e.description }}</span>
            </span>
            <button type="button" class="text-button pbx-quiet" :aria-expanded="open === e.id" @click="open = open === e.id ? '' : e.id">Details</button>
            <button
              type="button"
              class="icon-button"
              :class="{ 'pbx-pinned': pinned(e.id) }"
              :aria-label="(pinned(e.id) ? 'Unpin ' : 'Pin ') + e.name"
              :title="pinned(e.id) ? 'Unpin from the toolbar' : 'Pin to the toolbar'"
              @click="send({ t: 'extensionPin', extension: e.id, on: !pinned(e.id) })"
            >
              <PinIcon :on="pinned(e.id)" />
            </button>
            <input
              type="checkbox"
              role="switch"
              class="switch"
              :aria-label="`${e.name} on`"
              :checked="e.enabled"
              :aria-checked="String(e.enabled)"
              @change="send({ t: 'extensionSetEnabled', extension: e.id, on: $event.target.checked })"
            />
            <button type="button" class="text-button pbx-quiet" :aria-label="`Remove ${e.name}`" @click="send({ t: 'extensionRemove', extension: e.id })">Remove</button>
          </div>
          <dl v-if="open === e.id" class="pbx-details" :aria-label="`Details of ${e.name}`">
            <dt>ID</dt>
            <dd class="pbx-code">{{ e.id }}</dd>
            <dt>Version</dt>
            <dd>{{ e.version }}</dd>
            <dt>Source</dt>
            <dd>{{ e.unpacked ? `Unpacked, from ${e.path}` : 'Package' }}</dd>
            <dt>Permissions</dt>
            <dd>
              <ul v-if="e.permissions && e.permissions.length" class="pbx-code">
                <li v-for="p in e.permissions" :key="p">{{ p }}</li>
              </ul>
              <template v-else>None</template>
            </dd>
            <dt>Site access</dt>
            <dd>
              <ul v-if="e.hosts && e.hosts.length" class="pbx-code">
                <li v-for="h in e.hosts" :key="h">{{ h }}</li>
              </ul>
              <template v-else>None</template>
            </dd>
            <dt>Toolbar button</dt>
            <dd>{{ e.hasAction ? 'Yes' : 'No' }}</dd>
          </dl>
        </template>
      </div>
    </section>

    <section aria-labelledby="pbx-store">
      <h2 id="pbx-store">Store</h2>
      <div class="card">
        <form class="row" @submit.prevent="send({ t: 'extensionSetStore', text: storeText.trim() })">
          <label class="pbx-store-label" for="pbx-store-url">Address</label>
          <input id="pbx-store-url" v-model="storeText" class="pbx-url" autocomplete="off" autocapitalize="off" spellcheck="false" />
          <button type="submit" class="text-button">Set</button>
          <button type="button" class="text-button" :disabled="!view.storeUrl" @click="send({ t: 'extensionRefreshStore' })">Refresh</button>
        </form>
        <div v-if="!view.storeUrl" class="row">
          <span class="label pbx-muted">Set the store's URL to see what it offers.</span>
        </div>
        <div v-else-if="view.catalogueError" class="row">
          <span class="label pbx-message" role="alert">{{ view.catalogueError }}</span>
        </div>
        <div v-for="item in view.catalogue" :key="item.id" class="row pbx-extension" :aria-label="item.name" role="group">
          <ExtensionIcon :src="item.icon" :name="item.name" :size="24" />
          <span class="label">
            {{ item.name }} <span class="pbx-version">{{ item.version }}</span>
            <span class="pbx-description">{{ item.description }}</span>
          </span>
          <span class="pbx-muted pbx-size">{{ packageSize(item.size) }}</span>
          <button type="button" class="text-button" :disabled="installState(item).busy" @click="send({ t: 'extensionInstall', extension: item.id })">{{ installState(item).label }}</button>
        </div>
      </div>
    </section>

    <InstallPrompt v-if="view.prompt" :prompt="view.prompt" :send="send" />
  </div>
</template>

<style scoped>
/* Like Chrome's developer bar: the ways to add an extension sit beside its heading. */
.pbx-heading {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pbx-heading h2 {
  flex: 1;
}

.pbx-message {
  margin: 8px 0 0;
  color: var(--danger);
  font-size: 13px;
  font-weight: 400;
}

.pbx-muted {
  color: var(--muted);
  font-weight: 400;
}

.pbx-version {
  margin-left: 4px;
  color: var(--muted);
  font-weight: 400;
}

.pbx-description {
  display: block;
  overflow: hidden;
  color: var(--muted);
  font-size: 12px;
  font-weight: 400;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.pbx-extension {
  gap: 12px;
}

.pbx-size {
  flex: none;
  font-size: 12px;
}

.pbx-quiet {
  border-color: transparent;
  background: none;
  color: var(--muted);
}

.pbx-quiet:hover:not(:disabled),
.pbx-quiet[aria-expanded='true'] {
  background: var(--accent-tint);
  color: inherit;
}

.pbx-pinned {
  color: var(--accent);
}

/* The details open under their row, on the tint an open keyboard action has. */
.pbx-details {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 6px 16px;
  margin: 0;
  padding: 10px 14px 12px 50px;
  border-top: 1px solid var(--line);
  background: var(--accent-tint);
  font-size: 13px;
}

.pbx-details dt {
  color: var(--muted);
}

.pbx-details dd {
  min-width: 0;
  margin: 0;
  overflow-wrap: anywhere;
}

.pbx-details ul {
  margin: 0;
  padding: 0;
  list-style: none;
}

.pbx-details + .row {
  border-top: 1px solid var(--line);
}

.pbx-code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
}

.pbx-store-label {
  flex: none;
  font-weight: 500;
}

.pbx-url {
  flex: 1;
  min-width: 0;
  height: 30px;
  padding: 0 8px;
  border: 1px solid var(--line);
  border-radius: 6px;
  box-sizing: border-box;
  background: var(--surface);
  color: inherit;
  font: inherit;
  font-size: 13px;
}

.pbx-url:focus-visible {
  outline: none;
  box-shadow: 0 0 0 1px var(--accent), 0 0 0 3px var(--accent-soft);
}
</style>
