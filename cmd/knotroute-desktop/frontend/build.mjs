import { execFileSync } from 'node:child_process';
import { cpSync, mkdirSync, rmSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const root = dirname(fileURLToPath(import.meta.url));
const dist = join(root, 'dist');
const tscEntry = join(root, 'node_modules', 'typescript', 'bin', 'tsc');

rmSync(dist, { recursive: true, force: true });
mkdirSync(join(dist, 'assets'), { recursive: true });

// Invoke the TypeScript JS entrypoint through Node itself. Running the
// node_modules/.bin/tsc.cmd shim with spawn/execFile is not portable on
// Windows and fails with EINVAL on Node 24 GitHub runners.
execFileSync(process.execPath, [tscEntry, '-p', join(root, 'tsconfig.json')], {
  stdio: 'inherit',
  cwd: root,
});

cpSync(join(root, 'index.html'), join(dist, 'index.html'));
cpSync(join(root, 'src', 'app.css'), join(dist, 'assets', 'app.css'));
console.log('desktop frontend built');
