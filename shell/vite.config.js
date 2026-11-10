import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// `npm run dev` gives hot reload against core/cmd/dev running on :8765.
const dev = 'http://127.0.0.1:8765';

// Two pages, built one at a time so each is one script and one stylesheet: the shell
// (index.html), and with `--mode settings` the settings page (settings.html).
export default defineConfig(({ mode }) => {
  const name = mode === 'settings' ? 'settings' : 'shell';
  return {
    plugins: [vue()],
    base: './',
    build: {
      target: 'esnext',
      assetsInlineLimit: 0,
      cssCodeSplit: false,
      emptyOutDir: name === 'shell',
      rollupOptions: {
        input: name === 'settings' ? 'settings.html' : 'index.html',
        output: {
          entryFileNames: `${name}.js`,
          assetFileNames: `${name}.[ext]`,
        },
      },
    },
    server: {
      proxy: { '/shell': dev, '/mock': dev, '/settings/': dev },
    },
  };
});
