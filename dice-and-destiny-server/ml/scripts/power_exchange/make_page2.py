"""Final pricing page: the balancing points (page_data2.json), the search grid (robust_scan.log) and real battles."""
import json, os, re
OUT = os.environ.get('EXCHANGE_DIR', os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../runs/exchange'))
src = open(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'make_page.py')).read()
HTML = src[src.index("HTML = r'''") + len("HTML = r'''"):src.index("</html>'''") + len('</html>')]
exec(src[src.index('def compact(data):'):src.index("only = os.environ")])  # compact()
data = json.load(open(os.path.join(OUT, 'page_data2.json')))
cd = compact(data)
for k, s in data['schemes'].items():
    for f in ('lo', 'hi', 'share', 'kind', 'keep'):
        cd['schemes'][k][f] = s[f]
checks = []
cp = os.path.join(OUT, 'final-checks.json')
if os.path.exists(cp):
    for p in json.load(open(cp)):
        s, X, i, j = p['name'].split('|')
        checks.append([s, int(X), int(i), int(j), int(round(p['model'] * 1000)), int(round(p['score'] * 1000)), p['wins'], p['draws'], p['losses']])
grid = []
for ln in open(os.path.join(OUT, 'robust_scan.log')):
    m = re.match(r'nearest high\s+([\d.]+)\s+(\d+)%: inside 40-60 at\s+(\d+)% .* within (\d+)-(\d+), worst (\d+) \| at 400/650/900 (\d+)-(\d+)', ln)
    if m: grid.append([float(m.group(1)), int(m.group(2)), int(m.group(3)), int(m.group(6)), int(m.group(7))])
H = HTML
H = H.replace("const RAW = __DATA__, RAWC = __CHECKS__;", "const RAW = __DATA__, RAWC = __CHECKS__, GRID = __GRID__;")
H = H.replace("""function names(scheme, pair) {
  const [lo, hi] = pair.split('-');
  const d = DEFNAME[DATA.schemes[scheme].defense];""", """function names(scheme, pair) {
  const sc = RAW.schemes[scheme], lo = sc.lo.toFixed(1), hi = sc.hi.toFixed(1);
  const d = DEFNAME[sc.defense] || sc.defense;""")
H = H.replace("const DEFNAME = { holdfast: 'Holdfast', light: 'Light Guard', ward37: '37% defense', none: 'no defense' };",
              "const DEFNAME = { half: '0.5 flat defense', ward15: '15% defense', ward20: '20% defense', ward37: '37% defense', none: 'no defense' };")
H = H.replace("let S = { scheme: 'pct', pair: '7-12', level: 400 };", "let S = { scheme: 'rec', pair: 'main', level: 400 };")
H = H.replace("""    seg('Damage', DATA.pairs.map(p => [p, p.replace('-', ' vs ') + ' dmg']), 'pair') +
""", "")
H = H.replace("h += `<p class=\"sub\">${SCHEME_TEXT[S.scheme]}</p>`;", "h += `<p class=\"sub\">${schemeText(S.scheme)}</p>`;")
H = H.replace("const pct = v => (v * 100).toFixed(0);", """const pct = v => (v * 100).toFixed(0);
function schemeText(k) {
  const s = RAW.schemes[k], kp = s.keep, lo = s.lo.toFixed(2), hi = s.hi.toFixed(2);
  const dtext = s.kind === 'pct' ? `the defense prevents ${Math.round(100 * s.share)}% of every hit and costs ${Math.round(100 * s.share)}% of the XP left after the board`
    : `the defense prevents 1 on a 4–6 (0.5 on average) and costs ${Math.round(100 * (1 - kp.defense))}% of the XP left after the board`;
  return `Low damage ${lo}, high ${hi}. The board costs (1 − 6.2 ÷ damage) of your total XP: ${(100 * (1 - kp.low)).toFixed(1)}% at ${lo}, ${(100 * (1 - kp.high)).toFixed(1)}% at ${hi}; ${dtext}; every remaining 10 XP is a blank card (rounded to the nearest card).` +
    (s.kind === 'pct' ? ' Model only: the engine cannot prevent a share of a hit yet.' : ' Playable in the engine today; checked in real battles below.');
}""")
H = H.replace("Hover for the rounds each side needs${S.pair === '7-12' && S.scheme !== 'ward' && ASKED.includes(S.level) ? ' and the real-battle result' : ''}", "Hover for the rounds each side needs${S.scheme === 'today' && ASKED.includes(S.level) ? ' and the real-battle result' : ''}")
H = H.replace("const ch = CHECKS.find(x => x.scheme === S.scheme && x.X === S.level && S.pair === '7-12' &&", "const ch = CHECKS.find(x => x.scheme === S.scheme && x.X === S.level &&")
H = H.replace("for every pricing scheme (${S.pair.replace('-', ' vs ')} damage)", "for every configuration")
H = H.replace("""  if (!CHECKS.length) h += '<p class="sub">Real-battle checks are still running.</p>';
  else if (S.scheme === 'ward') h += '<p class="sub">No real battles for the 37% defense: the engine cannot prevent a percentage of a hit yet.</p>';
  else {""", """  if (!CHECKS.length) h += '<p class="sub">Real-battle checks are still running.</p>';
  else if (S.scheme !== 'today') h += '<p class="sub">No real battles for percentage defenses: the engine cannot prevent a share of a hit yet. The race model has matched real battles at every one of 118 checked matchups so far (average gap under 1 point). Select "Today\\'s engine" for its real battles.</p>';
  else {""")
