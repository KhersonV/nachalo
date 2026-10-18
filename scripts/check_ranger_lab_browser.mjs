/** Local Chrome smoke check. Playwright is installed outside the project. */
import { createRequire } from 'node:module';
import fs from 'node:fs/promises';
import path from 'node:path';
const require = createRequire(import.meta.url);
const { chromium } = require(path.join(process.env.TEMP, 'nachalo-ranger-browser/node_modules/playwright'));
const out = path.resolve('artifacts/ranger-lab');
await fs.mkdir(out, { recursive: true });
const browser = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1200 }, deviceScaleFactor: 1 });
const page = await context.newPage();
const errors = [];
const failedRequests = [];
const unrelatedWarnings = [];
async function check() {
page.on('pageerror', e => errors.push(e.message));
page.on('console', m => {
  if (m.type() !== 'error') return;
  const entry = `${m.text()} (${m.location().url})`;
  if (m.location().url.endsWith('/favicon.ico') && m.text().includes('404')) unrelatedWarnings.push(entry);
  else errors.push(entry);
});
page.on('response', response => { if (response.status() >= 400) failedRequests.push({ url: response.url(), status: response.status() }); });
await page.goto(process.env.RANGER_LAB_URL ?? 'http://localhost:3000/graphics-lab', { waitUntil: 'networkidle' });
await page.getByLabel('Состояние Ranger').filter({ hasText: '65 костей' }).waitFor({ timeout: 90000 });
for (const name of ['Ranger: выстрел', 'Ranger: урон', 'Ranger: смерть']) {
  if (!await page.getByRole('button', { name, exact: true }).isEnabled()) throw new Error('Combat preview disabled');
}
await page.screenshot({ path: path.join(out, 'lab-animated-idle.png'), fullPage: true });
console.log(await page.getByLabel('Состояние Ranger').textContent());
await page.locator('canvas').evaluate(canvas => { window.__rangerLabCanvas = canvas; });
const canvas = page.getByRole('application');
await canvas.hover();
await page.mouse.wheel(0, -1000);
await page.waitForTimeout(400);
// Move off the fixture's portal/start sprites before inspecting feet and bow.
for (let i=0; i<2; i++) {
  await page.getByRole('button', { name: '←', exact: true }).click();
  await page.getByLabel('Состояние Ranger').filter({ hasText: 'Animated · Walk' }).waitFor({ timeout: 10000 });
  await page.getByLabel('Состояние Ranger').filter({ hasText: 'Animated · Idle' }).waitFor({ timeout: 10000 });
}
await page.screenshot({ path: path.join(out, 'lab-animated-closeup.png'), fullPage: true });
const cdp = await context.newCDPSession(page);
const frames = [];
cdp.on('Page.screencastFrame', event => {
  if (frames.length < 240) frames.push({ data: event.data, timestamp: event.metadata.timestamp });
  void cdp.send('Page.screencastFrameAck', { sessionId: event.sessionId }).catch(() => {});
});
await cdp.send('Page.startScreencast', { format: 'jpeg', quality: 65, maxWidth: 1000, maxHeight: 900, everyNthFrame: 2 });
const movementStates = [];
for (const [name, button] of [['east','→'],['west','←'],['south','↓'],['north','↑']]) {
  await page.getByRole('button', { name: button, exact: true }).click();
  await page.getByLabel('Состояние Ranger').filter({ hasText: 'Animated · Walk' }).waitFor({ timeout: 10000 });
  movementStates.push({ direction: name, state: await page.getByLabel('Состояние Ranger').textContent() });
  await page.screenshot({ path: path.join(out, `lab-walk-${name}.png`), fullPage: true });
  await page.getByLabel('Состояние Ranger').filter({ hasText: 'Animated · Idle' }).waitFor({ timeout: 10000 });
  if (!await page.locator('canvas').evaluate(canvas => canvas === window.__rangerLabCanvas)) throw new Error('Canvas recreated on move');
}
await cdp.send('Page.stopScreencast');
for (let i=0; i<frames.length; i++) await fs.writeFile(path.join(out, `movement-${String(i).padStart(3,'0')}.jpg`), Buffer.from(frames[i].data,'base64'));
await fs.writeFile(path.join(out,'movement-timestamps.json'), JSON.stringify(frames.map(f => f.timestamp)));
await page.getByRole('button', { name: 'Ranger Legacy · 5 клипов', exact: true }).click();
await page.getByLabel('Состояние Ranger').filter({ hasText: 'Legacy · Idle' }).waitFor({ timeout: 30000 });
await page.screenshot({ path: path.join(out, 'lab-legacy-idle.png'), fullPage: true });
for (const [button, clip] of [['Ranger: выстрел','Attack_Bow'], ['Ranger: урон','Hit'], ['Ranger: смерть','Death']]) {
  await page.getByRole('button', { name: button, exact: true }).click();
  await page.getByLabel('Состояние Ranger').filter({ hasText: `Legacy · ${clip}` }).waitFor({ timeout: 10000 });
  if (clip !== 'Death') await page.getByLabel('Состояние Ranger').filter({ hasText: 'Legacy · Idle' }).waitFor({ timeout: 10000 });
}
await page.getByRole('button', { name: 'Ranger: сброс / Idle', exact: true }).click();
await page.getByLabel('Состояние Ranger').filter({ hasText: 'Legacy · Idle' }).waitFor({ timeout: 30000 });
for (let i = 0; i < 3; i++) {
  await page.getByRole('button', { name: 'Ranger Animated · 5 клипов', exact: true }).click();
  await page.getByLabel('Состояние Ranger').filter({ hasText: 'Animated · Idle' }).waitFor({ timeout: 30000 });
  await page.getByRole('button', { name: 'Ranger: сброс / Idle', exact: true }).click();
  await page.getByLabel('Состояние Ranger').filter({ hasText: 'Animated · Idle' }).waitFor({ timeout: 30000 });
  await page.getByRole('button', { name: 'Ranger Legacy · 5 клипов', exact: true }).click();
  await page.getByLabel('Состояние Ranger').filter({ hasText: 'Legacy · Idle' }).waitFor({ timeout: 30000 });
}
if (await page.locator('canvas').count() !== 1 || !await page.locator('canvas').evaluate(c => c === window.__rangerLabCanvas)) {
  throw new Error('Canvas duplicated or replaced on model switch/reset');
}
// A failed new-model download retains the existing primitive/sprite fallback.
const failure = await context.newPage();
await failure.route('**/ranger_walk_tuned.glb', route => route.abort());
await failure.goto(process.env.RANGER_LAB_URL ?? 'http://localhost:3000/graphics-lab', { waitUntil: 'networkidle' });
await failure.getByLabel('Состояние Ranger').filter({ hasText: 'резервный персонаж' }).waitFor({ timeout: 20000 });
if (await failure.locator('canvas').count() !== 1) throw new Error('Load fallback missing Canvas');
await failure.screenshot({ path: path.join(out, 'lab-load-fallback.png'), fullPage: true });
await failure.close();
await fs.writeFile(path.join(out, 'browser-validation.json'), JSON.stringify({ errors, failedRequests, unrelatedWarnings,
  persistentCanvas: true, directions: movementStates, modelSwitches: 7, resets: 4,
  legacyCombatClips: ['Attack_Bow','Hit','Death'], newCombatEnabled: true,
  loadFailureFallback: true, screencastFrames: frames.length }, null, 2));
if (errors.length) throw new Error(errors.join('\n'));
console.log('RANGER_LAB_BROWSER_OK');
}
try { await check(); } finally { await browser.close(); }
