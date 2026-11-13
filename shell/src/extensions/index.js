// The extensions UI. The shell and the settings page import it as @extensions; a build
// with WITHOUT=extensions gets src/extensions-off instead, with the same names and nothing
// in them (vite.config.js).
import ExtensionsSettings from './ExtensionsSettings.vue';
import ExtensionsToolbar from './ExtensionsToolbar.vue';
import { extensions } from './state.js';

export { ExtensionsToolbar };
export { extensionsMessage } from './state.js';

// Whether the build has extensions. A shell that draws the extensions' buttons itself, not
// with ExtensionsToolbar, draws them only when it has, so a build without them drops that
// code too.
export const extensionsBuilt = true;

// The settings page's Extensions section, shown while Go sends the extension manager: only
// with the Extensions flag on.
export const extensionsSection = {
  id: 'extensions',
  label: 'Extensions',
  component: ExtensionsSettings,
  shown: () => extensions.value != null,
};
