// The extensions UI, in a build with EXTENSIONS=1. The shell and the settings page import
// it as @extensions; a build without extensions gets src/extensions-off instead, with the
// same names and nothing in them.
import ExtensionsToolbar from './ExtensionsToolbar.vue';

export { ExtensionsToolbar };
export { extensionsMessage } from './state.js';
