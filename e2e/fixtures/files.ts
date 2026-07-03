import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

// Writes a fixture file into a unique temp subdir and returns its path. The
// basename is preserved so the page shows the intended filename; the unique
// parent keeps parallel workers from clobbering each other. The temp dirs are
// tracked so the caller can drop them in test.afterAll via cleanupFixtureDirs()
// — the suite otherwise litters $TMPDIR across runs.
let fixtureCounter = 0;
const fixtureDirs: string[] = [];

export function fixture(name: string, contents: string): string {
  const dir = path.join(tmpdir(), `hd-e2e-${process.pid}-${fixtureCounter++}`);
  fixtureDirs.push(dir);
  mkdirSync(dir, { recursive: true });
  const p = path.join(dir, name);
  writeFileSync(p, contents);
  return p;
}

// Drops every temp dir created by fixture(). Meant for test.afterAll.
export function cleanupFixtureDirs(): void {
  for (const dir of fixtureDirs) rmSync(dir, { recursive: true, force: true });
}
