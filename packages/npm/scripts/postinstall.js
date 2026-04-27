#!/usr/bin/env node

const fs = require("node:fs");
const fsp = require("node:fs/promises");
const crypto = require("node:crypto");
const https = require("node:https");
const path = require("node:path");
const { pipeline } = require("node:stream/promises");
const { spawn } = require("node:child_process");

const pkg = require("../package.json");

const owner = process.env.SKILL_ORGANIZER_GITHUB_OWNER || "sergiocarracedo";
const repo = process.env.SKILL_ORGANIZER_GITHUB_REPO || "skill-organizer";
const version = pkg.version;
const tag = `v${version}`;
const osMap = {
  linux: "Linux",
  darwin: "Darwin",
  win32: "Windows"
};
const archMap = {
  x64: "x86_64",
  arm64: "arm64"
};

async function main() {
  const osName = osMap[process.platform];
  const archName = archMap[process.arch];
  if (!osName || !archName) {
    throw new Error(`unsupported platform: ${process.platform}/${process.arch}`);
  }

  const ext = process.platform === "win32" ? "zip" : "tar.gz";
  const archiveName = `skill-organizer_${version}_${osName}_${archName}.${ext}`;
  const url = `https://github.com/${owner}/${repo}/releases/download/${tag}/${archiveName}`;
  const checksumsUrl = `https://github.com/${owner}/${repo}/releases/download/${tag}/checksums.txt`;
  const tmpDir = path.join(__dirname, "..", ".tmp");
  const vendorDir = path.join(__dirname, "..", "vendor");
  const archivePath = path.join(tmpDir, archiveName);
  const checksumsPath = path.join(tmpDir, "checksums.txt");

  await fsp.rm(tmpDir, { recursive: true, force: true });
  await fsp.mkdir(tmpDir, { recursive: true });
  await fsp.mkdir(vendorDir, { recursive: true });

  console.log(`Downloading ${url}`);
  await download(url, archivePath);
  console.log(`Downloading ${checksumsUrl}`);
  await download(checksumsUrl, checksumsPath);
  await verifyChecksum(archivePath, checksumsPath, archiveName);

  if (process.platform === "win32") {
    await unzip(archivePath, vendorDir);
  } else {
    await untar(archivePath, vendorDir);
  }

  const binaryName = process.platform === "win32" ? "skill-organizer.exe" : "skill-organizer";
  const extracted = path.join(vendorDir, binaryName);
  if (!fs.existsSync(extracted)) {
    throw new Error(`binary not found after extraction: ${extracted}`);
  }

  if (process.platform !== "win32") {
    await fsp.chmod(extracted, 0o755);
  }

  await fsp.rm(tmpDir, { recursive: true, force: true });
}

function download(url, destination) {
  return new Promise((resolve, reject) => {
    https.get(url, (response) => {
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        download(response.headers.location, destination).then(resolve, reject);
        return;
      }

      if (response.statusCode !== 200) {
        reject(new Error(`download failed: ${response.statusCode} ${response.statusMessage}`));
        response.resume();
        return;
      }

      const file = fs.createWriteStream(destination);
      pipeline(response, file).then(resolve, reject);
    }).on("error", reject);
  });
}

function untar(archivePath, destination) {
  return run("tar", ["-xzf", archivePath, "-C", destination]);
}

function unzip(archivePath, destination) {
  return run("powershell", [
    "-NoProfile",
    "-Command",
    `Expand-Archive -LiteralPath '${archivePath.replace(/'/g, "''")}' -DestinationPath '${destination.replace(/'/g, "''")}' -Force`
  ]);
}

function run(command, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: "inherit" });
    child.on("exit", (code) => {
      if (code === 0) {
        resolve();
        return;
      }
      reject(new Error(`${command} exited with code ${code}`));
    });
    child.on("error", reject);
  });
}

async function verifyChecksum(archivePath, checksumsPath, archiveName) {
  const content = await fsp.readFile(checksumsPath, "utf8");
  const line = content
    .split(/\r?\n/)
    .find((entry) => entry.trim().endsWith(`  ${archiveName}`) || entry.trim().endsWith(` ${archiveName}`));

  if (!line) {
    throw new Error(`checksum entry not found for ${archiveName}`);
  }

  const expected = line.trim().split(/\s+/)[0].toLowerCase();
  const actual = await sha256File(archivePath);
  if (expected !== actual) {
    throw new Error(`checksum mismatch for ${archiveName}`);
  }
}

function sha256File(filePath) {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash("sha256");
    const stream = fs.createReadStream(filePath);
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("end", () => resolve(hash.digest("hex")));
    stream.on("error", reject);
  });
}

main().catch((error) => {
  console.error(`Failed to install skill-organizer: ${error.message}`);
  process.exit(1);
});
