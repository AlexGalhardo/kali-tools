import assert from "node:assert/strict";
import test from "node:test";
import { runLab } from "./lab.mjs";

for (const [name, codes, expected, commands] of [
  ["success cleans up", [0, 0, 0], 0, ["build", "up", "down"]],
  ["build failure cleans up", [3, 0], 3, ["build", "down"]],
  ["test failure survives cleanup", [0, 7, 0], 7, ["build", "up", "down"]],
  ["cleanup failure fails the run", [0, 0, 4], 4, ["build", "up", "down"]],
]) {
  test(name, () => {
    const calls = [];
    const status = runLab((args, timeout) => {
      calls.push(args[0]);
      assert.ok(timeout > 0 && timeout <= 600_000);
      return codes[calls.length - 1];
    });
    assert.equal(status, expected);
    assert.deepEqual(calls, commands);
  });
}
test("startup exception still cleans up", () => {
  const calls = [];
  assert.throws(() => runLab((args) => {
    calls.push(args[0]);
    if (args[0] === "build") throw Error("spawn failed");
    return 0;
  }), /spawn failed/);
  assert.deepEqual(calls, ["build", "down"]);
});
