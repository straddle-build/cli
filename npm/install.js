#!/usr/bin/env node
// Postinstall for @straddleio/cli: downloads the straddle binary for this
// platform from GitHub Releases, verifies its sha256 against the release
// checksums.txt, and unpacks it into vendor/. Node builtins only.
'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const pkg = require('./package.json');

const REPO = 'straddle-build/cli';

function fail(msg) {
  console.error(`@straddleio/cli install: ${msg}`);
  process.exit(1);
}

function goPlatform() {
  const os = { darwin: 'darwin', linux: 'linux', win32: 'windows' }[process.platform];
  const arch = { x64: 'amd64', arm64: 'arm64' }[process.arch];
  if (!os || !arch) {
    fail(
      `unsupported platform ${process.platform}/${process.arch} — ` +
        `download a binary from https://github.com/${REPO}/releases`
    );
  }
  return { os, arch };
}

async function download(url) {
  const res = await fetch(url, { redirect: 'follow' });
  if (!res.ok) fail(`download failed (HTTP ${res.status}) for ${url}`);
  return Buffer.from(await res.arrayBuffer());
}

async function main() {
  const version = pkg.version;
  if (!version || version === '0.0.0') {
    fail('package version is unstamped; install a released @straddleio/cli version');
  }
  const { os, arch } = goPlatform();
  const ext = os === 'windows' ? 'zip' : 'tar.gz';
  const archive = `straddle_${version}_${os}_${arch}.${ext}`;
  const base = `https://github.com/${REPO}/releases/download/v${version}`;

  const [archiveBuf, checksums] = await Promise.all([
    download(`${base}/${archive}`),
    download(`${base}/checksums.txt`),
  ]);

  const line = checksums
    .toString('utf8')
    .split('\n')
    .find((l) => l.trim().endsWith(archive));
  if (!line) fail(`no checksum for ${archive} in checksums.txt`);
  const expected = line.trim().split(/\s+/)[0];
  const actual = crypto.createHash('sha256').update(archiveBuf).digest('hex');
  if (actual !== expected) {
    fail(`checksum mismatch for ${archive}: expected ${expected}, got ${actual}`);
  }

  const vendorDir = path.join(__dirname, 'vendor');
  fs.rmSync(vendorDir, { recursive: true, force: true });
  fs.mkdirSync(vendorDir, { recursive: true });

  const archivePath = path.join(vendorDir, archive);
  fs.writeFileSync(archivePath, archiveBuf);

  const bin = os === 'windows' ? 'straddle.exe' : 'straddle';
  let result;
  if (os === 'windows') {
    result = spawnSync(
      'powershell',
      [
        '-NoProfile',
        '-Command',
        `Expand-Archive -Path "${archivePath}" -DestinationPath "${vendorDir}" -Force`,
      ],
      { stdio: 'inherit' }
    );
  } else {
    result = spawnSync('tar', ['-xzf', archivePath, '-C', vendorDir, bin], {
      stdio: 'inherit',
    });
  }
  if (!result || result.status !== 0) fail(`could not extract ${archive}`);

  const binPath = path.join(vendorDir, bin);
  if (!fs.existsSync(binPath)) fail(`archive did not contain ${bin}`);
  fs.chmodSync(binPath, 0o755);
  fs.rmSync(archivePath, { force: true });
  console.log(`@straddleio/cli: installed straddle v${version} (${os}/${arch})`);
}

main().catch((err) => fail(err && err.message ? err.message : String(err)));
