// Real authorized PvE. No gameplay state or server responses are injected.
import {createRequire} from 'node:module';
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
const require=createRequire(import.meta.url);
const {chromium}=require(path.join(process.env.TEMP,'nachalo-ranger-browser/node_modules/playwright'));
const label=process.argv[2]??'before',base=process.env.RANGER_WALK_URL??'http://localhost';
const out=path.resolve('artifacts/ranger-walk'),accountPath=path.join(process.env.TEMP,'nachalo-ranger-combat-death-account.json');
const account=JSON.parse(await fs.readFile(accountPath,'utf8'));
const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const context=await browser.newContext({viewport:{width:1600,height:1100}}),page=await context.newPage();
const report={label,origin:base,realAuthorizedPVE:true,moves:[],states:[],errors:[]};let sockets=0;
const assetRequests=[];page.on('request',r=>{if(r.url().endsWith('.glb'))assetRequests.push(new URL(r.url()).pathname);});
page.on('pageerror',e=>report.errors.push(e.message));page.on('websocket',ws=>{if(new URL(ws.url()).pathname==='/ws')sockets++;});
const output=()=>page.getByLabel('Анимация Ranger');
async function snap(){return page.evaluate(async id=>{const u=JSON.parse(localStorage.getItem('user'));const r=await fetch('/game/match?instance_id='+id,{headers:{Authorization:`Bearer ${u.token}`}});return {data:await r.json(),id:u.id};},account.instanceId);}
let frames=[];
try{
 await page.goto(base+'/login');await page.getByPlaceholder('you@example.com').fill(account.email);await page.getByPlaceholder('Enter your password').fill(account.password);
 await page.getByRole('button',{name:'Sign In',exact:true}).click();await page.waitForURL('**/mode');
 const current=await page.evaluate(async()=>{const u=JSON.parse(localStorage.getItem('user'));const r=await fetch('/matchmaking/currentMatch?player_id='+u.id,{headers:{Authorization:`Bearer ${u.token}`}});return r.ok?await r.json():null;});
 if(current?.instance_id)await page.goto(base+'/game?instance_id='+current.instance_id);
 else {await page.getByRole('button',{name:'Join Queue',exact:true}).click();await page.waitForURL('**/game?instance_id=*',{timeout:90000});}
 account.instanceId=new URL(page.url()).searchParams.get('instance_id');await fs.writeFile(accountPath,JSON.stringify(account));report.instanceId=account.instanceId;
 await page.getByRole('button',{name:'3D',exact:true}).waitFor();const close=page.getByRole('button',{name:'Close',exact:true}).first();if(await close.isVisible())await close.click();
 const got=page.getByRole('button',{name:'Got it',exact:true});await got.waitFor({timeout:4000}).catch(()=>{});if(await got.isVisible())await got.click();
 await page.getByRole('button',{name:'3D',exact:true}).click();await output().filter({hasText:/^Idle ·/}).waitFor({timeout:60000});
 await page.locator('canvas').evaluate(c=>window.__walkCanvas=c);const socketBaseline=sockets;
 await page.getByRole('application').hover();await page.mouse.wheel(0,-1000);await page.waitForTimeout(400);
 const cdp=await context.newCDPSession(page);
 cdp.on('Page.screencastFrame',e=>{if(frames.length<400)frames.push({data:e.data,time:e.metadata.timestamp});void cdp.send('Page.screencastFrameAck',{sessionId:e.sessionId}).catch(()=>{});});
 await cdp.send('Page.startScreencast',{format:'jpeg',quality:85,maxWidth:1400,maxHeight:963,everyNthFrame:2});
 const dirs=[[1,0,'ArrowRight'],[-1,0,'ArrowLeft'],[0,1,'ArrowDown'],[0,-1,'ArrowUp']];
 let lastDir;
 for(let i=0;i<12;i++){
   const s=await snap(),p=s.data.players.find(v=>(v.user_id??v.player_id)===s.id);assert.ok(p&&p.health>0);
   if(p.energy<9){await page.getByRole('button',{name:'End Turn',exact:true}).click();await page.waitForTimeout(450);}
   const safe=d=>s.data.map.some(c=>c.x===p.position.x+d[0]&&c.y===p.position.y+d[1]&&![32,49].includes(c.tileCode)&&!c.monster&&!c.resource&&!c.barbel&&!c.isPortal&&c.structure_type!=='wall');
   const d=(i%3!==0&&lastDir&&safe(lastDir)?lastDir:null)??dirs.find(v=>safe(v)&&v!==lastDir)??dirs.find(safe);assert.ok(d,'safe adjacent move');lastDir=d;
   const [r]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname.endsWith('/move')),page.keyboard.press(d[2])]);assert.ok(r.ok());
   report.moves.push({direction:d[2],from:p.position,status:r.status()});
   for(let k=0;k<4;k++){await page.waitForTimeout(65);report.states.push(await output().textContent());}
 }
 await page.waitForTimeout(500);await cdp.send('Page.stopScreencast');
 assert.equal(await page.locator('canvas').count(),1);assert.ok(await page.locator('canvas').evaluate(c=>c===window.__walkCanvas));assert.equal(sockets,socketBaseline);
 report.finalState=await output().textContent();assert.match(report.finalState,/^Idle ·/);assert.deepEqual(report.errors,[]);
 const rates=report.states.filter(s=>s.startsWith('Walk')).map(s=>Number(s.split('×')[1]));assert.ok(rates.length);
 assert.ok(rates.every(v=>v<=(label==='before'?7.46:2.001)));
 if(label==='after'){
   assert.ok(assetRequests.some(p=>p.endsWith('/ranger_walk_tuned.glb')));
   const remote=await page.evaluate(async()=>{const b=await (await fetch('/assets/nachalo/characters/ranger/ranger_walk_tuned.glb')).arrayBuffer();return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',b))).map(v=>v.toString(16).padStart(2,'0')).join('');});
   const local=crypto.createHash('sha256').update(await fs.readFile('public/assets/nachalo/characters/ranger/ranger_walk_tuned.glb')).digest('hex');assert.equal(remote,local);report.assetSHA256=remote;
 }
 report.assetRequests=assetRequests;
 await page.screenshot({path:path.join(out,label+'.png'),fullPage:true});
 for(let i=0;i<frames.length;i++)await fs.writeFile(path.join(out,`${label}-${String(i).padStart(3,'0')}.jpg`),Buffer.from(frames[i].data,'base64'));
 report.frameTimes=frames.map(f=>f.time);report.persistentCanvasAndSocket=true;
 console.log('REAL_WALK_OK',label,[...new Set(report.states.filter(s=>s.startsWith('Walk')))]);
}catch(e){report.failure=String(e);await page.screenshot({path:path.join(out,label+'-failure.png'),fullPage:true}).catch(()=>{});throw e;}
finally{await fs.writeFile(path.join(out,label+'-validation.json'),JSON.stringify(report,null,2));await browser.close();}
