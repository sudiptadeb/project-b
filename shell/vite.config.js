import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// `npm run dev` gives hot reload against core/cmd/dev running on :8765.
const dev = 'http://127.0.0.1:8765';

export default defineConfig({
  plugins: [vue()],
  base: './',
  build: {
    target: 'esnext',
    assetsInlineLimit: 0,
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        entryFileNames: 'shell.js',
        assetFileNames: 'shell.[ext]',
      },
    },
  },
  server: {
    proxy: { '/shell': dev, '/mock': dev },
  },
});
