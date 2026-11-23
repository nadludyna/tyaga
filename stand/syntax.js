// Compiles every inline script of the pages (index.html, menu.html) without running it. One
// typo in the page's script and the page does nothing at all while the Go tests stay green
// (it happened in a test build): CI runs this so it can't reach a build again.
//   node stand/syntax.js
const fs = require('fs'), path = require('path'), vm = require('vm');
let bad = 0;
for (const page of ['index.html', 'menu.html']) {
  const html = fs.readFileSync(path.join(__dirname, '..', page), 'utf8');
  const re = /<script(\s[^>]*)?>([\s\S]*?)<\/script>/gi;
  let m, n = 0;
  while ((m = re.exec(html))) {
    if (/\bsrc=/.test(m[1] || '') || !m[2].trim()) continue;
    n++;
    const line = html.slice(0, m.index).split('\n').length;
    try { new vm.Script(m[2], { filename: page + ' (script from line ' + line + ')' }); }
    catch (e) { bad++; console.error(page + ', script from line ' + line + ': ' + e.message + '\n' + (e.stack || '').split('\n').slice(0, 3).join('\n')); }
  }
  console.log(page + ': ' + n + ' script(s) compiled');
}
process.exit(bad ? 1 : 0);
