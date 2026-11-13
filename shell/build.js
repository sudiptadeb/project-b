// Builds every app, one vite build each, into dist/<app>/ or, with --outDir, into that
// folder's <app>/ (engine/build_shell.py). --logLevel goes to vite.
//
//   node build.js [--outDir <dir>] [--logLevel <level>]
import { resolve } from 'path';
import { parseArgs } from 'util';
import { build } from 'vite';
import { apps } from './apps.js';

const { values } = parseArgs({ options: { outDir: { type: 'string' }, logLevel: { type: 'string' } } });
const out = resolve(values.outDir || resolve(import.meta.dirname, 'dist'));
for (const app of apps()) {
  process.env.VITE_APP = app;
  await build({
    configFile: resolve(import.meta.dirname, 'vite.config.js'),
    logLevel: values.logLevel,
    build: { outDir: resolve(out, app) },
  });
}
