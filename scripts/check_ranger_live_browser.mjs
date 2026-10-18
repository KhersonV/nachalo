// Real local services only. Credentials for this disposable test account stay in TEMP.
import { createRequire } from 'node:module';
import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import * as THREE from 'three';
const require=createRequire(import.meta.url);
const {chromium}=require(path.join(process.env.TEMP,'nachalo-ranger-browser/node_modules/playwright'));
const out=path.resolve('artifacts/ranger-live'); await fs.mkdir(out,{recursive:true});
const accountFile=path.join(process.env.TEMP,'nachalo-ranger-live-account.json');
let account;
try { account=JSON.parse(await fs.readFile(accountFile,'utf8')); }
catch { account={email:`ranger-live-${Date.now()}@example.test`,password:crypto.randomBytes(16).toString('hex'),name:'RangerLiveTest'}; }
const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const context=await browser.newContext({viewport:{width:1600,height:1100}});
const page=await context.newPage(); const errors=[]; const events=[]; const commands=[]; let sockets=0;
page.on('pageerror',e=>errors.push(e.message));
page.on('websocket',ws=>{if(new URL(ws.url()).pathname!='/ws')return;sockets++;ws.on('framereceived',frame=>{
  try{const m=JSON.parse(String(frame.payload));events.push(m.type);}catch{}
});});
page.on('request',r=>{if(r.method()==='POST'&&/\/game\//.test(r.url())) commands.push({path:new URL(r.url()).pathname,body:r.postData()});});
const base='http://localhost:3003';
async function snapshot(){return page.evaluate(async()=>{const u=JSON.parse(localStorage.getItem('user'));const id=new URL(location.href).searchParams.get('instance_id');
  const res=await fetch(`/game/match?instance_id=${id}`,{headers:{Authorization:`Bearer ${u.token}`}});return {status:res.status,data:res.ok?await res.json():null,userId:u.id};});}
try {
  if(!account.registered){
    await page.goto(base+'/register');
    await page.getByPlaceholder('you@example.com').fill(account.email);
    await page.getByPlaceholder('Your nickname').fill(account.name);
    await page.getByPlaceholder('Minimum 6 characters').fill(account.password);
    await page.locator('input[value="ranger"]').locator('..').click();
    await page.getByRole('button',{name:'Register',exact:true}).click();
    await page.waitForURL('**/login',{timeout:30000});
    account.registered=true; await fs.writeFile(accountFile,JSON.stringify(account));
  } else await page.goto(base+'/login');
  await page.getByPlaceholder('you@example.com').fill(account.email);
  await page.getByPlaceholder('Enter your password').fill(account.password);
  await page.getByRole('button',{name:'Sign In',exact:true}).click();
  await page.waitForURL('**/mode',{timeout:30000});
  if(account.instanceId) await page.goto(base+`/game?instance_id=${account.instanceId}`);
  else {
    await page.getByRole('button',{name:'Join Queue',exact:true}).click();
    await page.waitForURL('**/game?instance_id=*',{timeout:90000});
    account.instanceId=new URL(page.url()).searchParams.get('instance_id');
    await fs.writeFile(accountFile,JSON.stringify(account));
  }
  await page.getByRole('button',{name:'3D',exact:true}).waitFor({timeout:30000});
  const closeReady = page.getByRole('button',{name:'Close',exact:true}).first();
  if(await closeReady.isVisible().catch(()=>false)) await closeReady.click();
  await page.waitForTimeout(500);
  const gotIt=page.getByRole('button',{name:'Got it',exact:true});
  if(await gotIt.isVisible().catch(()=>false)) await gotIt.click();
  await page.screenshot({path:path.join(out,'real-match-classic.png'),fullPage:true});
  await page.getByRole('button',{name:'3D',exact:true}).click();
  await page.getByLabel('Анимация Ranger').filter({hasText:'Idle'}).waitFor({timeout:60000});
  await page.screenshot({path:path.join(out,'real-match-animated-idle.png'),fullPage:true});
  await page.locator('canvas').evaluate(c=>window.__liveCanvas=c);
  const before=await snapshot();
  const player=before.data.players.find(p=>(p.user_id??p.player_id)===before.userId);
  const report={instanceId:account.instanceId,registration:true,login:true,serverSnapshot:before.status,
    character:player.characterType,position:player.position,health:player.health,map:[before.data.map_width,before.data.map_height],events,commands,errors,sockets};
  await fs.writeFile(path.join(out,'real-validation.json'),JSON.stringify(report,null,2));
  await fs.writeFile(path.join(out,'page-inspect.txt'),await page.locator('body').innerText());
  console.log('REAL_MATCH_LOADED',JSON.stringify({instanceId:account.instanceId,position:player.position,health:player.health,sockets}));
  const socketBaseline=sockets;
  const cdp=await context.newCDPSession(page);const frames=[];
  cdp.on('Page.screencastFrame',e=>{if(frames.length<240)frames.push({data:e.data,time:e.metadata.timestamp});void cdp.send('Page.screencastFrameAck',{sessionId:e.sessionId}).catch(()=>{});});
  await cdp.send('Page.startScreencast',{format:'jpeg',quality:65,maxWidth:1100,maxHeight:800,everyNthFrame:3});
  const dirs=[['east',1,0,'ArrowRight'],['west',-1,0,'ArrowLeft'],['south',0,1,'ArrowDown'],['north',0,-1,'ArrowUp']];
  const moves=[];
  async function step(dir){
    const old=await snapshot();const p=old.data.players.find(v=>(v.user_id??v.player_id)===old.userId);
    const n=commands.filter(c=>c.path.endsWith('/move')).length;
    const [response]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname.endsWith('/move'),{timeout:8000}),page.keyboard.press(dir[3])]);
    if(!response.ok())throw new Error(`Real move rejected: ${response.status()} ${await response.text()}`);
    await page.getByLabel('Анимация Ranger').filter({hasText:/^Walk ·/}).waitFor({timeout:5000});
    const motion=await page.getByLabel('Анимация Ranger').textContent();
    await page.screenshot({path:path.join(out,`real-walk-${dir[0]}.png`),fullPage:true});
    await page.getByLabel('Анимация Ranger').filter({hasText:'Idle'}).waitFor({timeout:5000});
    const current=await snapshot(); const next=current.data.players.find(v=>(v.user_id??v.player_id)===current.userId);
    if(next.position.x!==p.position.x+dir[1]||next.position.y!==p.position.y+dir[2])throw new Error('Server did not confirm target');
    if(commands.filter(c=>c.path.endsWith('/move')).length!==n+1)throw new Error('Duplicate move command');
    moves.push({direction:dir[0],position:next.position,motion});
  }
  for(const desired of dirs){
    for(let attempt=0;attempt<5;attempt++){
      const s=await snapshot();const p=s.data.players.find(v=>(v.user_id??v.player_id)===s.userId);
      const good=(x,y)=>s.data.map.some(c=>c.x===x&&c.y===y&&![32,49].includes(c.tileCode)&&!c.monster&&!c.barbel&&!c.resource&&!c.isPortal&&c.structure_type!=='wall');
      if(good(p.position.x+desired[1],p.position.y+desired[2])){await step(desired);break;}
      const alternative=dirs.find(d=>good(p.position.x+d[1],p.position.y+d[2])&&good(p.position.x+d[1]+desired[1],p.position.y+d[2]+desired[2]));
      if(!alternative)break;
      await step(alternative);
    }
  }
  report.moves=moves;
  for(const name of ['Legacy','Animated','Legacy','Animated']){
    await page.getByRole('button',{name,exact:true}).click();
    await page.getByLabel('Анимация Ranger').filter({hasText:'Idle'}).waitFor({timeout:30000});
    if(await page.locator('canvas').count()!==1||!await page.locator('canvas').evaluate(c=>c===window.__liveCanvas))throw new Error('Canvas replaced');
  }
  report.modelSwitches=4;report.persistentCanvas=true;report.noSocketResubscribe=sockets===socketBaseline;
  // Return along confirmed visited cells, toward the monsters seen at spawn.
  const outward=[...moves];
  for(const move of outward.reverse()) await step(dirs.find(d=>d[0]===({east:'west',west:'east',south:'north',north:'south'}[move.direction])));
  for(let i=0;i<6;i++){
    const ss=await snapshot();const pp=ss.data.players.find(v=>(v.user_id??v.player_id)===ss.userId);
    const target=ss.data.map.filter(c=>c.monster&&Math.max(Math.abs(c.x-pp.position.x),Math.abs(c.y-pp.position.y))<=pp.sightRange)
      .sort((a,b)=>Math.abs(a.x-pp.position.x)+Math.abs(a.y-pp.position.y)-Math.abs(b.x-pp.position.x)-Math.abs(b.y-pp.position.y))[0];
    if(!target||Math.abs(target.x-pp.position.x)+Math.abs(target.y-pp.position.y)===1)break;
    const closer=dirs.find(d=>ss.data.map.some(c=>c.x===pp.position.x+d[1]&&c.y===pp.position.y+d[2]&&![32,49].includes(c.tileCode)&&!c.monster&&!c.barbel&&!c.resource&&c.structure_type!=='wall'&&
      Math.abs(target.x-c.x)+Math.abs(target.y-c.y)<Math.abs(target.x-pp.position.x)+Math.abs(target.y-pp.position.y)));
    if(!closer)break;
    await step(closer);
  }
  const s=await snapshot();const p=s.data.players.find(v=>(v.user_id??v.player_id)===s.userId);
  const monster=s.data.map.filter(c=>c.monster&&Math.abs(c.x-p.position.x)+Math.abs(c.y-p.position.y)<=(p.attackRange??1))
    .sort((a,b)=>Math.abs(a.x-p.position.x)+Math.abs(a.y-p.position.y)-Math.abs(b.x-p.position.x)-Math.abs(b.y-p.position.y))[0];
  if(monster){
    const box=await page.getByRole('application').boundingBox();
    const cam=new THREE.OrthographicCamera(-11*box.width/box.height/2,11*box.width/box.height/2,5.5,-5.5,.1,2000);
    cam.position.set(p.position.x+7,12,p.position.y+9);cam.lookAt(p.position.x,0,p.position.y);cam.updateMatrixWorld();
    const projected=new THREE.Vector3(monster.x,0,monster.y).project(cam);
    const [attack]=await Promise.all([page.waitForResponse(r=>new URL(r.url()).pathname==='/game/attack',{timeout:10000}),
      page.mouse.click(box.x+(projected.x+1)*box.width/2,box.y+(1-projected.y)*box.height/2)]);
    report.attack={status:attack.status(),target:monster.monster.name};
    await page.waitForTimeout(350);
    report.visibleCombatEffects=await page.locator('[data-effect]').count();
    await page.screenshot({path:path.join(out,'real-match-combat.png'),fullPage:true});
    await page.waitForTimeout(1300);
    const after=await snapshot();const ap=after.data?.players.find(v=>(v.user_id??v.player_id)===after.userId);
    report.healthAfterAttack=ap?.health;report.damageObserved=ap?.health<p.health;
  }
  await cdp.send('Page.stopScreencast');
  for(let i=0;i<frames.length;i++)await fs.writeFile(path.join(out,`real-frame-${String(i).padStart(3,'0')}.jpg`),Buffer.from(frames[i].data,'base64'));
  await fs.writeFile(path.join(out,'real-frame-timestamps.json'),JSON.stringify(frames.map(f=>f.time)));
  report.recordedFrames=frames.length;
  await fs.writeFile(path.join(out,'real-validation.json'),JSON.stringify({...report,sockets,events,commands,errors},null,2));
  await page.reload();
  await page.getByRole('button',{name:'3D',exact:true}).waitFor({timeout:30000});
  report.classicAfterReload=await page.getByRole('button',{name:'Classic',exact:true}).getAttribute('aria-pressed')==='true';
  const gotItReload=page.getByRole('button',{name:'Got it',exact:true});
  await gotItReload.waitFor({timeout:4000}).catch(()=>{});
  if(await gotItReload.isVisible().catch(()=>false))await gotItReload.click();
  await page.getByRole('button',{name:'3D',exact:true}).click();
  await page.getByLabel('Анимация Ranger').filter({hasText:'Idle'}).waitFor({timeout:60000});
  report.reloadRestored=true;
  const wsAfterReload=sockets;
  await page.route('**/ranger_walk_tuned.glb',route=>route.abort());
  await page.getByRole('button',{name:'Classic',exact:true}).click();
  await page.getByRole('button',{name:'3D',exact:true}).click();
  await page.getByRole('button',{name:'Legacy',exact:true}).and(page.locator('[aria-pressed="true"]')).waitFor({timeout:30000});
  await page.getByLabel('Анимация Ranger').filter({hasText:'Idle'}).waitFor({timeout:30000});
  report.glbFailureFallsBackToLegacy=true;report.noSocketResubscribeOnViewSwitch=sockets===wsAfterReload;
  await page.unroute('**/ranger_walk_tuned.glb');
  await page.getByRole('button',{name:'Animated',exact:true}).click();
  await page.getByLabel('Анимация Ranger').filter({hasText:'Idle'}).waitFor({timeout:30000});
  await page.getByRole('application').hover();await page.mouse.wheel(0,-1000);await page.waitForTimeout(200);
  await page.screenshot({path:path.join(out,'real-match-reconnected.png'),fullPage:true});
  await fs.writeFile(path.join(out,'real-validation.json'),JSON.stringify({...report,sockets,events,commands,errors},null,2));
  console.log('REAL_MATCH_CHECKS_OK',JSON.stringify({directions:[...new Set(moves.map(m=>m.direction))],attack:report.attack,damage:report.damageObserved,reload:report.reloadRestored}));
} catch(e) {
  await page.screenshot({path:path.join(out,'failure.png'),fullPage:true}).catch(()=>{});
  await fs.writeFile(path.join(out,'failure.txt'),String(e)+'\n'+await page.locator('body').innerText()).catch(()=>{});
  throw e;
} finally {await browser.close();}
