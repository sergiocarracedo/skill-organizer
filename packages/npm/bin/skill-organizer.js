#!/usr/bin/env node

const { spawn } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const extension = process.platform === "win32" ? ".exe" : "";
const binaryPath = path.join(__dirname, "..", "vendor", `skill-organizer${extension}`);

if (!fs.existsSync(binaryPath)) {
  console.error("skill-organizer binary is missing. Reinstall the package without --ignore-scripts.");
  process.exit(1);
}

const child = spawn(binaryPath, process.argv.slice(2), { stdio: "inherit" });
child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});
