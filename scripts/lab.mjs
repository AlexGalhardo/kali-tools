import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../", import.meta.url));
const project = `kali-tools-lab-${process.pid}`;
function compose(args, timeout) {
  const result = spawnSync("docker", ["compose", "--project-name", project, "-f", "docker/compose.yml", ...args], {
    cwd: root, stdio: "inherit", timeout,
  });
  if (result.error) console.error(result.error.message);
  return result.status ?? 1;
}

export function runLab(run = compose) {
  let status = 1;
  try {
    status = run(["build"], 600_000);
    if (status === 0) status = run(["up", "--no-build", "--abort-on-container-exit", "--exit-code-from", "test"], 180_000);
  } finally {
    const cleanup = run(["down", "--volumes", "--remove-orphans", "--timeout", "5"], 30_000);
    if (status === 0) status = cleanup;
  }
  return status;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = runLab();
}
