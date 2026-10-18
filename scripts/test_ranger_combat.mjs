/** Real GLB tracks and Three.js mixer. This is not a real-match test. */
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import ts from 'typescript';
import * as THREE from 'three';
import {GLTFLoader} from 'three/examples/jsm/loaders/GLTFLoader.js';
globalThis.ProgressEvent ??= class {constructor(type,init){Object.assign(this,{type},init);}};
const metadata=JSON.parse(await fs.readFile('src/data/rangerCombat.json','utf8'));
const input=await fs.readFile('src/components/AnimatedRanger.ts','utf8');
const code=ts.transpileModule(input,{compilerOptions:{target:99,module:99}}).outputText
  .replace(/import combatMetadata from [^;]+;/,`const combatMetadata=${JSON.stringify(metadata)};`)
  .replace(/from "(three[^"]*)"/g,(_,name)=>`from ${JSON.stringify(import.meta.resolve(name))}`);
const {AnimatedRanger}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));
const native=GLTFLoader.prototype.loadAsync, loads=[],disposals=[];
let delayed;
GLTFLoader.prototype.loadAsync=async function(url){
  loads.push(url);if(delayed){const p=delayed;delayed=null;await p;}
  const bytes=await fs.readFile('public'+url),size=bytes.readUInt32LE(12);
  const doc=JSON.parse(bytes.subarray(20,20+size)),binary=bytes.subarray(28+size);
  for(const k of ['images','textures','materials','samplers','extensionsUsed','extensionsRequired'])delete doc[k];
  for(const m of doc.meshes)for(const p of m.primitives)delete p.material;
  doc.buffers[0].uri='data:application/octet-stream;base64,'+binary.toString('base64');
  const gltf=await new GLTFLoader().parseAsync(JSON.stringify(doc),'');
  const disposed={geometries:0};disposals.push(disposed);
  const geometries=new Set();gltf.scene.traverse(o=>{if(o.isMesh)geometries.add(o.geometry);});
  for(const g of geometries)g.addEventListener('dispose',()=>disposed.geometries++);
  return gltf;
};
async function ready(r){for(let i=0;i<1000&&!r.isReady;i++)await new Promise(r=>setTimeout(r,10));assert.ok(r.isReady);}
const result={};
try{
  const r=new AnimatedRanger({model:'animated',position:{x:0,y:0}});await ready(r);
  assert.equal(r.status.bones,65);assert.equal(r.status.clip,'Idle');
  for(const [name,duration] of Object.entries(metadata.durations))assert.ok(Math.abs(r.clips.get(name).getClip().duration-duration)<1e-5,name);
  let now=0;
  const tick=(seconds)=>{const end=now+seconds;while(now+1e-6<end){const dt=Math.min(1/60,end-now);now+=dt;r.update(dt,now);}};
  for(const [x,y,heading] of [[1,0,Math.PI/2],[1,1,0],[0,1,-Math.PI/2],[0,0,Math.PI]]){
    const previousWalkTime=r.clips.get('Walk').time;
    r.setSpec({position:{x,y}},now);tick(.1);assert.equal(r.status.clip,'Walk');
    assert.ok(Math.abs(r.clips.get('Walk').time-(previousWalkTime+.2)%r.clips.get('Walk').getClip().duration)<1e-5,'walk phase continues between tiles');
    assert.ok(Math.abs(r.status.heading-heading)<1e-6);assert.equal(r.status.walkTimeScale,2);
    tick(.25);assert.ok(Math.hypot(r.status.x-x,r.status.z-y)>.03,'tile still moving after .35 s');
    tick(1/60);assert.equal(r.status.x,x);assert.equal(r.status.z,y);
    tick(.25-1/60);assert.equal(r.status.clip,'Idle');
  }
  result.locomotion={unchangedSpeed:2.75,walkMultiplier:2,directions:4};
  const start=now;
  r.setSpec({position:{x:0,y:0},cues:[{id:'shot',clip:'Attack_Bow',target:{x:0,y:2},startedAt:start,releaseAt:start+metadata.attackReleaseSeconds}]},now);
  tick(1.3);assert.equal(r.status.clip,'Attack_Bow');assert.equal(r.status.arrowVisible,true);
  const string=r.root.getObjectByName('NAC_FixedBowString');assert.ok(string.morphTargetInfluences[0]>.99);
  // Simulate a stalled frame: release still follows the existing timeline clock.
  now=start+metadata.attackReleaseSeconds+.01;r.update(.02,now);
  assert.equal(r.status.arrowVisible,false);assert.ok(Math.abs(r.status.clipTime-(metadata.attackReleaseSeconds+.01))<1e-5);
  assert.equal(string.morphTargetInfluences[0],0);tick(.7);assert.equal(r.status.clip,'Idle');
  for(let i=0;i<6;i++){
    r.setSpec({position:{x:0,y:0},cues:[{id:'hit'+i,clip:'Hit'}]},now);tick(.05);
    assert.equal(r.status.clip,'Hit');assert.ok([...r.clips.values()].filter(a=>a.isScheduled()&&a.enabled&&a.getEffectiveWeight()>0).length<=2);
  }
  tick(.6);assert.equal(r.status.clip,'Idle');
  r.setSpec({position:{x:0,y:0},cues:[{id:'hit-walk',clip:'Hit'}]},now);tick(.1);
  r.setSpec({position:{x:4,y:0}},now);tick(.45);assert.equal(r.status.clip,'Walk');
  r.setSpec({position:{x:4,y:0},defeated:true,cues:[{id:'late-attack',clip:'Attack_Bow'}]},now);
  const deathPosition=r.root.position.clone();tick(2.6);assert.equal(r.status.clip,'Death');assert.equal(r.status.arrowVisible,false);
  assert.ok(Math.abs(r.status.clipTime-2)<1e-5);r.root.updateMatrixWorld(true);
  const head=r.root.getObjectByName('Head').getWorldPosition(new THREE.Vector3());
  assert.ok(head.y<.6&&head.y>0,'fallen head above floor');
  r.setSpec({position:{x:9,y:9},defeated:false,cues:[{id:'revive-hit',clip:'Hit'}]},now);tick(1);
  assert.equal(r.status.clip,'Death');assert.ok(r.root.position.equals(deathPosition));
  result.combat={releaseSeconds:metadata.attackReleaseSeconds,drawMorph:true,arrowHiddenAtRelease:true,
    stallClockSynchronized:true,repeatedHitsBounded:true,hitReturnsToWalk:true,deathClamped:true,deadCannotRevive:true};
  const second=new AnimatedRanger({model:'animated',position:{x:0,y:0},defeated:true});await ready(second);
  assert.equal(loads.length,1);assert.notEqual(r.root.getObjectByName('Head'),second.root.getObjectByName('Head'));
  r.dispose();r.dispose();await new Promise(setImmediate);assert.equal(disposals[0].geometries,0);
  second.update(3,3);assert.equal(second.status.clip,'Death');assert.ok(Math.abs(second.status.clipTime-2)<1e-5);
  second.dispose();await new Promise(setImmediate);assert.ok(disposals[0].geometries>0);
  let finish;delayed=new Promise(resolve=>{finish=resolve;});const late=new AnimatedRanger({model:'animated',position:{x:0,y:0}});
  late.dispose();finish();for(let i=0;i<100&&disposals.length<2;i++)await new Promise(r=>setTimeout(r,10));
  await new Promise(setImmediate);assert.equal(late.root.children.length,0);assert.ok(disposals.at(-1).geometries>0);
  result.lifecycle={oneSharedAsset:true,independentSkeletons:true,finalOwnerDisposes:true,lateLoadDisposes:true,defeatedModelSwitch:true};
  const legacy=new AnimatedRanger({position:{x:0,y:0}});await ready(legacy);
  assert.equal(legacy.model,'legacy');legacy.setSpec({position:{x:1,y:0}},0);legacy.update(.02,.02);
  assert.equal(legacy.status.walkTimeScale,2.5);assert.equal(legacy.status.x,.055);
  legacy.setSpec({position:{x:1,y:0},cues:[{id:'legacy-hit',clip:'Hit'}]},.03);assert.equal(legacy.status.clip,'Hit');
  legacy.setSpec({position:{x:1,y:0},defeated:true},.04);assert.equal(legacy.status.clip,'Death');legacy.dispose();
  result.legacy={defaultUnchanged:true,walkRateUnchanged:true,originalHitAndDeath:true};
  // Isolate the pure planner; its hook and Redux imports are not executed here.
  const playback=await fs.readFile('src/hooks/useCombatPresentationPlayback.ts','utf8');
  const pure=playback.slice(playback.indexOf('const MELEE_MOTION_MS'),playback.indexOf('export function useCombatPresentationPlayback'));
  const plannerCode=ts.transpileModule(pure,{compilerOptions:{target:99,module:99}}).outputText;
  const {buildCombatPlaybackPlan}=await import('data:text/javascript;base64,'+Buffer.from(plannerCode).toString('base64'));
  const exchange={exchangeId:'test',attackerId:1,attackerType:'player',targetId:2,targetType:'monster',attackStyle:'ranged',
    attackerSnapshot:{id:1,type:'player',position:{x:0,y:0}},targetSnapshot:{id:2,type:'monster',position:{x:0,y:2}},
    steps:[{kind:'hit',source:{id:1,type:'player'},target:{id:2,type:'monster'},damage:5,targetHpAfter:10}],effects:[]};
  const before=JSON.stringify(exchange),base=1000,timing={rangedActorId:1,rangedReleaseMs:metadata.attackReleaseSeconds*1000};
  for(const [options,delay] of [[undefined,100],[timing,timing.rangedReleaseMs],[{...timing,rangedActorId:3},100]]){
    const plan=buildCombatPlaybackPlan(exchange,base,options),arrows=plan.effects.filter(e=>e.kind==='projectile');
    assert.equal(arrows.length,1);assert.equal(arrows[0].startMs,base+delay);
  }
  assert.equal(JSON.stringify(exchange),before);result.timeline={oneProjectile:true,localRangerOnly:true,serverPayloadUnchanged:true};
  await fs.writeFile('artifacts/ranger-combat/mixer-validation.json',JSON.stringify(result,null,2));
  console.log('RANGER_COMBAT_TESTS_OK',JSON.stringify(result));
}finally{GLTFLoader.prototype.loadAsync=native;}
