import { execFileSync } from 'node:child_process';
import { cpSync, mkdirSync, rmSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const root = dirname(fileURLToPath(import.meta.url));
const dist = join(root, 'dist');
const tsc = process.platform === 'win32'
  ? join(root, 'node_modules', '.bin', 'tsc.cmd')
  : join(root, 'node_modules', '.bin', 'tsc');

rmSync(dist, { recursive: true, force: true });
mkdirSync(join(dist, 'assets'), { recursive: true });
execFileSync(tsc, ['-p', join(root, 'tsconfig.json')], { stdio: 'inherit', cwd: root });
cpSync(join(root, 'index.html'), join(dist, 'index.html'));
cpSync(join(root, 'src', 'app.css'), join(dist, 'assets', 'app.css'));
console.log('desktop frontend built');
