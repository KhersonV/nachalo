/** Real Three.js mixer + real GLB animation tracks; no browser/GPU required. */
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import ts from 'typescript';
import * as THREE from 'three';
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';

const out = new URL('../artifacts/ranger-lab/', import.meta.url);
await fs.mkdir(out, { recursive: true });
globalThis.ProgressEvent ??= class { constructor(type, init) { this.type = type; Object.assign(this, init); } };
const input = await fs.readFile(new URL('../src/components/AnimatedRanger.ts', import.meta.url), 'utf8');
const code = ts.transpileModule(input, { compilerOptions: { target: ts.ScriptTarget.ES2022,
  module: ts.ModuleKind.ES2022 } }).outputText.replace(/from "(three[^"]*)"/g,
  (_, name) => `from ${JSON.stringify(import.meta.resolve(name))}`)
  .replace(/import combatMetadata from [^;]+;/,`const combatMetadata=${await fs.readFile(new URL('../src/data/rangerCombat.json',import.meta.url),'utf8')};`);
const { AnimatedRanger } = await import('data:text/javascript;base64,' + Buffer.from(code).toString('base64'));
const nativeLoad = GLTFLoader.prototype.loadAsync;
const nativeParse = GLTFLoader.prototype.parseAsync;
const loads = [];
const disposals = [];
let delayNext = null;
async function parseAsset(url) {
  const bytes = await fs.readFile(new URL('../public' + url, import.meta.url));
  const size = bytes.readUInt32LE(12);
  const doc = JSON.parse(bytes.subarray(20, 20 + size).toString());
  const binary = bytes.subarray(28 + size);
  // Skip image decoding in Node only. Keep all real vertices, weights, skin,
  // transforms, keyframes and accessories. Browser checks use original images.
  delete doc.images; delete doc.textures; delete doc.materials; delete doc.samplers;
  delete doc.extensionsUsed; delete doc.extensionsRequired;
  for (const mesh of doc.meshes) for (const p of mesh.primitives) delete p.material;
  doc.buffers[0].uri = 'data:application/octet-stream;base64,' + binary.toString('base64');
  const gltf = await nativeParse.call(new GLTFLoader(), JSON.stringify(doc), '');
  const disposed = { url, geometries: 0, materials: 0 };
  disposals.push(disposed);
  const geometries = new Set(), materials = new Set();
  gltf.scene.traverse(o => { if (o.isMesh) { geometries.add(o.geometry);
    for (const m of Array.isArray(o.material) ? o.material : [o.material]) materials.add(m); } });
  for (const g of geometries) g.addEventListener('dispose', () => disposed.geometries++);
  for (const m of materials) m.addEventListener('dispose', () => disposed.materials++);
  return gltf;
}
GLTFLoader.prototype.loadAsync = async function(url) {
  loads.push(url);
  if (delayNext) { const delay = delayNext; delayNext = null; await delay; }
  return parseAsset(url);
};
async function ready(r) {
  for (let i = 0; i < 1000 && !r.isReady; i++) await new Promise(resolve => setTimeout(resolve, 10));
  assert.ok(r.isReady, 'asset loaded');
}
const result = {};
try {
  const ranger = new AnimatedRanger({ model: 'animated', position: { x: 0, y: 0 } });
  await ready(ranger);
  assert.equal(ranger.status.bones, 65);
  assert.equal(ranger.status.clip, 'Idle');
  assert.ok(Math.abs(ranger.clips.get('Walk').getClip().duration - 4 / 3) < 1e-5, 'no frame-1 hold');
  assert.ok(Math.abs(ranger.clips.get('Idle').getClip().duration - 3) < 1e-5);
  ranger.setSpec({ position: { x: 10, y: 0 } }, 0);
  const foot = ranger.root.getObjectByName('foot_l');
  assert.ok(foot);
  let previous = null;
  const stanceErrors = [];
  const weightErrors = [];
  const walk = ranger.clips.get('Walk');
  for (let i = 1; i <= 180; i++) {
    ranger.update(1 / 60, i / 60);
    ranger.root.updateMatrixWorld(true);
    const phase = walk.time / walk.getClip().duration;
    const point = foot.getWorldPosition(new THREE.Vector3());
    const weights = [...ranger.clips.values()].reduce((sum, a) => sum + a.getEffectiveWeight() * Number(a.enabled && a.isScheduled()), 0);
    weightErrors.push(Math.abs(weights - 1));
    if (previous && i > 15 && phase > .13 && phase < .47 && previous.phase > .13 && previous.phase < .47) {
      stanceErrors.push(Math.abs(point.x - previous.point.x) * 60);
    }
    previous = { phase, point };
    assert.equal(ranger.status.clip, 'Walk');
    assert.ok(Math.abs(ranger.status.speed - 2.75) < 1e-9);
    assert.equal(walk.getEffectiveTimeScale(),2);
    assert.ok(Math.abs(ranger.root.rotation.y - Math.PI / 2) < 1e-9);
  }
  assert.ok(stanceErrors.length > 5);
  const maxStanceDrift = Math.max(...stanceErrors);
  // A natural cadence at fixed game speed retains measurable foot sliding.
  assert.ok(maxStanceDrift > .6 && maxStanceDrift < 1.1, `stance compromise ${maxStanceDrift} units/s`);
  assert.ok(Math.max(...weightErrors) < 1e-5, 'crossfade weights never expose rest/T pose');
  for (let i = 181; i <= 280; i++) ranger.update(1 / 60, i / 60);
  assert.equal(ranger.status.clip, 'Idle');
  assert.equal(ranger.root.position.x, 10);
  for (const [x, z, heading] of [[10,1,0],[9,1,-Math.PI/2],[9,0,Math.PI],[10,0,Math.PI/2]]) {
    ranger.setSpec({ position: { x, y: z } }, 10);
    ranger.update(.01, 10);
    assert.ok(Math.abs(ranger.root.rotation.y - heading) < 1e-9, 'correct cardinal heading');
    for (let i = 0; i < 60; i++) ranger.update(1/60, 10+i/60);
    assert.equal(ranger.root.position.x, x); assert.equal(ranger.root.position.z, z);
  }
  result.animated = { bones: 65, headings: 4, rootSpeed: 2.75,
    walkTimeScale: 2, maxPlantedAnkleDriftPerSecond: maxStanceDrift,
    crossfadeWeightError: Math.max(...weightErrors), noFrameOneHold: true };

  const second = new AnimatedRanger({ model: 'animated', position: { x: 0, y: 0 } });
  await ready(second);
  assert.equal(loads.length, 1, 'simultaneous instances share one template');
  assert.notEqual(second.root.getObjectByName('Head'), ranger.root.getObjectByName('Head'), 'independent cloned skeleton');
  ranger.dispose(); ranger.dispose();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(disposals[0].geometries, 0, 'shared geometry lives until final owner');
  second.dispose();
  await new Promise(resolve => setImmediate(resolve));
  assert.ok(disposals[0].geometries > 0 && disposals[0].materials > 0);
  assert.equal(second.root.children.length, 0);

  const legacy = new AnimatedRanger({ position: { x: 0, y: 0 } });
  await ready(legacy);
  assert.equal(legacy.model, 'legacy');
  legacy.setSpec({ position: { x: 1, y: 0 } }, 0); legacy.update(.02, .02);
  assert.equal(legacy.status.walkTimeScale, 2.5);
  assert.equal(legacy.root.position.x, .055);
  legacy.setSpec({ position: { x: 1, y: 0 }, cues: [{ id: 'death', clip: 'Death' }] }, .03);
  const last = legacy.root.position.clone(); legacy.update(.2, .23);
  assert.equal(legacy.status.clip, 'Death'); assert.ok(legacy.root.position.equals(last));
  legacy.dispose();
  result.legacy = { defaultModelUnchanged: true, walkTimeScale: 2.5, deathStopsMovement: true };

  let finish;
  delayNext = new Promise(resolve => { finish = resolve; });
  const late = new AnimatedRanger({ model: 'animated', position: { x: 0, y: 0 } });
  late.dispose(); finish();
  for (let i=0; i<100 && disposals.length<3; i++) await new Promise(resolve => setTimeout(resolve,10));
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(late.root.children.length, 0);
  assert.ok(disposals.at(-1).geometries > 0, 'late asset released after disposal');
  result.lifecycle = { sharedAssetReferenceCounting: true, clonedSkeletons: true,
    idempotentDispose: true, lateLoadReleased: true };
  for (const model of ['animated','legacy']) {
    const defeated = new AnimatedRanger({model,position:{x:2,y:3},defeated:true});
    await ready(defeated);
    defeated.setSpec({position:{x:10,y:10},defeated:true},0);
    defeated.update(.2,.2);
    assert.equal(defeated.root.position.x,2);
    assert.equal(defeated.root.position.z,3);
    assert.equal(defeated.status.clip,'Death');
    defeated.setSpec({position:{x:10,y:10},defeated:false},1);
    defeated.update(.2,1.2);
    assert.equal(defeated.root.position.x,2,'a later visual update cannot revive a defeated actor');
    defeated.dispose();
  }
  result.authoritativeDefeat = { bothModelsStop: true, animatedUsesDeathClip: true,
    lateLoadAndModelSwitchRemainDefeated: true, cannotReviveFromVisualSpec: true };
  await fs.writeFile(new URL('mixer-validation.json', out), JSON.stringify(result,null,2));
  console.log('RANGER_MIXER_TESTS_OK', JSON.stringify(result,null,2));
} finally { GLTFLoader.prototype.loadAsync = nativeLoad; }
