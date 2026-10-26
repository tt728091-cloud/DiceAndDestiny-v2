"""Final pricing page: the balancing points (page_data2.json), the search grid (robust_scan.log) and real battles."""
import json, os, re
OUT = os.environ.get('EXCHANGE_DIR', os.path.join(os.path.dirname(os.path.abspath(__file__)), '../../runs/exchange'))
src = open(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'make_page.py')).read()
HTML = src[src.index("HTML = r'''") + len("HTML = r'''"):src.index("</html>'''") + len('</html>')]
exec(src[src.index('def compact(data):'):src.index("only = os.environ")])  # compact()
data = json.load(open(os.path.join(OUT, 'page_data3.json')))
cd = compact(data)
for k, s in data['schemes'].items():
    for f in ('lo', 'hi', 'share', 'kind', 'keep'):
        cd['schemes'][k][f] = s[f]
checks = []
cp = os.path.join(OUT, 'rolled-checks.json')
if os.path.exists(cp):
    for p in json.load(open(cp)):
        s, X, i, j = p['name'].split('|')
        checks.append([s, int(X), int(i), int(j), int(round(p['model'] * 1000)), int(round(p['score'] * 1000)), p['wins'], p['draws'], p['losses']])
grid = []
NAMES_G = {'block1': 'Block: 1 die, a 6 (17%)', 'block2': 'Block: 2 dice, any 6 (31%)', 'block1b': 'Block: 1 die, 5–6 (33%)', 'block3': 'Block: 3 dice, any 6 (42%)',
           'half': 'Small: 1 on a 4–6 (0.5)', 'light': 'Small: Light Guard (1.0)', 'light2': 'Small: 2 Light Guard dice (2.0)', 'holdfast': 'Small: Holdfast (3.5)'}
for f in ('rolled.json', 'rolled_small.json'):
    for r in json.load(open(os.path.join(OUT, f))):
        grid.append([NAMES_G[r['defense']], round(r['high'], 2), int(round(100 * r['inside'])), int(round(100 * (0.5 - r['worst']))), int(round(100 * (0.5 - r['at3']))), int(round(100 * (1 - r['keep_def'])))])
H = HTML
H = H.replace("const RAW = __DATA__, RAWC = __CHECKS__;", "const RAW = __DATA__, RAWC = __CHECKS__, GRID = __GRID__;")
H = H.replace("""function names(scheme, pair) {
  const [lo, hi] = pair.split('-');
  const d = DEFNAME[DATA.schemes[scheme].defense];""", """function names(scheme, pair) {
  const sc = RAW.schemes[scheme], lo = sc.lo.toFixed(1), hi = sc.hi.toFixed(1);
  const d = DEFNAME[sc.defense] || sc.defense;""")
H = H.replace("const DEFNAME = { holdfast: 'Holdfast', light: 'Light Guard', ward37: '37% defense', none: 'no defense' };",
              "const DEFNAME = { block2: 'block (2 dice)', block1b: 'block (5–6)', block3: 'block (3 dice)', holdfast: 'Holdfast', none: 'no defense' };")
