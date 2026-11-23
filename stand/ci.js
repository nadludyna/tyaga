// The stand's layout checks, run by CI in a browser without a window: the same checks as by hand
// before every build (docs/ui-principles.md), so a pull request can't break what worked.
//   stand/run.sh                       → prints the page address
//   node stand/ci.js <address>         → needs Playwright (npm i playwright; npx playwright install chromium)
// Every state: the empty window, a link pasted and looked at, downloads (a playlist, a failure,
// a video) running and done; dark and light, English and Russian, ТЯГА's width and a narrow one.
// Any page error or any problem a check reports fails the run.
const path = require('path');
const { chromium } = require('playwright');

const url = process.argv[2];
if (!url) { console.error('usage: node stand/ci.js <stand address>'); process.exit(2); }
const page = url + (url.includes('?') ? '&' : '?') + 'w=1'; // the page as ТЯГА's window shows it
const CHECK = path.join(__dirname, 'check.js');
// ТЯГА's window shows about 643 px of its 653; 440 is the narrow window the checks also cover.
const WIDTHS = [643, 440], THEMES = ['dark', 'light'], LANGS = ['en', 'ru'];
const LINKS = ['https://www.youtube.com/playlist?list=PLci', 'https://www.youtube.com/watch?v=FAILci00001', 'https://www.youtube.com/watch?v=TINYci00001'];

const problems = [];
const sleep = ms => new Promise(r => setTimeout(r, ms));

async function open(browser, o) {
  const ctx = await browser.newContext({ viewport: { width: o.width, height: 800 }, colorScheme: o.theme });
  // Set before the page's own script reads them: clicking the theme button plays a circle over
  // the page that the checks would measure.
  await ctx.addInitScript(([lang, theme]) => {
    try { localStorage.setItem('tyaga-lang', lang); localStorage.setItem('tyaga-theme', theme); } catch (e) {}
  }, [o.lang, o.theme]);
  const pg = await ctx.newPage();
  const where = `${o.theme} ${o.lang} ${o.width}px`;
  pg.on('pageerror', e => problems.push(`${where}: page error: ${e.message}`));
  await pg.goto(page);
  await pg.waitForFunction(() => typeof window.LANG !== 'undefined' && document.fonts && document.fonts.status === 'loaded', null, { timeout: 30000 });
  // Linux draws fonts a little differently from Windows: measures by the letters get 1 px, not 0.5.
  if (process.platform === 'linux') await pg.evaluate(() => { window.TYAGA_TEXT_TOL = 1; });
  await pg.addScriptTag({ path: CHECK });
  return { ctx, pg, where };
}

// Waits until nothing in the list is opening, folding or animating, so the checks measure the
// settled page (they report false alarms mid-animation).
async function settle(pg) {
  await pg.waitForFunction(() => !document.querySelector('.job.grow,.job.bye,.job.lining,.notice.anim') &&
    !document.getAnimations().some(a => a.playState === 'running' && !(a.effect && a.effect.getTiming().iterations === Infinity)), null, { timeout: 15000 }).catch(() => {});
  await sleep(300);
}

async function checks(pg, where, state) {
  await settle(pg);
  const r = await pg.evaluate(async () => ({
    layout: tyagaCheck(), flyouts: tyagaFlyCheck(), mode: tyagaModeCheck(), lang: await tyagaLangCheck(),
  }));
  let n = 0;
  for (const [k, list] of Object.entries(r)) for (const p of list) { problems.push(`${where}, ${state}, ${k}: ${p}`); n++; }
  console.log(`${where}, ${state}: ${n ? n + ' problem(s)' : 'ok'}`);
}

async function paste(pg, text) {
  await pg.fill('#links', text);
  await pg.waitForTimeout(2500); // the link is looked at (yt-dlp -J) and its preview drawn
}

(async () => {
  const browser = await chromium.launch({ executablePath: process.env.CHROME || undefined }); // CHROME: a browser already installed, instead of Playwright's
  try {
    // 1. Before anything is downloaded: the empty window and a link pasted, in every look.
    for (const width of WIDTHS) for (const theme of THEMES) for (const lang of LANGS) {
      const { ctx, pg, where } = await open(browser, { width, theme, lang });
      await checks(pg, where, 'empty');
      await paste(pg, LINKS[2]);
      await checks(pg, where, 'a link pasted');
      await ctx.close();
    }
    // 2. Downloads: started once (the list lives in ТЯГА, every page after sees it), checked
    // while they run and once they're all over.
    {
      const { ctx, pg, where } = await open(browser, { width: WIDTHS[0], theme: 'dark', lang: 'ru' });
      await paste(pg, LINKS.join('\n'));
      await pg.click('.opts .go');
      await pg.waitForSelector('.job .lane:not(.shut)', { timeout: 20000 });
      await checks(pg, where, 'downloading');
      await pg.waitForFunction(() => document.querySelectorAll('.job').length >= 3 &&
        [...document.querySelectorAll('.job .lane')].every(l => l.classList.contains('shut')), null, { timeout: 120000 });
      await ctx.close();
    }
    for (const width of WIDTHS) for (const theme of THEMES) for (const lang of LANGS) {
      const { ctx, pg, where } = await open(browser, { width, theme, lang });
      await pg.waitForSelector('.job', { timeout: 10000 });
      await checks(pg, where, 'downloads done and failed');
      await ctx.close();
    }
  } finally {
    await browser.close();
  }
  if (problems.length) {
    console.error(`\n${problems.length} problem(s):\n` + problems.map(p => '- ' + p).join('\n'));
    annotate(problems);
    process.exit(1);
  }
  console.log('\nAll layout checks are clean.');
})().catch(e => { console.error(e); annotate([String(e && e.stack || e)]); process.exit(1); });

// On GitHub the problems also go up as an annotation on the run: seen on the pull request
// without opening (or signing in to read) the log.
function annotate(list) {
  if (!process.env.GITHUB_ACTIONS) return;
  // The same problem in many looks is one line, with where it was seen first and how often.
  const kinds = new Map();
  for (const p of list) {
    const i = p.indexOf(': '), what = i < 0 ? p : p.slice(i + 2);
    const k = kinds.get(what) || { n: 0, first: i < 0 ? '' : p.slice(0, i) };
    k.n++; kinds.set(what, k);
  }
  const text = `${list.length} problem(s), ${kinds.size} kind(s):\n` + [...kinds].slice(0, 60)
    .map(([what, k]) => `- ${what} (${k.n}×, first: ${k.first})`).join('\n');
  console.log('::error title=Layout checks::' + text.replace(/%/g, '%25').replace(/\r/g, '').replace(/\n/g, '%0A'));
}
