#!/usr/bin/env node
// Runs diskdiet with the given arguments: `npx diskdiet 3000`.
"use strict";

const { spawnSync } = require("node:child_process");
const { ensureBinary } = require("../lib/install.js");
const { version } = require("../package.json");
const checksums = require("../checksums.json");

ensureBinary({ version, checksums }).then(
  (bin) => {
    const r = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
    if (r.error) {
      console.error(`diskdiet: couldn't run diskdiet: ${r.error.message}`);
      process.exit(3);
    }
    if (r.signal) {
      process.kill(process.pid, r.signal); // e.g. Ctrl-C: exit the way diskdiet did
    }
    process.exit(r.status ?? 1);
  },
  (err) => {
    console.error(`diskdiet: ${err.message}`);
    process.exit(3);
  },
);
