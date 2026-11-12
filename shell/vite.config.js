import { readFileSync, readdirSync } from 'fs';
import { resolve } from 'path';
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// Each page is an app of its own, as pweb's pages are: src/apps/<app>/index.html, with
// what apps share in src/ (imported as @/). `VITE_APP=<app> vite build` builds one app into
// dist/<app>/ as index.html, <app>.js and <app>.css; `npm run build` builds every app.
// `npm run dev` serves them all, with hot reload, against core/cmd/dev on :8765.
// Each build flag in core/flags/registry.json is @<id>: src/<id>, or src/<id>-off, which has
// the same names with nothing in them, when WITHOUT names it, as the ninja build does
// (engine/build_shell.py, which also points --outDir into the build folder).
const dev = 'http://127.0.0.1:8765';
const apps = resolve(__dirname, 'src/apps');
const without = (process.env.WITHOUT || '').split(',').filter((id) => id);
const builds = JSON.parse(readFileSync(resolve(__dirname, '../core/flags/registry.json'), 'utf8')).filter((f) => f.build);
const unknown = without.filter((id) => !builds.some((f) => f.id === id));
if (unknown.length) throw new Error(`WITHOUT names what isn't a build flag: ${unknown.join(', ')}`);
const buildFlags = Object.fromEntries(
  builds.map((f) => ['@' + f.id, resolve(__dirname, 'src', without.includes(f.id) ? f.id + '-off' : f.id)]),
);

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
    resolve: {
      alias: {
        ...buildFlags,
        '@': resolve(__dirname, 'src'),
      },
    },
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
