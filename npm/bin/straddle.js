#!/usr/bin/env node
// Launcher for @straddleio/cli: executes the vendored straddle binary,
// installing it first if the postinstall never ran (--ignore-scripts).
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const bin = process.platform === 'win32' ? 'straddle.exe' : 'straddle';
const binPath = path.join(__dirname, '..', 'vendor', bin);

if (!fs.existsSync(binPath)) {
  const installer = path.join(__dirname, '..', 'install.js');
  const result = spawnSync(process.execPath, [installer], { stdio: 'inherit' });
  if (result.status !== 0 || !fs.existsSync(binPath)) {
    console.error('@straddleio/cli: binary install failed; see errors above');
    process.exit(result.status === null || result.status === 0 ? 1 : result.status);
  }
}

const child = spawnSync(binPath, process.argv.slice(2), { stdio: 'inherit' });
process.exit(child.status === null ? 1 : child.status);
