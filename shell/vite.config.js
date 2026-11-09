import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// One page, one build, from this one repo. `npm run build` builds the shell (index.html,
// shell.js, shell.css), then the settings page (settings.html, settings.js, settings.css)
// beside it; each stands alone. `npm run dev` gives hot reload for both against
// core/cmd/dev running on :8765.
const dev = 'http://127.0.0.1:8765';
const app = process.env.VITE_APP || 'shell';

export default defineConfig({
  plugins: [vue()],
  base: './',
  build: {
    target: 'esnext',
    assetsInlineLimit: 0,
    cssCodeSplit: false,
    // The shell's build empties dist; the settings page's adds its files beside it.
    emptyOutDir: app === 'shell',
    rollupOptions: {
      input: app === 'shell' ? 'index.html' : `${app}.html`,
      output: {
        entryFileNames: `${app}.js`,
        assetFileNames: `${app}.[ext]`,
      },
    },
  },
  server: {
    proxy: { '/shell': dev, '/mock': dev, '/tab': dev },
  },
});
