import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import vm from "node:vm";

const require = createRequire(import.meta.url);
const ts = require("typescript");
const source = readFileSync(resolve("src/types/geometry.ts"), "utf8");
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  reportDiagnostics: true,
});
assert.equal(compiled.diagnostics?.filter(d => d.category === ts.DiagnosticCategory.Error).length, 0);
const module = { exports: {} };
vm.runInNewContext(compiled.outputText, { exports: module.exports, module, Error, Number, Object, Array, JSON, String }, { filename: "geometry.js" });
const { assertSquareSnapshot, assertSquarePatch, parseGeometry, parsePosition } = module.exports;

const legacy = { map: [{ x: 0, y: 0, cell_id: 0 }], players: [{ position: { x: 0, y: 0 } }], start_positions: [[0, 0]], portal_position: [0, 0] };
assertSquareSnapshot(legacy);
assert.deepEqual(JSON.parse(JSON.stringify(parseGeometry(undefined, undefined))), { kind: "square", coordinates: "cartesian", version: 1 });
assert.deepEqual(JSON.parse(JSON.stringify(parsePosition({ q: -4, r: -9 }, { kind: "hex", coordinates: "axial", orientation: "pointy", version: 1 }))), { q: -4, r: -9 });
for (const bad of [
  { ...legacy, schemaVersion: 2, geometry: { kind: "hex", coordinates: "axial", orientation: "pointy", version: 1 } },
  { ...legacy, schemaVersion: 9 },
  { ...legacy, map: [{ x: 0, y: 0, q: 0 }] },
  { ...legacy, players: [{ position: { x: 0 } }] },
]) assert.throws(() => assertSquareSnapshot(bad));
assert.throws(() => parsePosition({ q: 0, r: 0, x: 0 }, { kind: "hex", coordinates: "axial", orientation: "pointy", version: 1 }));
assert.throws(() => assertSquarePatch({ newPosition: { q: 0, r: 0 } }));
assert.throws(() => assertSquarePatch({ effects: [{ positionAfter: { q: -1, r: 2 } }] }));
console.log("geometry contract: PASS");
