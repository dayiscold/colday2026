// Run the actual Go router with disposable data; never use the developer's store.
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawn, spawnSync } from 'node:child_process';

const directory = mkdtempSync(join(tmpdir(), 'colday-public-e2e-'));
const binary = join(directory, process.platform === 'win32' ? 'colday.exe' : 'colday');
const build = spawnSync('go', ['build', '-buildvcs=false', '-o', binary, '.'], { stdio: 'inherit' });
if (build.status !== 0) {
  rmSync(directory, { recursive: true, force: true });
  process.exit(build.status ?? 1);
}
const server = spawn(binary, [], {
  stdio: 'inherit',
  env: {
    ...process.env,
    COLDAY_ADDR: '127.0.0.1:18765',
    COLDAY_ADMIN_PASSWORD: 'test-only-public-preview',
    COLDAY_SESSION_KEY: 'ab'.repeat(32),
    COLDAY_COOKIE_SECURE: '0',
    COLDAY_DATA_FILE: join(directory, 'data.json'),
    COLDAY_UPLOADS_DIR: join(directory, 'uploads'),
    COLDAY_STATIC_DIR: resolve('static'),
  },
});
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => server.kill());
server.on('exit', code => {
  rmSync(directory, { recursive: true, force: true });
  process.exit(code ?? 0);
});
server.on('error', error => {
  console.error(error);
  rmSync(directory, { recursive: true, force: true });
  process.exit(1);
});