H = H.replace("Every pairing of the four builds at 7 vs 12 damage, at 400, 650 and 900 XP, for the three schemes the engine can play:", "Every pairing of the four builds at 400, 650 and 900 XP:")
H = H.replace("const NN = names(S.scheme, '7-12');", "const NN = names(S.scheme, 'main');")
H = H.replace("${NN[c.i]} vs ${NN[c.j]}", "${N[c.i]} vs ${N[c.j]}")
# search grid section, appended after the band panels
H = H.replace("""  const ck = CHECKS.filter(c => c.scheme === S.scheme);""", """  h += `<h2>Every balancing point searched</h2><p class="sub">Low damage 7 against every high damage, with a defense preventing a share of each hit, priced by the formula above. Each cell: the share of XP levels from 400 to 1,500 (every 10 XP) where all six matchups land inside 40–60; underneath, the worst matchup at any level. Hover for the result at 400 / 650 / 900.</p>`;
  const highs = [...new Set(GRID.map(g => g[0]))].sort((a, b) => a - b), shares = [...new Set(GRID.map(g => g[1]))].sort((a, b) => a - b);
  h += '<div class="chart" id="gr"><table class="matrix"><tr><th class="row">high damage →<br>defense share ↓</th>' + highs.map(x => `<th>${x.toFixed(2)}</th>`).join('') + '</tr>';
  for (const s of shares) {
    h += `<tr><th class="row">${s}%</th>`;
    for (const x of highs) {
      const g = GRID.find(r => r[0] === x && r[1] === s);
      const t = Math.max(0, Math.min(1, (g[2] - 80) / 20));
      h += `<td class="cell gcell" title="400 / 650 / 900 XP: ${g[4]}–${100 - g[4]}%" style="background:color-mix(in oklab, var(--win) ${Math.round(t * 70)}%, var(--secondary))"><div class="v">${g[2]}%</div><div class="w">worst ${g[3]}–${100 - g[3]}</div></td>`;
    }
    h += '</tr>';
  }
  h += '</table></div>';
  const ck = CHECKS.filter(c => c.scheme === S.scheme);""")
H = H.replace("const x = v => L + (v - 300) / 1200 * (W - L - R), y = v => Tp + (1 - v) * (H - Tp - B);\n  el('rect', { x: L, y: y(0.55)",
              "const x = v => L + (v - 300) / 1200 * (W - L - R), y = v => Tp + (0.7 - Math.max(0.3, Math.min(0.7, v))) / 0.4 * (H - Tp - B);\n  el('rect', { x: L, y: y(0.55)")
H = H.replace("for (let v = 0; v <= 1.0001; v += 0.2) {", "for (let v = 0.3; v <= 0.7001; v += 0.1) {")
H = H.replace("Shaded: 45–55%. Dashed: the levels you asked for (400, 650, 900).", "Scale 30–70%. Shaded: 45–55%. Dashed: the levels you asked for (400, 650, 900).")
html = H.replace('__DATA__', json.dumps(cd, separators=(',', ':'))).replace('__CHECKS__', json.dumps(checks, separators=(',', ':'))).replace('__GRID__', json.dumps(grid, separators=(',', ':')))
path = os.environ.get('PAGE_OUT') or os.path.join(OUT, 'final-page.html')
open(path, 'w').write(html)
print(path, len(html), 'bytes;', len(checks), 'checks;', len(grid), 'grid cells')
