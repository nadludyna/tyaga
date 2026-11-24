// Pixel snapshots of every state of the page, to compare before and after a change that must not
// change how anything looks (a clean-up, a refactor): every pixel that differs is reported.
//   node stand/snapshot.js <stand address> <dir>                    → saves the PNGs
//   node stand/snapshot.js <stand address> <dir> --compare <before>  → saves, then compares
// The page is given a fixed state (the list, the previews, the pictures) instead of real downloads,
// and motion is reduced, so the same code draws the same picture every time.
const { chromium } = require('playwright');
const fs = require('fs'), path = require('path');
const [url, dir, flag, before] = process.argv.slice(2);
if (!url || !dir) { console.error('usage: node stand/snapshot.js <stand address> <dir> [--compare <dir>]'); process.exit(2); }
fs.mkdirSync(dir, { recursive: true });
const sleep = ms => new Promise(r => setTimeout(r, ms));

// Pictures: a 16:9 "thumbnail" and a square "cover", drawn as SVG (no image files needed).
const thumb = c => `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><defs><linearGradient id="g" x2="1" y2="1"><stop offset="0" stop-color="${c}"/><stop offset="1" stop-color="#222"/></linearGradient></defs><rect width="320" height="180" fill="url(#g)"/><circle cx="160" cy="90" r="40" fill="#fff" fill-opacity=".35"/></svg>`;
const cover = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="300"><rect width="300" height="300" fill="#2d6a4f"/><rect x="75" y="75" width="150" height="150" fill="#f1c0e8"/></svg>`;

const job = (o) => Object.assign({ id: 1, url: 'https://youtu.be/aaaaaaaaaaa', mode: 'audio', format: 'm4a', quality: 0, playlist: false, split: false, edit: false, merge: false, sponsors: false, from: '', to: '', lang: 'en',
  list: false, note: '', status: 'done', title: 'Song', channel: 'Pilotredsky', now: '', thumb: '', log: [], progress: 100, speed: '', size: '', cover: 0, wait: 0, pos: 0, of: 0, eta: '', error: '', errorRaw: '', files: [], already: false, stopped: false }, o);
const JOBS = [
  job({ id: 1, title: 'Rotten Apple Juice', files: ['Pilotredsky - Rotten Apple Juice.m4a'], cover: 1 }),
  job({ id: 2, url: 'https://youtu.be/bbbbbbbbbbb', mode: 'video', format: 'm4a', title: 'Nice Little Penguin (Official Video)', channel: 'Lazy Lizard', files: ['Nice Little Penguin.mp4'], thumb: 'https://i.ytimg.com/vi/bbbbbbbbbbb/mqdefault.jpg' }),
  job({ id: 3, url: 'https://youtu.be/ccccccccccc', status: 'running', title: 'Digital Love', channel: 'Daft Punk', progress: 42, speed: '3.1 MB/s', size: '8 MB of 19 MB', eta: '0:04', thumb: 'https://i.ytimg.com/vi/ccccccccccc/mqdefault.jpg', log: ['Downloading the sound'] }),
  job({ id: 4, url: 'https://youtu.be/ddddddddddd', status: 'queued', title: 'Song in the queue', progress: 0, thumb: 'https://i.ytimg.com/vi/ddddddddddd/mqdefault.jpg' }),
  job({ id: 5, url: 'https://youtu.be/FAILfail001', status: 'error', title: '', channel: '', progress: 0, error: "YouTube refused (403): Retry usually helps.", errorRaw: 'ERROR: HTTP Error 403' }),
  job({ id: 6, url: 'https://youtu.be/eeeeeeeeeee', status: 'cancelled', title: 'Cancelled song', progress: 0 }),
  job({ id: 7, url: 'https://youtu.be/fffffffffff', status: 'cancelled', stopped: true, title: 'Not finished song', progress: 0 }),
];
const PLAYLIST = job({ id: 8, url: 'https://www.youtube.com/playlist?list=PLsnap', list: true, status: 'running', title: '[2/3] Digital Love', channel: 'Daft Punk', progress: 50, pos: 2, of: 3, speed: '2.0 MB/s', size: '3 MB of 6 MB', eta: '0:02', log: ['Downloading the sound'] });

const facts = (id, o) => Object.assign({ kind: 'video', id, title: 'A video called ' + id, channel: 'Some Channel', duration: 245, thumb: '', chapters: 0, parts: [], music: false, heights: [1080, 720, 480, 360], listTitle: '', listCount: 0, ads: 0, adsSec: 0, offtopic: 0 }, o);
const FACTS = {
  'https://youtu.be/vvvvvvvvvvv': facts('vvvvvvvvvvv', { title: 'Pink Floyd - The Dark Side of the Moon (Full Album)', channel: 'Pink Floyd', duration: 2580, chapters: 3, music: true, parts: [{ title: 'Speak to Me', start: 0, end: 67 }, { title: 'Breathe', start: 67, end: 236 }, { title: 'On the Run', start: 236, end: 2580 }], ads: 2, adsSec: 80 }),
};
for (let i = 0; i < 10; i++) { const id = 'many' + String(i).padStart(7, '0'); FACTS['https://youtu.be/' + id] = facts(id, { title: 'Song number ' + (i + 1), duration: 180 + i }); }

function stateWith(jobs) {
  return { version: 'snapshot', ready: true, setup: '', problem: '', out: 'C:\\Users\\me\\Music', outName: 'me › Music', picked: false, canPick: true, musicName: 'me › Music', videoName: 'me › Videos', jobs, login: { can: true, in: false, open: false }, hidden: false };
}

async function page(browser, look, jobs) {
  const ctx = await browser.newContext({ viewport: { width: look.w, height: 900 }, colorScheme: look.theme, reducedMotion: 'reduce', deviceScaleFactor: 1 });
  await ctx.addInitScript(([l, t]) => { try { localStorage.setItem('tyaga-lang', l); localStorage.setItem('tyaga-theme', t); } catch (e) {} }, [look.lang, look.theme]);
  // The text cursor blinks: hidden, or the same state would differ by a few pixels now and then.
  await ctx.addInitScript(() => addEventListener('DOMContentLoaded', () => { const st = document.createElement('style'); st.textContent = '*{caret-color:transparent!important}'; document.head.append(st); }));
  let current = jobs;
  await ctx.route('**/api/state**', r => r.fulfill({ contentType: 'application/json', body: JSON.stringify(stateWith(current)) }));
  await ctx.route('**/api/inspect', async r => { const t = JSON.parse(r.request().postData() || '{}').text; r.fulfill({ contentType: 'application/json', body: JSON.stringify(FACTS[t] || facts('xxxxxxxxxxx')) }); });
  await ctx.route('**/api/peek', r => r.fulfill({ contentType: 'application/json', body: '{}' }));
  await ctx.route('**/api/window', r => r.fulfill({ contentType: 'application/json', body: '{}' }));
  await ctx.route('**/api/cover**', r => r.fulfill({ contentType: 'image/svg+xml', body: cover }));
  await ctx.route('https://i.ytimg.com/**', r => r.fulfill({ contentType: 'image/svg+xml', body: thumb(/b{11}/.test(r.request().url()) ? '#3a86ff' : /c{11}/.test(r.request().url()) ? '#ff006e' : '#8338ec') }));
  const pg = await ctx.newPage();
  const errs = [];
  pg.on('pageerror', e => errs.push(e.message));
  await pg.goto(url + (url.includes('?') ? '&' : '?') + 'w=1');
  await pg.waitForFunction(() => typeof window.LANG !== 'undefined' && document.fonts.status === 'loaded');
  await sleep(1500);
  return { ctx, pg, errs, setJobs: j => { current = j; } };
}
// Settled: no blinking caret, every picture loaded, nothing mid-animation.
async function still(pg) {
  await pg.evaluate(() => { if (!document.getElementById('snapstill')) { const st = document.createElement('style'); st.id = 'snapstill'; st.textContent = '*{caret-color:transparent!important}'; document.head.append(st); } });
  await pg.waitForFunction(() => [...document.images].every(i => i.complete), null, { timeout: 5000 }).catch(() => {});
  await sleep(250);
  await pg.evaluate(() => document.getAnimations().forEach(a => { try { if (a.effect && a.effect.getTiming().iterations !== Infinity) a.finish(); } catch (e) {} }));
  await sleep(150);
}
async function shot(pg, name) {
  await sleep(700); // a state poll and its pictures
  await still(pg);
  await pg.evaluate(() => document.getAnimations().forEach(a => { try { if (a.effect && a.effect.getTiming().iterations !== Infinity) a.finish(); } catch (e) {} }));
  await sleep(150);
  const h = await pg.evaluate(() => { const cs = [...document.querySelectorAll('main > .card')].filter(c => !c.hidden && c.offsetHeight); const flys = [...document.querySelectorAll('.fly:not([hidden])')]; return Math.ceil(Math.max(...cs.map(c => c.getBoundingClientRect().bottom), ...flys.map(f => f.getBoundingClientRect().bottom)) + 16); });
  await pg.screenshot({ path: path.join(dir, name + '.png'), clip: { x: 0, y: 0, width: pg.viewportSize().width, height: Math.min(900, h) } });
}
async function paste(pg, text) {
  await pg.click('#links');
  await pg.evaluate(t => { const dt = new DataTransfer(); dt.setData('text/plain', t); document.getElementById('links').dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true })); }, text);
  await sleep(900);
  await pg.mouse.move(1, 1); // no hover on anything
}

(async () => {
  const browser = await chromium.launch({ executablePath: process.env.CHROME || undefined });
  const looks = [];
  for (const theme of ['dark', 'light']) for (const lang of ['en', 'ru']) for (const w of [653, 440]) looks.push({ theme, lang, w });
  let errors = 0;
  for (const look of looks) {
    const tag = `${look.theme}-${look.lang}-${look.w}`;
    // 1. Empty, a link pasted (the preview), ten links, the drop-downs.
    let s = await page(browser, look, []);
    await shot(s.pg, `${tag}-01-empty`);
    await paste(s.pg, 'https://youtu.be/vvvvvvvvvvv');
    await shot(s.pg, `${tag}-02-preview`);
    for (const [btn, n] of [['#fmtB', '03-format'], ['#cutB', '04-fragment'], ['#dest', '05-folder']]) {
      const b = await s.pg.$(btn); if (b && await b.isVisible()) { await b.click(); await shot(s.pg, `${tag}-${n}`); await s.pg.keyboard.press('Escape'); await sleep(300); }
    }
    await s.pg.click('#mode [data-v="video"]'); await sleep(300);
    const q = await s.pg.$('#qualB'); if (q && await q.isVisible()) { await q.click(); await shot(s.pg, `${tag}-06-quality`); await s.pg.keyboard.press('Escape'); await sleep(300); }
    await s.pg.click('#mode [data-v="audio"]'); await s.pg.fill('#links', ''); await sleep(400);
    await paste(s.pg, Object.keys(FACTS).filter(k => /many/.test(k)).join('\n'));
    await shot(s.pg, `${tag}-07-ten-links`);
    errors += s.errs.length; s.errs.forEach(e => console.log('PAGE ERROR', tag, e)); await s.ctx.close();
    // 2. The list in every kind of row; a playlist and YouTube's pause; folded; the Clear note.
    s = await page(browser, look, JOBS);
    await shot(s.pg, `${tag}-08-list`);
    s.setJobs([PLAYLIST, JOBS[0]]); await sleep(900); // (YouTube's pause counts down in seconds: not still enough for a pixel compare)
    await shot(s.pg, `${tag}-09-playlist`);
    s.setJobs([JOBS[0], JOBS[1], JOBS[2]]); await sleep(900);
    await s.pg.evaluate(() => notice(LANG === 'ru' ? 'Убрано из списка: 3. Файлы на месте.' : 'Cleared 3 downloads from the list. The files stay.', function () {}, 'clear', true));
    await shot(s.pg, `${tag}-10-note-header`);
    await s.pg.evaluate(() => hideNotice(true));
    await s.pg.click('#lchev'); await sleep(600);
    await shot(s.pg, `${tag}-11-folded`);
    s.setJobs([]); await sleep(1200);
    await s.pg.evaluate(() => notice(LANG === 'ru' ? 'Убрано из списка: 3. Файлы на месте.' : 'Cleared 3 downloads from the list. The files stay.', function () {}, 'clear', false));
    await shot(s.pg, `${tag}-12-note-empty`);
    errors += s.errs.length; s.errs.forEach(e => console.log('PAGE ERROR', tag, e)); await s.ctx.close();
    // 2b. Under the mouse: every kind of button, hovered (the styles' :hover rules), in the wide look.
    if (look.w === 653) {
      s = await page(browser, look, JOBS);
      await paste(s.pg, 'https://youtu.be/vvvvvvvvvvv');
      const HOVER = [['seg', '#mode [data-v="video"]'], ['dd', '#fmtB'], ['go', '#go'], ['theme', '#theme'], ['lang', '#lang'], ['pin', '#pin'],
        ['close', '#wClose'], ['min', '#wMin'], ['logo', '#acct'], ['chev', '#lchev'], ['copyall', '#copyall'], ['clear', '#clear'],
        ['show', '.job.done .acts .ibtn'], ['retry', '.job.error .acts .ibtn'], ['cancel', '.job.running .x'], ['chip', '.pv .chip'], ['dest', '#dest']];
      for (const [n, sel] of HOVER) {
        const e = await s.pg.$(sel);
        if (!e || !(await e.isVisible())) { console.log(`(no ${n} to hover: ${sel})`); continue; }
        const b = await e.boundingBox();
        await s.pg.mouse.move(b.x + b.width / 2, b.y + b.height / 2); await sleep(500); await still(s.pg);
        await s.pg.screenshot({ path: path.join(dir, `${tag}-h-${n}.png`), clip: { x: Math.max(0, b.x - 5), y: Math.max(0, b.y - 5), width: Math.min(b.width + 10, look.w - Math.max(0, b.x - 5)), height: b.height + 10 } });
        await s.pg.mouse.move(1, 1); await sleep(200);
      }
      errors += s.errs.length; s.errs.forEach(e => console.log('PAGE ERROR', tag, e)); await s.ctx.close();
    }
    // 3. The tray menu: quiet and busy.
    if (look.w === 653) {
      const ctx = await browser.newContext({ viewport: { width: 264, height: 246 }, colorScheme: look.theme, reducedMotion: 'reduce' });
      await ctx.addInitScript(([l, t]) => { try { localStorage.setItem('tyaga-lang', l); localStorage.setItem('tyaga-theme', t); } catch (e) {} }, [look.lang, look.theme]);
      await ctx.route('**/api/tray', r => r.fulfill({ contentType: 'application/json', body: '{}' }));
      const m = await ctx.newPage();
      await m.goto(url.replace('/?', '/menu?')); await sleep(900);
      for (const [d, n] of [[{ busy: 0, std: true }, '13-menu-idle'], [{ busy: 3, title: 'Adele - Hello (Official Music Video)', progress: 34 }, '14-menu-busy']]) {
        await m.evaluate(x => showNow(x), d); await sleep(300);
        await m.screenshot({ path: path.join(dir, `${tag}-${n}.png`), omitBackground: true });
      }
      await ctx.close();
    }
    process.stdout.write('.');
  }
  console.log(`\n${fs.readdirSync(dir).filter(f => f.endsWith('.png')).length} snapshots in ${dir}${errors ? `, ${errors} page error(s)` : ''}`);

  if (flag === '--compare' && before) {
    // Pixel by pixel, in the browser (no image library needed); a diff image for each that differs.
    const cmp = await (await browser.newContext()).newPage();
    let differ = 0;
    for (const f of fs.readdirSync(before).filter(f => f.endsWith('.png') && !f.endsWith('.diff.png'))) {
      const a = path.join(before, f), b = path.join(dir, f);
      if (!fs.existsSync(b)) { console.log(`MISSING ${f}`); differ++; continue; }
      if (fs.readFileSync(a).equals(fs.readFileSync(b))) continue;
      const r = await cmp.evaluate(async ([x, y]) => {
        const load = src => new Promise(ok => { const i = new Image(); i.onload = () => ok(i); i.src = src; });
        const [i1, i2] = await Promise.all([load(x), load(y)]);
        const w = Math.max(i1.width, i2.width), h = Math.max(i1.height, i2.height);
        const get = i => { const c = document.createElement('canvas'); c.width = w; c.height = h; const g = c.getContext('2d'); g.drawImage(i, 0, 0); return g.getImageData(0, 0, w, h); };
        const d1 = get(i1), d2 = get(i2), out = document.createElement('canvas'); out.width = w; out.height = h;
        const go = out.getContext('2d'), od = go.createImageData(w, h);
        let n = 0, box = [w, h, 0, 0];
        for (let p = 0; p < d1.data.length; p += 4) {
          const diff = Math.abs(d1.data[p] - d2.data[p]) + Math.abs(d1.data[p + 1] - d2.data[p + 1]) + Math.abs(d1.data[p + 2] - d2.data[p + 2]) + Math.abs(d1.data[p + 3] - d2.data[p + 3]);
          const q = p / 4, px = q % w, py = (q - px) / w;
          if (diff > 0) { n++; od.data[p] = 255; od.data[p + 3] = 255; box = [Math.min(box[0], px), Math.min(box[1], py), Math.max(box[2], px), Math.max(box[3], py)]; }
          else { od.data[p] = od.data[p + 1] = od.data[p + 2] = d1.data[p] / 4; od.data[p + 3] = 255; }
        }
        go.putImageData(od, 0, 0);
        return { n, size: i1.width + 'x' + i1.height + (i1.width !== i2.width || i1.height !== i2.height ? ' → ' + i2.width + 'x' + i2.height : ''), box, png: out.toDataURL('image/png') };
      }, ['data:image/png;base64,' + fs.readFileSync(a).toString('base64'), 'data:image/png;base64,' + fs.readFileSync(b).toString('base64')]);
      if (!r.n && !/→/.test(r.size)) continue;
      differ++;
      fs.writeFileSync(path.join(dir, f.replace('.png', '.diff.png')), Buffer.from(r.png.split(',')[1], 'base64'));
      console.log(`DIFFERS ${f}: ${r.n} px (${r.size}) in x ${r.box[0]}–${r.box[2]}, y ${r.box[1]}–${r.box[3]}`);
    }
    console.log(differ ? `\n${differ} snapshot(s) differ` : '\nIdentical, every pixel.');
    await browser.close();
    process.exit(differ ? 1 : 0);
  }
  await browser.close();
})();
