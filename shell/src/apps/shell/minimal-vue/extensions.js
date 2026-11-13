// The toolbar's extensions, imported only in a build with extensions, so a build without has
// none of them, their styles too. The import is inlined, so the shell is still one script.
import { extensionsBuilt } from '@extensions';

export const Extensions = extensionsBuilt ? (await import('./Extensions.vue')).default : null;
