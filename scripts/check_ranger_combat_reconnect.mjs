// Real authorized match at the ordinary production URL. No state injection.
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import fs from 'node:fs/promises';
import path from 'node:path';
const require=createRequire(import.meta.url);
const {chromium}=require(path.join(process.env.TEMP,'nachalo-ranger-browser/node_modules/playwright'));
const account=JSON.parse(await fs.readFile(process.env.RANGER_ACCOUNT_FILE??path.join(process.env.TEMP,'nachalo-ranger-live-account.json'),'utf8'));
const out=path.resolve('artifacts/ranger-combat'),base='http://localhost';
const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const page=await browser.newPage({viewport:{width:1600,height:1100}});
let sockets=0;const errors=[],commands=[],report={origin:base,realServices:true,instanceId:account.instanceId};
page.on('pageerror',e=>errors.push(e.message));page.on('websocket',ws=>{if(new URL(ws.url()).pathname==='/ws')sockets++;});
page.on('request',r=>{if(r.method()==='POST'&&new URL(r.url()).pathname.startsWith('/game/'))commands.push(new URL(r.url()).pathname);});
const status=()=>page.getByLabel('Анимация Ranger');
async function dismiss(){const close=page.getByRole('button',{name:'Close',exact:true}).first();if(await close.isVisible())await close.click();
  const got=page.getByRole('button',{name:'Got it',exact:true});await got.waitFor({timeout:3500}).catch(()=>{});if(await got.isVisible())await got.click();}
async function idle(){await status().filter({hasText:/^Idle ·/}).waitFor({timeout:60000});}
async function snap(){return page.evaluate(async id=>{const u=JSON.parse(localStorage.getItem('user'));const r=await fetch('/game/match?instance_id='+id,{headers:{Authorization:`Bearer ${u.token}`}});return{data:await r.json(),id:u.id};},account.instanceId);}
try{
  await page.goto(base+'/login');await page.getByPlaceholder('you@example.com').fill(account.email);
  await page.getByPlaceholder('Enter your password').fill(account.password);await page.getByRole('button',{name:'Sign In',exact:true}).click();await page.waitForURL('**/mode');
  await page.goto(base+'/game?instance_id='+account.instanceId);await page.getByRole('button',{name:'3D',exact:true}).waitFor();await dismiss();
  assert.equal(await page.getByRole('button',{name:'Classic',exact:true}).getAttribute('aria-pressed'),'true');report.classicDefault=true;
  await page.getByRole('button',{name:'3D',exact:true}).click();await idle();
  assert.equal(await page.getByRole('button',{name:'Animated',exact:true}).getAttribute('aria-pressed'),'true');
  await page.locator('canvas').evaluate(c=>window.__combatCanvas=c);const socketsBefore=sockets;
  const initial=await snap(),p=initial.data.players.find(p=>(p.user_id??p.player_id)===initial.id);assert.ok(p&&p.health>0);report.character=p.characterType;
  if(p.energy<15){await page.getByRole('button',{name:'End Turn',exact:true}).click();await page.waitForTimeout(500);}
  const directions=[[1,0,'ArrowRight'],[-1,0,'ArrowLeft'],[0,1,'ArrowDown'],[0,-1,'ArrowUp']];
  const moves=[];
  for(const d of directions){
    const s=await snap(),player=s.data.players.find(p=>(p.user_id??p.player_id)===s.id);
    const walkable=dir=>s.data.map.some(c=>c.x===player.position.x+dir[0]&&c.y===player.position.y+dir[1]&&![32,49].includes(c.tileCode)&&!c.monster&&!c.resource&&!c.barbel&&!c.isPortal&&c.structure_type!=='wall');
    if(!walkable(d))continue;
    for(const direction of [d,[-d[0],-d[1],directions.find(v=>v[0]===-d[0]&&v[1]===-d[1])[2]]]){
      const count=commands.length;
      const [r]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname.endsWith('/move')),page.keyboard.press(direction[2])]);
      assert.ok(r.ok());await status().filter({hasText:/^Walk ·/}).waitFor({timeout:5000});await idle();assert.equal(commands.length-count,1);
      moves.push(direction[2]);
    }
  }
  assert.ok(moves.length>=2);report.movementDirections=[...new Set(moves)];
  for(const model of ['Legacy','Animated']){await page.getByRole('button',{name:model,exact:true}).click();await idle();}
  assert.equal(await page.locator('canvas').count(),1);assert.ok(await page.locator('canvas').evaluate(c=>c===window.__combatCanvas));assert.equal(sockets,socketsBefore);
  report.persistentCanvasAndSocket=true;
  await page.reload();await page.getByRole('button',{name:'3D',exact:true}).waitFor();await dismiss();
  assert.equal(await page.getByRole('button',{name:'Classic',exact:true}).getAttribute('aria-pressed'),'true');await page.getByRole('button',{name:'3D',exact:true}).click();await idle();
  const after=await snap(),restored=after.data.players.find(p=>(p.user_id??p.player_id)===after.id);assert.ok(restored&&restored.health>0);report.reloadRestoresServerMatch=true;
  const socketAfterReload=sockets;
  await page.route('**/ranger_walk_tuned.glb',route=>route.abort());
  await page.getByRole('button',{name:'Classic',exact:true}).click();await page.getByRole('button',{name:'3D',exact:true}).click();
  await page.getByRole('button',{name:'Legacy',exact:true}).and(page.locator('[aria-pressed="true"]')).waitFor({timeout:30000});await idle();
  report.failedCombatGLBFallsBackToLegacy=true;assert.equal(sockets,socketAfterReload);
  await page.unroute('**/ranger_walk_tuned.glb');await page.getByRole('button',{name:'Animated',exact:true}).click();await idle();
  await page.getByRole('application').hover();await page.mouse.wheel(0,-1000);await page.waitForTimeout(300);
  await page.screenshot({path:path.join(out,'deployed-real-reconnected.png'),fullPage:true});
  report.animatedRecovered=true;assert.deepEqual(errors,[]);console.log('RANGER_COMBAT_RECONNECT_OK',JSON.stringify(report));
}catch(e){report.failure=String(e);await page.screenshot({path:path.join(out,'reconnect-failure.png'),fullPage:true}).catch(()=>{});throw e;}
finally{await fs.writeFile(path.join(out,'reconnect-validation.json'),JSON.stringify({...report,sockets,commands,errors},null,2));await browser.close();}
