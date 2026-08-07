import { cp, mkdir, rm } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const root = dirname(fileURLToPath(import.meta.url));
const output = resolve(root, "../internal/overlay/assets");
const tsc = process.platform === "win32" ? "tsc.cmd" : "tsc";
const result = spawnSync(tsc, ["-p", resolve(root, "tsconfig.json")], { stdio: "inherit" });
if (result.status !== 0) process.exit(result.status ?? 1);
await rm(output, { recursive: true, force: true });
await mkdir(output, { recursive: true });
await cp(resolve(root, "static"), output, { recursive: true });
await cp(resolve(root, ".build/app.js"), resolve(output, "app.js"));
await rm(resolve(root, ".build"), { recursive: true, force: true });
console.log(`dashboard built into ${output}`);
