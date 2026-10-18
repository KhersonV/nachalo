// Bounded normal UI play in the disposable real PVE session; no state injection.
import {createRequire} from 'node:module';
import fs from 'node:fs/promises';
import path from 'node:path';
const require=createRequire(import.meta.url);
const {chromium}=require(path.join(process.env.TEMP,'nachalo-ranger-browser/node_modules/playwright'));
const account=JSON.parse(await fs.readFile(path.join(process.env.TEMP,'nachalo-ranger-live-account.json'),'utf8'));
const out=path.resolve('artifacts/ranger-live');
const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const page=await browser.newPage({viewport:{width:1600,height:1100}});
const events=[],actions=[],errors=[];const report={realServices:true,death:false,finished:false};
page.on('pageerror',e=>errors.push(e.message));
page.on('websocket',ws=>{if(new URL(ws.url()).pathname==='/ws')ws.on('framereceived',f=>{try{events.push(JSON.parse(String(f.payload)).type);}catch{}});});
const dirs=[[1,0,'ArrowRight'],[-1,0,'ArrowLeft'],[0,1,'ArrowDown'],[0,-1,'ArrowUp']];
async function snap(){return page.evaluate(async id=>{const u=JSON.parse(localStorage.getItem('user'));const r=await fetch('/game/match?instance_id='+id,{headers:{Authorization:`Bearer ${u.token}`}});
return{status:r.status,data:r.ok?await r.json():null,id:u.id};},account.instanceId);}
try{
  await page.goto('http://localhost:3003/login');await page.getByPlaceholder('you@example.com').fill(account.email);
  await page.getByPlaceholder('Enter your password').fill(account.password);await page.getByRole('button',{name:'Sign In',exact:true}).click();
  await page.waitForURL('**/mode');await page.goto('http://localhost:3003/game?instance_id='+account.instanceId);
  await page.getByRole('button',{name:'3D',exact:true}).waitFor();
  const got=page.getByRole('button',{name:'Got it',exact:true});await got.waitFor({timeout:5000}).catch(()=>{});if(await got.isVisible())await got.click();
  await page.getByRole('button',{name:'3D',exact:true}).click();
  await page.getByLabel('Анимация Ranger').filter({hasText:'Idle'}).waitFor({timeout:60000});
  const visited=new Map();
  for(let turn=0;turn<80;turn++){
    const s=await snap();const p=s.data?.players.find(v=>(v.user_id??v.player_id)===s.id);
    if(!p||p.health<=0){report.death=events.includes('PLAYER_DEFEATED');break;}
    report.health=p.health;const key=`${p.position.x},${p.position.y}`;visited.set(key,(visited.get(key)??0)+1);
    if(p.energy<9){await page.getByRole('button',{name:'End Turn',exact:true}).click();actions.push({kind:'endTurn'});await page.waitForTimeout(500);continue;}
    const monsters=s.data.map.filter(c=>c.monster&&Math.max(Math.abs(c.x-p.position.x),Math.abs(c.y-p.position.y))<=p.sightRange)
      .sort((a,b)=>Math.abs(a.x-p.position.x)+Math.abs(a.y-p.position.y)-Math.abs(b.x-p.position.x)-Math.abs(b.y-p.position.y));
    const adjacent=monsters.find(c=>Math.abs(c.x-p.position.x)+Math.abs(c.y-p.position.y)===1);
    let d,kind;
    if(adjacent){d=dirs.find(d=>p.position.x+d[0]===adjacent.x&&p.position.y+d[1]===adjacent.y);kind='attack';}
    else{
      const choices=dirs.filter(d=>s.data.map.some(c=>c.x===p.position.x+d[0]&&c.y===p.position.y+d[1]&&![32,49].includes(c.tileCode)&&!c.monster&&!c.barbel&&!c.resource&&!c.isPortal&&c.structure_type!=='wall'));
      choices.sort((a,b)=>{
        const score=d=>(visited.get(`${p.position.x+d[0]},${p.position.y+d[1]}`)??0)*5+
          (monsters[0]?Math.abs(monsters[0].x-p.position.x-d[0])+Math.abs(monsters[0].y-p.position.y-d[1]):0);
        return score(a)-score(b);
      });d=choices[0];kind='move';
    }
    if(!d)break;
    const [r]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname.endsWith(kind==='attack'?'/attack':'/move'),{timeout:8000}),page.keyboard.press(d[2])]);
    actions.push({kind,status:r.status(),health:p.health,position:p.position});
    if(kind==='attack'){
      await page.waitForTimeout(350);await page.screenshot({path:path.join(out,'real-damage-combat.png'),fullPage:true});
      report.combatEffectsSeen=(report.combatEffectsSeen??0)+await page.locator('[data-effect]').count();
    }
    await page.waitForTimeout(450);
    if(events.includes('PLAYER_DEFEATED')){report.death=true;break;}
  }
  await page.screenshot({path:path.join(out,report.death?'real-player-defeated.png':'real-combat-final.png'),fullPage:true});
  report.matchEndedEvent=events.includes('MATCH_ENDED');
  if(report.death){
    const stats=page.getByRole('button',{name:'To Stats',exact:true});await stats.waitFor({timeout:15000});await stats.click();
    await page.waitForURL('**/game/stats*',{timeout:20000});report.finished=true;
    await page.screenshot({path:path.join(out,'real-match-stats.png'),fullPage:true});
  }
  console.log('REAL_COMBAT_CHECK',JSON.stringify({...report,actions:actions.length}));
}catch(e){report.failure=String(e);await page.screenshot({path:path.join(out,'real-combat-failure.png'),fullPage:true}).catch(()=>{});throw e;}
finally{
  report.lastAliveHealth=report.health;delete report.health;
  report.matchEndedEvent=events.includes('MATCH_ENDED');
  await fs.writeFile(path.join(out,'real-combat-validation.json'),JSON.stringify({...report,actions,events,errors},null,2));await browser.close();
}
