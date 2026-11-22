// The stand's layout check: run it in the page (the browser's console, or a test tool's
// "run JavaScript") in every state you look at; it returns a list of problems, empty when
// all is well. What it measures is what the owner had to point out by eye before:
// - a button's label not in the middle of its icon or of the button (by the letters
//   themselves, not by the font's line box), more than half a pixel off;
// - controls in one row overlapping each other;
// - anything sticking out of the window sideways, or a label cut off without "…";
// - in Russian, a label still in English; a label not in Onest (a button's own font);
// - words side by side in a row not standing on one line.
(function () {
  var cv = document.createElement('canvas').getContext('2d');
  function seen(e) {
    if (!e.isConnected || e.closest('[hidden]')) return false;
    var r = e.getBoundingClientRect(), cs = getComputedStyle(e);
    return r.width > 0 && r.height > 0 && cs.visibility !== 'hidden' && +cs.opacity > 0.05;
  }
  function name(e) {
    return (e.id ? '#' + e.id : e.tagName.toLowerCase() + (e.className && e.className.baseVal === undefined ? '.' + String(e.className).trim().split(/\s+/).join('.') : '')) +
      ' "' + (e.textContent || '').trim().slice(0, 24) + '"';
  }
  // The middle of the capital letters of a label (where the eye puts its middle).
  function capMid(t) {
    var cs = getComputedStyle(t), r = document.createRange();
    r.selectNodeContents(t);
    var b = r.getBoundingClientRect();
    if (!b.height) return null;
    cv.font = cs.fontWeight + ' ' + cs.fontSize + ' ' + cs.fontFamily;
    var m = cv.measureText('Hн'), cap = cv.measureText('H').actualBoundingBoxAscent;
    // The text's own box (not the element's) is the font's box: baseline = its top + ascent.
    var base = b.top + (b.height - m.fontBoundingBoxAscent - m.fontBoundingBoxDescent) / 2 + m.fontBoundingBoxAscent;
    return base - cap / 2;
  }
  function textEl(b) {
    var s = [].slice.call(b.querySelectorAll('span')).filter(function (x) { return x.textContent.trim() && seen(x); })[0];
    return s || (b.textContent.trim() ? b : null);
  }
  window.tyagaCheck = function () {
    var bad = [];
    // Measure things where they end up, not halfway through their appearing (a badge pops in from 30 %).
    document.getAnimations().forEach(function (x) { try { if (x.effect && x.effect.getTiming().iterations !== Infinity) x.finish(); } catch (e) {} });
    // 1. Labels in the middle.
    document.querySelectorAll('button,.chip,.seg button').forEach(function (b) {
      if (!seen(b)) return;
      var t = textEl(b);
      if (!t) return;
      if (!/Onest/.test(getComputedStyle(t).fontFamily)) bad.push('not in Onest: ' + name(b));
      var c = capMid(t);
      if (c === null) return;
      var icon = [].slice.call(b.querySelectorAll(':scope>svg')).filter(seen)[0];
      var ir = icon && icon.getBoundingClientRect();
      // The icon above the word (Download): nothing to line up.
      if (ir && ir.bottom < c) icon = null;
      if (icon) {
        var d = c - (ir.top + ir.bottom) / 2;
        if (Math.abs(d) > 0.5) bad.push('label vs icon ' + d.toFixed(2) + ' px: ' + name(b));
      } else if (b.getBoundingClientRect().height <= 40) {
        var br = b.getBoundingClientRect(), d2 = c - (br.top + br.bottom) / 2;
        if (Math.abs(d2) > 0.75) bad.push('label vs button ' + d2.toFixed(2) + ' px: ' + name(b));
      }
      if (t.scrollWidth > t.clientWidth + 1 && getComputedStyle(t).textOverflow !== 'ellipsis' && t !== b) bad.push('label cut off: ' + name(b));
    });
    // 1b. A number in a round badge (how many links on Download) in the middle of its circle.
    document.querySelectorAll('.gon').forEach(function (g) {
      var t = g.firstElementChild || g;
      if (!seen(g) || !t.textContent.trim()) return;
      var r = document.createRange(); r.selectNodeContents(t);
      var tr = r.getBoundingClientRect(), gr = g.getBoundingClientRect();
      cv.font = getComputedStyle(t).fontWeight + ' ' + getComputedStyle(t).fontSize + ' ' + getComputedStyle(t).fontFamily;
      var m = cv.measureText(t.textContent), base = tr.top + (tr.height - m.fontBoundingBoxAscent - m.fontBoundingBoxDescent) / 2 + m.fontBoundingBoxAscent;
      var mid = base - m.actualBoundingBoxAscent / 2, d = mid - (gr.top + gr.bottom) / 2;
      if (Math.abs(d) > 0.5) bad.push('number vs its circle ' + d.toFixed(2) + ' px: ' + name(g));
    });
    // 2. Nothing overlaps in a row.
    document.querySelectorAll('.row,.opts,.lbar,.top,.line2,header,.pvc,.input').forEach(function (row) {
      if (!seen(row)) return;
      var kids = [].slice.call(row.children).filter(function (k) { return seen(k) && getComputedStyle(k).position !== 'absolute'; });
      for (var i = 0; i < kids.length; i++) for (var j = i + 1; j < kids.length; j++) {
        var a = kids[i].getBoundingClientRect(), b = kids[j].getBoundingClientRect();
        var w = Math.min(a.right, b.right) - Math.max(a.left, b.left), h = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
        if (w > 1 && h > 1) bad.push('overlap: ' + name(kids[i]) + ' and ' + name(kids[j]));
      }
    });
    // 2b. Words side by side in one row stand on one line: their letters' middles within 1 px.
    document.querySelectorAll('.lbar,.opts,.line2,.top').forEach(function (row) {
      if (!seen(row)) return;
      var mids = [];
      row.querySelectorAll('span,button,.kind,.nums,.ovr,.st').forEach(function (e) {
        if (!seen(e) || e.closest('.menu') || ![].some.call(e.childNodes, function (n) { return n.nodeType === 3 && n.textContent.trim(); })) return;
        var c = capMid(e);
        if (c !== null) mids.push([c, e]);
      });
      // (a row that wraps in a narrow window puts things on a second line: those aren't compared)
      mids.forEach(function (m) { var d = Math.abs(m[0] - mids[0][0]); if (d > 1 && d < 12) bad.push('not on one line with ' + name(mids[0][1]) + ' by ' + (m[0] - mids[0][0]).toFixed(1) + ' px: ' + name(m[1])); });
    });
    // 2c. A wide window: the options row (audio/video, the drop-downs, the folder) stays one row.
    if (innerWidth > 560) {
      var tops = [].slice.call(document.querySelectorAll('.opts>.seg,.opts>.ddw>.dd')).filter(seen).map(function (e) { return e.getBoundingClientRect().top; });
      if (tops.length && Math.max.apply(null, tops) - Math.min.apply(null, tops) > 6) bad.push('the options row broke into lines');
      // and the folder's name on its button is whole (where the files go: one of the four questions)
      var dt = document.getElementById('destT');
      if (dt && seen(dt) && dt.scrollWidth > dt.clientWidth + 1) bad.push('the folder name is cut: ' + name(dt));
    }
    // 2d. Letters whole: where a label is clipped (… at the end), the clip must not cut the
    // tails of у, р, g (the label's box, trimmed to the letters, plus its padding).
    document.querySelectorAll('span,div,b').forEach(function (e) {
      if (!seen(e) || !e.textContent.trim() || e.children.length) return;
      var cs = getComputedStyle(e);
      if (cs.overflowY === 'visible' && cs.overflowX === 'visible') return;
      var r = document.createRange(); r.selectNodeContents(e);
      var t = r.getBoundingClientRect(), b = e.getBoundingClientRect();
      if (!t.height) return;
      cv.font = cs.fontWeight + ' ' + cs.fontSize + ' ' + cs.fontFamily;
      var m = cv.measureText('уgр'), base = t.top + (t.height - m.fontBoundingBoxAscent - m.fontBoundingBoxDescent) / 2 + m.fontBoundingBoxAscent;
      if (base + m.actualBoundingBoxDescent > b.bottom + 0.5) bad.push('letters cut at the bottom: ' + name(e));
    });
    // 3. Nothing sticks out sideways.
    var se = document.scrollingElement;
    if (se.scrollWidth > se.clientWidth + 1) bad.push('the page scrolls sideways by ' + (se.scrollWidth - se.clientWidth) + ' px');
    document.querySelectorAll('.card').forEach(function (card) {
      if (!seen(card)) return;
      var cr = card.getBoundingClientRect();
      card.querySelectorAll('button,.chip,.seg,.pvi,.job').forEach(function (e) {
        if (!seen(e) || e.closest('.menu')) return;
        var r = e.getBoundingClientRect();
        if (r.right > cr.right + 0.5 || r.left < cr.left - 0.5) bad.push('out of its card: ' + name(e));
      });
    });
    // 4. Russian: no label left in English.
    if (document.documentElement.lang === 'ru' || (window.LANG === 'ru')) {
      document.querySelectorAll('[data-t]').forEach(function (e) {
        if (seen(e) && e.textContent.trim() === e.dataset.t.trim() && /[a-z]{3}/i.test(e.dataset.t)) bad.push('not translated: ' + name(e));
      });
    }
    // English: no Russian left in the window's own words (titles and links are the user's).
    if (window.LANG === 'en') {
      var w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
      for (var n; (n = w.nextNode());) {
        var p = n.parentElement;
        if (/[а-яё]/i.test(n.textContent) && seen(p) && !p.closest('.name,.pvt,.pvch,textarea,#tip,h1,.kind,.nums')) bad.push('Russian in English: ' + name(p));
      }
    }
    // 5. An open drop-down lies inside the window.
    document.querySelectorAll('.fly').forEach(function (f) {
      if (!seen(f)) return;
      var r = f.getBoundingClientRect();
      if (r.top < 0 || r.left < 0 || r.right > innerWidth + 0.5 || r.bottom > innerHeight + 0.5) bad.push('drop-down outside the window: ' + name(f));
    });
    return bad;
  };
  // The drop-downs (docs/ui-principles.md, 1b): each one opened in turn must fit in the window,
  // not change the page's height (the window would follow) and pass tyagaCheck while open.
  window.tyagaFlyCheck = function () {
    var bad = [], h0 = document.querySelector('main').scrollHeight;
    document.querySelectorAll('.opts .dd').forEach(function (b) {
      if (!seen(b)) return;
      b.click();
      var h = document.querySelector('main').scrollHeight;
      // the open drop-down is really on top: nothing covers or clips any of its corners
      var f = document.querySelector('.fly:not([hidden])');
      if (f) {
        // it comes in as one piece: nothing inside it moves or fades by itself (the owner saw the
        // fragment's fields rise inside it)
        document.getAnimations().forEach(function (a) {
          var t = a.effect && a.effect.target;
          if (t && t !== f && f.contains(t) && a.playState === 'running') bad.push(name(b) + ' open: something inside it animates: ' + name(t) + ' (' + (a.animationName || a.transitionProperty) + ')');
        });
        var fr = f.getBoundingClientRect();
        [[fr.left + 6, fr.top + 6], [fr.right - 6, fr.top + 6], [fr.left + 6, fr.bottom - 6], [fr.right - 6, fr.bottom - 6]].forEach(function (p) {
          var at = document.elementFromPoint(p[0], p[1]);
          if (!at || !f.contains(at)) bad.push(name(b) + ' open: its drop-down is covered or cut at ' + p.map(Math.round) + ' by ' + (at ? name(at) : 'nothing'));
        });
      }
      if (Math.abs(h - h0) > 1) bad.push('opening ' + name(b) + ' changed the height by ' + (h - h0) + ' px');
      window.tyagaCheck().forEach(function (x) { bad.push(name(b) + ' open: ' + x); });
      document.body.click();
    });
    return bad;
  };
  // Switching video/audio moves nothing in the options row.
  window.tyagaModeCheck = function () {
    var at = function () { return [].map.call(document.querySelectorAll('.opts>*'), function (e) { return e.hidden ? 'h' : Math.round(e.getBoundingClientRect().right); }); };
    var btn = document.querySelectorAll('#mode button'), was = document.querySelector('#mode .on'), out = [];
    var a = null;
    [].forEach.call(btn, function (b) {
      b.click();
      var r = at().filter(function (x) { return x !== 'h'; }).join(',');
      if (a !== null && r !== a) out.push('switching to ' + b.dataset.v + ' moved the row: ' + a + ' → ' + r);
      a = r;
    });
    was.click();
    return out;
  };
})();