H = H.replace("let S = { scheme: 'pct', pair: '7-12', level: 400 };", "let S = { scheme: 'block2', pair: 'main', level: 400 };")
H = H.replace("""    seg('Damage', DATA.pairs.map(p => [p, p.replace('-', ' vs ') + ' dmg']), 'pair') +
""", "")
H = H.replace("h += `<p class=\"sub\">${SCHEME_TEXT[S.scheme]}</p>`;", "h += `<p class=\"sub\">${schemeText(S.scheme)}</p>`;")
H = H.replace("const pct = v => (v * 100).toFixed(0);", """const pct = v => (v * 100).toFixed(0);
function schemeText(k) {
  const s = RAW.schemes[k], kp = s.keep, lo = s.lo.toFixed(2), hi = s.hi.toFixed(2);
  const DT = { block2: 'Defense: roll 2 dice; any 6 prevents all damage from the attack (blocks 31% of attacks)',
    block1b: 'Defense: roll 1 die; a 5 or 6 prevents all damage from the attack (blocks 33%)',
    block3: 'Defense: roll 3 dice; any 6 prevents all damage from the attack (blocks 42%)',
    holdfast: 'Defense: Holdfast as it is (3 dice, prevents 0–6, 3.5 on average), at the price that fits best. Even at its best price it cannot keep these matchups inside 40–60' };
  return `Low damage ${lo}, high ${hi}. Prices are percentages of total XP: the board costs (1 − 6.2 ÷ damage): ${(100 * (1 - kp.low)).toFixed(1)}% at ${lo}, ${(100 * (1 - kp.high)).toFixed(1)}% at ${hi}. ${DT[k]}; it costs ${(100 * (1 - kp.defense)).toFixed(1)}% of the XP left after the board. Every remaining 10 XP is a blank card (rounded to the nearest card). Playable in the engine today.`;
}""")
H = H.replace("Hover for the rounds each side needs${S.pair === '7-12' && S.scheme !== 'ward' && ASKED.includes(S.level) ? ' and the real-battle result' : ''}", "Hover for the rounds each side needs${(S.scheme === 'block2' || S.scheme === 'holdfast') && ASKED.includes(S.level) ? ' and the real-battle result' : ''}")
H = H.replace("const ch = CHECKS.find(x => x.scheme === S.scheme && x.X === S.level && S.pair === '7-12' &&", "const ch = CHECKS.find(x => x.scheme === S.scheme && x.X === S.level &&")
H = H.replace("for every pricing scheme (${S.pair.replace('-', ' vs ')} damage)", "for every configuration")
H = H.replace("""  if (!CHECKS.length) h += '<p class="sub">Real-battle checks are still running.</p>';
  else if (S.scheme === 'ward') h += '<p class="sub">No real battles for the 37% defense: the engine cannot prevent a percentage of a hit yet.</p>';
  else {""", """  if (!CHECKS.length) h += '<p class="sub">Real-battle checks are still running.</p>';
  else if (!ck.length) h += '<p class="sub">Not run for this defense; select the recommended block defense or Holdfast for real battles. The race model has matched real battles at every checked matchup so far (average gap under 1 point).</p>';
  else {""")
H = H.replace("Every pairing of the four builds at 7 vs 12 damage, at 400, 650 and 900 XP, for the three schemes the engine can play:", "Every pairing of the four builds at 400, 650 and 900 XP:")
H = H.replace("const NN = names(S.scheme, '7-12');", "const NN = names(S.scheme, 'main');")
H = H.replace("${NN[c.i]} vs ${NN[c.j]}", "${N[c.i]} vs ${N[c.j]}")
# search grid section, appended after the band panels
H = H.replace("""  const ck = CHECKS.filter(c => c.scheme === S.scheme);""", """  h += `<h2>Every rolled defense searched</h2><p class="sub">Low damage 7 against each high damage, every defense a roll with flat amounts on its faces. Block defenses prevent the whole attack on some faces; small ones prevent a few points on every face, like Holdfast. Each defense at its best-fitting price (a percentage of the XP left after the board). Each cell: the share of XP levels from 400 to 1,500 (every 10 XP) where all six matchups land inside 40–60; underneath, the worst matchup at any level. Hover for 400 / 650 / 900 and the price.</p>`;
  const highs = [...new Set(GRID.map(g => g[1]))].sort((a, b) => a - b), rowsG = [...new Set(GRID.map(g => g[0]))];
  h += '<div class="chart" id="gr"><table class="matrix"><tr><th class="row">defense ↓ · high damage →</th>' + highs.map(x => `<th>${x.toFixed(2)}</th>`).join('') + '</tr>';
  for (const r of rowsG) {
    h += `<tr><th class="row">${r}</th>`;
    for (const x of highs) {
      const g = GRID.find(q => q[0] === r && q[1] === x);
      if (!g) { h += '<td class="cell self note">—</td>'; continue; }
      const t = Math.max(-1, Math.min(1, (g[2] - 70) / 30));
      h += `<td class="cell gcell" title="400 / 650 / 900 XP: ${g[4]}–${100 - g[4]}% · defense costs ${g[5]}% of the XP left" style="background:${cellColor(0.5 + 0.3 * t)}"><div class="v">${g[2]}%</div><div class="w">worst ${g[3]}–${100 - g[3]}</div></td>`;
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
path = os.environ.get('PAGE_OUT') or os.path.join(OUT, 'rolled-page.html')
open(path, 'w').write(html)
print(path, len(html), 'bytes;', len(checks), 'checks;', len(grid), 'grid cells')
