<script setup>
import { computed, ref } from 'vue';
import { send } from '@/apps/settings/state.js';
import ExtensionIcon from './ExtensionIcon.vue';
import InstallPrompt from './InstallPrompt.vue';
import PinIcon from './PinIcon.vue';
import { extensions } from './state.js';

// The extension manager: what is installed, the ways to add one from this machine, and the
// way to the store. Go owns all of it; the page shows it and sends the user's changes back.
const view = computed(() => extensions.value);
const installed = computed(() => view.value.installed || []);
const pinned = (id) => (view.value.pinned || []).includes(id);

// The one extension whose details are open.
const open = ref('');
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
            <button type="button" class="text-button quiet" :aria-expanded="open === e.id" @click="open = open === e.id ? '' : e.id">Details</button>
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
            <button type="button" class="text-button quiet" :aria-label="`Remove ${e.name}`" @click="send({ t: 'extensionRemove', extension: e.id })">Remove</button>
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

    <!-- The store is a website: an extension is installed from its own page. -->
    <section aria-labelledby="pbx-store">
      <h2 id="pbx-store">Store</h2>
      <div class="card">
        <div class="row">
          <span class="label">Chrome Web Store</span>
          <button type="button" class="text-button" @click="send({ t: 'extensionOpenStore' })">Open</button>
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

</style>
