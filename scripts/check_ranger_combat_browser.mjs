// Actual Chrome, original GLB textures, and (live mode) authorized real services.
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import * as THREE from 'three';
const require=createRequire(import.meta.url);
const {chromium}=require(path.join(process.env.TEMP,'nachalo-ranger-browser/node_modules/playwright'));
const base=process.env.RANGER_COMBAT_URL??'http://localhost:3003',mode=process.argv[2]??'lab';
const out=path.resolve('artifacts/ranger-combat');
const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const context=await browser.newContext({viewport:{width:1600,height:1100}}),page=await context.newPage();
const errors=[],events=[],commands=[],assets=[];let sockets=0;
page.on('pageerror',e=>errors.push(e.message));
page.on('request',r=>{if(r.method()==='POST'&&new URL(r.url()).pathname.startsWith('/game/'))commands.push({path:new URL(r.url()).pathname,time:Date.now()});
  if(r.url().endsWith('ranger_walk_tuned.glb'))assets.push(r.url());});
page.on('websocket',ws=>{if(new URL(ws.url()).pathname!='/ws')return;sockets++;ws.on('framereceived',f=>{try{const m=JSON.parse(String(f.payload));events.push({type:m.type,time:Date.now(),...(m.type==='COMBAT_EXCHANGE'?{combat:m.payload??m.data}: {})});
  if(mode==='death'&&m.type==='PLAYER_DEFEATED')void page.addStyleTag({content:'#modal-root {visibility:hidden!important}'}).then(tag=>{modalStyle=tag;}).catch(()=>{});
}catch{}});});
let modalStyle;
const output=()=>page.getByLabel(mode==='lab'?'Состояние Ranger':'Анимация Ranger');
const ready=()=>output().filter({hasText:'Idle'}).waitFor({timeout:60000});
async function dismiss(){
  const close=page.getByRole('button',{name:'Close',exact:true}).first();if(await close.isVisible())await close.click();
  const got=page.getByRole('button',{name:'Got it',exact:true});await got.waitFor({timeout:4000}).catch(()=>{});if(await got.isVisible())await got.click();
}
const cdp=await context.newCDPSession(page);let recording=null;
cdp.on('Page.screencastFrame',e=>{if(recording&&recording.frames.length<150)recording.frames.push({data:e.data,time:e.metadata.timestamp});void cdp.send('Page.screencastFrameAck',{sessionId:e.sessionId}).catch(()=>{});});
async function record(action,ms=2800){
  const capture={frames:[],states:[]};recording=capture;
  await cdp.send('Page.startScreencast',{format:'jpeg',quality:75,maxWidth:1200,maxHeight:825,everyNthFrame:2});
  await action();const end=Date.now()+ms;
  while(Date.now()<end){
    capture.states.push(await output().evaluate(o=>({clip:o.dataset.clip,time:Number(o.dataset.clipTime),arrow:o.dataset.arrowVisible==='true',wall:Date.now()/1000,
      projectiles:document.querySelectorAll('[data-effect="projectile"]').length})).catch(()=>({clip:'missing'})));
    await page.waitForTimeout(60);
  }
  await cdp.send('Page.stopScreencast');recording=null;return capture;
}
async function saveCapture(name,capture){
  for(let i=0;i<capture.frames.length;i++)await fs.writeFile(path.join(out,`${name}-${String(i).padStart(3,'0')}.jpg`),Buffer.from(capture.frames[i].data,'base64'));
  await fs.writeFile(path.join(out,name+'-timeline.json'),JSON.stringify({states:capture.states,frames:capture.frames.map(f=>f.time)},null,2));
}
const report={mode,origin:base,realServices:mode!=='lab'};
try{
  if(mode==='lab'){
    await page.goto(base+'/graphics-lab');await ready();await page.locator('canvas').evaluate(c=>window.__combatCanvas=c);
    await page.getByRole('application').hover();await page.mouse.wheel(0,-900);await page.waitForTimeout(300);
    for(const [button,clip] of [['Ranger: выстрел','Attack_Bow'],['Ranger: урон','Hit'],['Ranger: смерть','Death']]){
      const cap=await record(()=>page.getByRole('button',{name:button,exact:true}).click());
      assert.ok(cap.states.some(s=>s.clip===clip),clip);await saveCapture('lab-'+clip,cap);
      await page.screenshot({path:path.join(out,'lab-'+clip+'.png'),fullPage:true});
      if(clip==='Death'){assert.equal(cap.states.at(-1).clip,'Death');assert.ok(cap.states.at(-1).time>=1.99);}
      else assert.equal(cap.states.at(-1).clip,'Idle');
    }
    await page.getByRole('button',{name:'Ranger: сброс / Idle',exact:true}).click();await ready();
    for(const model of ['Legacy','Animated']){await page.getByRole('button',{name:`Ranger ${model} · 5 клипов`,exact:true}).click();await ready();}
    assert.equal(await page.locator('canvas').count(),1);assert.ok(await page.locator('canvas').evaluate(c=>c===window.__combatCanvas));
    report.clips=['Attack_Bow','Hit','Death'];report.persistentCanvas=true;
  }else{
    const accountPath=path.join(process.env.TEMP,mode==='death'||mode==='bow'?'nachalo-ranger-combat-death-account.json':'nachalo-ranger-live-account.json');
    let account;
    try{account=JSON.parse(await fs.readFile(accountPath,'utf8'));}catch{account={email:`ranger-combat-${Date.now()}@example.test`,password:crypto.randomBytes(16).toString('hex'),name:'RangerCombatTest'};}
    if(!account.registered){
      await page.goto(base+'/register');await page.getByPlaceholder('you@example.com').fill(account.email);await page.getByPlaceholder('Your nickname').fill(account.name);
      await page.getByPlaceholder('Minimum 6 characters').fill(account.password);await page.locator('input[value="ranger"]').locator('..').click();
      await page.getByRole('button',{name:'Register',exact:true}).click();await page.waitForURL('**/login');account.registered=true;await fs.writeFile(accountPath,JSON.stringify(account));
    }
    await page.goto(base+'/login');await page.getByPlaceholder('you@example.com').fill(account.email);
    await page.getByPlaceholder('Enter your password').fill(account.password);await page.getByRole('button',{name:'Sign In',exact:true}).click();await page.waitForURL('**/mode');
    const current=await page.evaluate(async()=>{const u=JSON.parse(localStorage.getItem('user'));const r=await fetch('/matchmaking/currentMatch?player_id='+u.id,{headers:{Authorization:`Bearer ${u.token}`}});return r.ok?await r.json():null;});
    if(current?.instance_id)await page.goto(base+'/game?instance_id='+current.instance_id);
    else{await page.getByRole('button',{name:'Join Queue',exact:true}).click();await page.waitForURL('**/game?instance_id=*',{timeout:90000});}
    account.instanceId=new URL(page.url()).searchParams.get('instance_id');await fs.writeFile(accountPath,JSON.stringify(account));report.instanceId=account.instanceId;
    await page.getByRole('button',{name:'3D',exact:true}).waitFor({timeout:30000});await dismiss();
    assert.equal(await page.getByRole('button',{name:'Classic',exact:true}).getAttribute('aria-pressed'),'true');report.classicDefault=true;
    await page.getByRole('button',{name:'3D',exact:true}).click();await ready();await page.locator('canvas').evaluate(c=>window.__combatCanvas=c);
    const remoteHash=await page.evaluate(async()=>{
      const bytes=await (await fetch('/assets/nachalo/characters/ranger/ranger_walk_tuned.glb')).arrayBuffer();
      return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))).map(x=>x.toString(16).padStart(2,'0')).join('');
    });
    const localHash=crypto.createHash('sha256').update(await fs.readFile('public/assets/nachalo/characters/ranger/ranger_walk_tuned.glb')).digest('hex');
    assert.equal(remoteHash,localHash);report.assetSHA256=remoteHash;
    const socketBaseline=sockets;
    for(const model of ['Legacy','Animated']){await page.getByRole('button',{name:model,exact:true}).click();await ready();}
    assert.ok(await page.locator('canvas').evaluate(c=>c===window.__combatCanvas));assert.equal(sockets,socketBaseline);report.switchesKeepCanvasAndSocket=true;
    async function snapshot(){return page.evaluate(async id=>{const u=JSON.parse(localStorage.getItem('user'));const r=await fetch('/game/match?instance_id='+id,{headers:{Authorization:`Bearer ${u.token}`}});return{data:r.ok?await r.json():null,id:u.id};},account.instanceId);}
    const dirs=[[1,0,'ArrowRight'],[-1,0,'ArrowLeft'],[0,1,'ArrowDown'],[0,-1,'ArrowUp']],visited=new Map();
    const actions=[];let attackSaved=false,hitSaved=false,deathSaved=false;
    for(let i=0;i<90;i++){
      const s=await snapshot(),p=s.data?.players.find(v=>(v.user_id??v.player_id)===s.id);
      if(!p||p.health<=0)break;
      report.initialHealth??=p.health;report.lastAliveHealth=p.health;report.character=p.characterType;report.attackRange=p.attackRange;
      const key=`${p.position.x},${p.position.y}`;visited.set(key,(visited.get(key)??0)+1);
      if(p.energy<9){await page.getByRole('button',{name:'End Turn',exact:true}).click();actions.push({kind:'endTurn'});await page.waitForTimeout(450);continue;}
      const monsters=s.data.map.filter(c=>c.monster&&Math.max(Math.abs(c.x-p.position.x),Math.abs(c.y-p.position.y))<=(p.sightRange??3))
        .sort((a,b)=>Math.abs(a.x-p.position.x)+Math.abs(a.y-p.position.y)-Math.abs(b.x-p.position.x)-Math.abs(b.y-p.position.y));
      // Server correctly treats adjacent attacks as melee. A bow test needs a
      // visible target beyond one tile; do not relabel melee as a bow shot.
      const adjacent=monsters.find(c=>Math.abs(c.x-p.position.x)+Math.abs(c.y-p.position.y)===1);
      const ranged=mode!=='death'&&!attackSaved&&monsters.find(c=>Math.abs(c.x-p.position.x)+Math.abs(c.y-p.position.y)>1&&Math.abs(c.x-p.position.x)+Math.abs(c.y-p.position.y)<=(p.attackRange??2));
      const walkable=d=>s.data.map.some(c=>c.x===p.position.x+d[0]&&c.y===p.position.y+d[1]&&![32,49].includes(c.tileCode)&&!c.monster&&!c.barbel&&!c.resource&&!c.isPortal&&c.structure_type!=='wall');
      const retreat=mode!=='death'&&!attackSaved&&adjacent&&dirs.find(d=>walkable(d)&&Math.abs(adjacent.x-p.position.x-d[0])+Math.abs(adjacent.y-p.position.y-d[1])===2);
      let d,kind;
      if(ranged){kind='attack';}
      else if(retreat){d=retreat;kind='move';}
      else if(adjacent){d=dirs.find(d=>p.position.x+d[0]===adjacent.x&&p.position.y+d[1]===adjacent.y);kind='attack';}
      else{
        const choices=dirs.filter(walkable);
        const score=d=>(visited.get(`${p.position.x+d[0]},${p.position.y+d[1]}`)??0)*5+(monsters[0]?Math.abs(monsters[0].x-p.position.x-d[0])+Math.abs(monsters[0].y-p.position.y-d[1]):0);
        choices.sort((a,b)=>score(a)-score(b));d=choices[0];kind='move';
      }
      if(!d&&!ranged)break;
      const act=async()=>{
        const before=commands.length;
        const input=async()=>{
          if(!ranged)return page.keyboard.press(d[2]);
          const box=await page.getByRole('application').boundingBox();
          const cam=new THREE.OrthographicCamera(-11*box.width/box.height/2,11*box.width/box.height/2,5.5,-5.5,.1,2000);
          cam.position.set(p.position.x+7,12,p.position.y+9);cam.lookAt(p.position.x,0,p.position.y);cam.updateMatrixWorld();
          const projected=new THREE.Vector3(ranged.x,0,ranged.y).project(cam);
          await page.mouse.click(box.x+(projected.x+1)*box.width/2,box.y+(1-projected.y)*box.height/2);
        };
        const [r]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname.endsWith('/'+kind),{timeout:8000}),input()]);
        assert.equal(commands.length-before,1,'one keyboard command');actions.push({kind,status:r.status(),health:p.health,position:p.position});
        console.log('REAL_ACTION',actions.length,kind,r.status(),'HP',p.health);
      };
      if(kind==='attack'){
        const cap=await record(act,3000);
        if(!attackSaved&&cap.states.some(v=>v.clip==='Attack_Bow')){await saveCapture('real-Attack_Bow',cap);attackSaved=true;report.attackStates=cap.states;}
        if(mode==='bow'&&attackSaved)break;
        if(!hitSaved&&cap.states.some(v=>v.clip==='Hit')){await saveCapture('real-Hit',cap);hitSaved=true;report.hitStates=cap.states;}
        if(cap.states.some(v=>v.clip==='Death')){await saveCapture('real-Death',cap);deathSaved=true;report.deathStates=cap.states;await page.screenshot({path:path.join(out,'real-Death.png'),fullPage:true});break;}
      }else{await act();await page.waitForTimeout(500);}
    }
    report.actions=actions;report.attackObserved=attackSaved;report.hitObserved=hitSaved;report.deathObserved=deathSaved;
    if(mode!=='death')assert.ok(attackSaved,'Attack_Bow in real server combat');
    if(mode==='bow'){
      assert.ok(report.attackStates.some(s=>s.arrow&&s.projectiles===0));
      assert.ok(report.attackStates.some(s=>!s.arrow&&s.projectiles===1));
      assert.ok(report.attackStates.every(s=>s.projectiles<=1));
      assert.equal(report.attackStates.at(-1).clip,'Idle');
      report.releaseSynchronized=true;report.oneProjectile=true;report.returnedToIdle=true;
      await page.screenshot({path:path.join(out,'deployed-real-bow-after.png'),fullPage:true});
    }else{
    assert.ok(hitSaved,'Hit in real server combat');assert.ok(deathSaved,'Death in real server combat');
    assert.ok(events.some(e=>e.type==='PLAYER_DEFEATED'));report.serverDefeat=true;
    await page.waitForTimeout(1000);assert.equal(await output().getAttribute('data-clip'),'Death');assert.ok(Number(await output().getAttribute('data-clip-time'))>=1.99);
    // A defeated visual remains defeated when its model is replaced.
    for(const model of ['Legacy','Animated']){await page.getByRole('button',{name:model,exact:true}).click({force:true});await output().filter({hasText:'Death'}).waitFor({timeout:30000});}
    assert.equal(await page.locator('canvas').count(),1);assert.equal(sockets,socketBaseline);report.defeatedSwitchCannotRevive=true;
    if(modalStyle){await modalStyle.evaluate(tag=>tag.remove());report.modalHiddenForDeathCaptureOnly=true;}
    const stats=page.getByRole('button',{name:'To Stats',exact:true});await stats.waitFor({timeout:15000});await stats.click();await page.waitForURL('**/game/stats*');report.statsReached=true;
    await page.screenshot({path:path.join(out,'real-stats.png'),fullPage:true});
    }
  }
  assert.deepEqual(errors,[]);console.log('RANGER_COMBAT_BROWSER_OK',mode);
}catch(e){report.failure=String(e);await page.screenshot({path:path.join(out,mode+'-failure.png'),fullPage:true}).catch(()=>{});throw e;}
finally{await fs.writeFile(path.join(out,mode+'-browser-validation.json'),JSON.stringify({...report,errors,events,commands,sockets,assetRequests:assets.length},null,2));await browser.close();}
