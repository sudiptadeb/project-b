// The apps the build makes, each a folder of src/apps: settings and picker, and each shell,
// a folder of src/apps/shell (docs/shells.md). An app's name is its folder under src/apps,
// such as 'settings' or 'shell/standard'.
import { readdirSync } from 'fs';
import { resolve } from 'path';

const dir = resolve(import.meta.dirname, 'src/apps');

export function apps() {
  const shells = readdirSync(resolve(dir, 'shell')).map((id) => 'shell/' + id);
  return [...readdirSync(dir).filter((app) => app !== 'shell'), ...shells];
}
