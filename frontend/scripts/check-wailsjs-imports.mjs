import { readdir, readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../src/", import.meta.url));
async function check(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const file = path.join(dir, entry.name);
    if (entry.isDirectory()) { await check(file); continue; }
    if (!/\.[cm]?[jt]sx?$/.test(file)) continue;
    const relative = path.relative(root, file).split(path.sep).join("/");
    if (relative !== "shared/api/transport.ts" && (await readFile(file, "utf8")).includes("wailsjs")) {
      console.error(`${relative}: import Wails only through shared/api/transport.ts`);
      process.exitCode = 1;
    }
  }
}
await check(root);
