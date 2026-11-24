// Scenarios: the stand's page used the way people use it, not only the obvious way, and what
// should happen checked. tyagaCheck measures how the page looks; this checks what it does.
//   stand/run.sh (or run-windows.sh)      → prints the page address
//   node stand/scenarios.js <address> [name,name…]   → needs Playwright, like ci.js
// The fake yt-dlp: TINY is a short clip, NEW gives a video a title of its own ("Song 07"),
// FAIL fails, SKIP is a playlist with a private video (stand/README.md).
const { chromium } = require('playwright');
const [url, only] = process.argv.slice(2);
if (!url) { console.error('usage: node stand/scenarios.js <stand address> [scenario,…]'); process.exit(2); }
const sleep = ms => new Promise(r => setTimeout(r, ms));
// Each run's videos are new ones: the stand's folder keeps the last run's files ("already
// there"), and ТЯГА remembers what it looked up. The title is "Song " + R + tag.
const R = Math.random().toString(36).slice(2, 4);
const L = (tag, kind = '') => `https://youtu.be/${kind}NEW${R}${tag}`;
let pg, cur = '';
const fails = [];
function expect(ok, what) { console.log(`  ${ok ? 'ok  ' : 'FAIL'} ${what}`); if (!ok) fails.push(`${cur}: ${what}`); }

const box = () => pg.$eval('#links', b => b.value);
const msg = () => pg.$eval('#msg', m => m.textContent.trim());
const rows = () => pg.$$eval('.job', js => js.map(j => ({ cls: j.className, name: (j.querySelector('.name') || {}).textContent || '', text: j.innerText.replace(/\s+/g, ' ') })));
async function paste(text, wait = 2500) { // as Ctrl+V does it: through the page's own paste handling
  await pg.click('#links');
  await pg.evaluate(t => {
    const b = document.getElementById('links'), dt = new DataTransfer(); dt.setData('text/plain', t);
    if (b.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }))) { b.value += t; b.dispatchEvent(new Event('input', { bubbles: true })); }
  }, text);
  await sleep(wait);
}
async function go() { await pg.click('.opts .go'); await sleep(700); }
const busy = () => pg.evaluate(() => [...document.querySelectorAll('.job .lane')].some(l => !l.classList.contains('shut')));
async function idle(ms = 120000) {
  const t0 = Date.now();
  while (Date.now() - t0 < ms) { await sleep(700); if (!(await busy())) { await sleep(1200); if (!(await busy())) return; } }
  expect(false, 'downloads finished within ' + ms / 1000 + ' s');
}
async function clean() { await idle(); await pg.evaluate(() => api('/api/clear', {})); await pg.fill('#links', ''); await pg.evaluate(() => addMsg('')); await sleep(1500); }
// Is the row of the download that's running inside the list's visible part?
const runningInView = () => pg.evaluate(() => {
  const r = document.querySelector('.job.running'), l = document.getElementById('list');
  if (!r) return null;
  const a = r.getBoundingClientRect(), b = l.getBoundingClientRect();
  return a.top >= b.top - 1 && a.bottom <= b.bottom + 1;
});

