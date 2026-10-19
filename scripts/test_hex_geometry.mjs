// Run from repository root: node scripts/test_hex_geometry.mjs
// Transpiles the isolated TS module in memory with the project's installed TypeScript.
import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import assert from 'node:assert/strict';
import vm from 'node:vm';
const require=createRequire(import.meta.url);
const ts=require('typescript');
const src=readFileSync(resolve('src/utils/hexGrid.ts'),'utf8');
const compiled=ts.transpileModule(src,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020},reportDiagnostics:true});
assert.equal(compiled.diagnostics?.filter(d=>d.category===ts.DiagnosticCategory.Error).length,0);
const mod={exports:{}};
vm.runInNewContext(compiled.outputText,{exports:mod.exports,module:mod,RangeError,Math,Number}, {filename:'hexGrid.js'});
const h=mod.exports;
const f=JSON.parse(readFileSync(resolve('docs/hex-migration/fixtures/geometry-v1.json'),'utf8'));
const pair=c=>[c.q,c.r];
for(let i=0;i<6;i++){assert.deepEqual(pair(h.HEX_DIRECTIONS[i]),f.directions[i]);assert.deepEqual(pair(h.hexNeighbor({q:0,r:0},i)),f.directions[i]);assert.deepEqual(pair(h.hexNeighbor(h.hexNeighbor({q:0,r:0},i),h.hexOpposite(i))),[0,0]);}
for(const v of f.distances)assert.equal(h.hexDistance({q:v.a[0],r:v.a[1]},{q:v.b[0],r:v.b[1]}),v.want);
for(const v of f.round)assert.deepEqual(pair(h.roundAxial(...v.input)),v.want);
for(let i=0;i<f.ringSizes.length;i++){assert.equal(h.hexRing({q:0,r:0},i).length,f.ringSizes[i]);assert.equal(h.hexDisk({q:0,r:0},i).length,f.diskSizes[i]);}
for(let q=-20;q<=20;q++)for(let r=-20;r<=20;r++){const c={q,r};assert.deepEqual(pair(h.worldToAxial(h.axialToWorld(c,f.radius),f.radius)),[q,r]);assert.deepEqual(pair(h.cubeToAxial(h.axialToCube(c))),[q,r]);}
for(const d of h.HEX_DIRECTIONS){const w=h.axialToWorld(d,f.radius);assert.ok(Math.abs(Math.hypot(w.x,w.z)-1)<1e-9);}
assert.deepEqual(h.canonicalHexEdge({q:0,r:0},{q:1,r:0}),h.canonicalHexEdge({q:1,r:0},{q:0,r:0}));
assert.throws(()=>h.roundAxial(NaN,0));assert.throws(()=>h.hexNeighbor({q:0,r:0},6));assert.throws(()=>h.canonicalHexEdge({q:0,r:0},{q:2,r:0}));
console.log('HEX geometry TypeScript shared fixtures: PASS');
