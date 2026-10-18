// Smoke check the ordinary nginx/frontend URL after the frontend-only rebuild.
import {createRequire} from 'node:module';
import fs from 'node:fs/promises';
import path from 'node:path';
const require=createRequire(import.meta.url);
const {chromium}=require(path.join(process.env.TEMP,'nachalo-ranger-browser/node_modules/playwright'));
const accountPath=path.join(process.env.TEMP,'nachalo-ranger-live-account.json');
const account=JSON.parse(await fs.readFile(accountPath,'utf8'));
const out=path.resolve('artifacts/ranger-live');
const browser=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const page=await browser.newPage({viewport:{width:1600,height:1100}});const errors=[];
page.on('pageerror',e=>errors.push(e.message));
try{
  await page.goto('http://localhost/login');await page.getByPlaceholder('you@example.com').fill(account.email);
  await page.getByPlaceholder('Enter your password').fill(account.password);await page.getByRole('button',{name:'Sign In',exact:true}).click();
  await page.waitForURL('**/mode');
  const current=await page.evaluate(async()=>{const u=JSON.parse(localStorage.getItem('user'));const r=await fetch('/matchmaking/currentMatch?player_id='+u.id,{headers:{Authorization:`Bearer ${u.token}`}});return r.ok?await r.json():null;});
  if(current?.instance_id)await page.goto('http://localhost/game?instance_id='+current.instance_id);
  else await page.getByRole('button',{name:'Join Queue',exact:true}).click();
  await page.waitForURL('**/game?instance_id=*',{timeout:90000});
  account.instanceId=new URL(page.url()).searchParams.get('instance_id');await fs.writeFile(accountPath,JSON.stringify(account));
  await page.getByRole('button',{name:'3D',exact:true}).waitFor({timeout:30000});
  const close=page.getByRole('button',{name:'Close',exact:true}).first();if(await close.isVisible())await close.click();
  const got=page.getByRole('button',{name:'Got it',exact:true});await got.waitFor({timeout:7000}).catch(()=>{});if(await got.isVisible())await got.click();
  if(await page.getByRole('button',{name:'Classic',exact:true}).getAttribute('aria-pressed')!=='true')throw new Error('Classic default changed');
  await page.getByRole('button',{name:'3D',exact:true}).click();
  await page.getByLabel('Анимация Ranger').filter({hasText:'Idle'}).waitFor({timeout:60000});
  await page.locator('canvas').evaluate(c=>window.__deployedCanvas=c);
  for(const model of ['Legacy','Animated']){
    await page.getByRole('button',{name:model,exact:true}).click();
    await page.getByLabel('Анимация Ranger').filter({hasText:'Idle'}).waitFor({timeout:30000});
  }
  if(!await page.locator('canvas').evaluate(c=>c===window.__deployedCanvas))throw new Error('Canvas recreated');
  await page.getByRole('application').hover();await page.mouse.wheel(0,-1000);await page.waitForTimeout(300);
  await page.screenshot({path:path.join(out,'deployed-real-match.png'),fullPage:true});
  await fs.writeFile(path.join(out,'deployed-validation.json'),JSON.stringify({origin:'http://localhost',instanceId:account.instanceId,
    authorizedRealPVE:true,classicDefault:true,animatedReady:true,modelSwitches:2,persistentCanvas:true,errors},null,2));
  if(errors.length)throw new Error(errors.join('\n'));
  console.log('DEPLOYED_REAL_MATCH_OK',account.instanceId);
}finally{await browser.close();}