const S = {
  // A message from a chat: words around the links, one without https://, a comma after one.
  chat: async () => {
    await paste(`глянь ${L('c1', 'TINY')}, и ещё www.youtube.com/watch?v=TINYNEW${R}c2 огонь`);
    const b = (await box()).trim().split('\n');
    expect(b.length === 2, 'both links found in the text (' + b.length + ')');
    expect(b.every(l => /^https:\/\//.test(l) && !/,$/.test(l)), 'links get https:// and lose the comma');
  },
  // Plain words: said that it's not a link; the message goes once the box changes.
  words: async () => {
    await pg.fill('#links', 'привет'); await go();
    expect(/ссылк|link/i.test(await msg()), 'plain words: "not a link" is said');
    await pg.fill('#links', ''); await pg.type('#links', ' '); await sleep(300);
    expect(await msg() === '', 'the message goes when the box changes');
  },
  // The same video written three ways is one link.
  twice: async () => {
    await paste(`${L('t1', 'TINY')}\n${L('t1', 'TINY')}\nhttps://www.youtube.com/watch?v=TINYNEW${R}t1&si=x`);
    expect((await box()).trim().split('\n').length === 1, 'one video pasted three ways is one link');
  },
  // The same link again while it's downloading: not added twice, said so.
  again: async () => {
    await paste(L('ag', 'LONG')); await go(); await sleep(1500);
    await paste(L('ag', 'LONG')); await go();
    expect((await rows()).length === 1, 'not queued a second time');
    expect(/Уже в списке|Already in the list/.test(await msg()), '"already in the list" is said: ' + (await msg()));
    await pg.$$eval('.job .x:not(.off)', xs => xs.forEach(x => x.click()));
  },
  // Ten links: kept in the order they were pasted, the running one always in sight.
  many: async () => {
    await paste(Array.from({ length: 10 }, (_, i) => L(String(i).padStart(2, '0'), 'TINY')).join('\n'), 5000);
    await go(); await sleep(1500);
    const names = (await rows()).map(r => r.name);
    expect(names[0] === `Song ${R}00` && names[9] === `Song ${R}09`, 'rows in the order pasted: ' + names[0] + ' … ' + names[9]);
    // Out of sight for a moment when the next one starts (the list scrolls to it): fine. Longer: not.
    let seen = 0, outFor = 0, worst = 0, last = Date.now();
    const t0 = Date.now();
    while (Date.now() - t0 < 120000 && await busy()) {
      const v = await runningInView(), now = Date.now();
      if (v === true) { seen++; outFor = 0; } else if (v === false) { outFor += now - last; worst = Math.max(worst, outFor); }
      last = now;
      await sleep(300);
    }
    expect(seen > 0 && worst <= 1500, `the running download stays in sight (out of sight at most ${worst} ms at a time)`);
  },
  // Cancel the one that waits, then the one that runs.
  cancel: async () => {
    await paste(`${L('a1', 'LONG')}\n${L('a2', 'LONG')}`); await go(); await sleep(2500);
    const xs = await pg.$$('.job .x:not(.off)');
    if (xs[1]) { await xs[1].click(); await sleep(900); }
    if (xs[0]) { await xs[0].click(); await sleep(1500); }
    const st = (await rows()).map(r => r.cls);
    expect(st.length === 2 && st.every(c => /cancelled/.test(c)), 'both cancelled: ' + st.join(', '));
  },
  // A failure: plain words, the whole error readable (two lines, not cut), Retry there.
  fail: async () => {
    await paste('https://youtu.be/FAILfail001'); await go(); await idle();
    const e = await pg.$eval('.job.error .sub', k => ({ cut: k.scrollHeight > k.clientHeight + 1 || k.scrollWidth > k.clientWidth + 1, t: k.textContent }));
    expect(!e.cut, 'the error is shown whole: ' + e.t);
    expect(!!(await pg.$('.job.error .acts .ibtn')), 'Retry is on the row');
  },
  // A playlist with a private video: the rest downloads, the skip is said.
  skip: async () => {
    await paste('https://www.youtube.com/playlist?list=PLSKIPtest', 4000); await go(); await idle(200000);
    const r = await rows();
    expect(r.length === 1 && /done/.test(r[0].cls) && /Пропущено|skipped/i.test(r[0].text), 'done, with the skipped video said');
  },
  // Clear finished while something downloads: the running one stays; Undo brings the rest back.
  clear: async () => {
    await paste(L('k1', 'TINY')); await go(); await idle();
    await paste(L('k2', 'LONG')); await go(); await sleep(2500);
    await pg.click('#clear'); await sleep(1200);
    const r = await rows();
    expect(r.length === 1 && /running/.test(r[0].cls), 'the running download stays');
    await pg.$$eval('.job .x:not(.off)', xs => xs.forEach(x => x.click()));
  },
  // The fishing line, frame by frame: a short clip (100 % comes in one step), a normal song and
  // a playlist. It never goes back (but for a new track's cast), never lurches (a step that
  // changes by more than a few px from one frame to the next), and the lane folds only once the
  // fish has got to the end and swum off.
  line: async () => {
    await pg.evaluate(() => {
      window.__line = {};
      (function tick() {
        document.querySelectorAll('.job').forEach(row => {
          const st = waveState[row.dataset.id], t = waveTarget[row.dataset.id], lane = row.querySelector('.lane'), cv = row.querySelector('canvas.wave');
          if (!st || !t || !lane) return;
          (window.__line[row.dataset.id] = window.__line[row.dataset.id] || []).push({ s: st.shown, pos: t.pos || 0, c: !!st.catchAt, status: t.status, shut: lane.classList.contains('shut'), gone: !!st.gone, w: cv ? cv.clientWidth : 0 });
        });
        requestAnimationFrame(tick);
      })();
    });
    await paste([L('l1', 'TINY'), L('l2'), 'https://www.youtube.com/playlist?list=PL' + R + 'line'].join('\n'), 3500);
    await go(); await idle(200000); await sleep(1500);
    const lines = await pg.evaluate(() => window.__line);
    for (const [id, fr] of Object.entries(lines)) {
      let back = 0, lurch = 0, worst = 0;
      for (let i = 2; i < fr.length; i++) {
        const a = fr[i - 2], b = fr[i - 1], c = fr[i];
        if (c.pos !== b.pos || b.c || c.c || c.status !== b.status) continue;
        if (c.s < b.s - 0.05) back++;
        if (b.pos !== a.pos || a.c || b.status !== a.status) continue;
        const d1 = (b.s - a.s) * c.w / 100, d2 = (c.s - b.s) * c.w / 100;
        if (Math.abs(d2 - d1) > 6) { lurch++; worst = Math.max(worst, Math.abs(d2 - d1)); }
      }
      const fold = fr.findIndex((f, i) => i > 0 && f.shut && !fr[i - 1].shut);
      const before = fold > 0 ? fr[fold - 1] : null;
      expect(!back, `row ${id}: the line never goes back (${back})`);
      expect(lurch <= 2, `row ${id}: no lurches (${lurch}, worst ${worst.toFixed(1)} px a frame)`);
      if (before && before.status === 'done') expect(before.s >= 99.5 && before.gone, `row ${id}: folds once the fish got to the end and swam off (${before.s.toFixed(1)} %, gone ${before.gone})`);
    }
  },
};

(async () => {
  const b = await chromium.launch({ executablePath: process.env.CHROME || undefined });
  const ctx = await b.newContext({ viewport: { width: 653, height: 900 }, colorScheme: 'dark' });
  await ctx.addInitScript(() => { try { localStorage.setItem('tyaga-lang', 'ru'); } catch (e) {} });
  pg = await ctx.newPage();
  pg.on('pageerror', e => { fails.push(`${cur}: page error: ${e.message}`); console.log('  FAIL page error: ' + e.message); });
  await pg.goto(url + (url.includes('?') ? '&' : '?') + 'w=1');
  await pg.waitForFunction(() => typeof window.LANG !== 'undefined');
  await sleep(800);
  await clean();
  for (const [name, fn] of Object.entries(S)) {
    if (only && !only.split(',').includes(name)) continue;
    cur = name; console.log(name);
    try { await fn(); } catch (e) { expect(false, 'ran without crashing: ' + e.message.split('\n')[0]); }
    try { await clean(); } catch (e) {}
  }
  await b.close();
  if (fails.length) {
    console.error(`\n${fails.length} failed:\n- ` + fails.join('\n- '));
    // On GitHub: an annotation, readable on the pull request without the log (as ci.js does).
    if (process.env.GITHUB_ACTIONS) console.log('::error title=Scenarios::' + fails.join('\n').replace(/%/g, '%25').replace(/\r/g, '').replace(/\n/g, '%0A'));
    process.exit(1);
  }
  console.log('\nAll scenarios behave.');
})();
