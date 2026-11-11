// The extensions UI, in a build with EXTENSIONS=1. The shell and the settings page import
// it as @extensions; a build without extensions gets src/extensions-off instead, with the
// same names and nothing in them.
import ExtensionsSettings from './ExtensionsSettings.vue';
import ExtensionsToolbar from './ExtensionsToolbar.vue';
import { extensions } from './state.js';

export { ExtensionsToolbar };
export { extensionsMessage } from './state.js';

// The settings page's Extensions section, shown while Go sends the extension manager: only
// with the Extensions flag on.
export const extensionsSection = {
  id: 'extensions',
  label: 'Extensions',
  component: ExtensionsSettings,
  shown: () => extensions.value != null,
};
