"""Builds the pricing results page (self-contained HTML) from page_data.json and the real-battle checks."""
import json, os, sys
OUT = os.environ.get('EXCHANGE_DIR', os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../runs/exchange'))
data = json.load(open(os.path.join(OUT, 'page_data.json')))
checks = []
cp = os.path.join(OUT, 'page-checks.json')
if os.path.exists(cp):
    for p in json.load(open(cp)):
        s, X, i, j = p['name'].split('|')
        checks.append(dict(scheme=s, X=int(X), i=int(i), j=int(j), model=round(p['model'], 4), score=round(p['score'], 4),
                           w=p['wins'], d=p['draws'], l=p['losses'], n=p['battles']))

HTML = r'''<!doctype html>
<html><head><meta charset="utf-8">
<style>
:root { --win: #3987e5; --lose: #e66767; }
body { margin: 0; }
.wrap { font-family: var(--font-sans); color: var(--foreground); font-size: 14px; line-height: 1.45; }
h2 { font-size: 16px; margin: 26px 0 4px; font-weight: 600; }
p.sub { color: var(--muted-foreground); margin: 0 0 12px; font-size: 13px; }
.controls { display: flex; flex-wrap: wrap; gap: 8px 10px; margin: 4px 0 14px; }
.seg { display: inline-flex; border: 1px solid var(--border); border-radius: var(--radius); overflow: hidden; }
.seg button { font: inherit; font-size: 13px; background: transparent; color: var(--muted-foreground); border: 0; padding: 6px 11px; cursor: pointer; border-right: 1px solid var(--border); }
.seg button:last-child { border-right: 0; }
.seg button.on { background: var(--secondary); color: var(--foreground); font-weight: 600; }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 10px; }
.card { border: 1px solid var(--border); border-radius: var(--radius); padding: 12px 14px; cursor: pointer; background: transparent; text-align: left; font: inherit; color: inherit; }
.card.on { border-color: var(--foreground); background: var(--card); }
.card .k { color: var(--muted-foreground); font-size: 12px; }
.card .big { font-size: 22px; font-weight: 700; margin: 2px 0; }
.card .s { color: var(--muted-foreground); font-size: 12px; }
table { border-collapse: separate; border-spacing: 3px; }
table.builds { border-collapse: collapse; width: 100%; font-size: 13px; }
table.builds th, table.builds td { text-align: right; padding: 5px 8px; border-bottom: 1px solid var(--border); white-space: nowrap; }
table.builds th:first-child, table.builds td:first-child { text-align: left; }
table.builds th { color: var(--muted-foreground); font-weight: 500; }
.sw { display: inline-block; width: 10px; height: 10px; border-radius: 3px; margin-right: 6px; vertical-align: -1px; }
.matrix td.cell { width: 104px; height: 54px; text-align: center; border-radius: 6px; font-variant-numeric: tabular-nums; position: relative; }
.matrix td.cell .v { font-size: 16px; }
.matrix td.cell .w { font-size: 11px; color: var(--muted-foreground); }
.matrix td.cell.lop { outline: 2px solid var(--foreground); outline-offset: -2px; font-weight: 700; }
.matrix td.cell.self { background: transparent; border: 1px dashed var(--border); }
.matrix th { font-weight: 500; font-size: 12px; color: var(--muted-foreground); padding: 2px 6px; }
.matrix th.row { text-align: right; white-space: nowrap; }
.legend { display: flex; gap: 14px; flex-wrap: wrap; font-size: 12px; color: var(--muted-foreground); margin: 6px 0 10px; }
.chart { position: relative; }
.tip { position: absolute; pointer-events: none; background: var(--popover); color: var(--popover-foreground); border: 1px solid var(--border); border-radius: 8px; padding: 8px 10px; font-size: 12px; line-height: 1.5; display: none; z-index: 5; white-space: nowrap; box-shadow: 0 4px 16px rgba(0,0,0,.25); }
.mult { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 10px; }
.mult .panel { border: 1px solid var(--border); border-radius: var(--radius); padding: 10px 12px; cursor: pointer; }
.mult .panel.on { border-color: var(--foreground); }
.mult .t { font-size: 12px; font-weight: 600; }
.mult .s { font-size: 11px; color: var(--muted-foreground); }
table.checks { border-collapse: collapse; font-size: 12px; width: 100%; }
table.checks td, table.checks th { padding: 4px 6px; border-bottom: 1px solid var(--border); text-align: right; white-space: nowrap; font-variant-numeric: tabular-nums; }
table.checks th { color: var(--muted-foreground); font-weight: 500; }
table.checks td:first-child, table.checks th:first-child { text-align: left; }
.note { color: var(--muted-foreground); font-size: 12px; }
</style></head>
<body><div class="wrap" id="app"></div>
<script>
const RAW = __DATA__, RAWC = __CHECKS__;
const DATA = { levels: RAW.levels, curve: RAW.curve, pairs: RAW.pairs, schemes: {} };
for (const k in RAW.schemes) {
  const s = RAW.schemes[k], o = { label: s.label, defense: s.defense, data: {} };
  for (const p in s.data) {
    const d = s.data[p], tables = {}, f = v => v < 0 ? null : v / 1000;
    for (const X in d.t) tables[X] = { builds: d.t[X].b.map(b => ({ damage: b[0], cards: b[1], oxp: b[2], dxp: b[3] })),
      matrix: d.t[X].m.map(r => r.map(c => c ? { w: c[0] / 1000, d: c[1] / 1000, l: c[2] / 1000, rounds: c[3] } : null)) };
    o.data[p] = { tables, curve: d.c.map(c => c.map(f)), low: d.lo.map(f), high: d.hi.map(f), shares: d.sh };
  }
  DATA.schemes[k] = o;
}
const CHECKS = RAWC.map(c => ({ scheme: c[0], X: c[1], i: c[2], j: c[3], model: c[4] / 1000, score: c[5] / 1000, w: c[6], d: c[7], l: c[8], n: c[6] + c[7] + c[8] }));
const LEVELS = DATA.levels, CURVE = DATA.curve;
const ASKED = [400, 650, 900];
const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)'];
const DEFNAME = { holdfast: 'Holdfast', light: 'Light Guard', ward37: '37% defense', none: 'no defense' };
const SCHEME_TEXT = {
  flat: 'Cards 10 XP each. The board costs 40 XP per point of average damage above 6.2; Holdfast costs 140 XP (40 per point of average prevention). Cards = (total XP − board − defense) ÷ 10.',
  pct: 'Percentage of total XP. The board keeps a share of your XP for cards and the defense keeps a share of what is left, both fitted to the equal-rounds equation for this damage pair; every remaining 10 XP is a card.',
  pctlight: 'The same percentage pricing with Light Guard (prevents 0/1/2, average 1) as the defense instead of Holdfast.',
  ward: 'Exact solution: the defense prevents 37% of every hit. Cards = total XP ÷ 10 × (6.2 ÷ damage) × 0.63. Model only: the engine cannot prevent a percentage of a hit yet, so there are no real battles for it.',
};
let S = { scheme: 'pct', pair: '7-12', level: 400 };

const pct = v => (v * 100).toFixed(0);
const score = c => c.w + c.d / 2;
function names(scheme, pair) {
  const [lo, hi] = pair.split('-');
  const d = DEFNAME[DATA.schemes[scheme].defense];
  return [`Low ${lo} · no defense`, `Low ${lo} · ${d}`, `High ${hi} · no defense`, `High ${hi} · ${d}`];
}
function cellColor(s) {
  const t = Math.max(-1, Math.min(1, (s - 0.5) / 0.3));
  const col = t >= 0 ? 'var(--win)' : 'var(--lose)';
  return `color-mix(in oklab, ${col} ${Math.round(Math.abs(t) * 70)}%, var(--secondary))`;
}
function lopsided(scheme, pair, levels) {
  let n = 0, tot = 0, lo = 1, hi = 0;
  for (const X of levels) {
    const m = DATA.schemes[scheme].data[pair].tables[X].matrix;
    for (let i = 0; i < 4; i++) for (let j = i + 1; j < 4; j++) {
      const c = m[i][j]; if (!c) continue;
      const s = score(c); tot++; if (s < 0.4 || s > 0.6) n++;
      lo = Math.min(lo, s, 1 - s); hi = Math.max(hi, s, 1 - s);
    }
  }
  return { n, tot, lo, hi };
}
function seg(label, opts, key) {
  return `<div class="seg" role="group" aria-label="${label}">` + opts.map(([v, t]) =>
    `<button data-k="${key}" data-v="${v}" class="${String(S[key]) === String(v) ? 'on' : ''}">${t}</button>`).join('') + '</div>';
}
function render() {
  const app = document.getElementById('app');
  const sch = DATA.schemes[S.scheme], D = sch.data[S.pair], T = D.tables[S.level], N = names(S.scheme, S.pair);
  let h = '';
  h += '<div class="controls">' +
    seg('Pricing', Object.keys(DATA.schemes).map(k => [k, DATA.schemes[k].label]), 'scheme') +
    seg('Damage', DATA.pairs.map(p => [p, p.replace('-', ' vs ') + ' dmg']), 'pair') +
    seg('Total XP', LEVELS.map(x => [x, x + ' XP']), 'level') + '</div>';
  h += `<p class="sub">${SCHEME_TEXT[S.scheme]}</p>`;
  // summary cards
  h += '<div class="cards">';
  for (const k of Object.keys(DATA.schemes)) {
    const a = lopsided(k, S.pair, ASKED), all = lopsided(k, S.pair, LEVELS);
    h += `<button class="card ${k === S.scheme ? 'on' : ''}" data-k="scheme" data-v="${k}">
      <div class="k">${DATA.schemes[k].label}</div>
      <div class="big">${a.n} lopsided</div>
      <div class="s">of ${a.tot} pairings at 400 / 650 / 900 XP<br>all five levels: ${all.n} of ${all.tot}<br>pairings range ${pct(a.lo)}–${pct(a.hi)}%</div></button>`;
  }
  h += '</div>';
  // builds
  h += `<h2>The four builds at ${S.level} XP</h2><p class="sub">Damage and defense chosen, every XP left over in blank cards (10 XP each). No starting deck: with nothing bought, ${S.level} XP is ${S.level / 10} cards.</p>`;
  h += '<table class="builds"><tr><th>Build</th><th>Average damage</th><th>Board XP</th><th>Defense XP</th><th>Cards</th><th>Card XP</th><th>Total</th></tr>';
  T.builds.forEach((b, i) => {
    h += `<tr><td><span class="sw" style="background:${COLORS[i]}"></span>${N[i]}</td><td>${b.damage.toFixed(2)}</td><td>${Math.round(b.oxp)}</td><td>${Math.round(b.dxp)}</td><td>${b.cards}</td><td>${b.cards * 10}</td><td>${Math.round(b.oxp + b.dxp + b.cards * 10)}</td></tr>`;
  });
  h += '</table>';
  if (D.shares) {
    const f = D.shares.f, g = D.shares.g, ks = Object.keys(f);
    h += `<p class="note">Fitted shares: ${ks.map(o => `${o} damage costs ${(100 * (1 - f[o])).toFixed(1)}% of total XP`).join(', ')}; ${DEFNAME[sch.defense]} costs ${(100 * (1 - g[sch.defense])).toFixed(1)}% of the XP left after the board.</p>`;
  }
  // matrix
  h += `<h2>Every matchup at ${S.level} XP</h2><p class="sub">Row build's chance to beat the column build (wins + ½ draws), exact card counts, race model. Small line: win / draw / loss %. Outlined: lopsided (below 40% or above 60%). Hover for the rounds each side needs${S.pair === '7-12' && S.scheme !== 'ward' && ASKED.includes(S.level) ? ' and the real-battle result' : ''}.</p>`;
  h += '<div class="legend"><span><span class="sw" style="background:var(--lose)"></span>row loses more</span><span><span class="sw" style="background:var(--secondary)"></span>50%</span><span><span class="sw" style="background:var(--win)"></span>row wins more</span></div>';
  h += '<div class="chart" id="mx"><table class="matrix"><tr><th></th>' + N.map((n, j) => `<th>${j + 1}</th>`).join('') + '<th>Mean</th></tr>';
  for (let i = 0; i < 4; i++) {
    h += `<tr><th class="row"><span class="sw" style="background:${COLORS[i]}"></span>${i + 1} ${N[i]}</th>`;
    const vals = [];
    for (let j = 0; j < 4; j++) {
      const c = T.matrix[i][j];
      if (i === j) { h += '<td class="cell self"></td>'; continue; }
      if (!c) { h += '<td class="cell self note">—</td>'; continue; }
      const s = score(c); vals.push(s);
      h += `<td class="cell ${s < 0.4 || s > 0.6 ? 'lop' : ''}" style="background:${cellColor(s)}" data-i="${i}" data-j="${j}"><div class="v">${pct(s)}</div><div class="w">${pct(c.w)}/${pct(c.d)}/${pct(c.l)}</div></td>`;
    }
    h += `<th>${vals.length ? (100 * vals.reduce((a, b) => a + b, 0) / vals.length).toFixed(1) : '—'}</th></tr>`;
  }
  h += '</table><div class="tip" id="mxtip"></div></div>';
  // mean chart
  h += `<h2>Mean score against the other three, by total XP</h2><p class="sub">Each build's average chance to beat the other three at every total XP from 300 to 1,500 (every 20 XP), exact card counts. Shaded: 45–55%. Dashed: the levels you asked for (400, 650, 900). Hover to read the values.</p>`;
  h += '<div class="legend">' + N.map((n, i) => `<span><span class="sw" style="background:${COLORS[i]}"></span>${n}</span>`).join('') + '</div>';
  h += '<div class="chart" id="mc"><svg id="msvg" width="100%" height="300"></svg><div class="tip" id="mctip"></div></div>';
  // range small multiples
  h += `<h2>Worst and best matchup, by total XP</h2><p class="sub">The band runs from the most lopsided loss to the most lopsided win among the six pairings at each XP, for every pricing scheme (${S.pair.replace('-', ' vs ')} damage). Shaded: 40–60%, the not-lopsided zone. Click a panel to select it.</p>`;
  h += '<div class="mult">' + Object.keys(DATA.schemes).map(k => `<div class="panel ${k === S.scheme ? 'on' : ''}" data-k="scheme" data-v="${k}"><div class="t">${DATA.schemes[k].label}</div><svg class="band" data-s="${k}" width="100%" height="110"></svg><div class="s" id="bs-${k}"></div></div>`).join('') + '</div>';
  // real battles
  const ck = CHECKS.filter(c => c.scheme === S.scheme);
  h += `<h2>Real battles</h2>`;
  if (!CHECKS.length) h += '<p class="sub">Real-battle checks are still running.</p>';
  else if (S.scheme === 'ward') h += '<p class="sub">No real battles for the 37% defense: the engine cannot prevent a percentage of a hit yet.</p>';
  else {
    const gaps = CHECKS.map(c => Math.abs(c.model - c.score));
    h += `<p class="sub">Every pairing of the four builds at 7 vs 12 damage, at 400, 650 and 900 XP, for the three schemes the engine can play: ${CHECKS.length} matchups, 2,000 battles each (good to ±2.2 points), greedy players, blank decks. Gap from the model: average ${(100 * gaps.reduce((a, b) => a + b, 0) / gaps.length).toFixed(1)} points, largest ${(100 * Math.max(...gaps)).toFixed(1)}. This scheme:</p>`;
    const NN = names(S.scheme, '7-12');
    h += '<table class="checks"><tr><th>XP</th><th>Matchup</th><th>Model</th><th>Battles</th><th>Win / draw / loss</th></tr>' + ck.map(c =>
      `<tr><td>${c.X}</td><td style="text-align:left">${NN[c.i]} vs ${NN[c.j]}</td><td>${pct(c.model)}%</td><td>${(100 * c.score).toFixed(1)}%</td><td>${pct(c.w / c.n)} / ${pct(c.d / c.n)} / ${pct(c.l / c.n)}</td></tr>`).join('') + '</table>';
  }
  app.innerHTML = h;
  app.querySelectorAll('[data-k]').forEach(el => el.addEventListener('click', () => {
    const k = el.dataset.k, v = el.dataset.v; S[k] = (k === 'level') ? +v : v; render();
  }));
  matrixHover(T, N);
  drawMean(D, N);
  Object.keys(DATA.schemes).forEach(k => drawBand(k));
}
function matrixHover(T, N) {
  const box = document.getElementById('mx'), tip = document.getElementById('mxtip');
  box.querySelectorAll('td.cell[data-i]').forEach(td => {
    td.addEventListener('mousemove', e => {
      const i = +td.dataset.i, j = +td.dataset.j, c = T.matrix[i][j], r = T.matrix[j][i];
      let t = `<b>${N[i]}</b> vs <b>${N[j]}</b><br>score ${(100 * score(c)).toFixed(1)}% · win ${pct(c.w)}% · draw ${pct(c.d)}% · loss ${pct(c.l)}%<br>rounds for ${j + 1} to defeat ${i + 1}: ${c.rounds} · for ${i + 1} to defeat ${j + 1}: ${r.rounds}`;
      const ch = CHECKS.find(x => x.scheme === S.scheme && x.X === S.level && S.pair === '7-12' && ((x.i === i && x.j === j) || (x.i === j && x.j === i)));
      if (ch) {
        const flip = ch.i !== i, sc = flip ? 1 - ch.score : ch.score, w = flip ? ch.l : ch.w, l = flip ? ch.w : ch.l;
        t += `<br>real battles: ${(100 * sc).toFixed(1)}% · ${pct(w / ch.n)} / ${pct(ch.d / ch.n)} / ${pct(l / ch.n)}`;
      }
      tip.innerHTML = t; tip.style.display = 'block';
      const bb = box.getBoundingClientRect();
      let x = e.clientX - bb.left + 12; if (x + 300 > bb.width) x = e.clientX - bb.left - 310;
      tip.style.left = Math.max(0, x) + 'px'; tip.style.top = (e.clientY - bb.top + 12) + 'px';
    });
    td.addEventListener('mouseleave', () => tip.style.display = 'none');
  });
}
const NS = 'http://www.w3.org/2000/svg';
function el(tag, attrs, parent) { const e = document.createElementNS(NS, tag); for (const k in attrs) e.setAttribute(k, attrs[k]); if (parent) parent.appendChild(e); return e; }
function drawMean(D, N) {
  const svg = document.getElementById('msvg'), W = svg.clientWidth || 700, H = 300, L = 40, R = 10, Tp = 8, B = 26;
  svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
  const x = v => L + (v - 300) / 1200 * (W - L - R), y = v => Tp + (1 - v) * (H - Tp - B);
  el('rect', { x: L, y: y(0.55), width: W - L - R, height: y(0.45) - y(0.55), fill: 'var(--foreground)', 'fill-opacity': 0.06 }, svg);
  for (let v = 0; v <= 1.0001; v += 0.2) {
    el('line', { x1: L, x2: W - R, y1: y(v), y2: y(v), stroke: 'var(--border)', 'stroke-width': 1 }, svg);
    el('text', { x: L - 6, y: y(v) + 4, 'text-anchor': 'end', 'font-size': 11, fill: 'var(--muted-foreground)' }, svg).textContent = Math.round(v * 100) + '%';
  }
  for (let v = 300; v <= 1500; v += 200) el('text', { x: x(v), y: H - 8, 'text-anchor': 'middle', 'font-size': 11, fill: 'var(--muted-foreground)' }, svg).textContent = v;
  ASKED.forEach(v => el('line', { x1: x(v), x2: x(v), y1: Tp, y2: H - B, stroke: 'var(--muted-foreground)', 'stroke-dasharray': '3 4', 'stroke-width': 1 }, svg));
  D.curve.forEach((c, i) => {
    let d = '', pen = false;
    c.forEach((v, k) => { if (v == null) { pen = false; return; } d += (pen ? 'L' : 'M') + x(CURVE[k]).toFixed(1) + ' ' + y(v).toFixed(1); pen = true; });
    el('path', { d, fill: 'none', stroke: COLORS[i], 'stroke-width': 2, 'stroke-linejoin': 'round' }, svg);
  });
  const cross = el('line', { y1: Tp, y2: H - B, stroke: 'var(--foreground)', 'stroke-width': 1, opacity: 0 }, svg);
  const tip = document.getElementById('mctip'), box = document.getElementById('mc');
  svg.addEventListener('mousemove', e => {
    const bb = svg.getBoundingClientRect(), px = (e.clientX - bb.left) * W / bb.width;
    const v = 300 + (px - L) / (W - L - R) * 1200;
    let k = 0; CURVE.forEach((c, n) => { if (Math.abs(c - v) < Math.abs(CURVE[k] - v)) k = n; });
    cross.setAttribute('x1', x(CURVE[k])); cross.setAttribute('x2', x(CURVE[k])); cross.setAttribute('opacity', 0.4);
    tip.innerHTML = `<b>${CURVE[k]} XP</b><br>` + N.map((n, i) => `<span class="sw" style="background:${COLORS[i]}"></span>${n}: ${D.curve[i][k] == null ? '—' : (100 * D.curve[i][k]).toFixed(1) + '%'}`).join('<br>') +
      `<br><span style="color:var(--muted-foreground)">matchups ${D.low[k] == null ? '—' : pct(D.low[k]) + '–' + pct(D.high[k]) + '%'}</span>`;
    tip.style.display = 'block';
    let lx = e.clientX - bb.left + 14; if (lx + 230 > bb.width) lx = e.clientX - bb.left - 240;
    tip.style.left = lx + 'px'; tip.style.top = '10px';
  });
  svg.addEventListener('mouseleave', () => { tip.style.display = 'none'; cross.setAttribute('opacity', 0); });
}
function drawBand(k) {
  const svg = document.querySelector(`svg.band[data-s="${k}"]`); if (!svg) return;
  const D = DATA.schemes[k].data[S.pair], W = svg.clientWidth || 170, H = 110, L = 4, R = 4, Tp = 4, B = 4;
  svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
  const x = v => L + (v - 300) / 1200 * (W - L - R), y = v => Tp + (1 - v) * (H - Tp - B);
  el('rect', { x: L, y: y(0.6), width: W - L - R, height: y(0.4) - y(0.6), fill: 'var(--foreground)', 'fill-opacity': 0.07 }, svg);
  el('line', { x1: L, x2: W - R, y1: y(0.5), y2: y(0.5), stroke: 'var(--border)' }, svg);
  let top = '', bot = '';
  const idx = D.low.map((v, i) => v == null ? -1 : i).filter(i => i >= 0);
  idx.forEach((i, n) => { top += (n ? 'L' : 'M') + x(CURVE[i]).toFixed(1) + ' ' + y(D.high[i]).toFixed(1); });
  idx.slice().reverse().forEach(i => { bot += 'L' + x(CURVE[i]).toFixed(1) + ' ' + y(D.low[i]).toFixed(1); });
  el('path', { d: top + bot + 'Z', fill: 'var(--chart-1)', 'fill-opacity': 0.35, stroke: 'var(--chart-1)', 'stroke-width': 1 }, svg);
  const inside = idx.filter(i => D.low[i] >= 0.4 && D.high[i] <= 0.6).length;
  const lo = Math.min(...idx.map(i => D.low[i]));
  document.getElementById('bs-' + k).textContent = `not lopsided at ${Math.round(100 * inside / idx.length)}% of XP levels · worst ${pct(lo)}–${pct(1 - lo)}%`;
}
render();
window.addEventListener('resize', () => render());
</script></body></html>'''
def compact(data):
    step = 2  # every 20 XP
    out = {'levels': data['levels'], 'curve': data['curve'][::step], 'pairs': data['pairs'], 'schemes': {}}
    pm = lambda v: -1 if v is None else int(round(v * 1000))
    for k, sc in data['schemes'].items():
        o = {'label': sc['label'], 'defense': sc['defense'], 'data': {}}
        for pair, d in sc['data'].items():
            t = {}
            for X, tab in d['tables'].items():
                t[X] = {'b': [[b['damage'], b['cards'], round(b['oxp']), round(b['dxp'])] for b in tab['builds']],
                        'm': [[None if c is None else [pm(c['w']), pm(c['d']), pm(c['l']), c['rounds']] for c in row] for row in tab['matrix']]}
            sh = None
            if d.get('shares'):
                sh = {'f': {o_: round(v, 4) for o_, v in d['shares']['f'].items()}, 'g': {o_: round(v, 4) for o_, v in d['shares']['g'].items()}}
            o['data'][pair] = {'t': t, 'c': [[pm(v) for v in c[::step]] for c in d['curve']], 'lo': [pm(v) for v in d['low'][::step]], 'hi': [pm(v) for v in d['high'][::step]], 'sh': sh}
        out['schemes'][k] = o
    return out
only = os.environ.get('ONLY_PAIR')
cd = compact(data)
if only:
    cd['pairs'] = [only]
    for k in cd['schemes']:
        cd['schemes'][k]['data'] = {only: cd['schemes'][k]['data'][only]}
html = HTML.replace('__DATA__', json.dumps(cd, separators=(',', ':'))).replace('__CHECKS__', json.dumps([[c['scheme'], c['X'], c['i'], c['j'], int(round(c['model'] * 1000)), int(round(c['score'] * 1000)), c['w'], c['d'], c['l']] for c in checks], separators=(',', ':')))
path = os.environ.get('PAGE_OUT') or os.path.join(OUT, 'pricing-page.html')
open(path, 'w').write(html)
print(path, len(html), 'bytes;', len(checks), 'checks')
