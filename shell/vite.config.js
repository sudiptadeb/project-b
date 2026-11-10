import { readdirSync } from 'fs';
import { resolve } from 'path';
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// Each page is an app of its own, as pweb's pages are: src/apps/<app>/index.html, with
// what apps share in src/ (imported as @/). `VITE_APP=<app> vite build` builds one app into
// dist/<app>/ as index.html, <app>.js and <app>.css; `npm run build` builds every app.
// `npm run dev` serves them all, with hot reload, against core/cmd/dev on :8765.
const dev = 'http://127.0.0.1:8765';
const apps = resolve(__dirname, 'src/apps');

export default defineConfig(({ command }) => {
  const app = process.env.VITE_APP;
  if (command === 'build' && !app) {
    throw new Error('set VITE_APP to one of: ' + readdirSync(apps).join(', '));
  }
  return {
    plugins: [vue()],
    // A build's root is its app's folder, so its index.html lands at dist/<app>/index.html.
    root: resolve(apps, app || ''),
    base: './',
    resolve: { alias: { '@': resolve(__dirname, 'src') } },
    build: {
      target: 'esnext',
      assetsInlineLimit: 0,
      cssCodeSplit: false,
      outDir: resolve(__dirname, 'dist', app || ''),
      emptyOutDir: true,
      rollupOptions: {
        output: {
          entryFileNames: `${app}.js`,
          assetFileNames: `${app}.[ext]`,
        },
      },
    },
    server: {
      proxy: { '/shell/events': dev, '/shell/send': dev, '/settings/events': dev, '/settings/send': dev, '/mock': dev },
    },
  };
});
