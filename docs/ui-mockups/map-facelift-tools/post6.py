"""Overlays, composites and captions for set 6 (mountain occlusion fixes).
usage: python3 post6.py <out_dir> <dest_dir> [ids...]
"""
import json, os, subprocess, sys

SRC, DEST = sys.argv[1], sys.argv[2]
TMP = os.path.join(SRC, 'post_tmp6')
os.makedirs(TMP, exist_ok=True)
os.makedirs(DEST, exist_ok=True)
F = '/System/Library/Fonts/Supplemental/'
SERIF, BOLD, ITALIC = F + 'Georgia.ttf', F + 'Georgia Bold.ttf', F + 'Georgia Italic.ttf'
W, H = 1920, 1080
SHAPES = {'motif_plains': ['M6 22q4-8 8 0M18 20q4-8 8 0M12 27q3-5 6 0'], 'motif_hills': ['M3 24q13-18 26 0M9 26q7-9 14 0'], 'motif_forest': ['M16 3L6 21H26Z', 'M16 21V29'], 'motif_marsh': ['M10 29V12M16 29V6M22 29V13M10 12q-4-4-6-1M22 13q4-4 6-1'], 'motif_lake': ['M3 14q6-6 13 0t13 0M3 22q6-6 13 0t13 0'], 'motif_mountain': ['M3 27L13 8l5 7 3-4 8 16Z', 'M11 14l2 3 3-4'], 'site_village': ['M5 15L16 5l11 10M8 13v15h16V13M21 7v5', 'M14 28v-8h4v8'], 'site_keep': ['M5 29V12H9V8H13V12H19V8H23V12H27V29ZM13 29V21H19V29'], 'site_ruins': ['M8 29V12l4-2M20 29V9l4 2M4 29h24', 'M12 29v-5h5v5'], 'pawn': ['M16 2.5a4.5 4.5 0 1 1 0 9a4.5 4.5 0 0 1 0-9ZM12 12h8l3 9H9ZM6 29q10-6 20 0Z']}
WASH = {'plains': '#b5c27a', 'mountain': '#a39aa6', 'hills': '#c49a5e', 'lake': '#6d93b4', 'marsh': '#7d8f7a', 'forest': '#4f6e45'}

def run(args):
    r = subprocess.run(['magick'] + [str(a) for a in args], capture_output=True, text=True)
    if r.returncode or 'error' in r.stderr.lower():
        print('MAGICK', r.returncode, r.stderr.strip()[:400])


def size(path):
    out = subprocess.run(['magick', 'identify', '-format', '%w %h', path], capture_output=True, text=True, check=True).stdout
    w, h = out.split()
    return int(w), int(h)


_n = [0]


def tmp(name):
    _n[0] += 1
    return os.path.join(TMP, f'{_n[0]}_{name}.png')


def plate(lines, bg='#100e13d0', edge='#c9a35a', pad=(22, 14)):
    parts = []
    for (t, f, s, c) in lines:
        p = tmp('t')
        run(['-background', 'none', '-fill', c, '-font', f, '-pointsize', s, f'label:{t}', p])
        parts.append(p)
    stack = tmp('stack')
    run(['-background', 'none'] + parts + ['-gravity', 'West', '-append', stack])
    w, h = size(stack)
    W2, H2 = w + 2 * pad[0], h + 2 * pad[1]
    out = tmp('plate')
    run(['-size', f'{W2}x{H2}', 'xc:none', '-fill', bg, '-stroke', edge, '-strokewidth', '1.5',
         '-draw', f'roundrectangle 1,1 {W2 - 2},{H2 - 2} 7,7', stack, '-gravity', 'NorthWest', '-geometry', f'+{pad[0]}+{pad[1]}', '-composite', out])
    return out, (W2, H2)


def caption(img, num, title, sub, top=False):
    p, (w, h) = plate([(f'{num} · {title}', BOLD, 28, '#efe3c6'), (sub, ITALIC, 17, '#d9ccb0')])
    x, y = (84, H - h - 84) if top else (28, H - h - 26)
    run([img, p, '-geometry', f'+{x}+{y}', '-composite', img])


def vig(img, amount=0.4):
    v = os.path.join(TMP, f'vig{int(amount * 100)}.png')
    if not os.path.exists(v):
        run(['-size', f'{W}x{H}', 'radial-gradient:white-black', '-level', '0%,135%', '-evaluate', 'pow', '1.6',
             '-fill', 'white', '-colorize', f'{int((1 - amount) * 100)}%', v])
    run([img, v, '-compose', 'multiply', '-composite', img])


def grain(img, amp='46%,54%'):
    n = tmp('noise')
    run(['-size', f'{W}x{H}', 'xc:gray50', '-seed', '3', '+noise', 'Gaussian', '-colorspace', 'gray', '-blur', '0x0.6', '+level', amp, n])
    run([img, n, '-compose', 'soft-light', '-composite', img])


def bloom(img, thresh='55%', blur=14, strength=55):
    b = tmp('bloom')
    run([img, '-level', f'{thresh},100%', '-blur', f'0x{blur}', b])
    run([img, '(', b, '-evaluate', 'multiply', f'{strength / 100.0}', ')', '-compose', 'screen', '-composite', img])


def rain(img):
    r = tmp('rain')
    run(['-size', f'{W}x{H}', 'xc:black', '-seed', '11', '+noise', 'Random', '-channel', 'G', '-separate', '+channel',
         '-threshold', '99.7%', '-motion-blur', '0x26+102', '-level', '0,22%', '-blur', '0x0.4', r])
    run([img, r, '-compose', 'screen', '-composite', img])
    run([img, '-fill', '#7f8ea0', '-colorize', '6%', img])


def woodcut(img):
    gray = tmp('gray')
    run([img, '-colorspace', 'gray', '-level', '12%,88%', '-posterize', '4', gray])
    mask = tmp('mask')
    run([img, '-colorspace', 'HSL', '-channel', 'G', '-separate', '+channel', '-threshold', '32%', '-morphology', 'Dilate', 'Disk:1', mask])
    out = tmp('wc')
    run([gray, img, mask, '-composite', out])
    paper = tmp('paper')
    run(['-size', f'{W}x{H}', 'xc:#efe6d0', paper])
    run([out, paper, '-compose', 'multiply', '-composite', img])
    grain(img, '45%,55%')


def manuscript_border(img):
    run([img, '-fill', 'none',
         '-stroke', '#a8281a', '-strokewidth', '22', '-draw', f'rectangle 18,18 {W - 19},{H - 19}',
         '-stroke', '#d8a848', '-strokewidth', '6', '-draw', f'rectangle 36,36 {W - 37},{H - 37}',
         '-stroke', '#1e3aa0', '-strokewidth', '10', '-draw', f'rectangle 46,46 {W - 47},{H - 47}',
         '-stroke', '#d8a848', '-strokewidth', '3', '-draw', f'rectangle 56,56 {W - 57},{H - 57}',
         '-stroke', 'none', '-fill', '#d8a848',
         '-draw', f'rectangle 10,10 70,70', '-draw', f'rectangle {W - 71},10 {W - 11},70',
         '-draw', f'rectangle 10,{H - 71} 70,{H - 11}', '-draw', f'rectangle {W - 71},{H - 71} {W - 11},{H - 11}',
         '-fill', '#a8281a', '-draw', 'circle 40,40 40,26', '-draw', f'circle {W - 41},40 {W - 41},26',
         '-draw', f'circle 40,{H - 41} 40,{H - 55}', '-draw', f'circle {W - 41},{H - 41} {W - 41},{H - 55}', img])
    grain(img, '47%,53%')




def streaks(img, color, density='99.7%', angle=102, length=26, level='0,22%', seed=11, colorize=None):
    r = tmp('streak')
    run(['-size', f'{W}x{H}', 'xc:black', '-seed', str(seed), '+noise', 'Random', '-channel', 'G', '-separate', '+channel',
         '-threshold', density, '-motion-blur', f'0x{length}+{angle}', '-level', level, '-blur', '0x0.5', r])
    tint = tmp('tint')
    run([r, '(', '-size', f'{W}x{H}', f'xc:{color}', ')', '-compose', 'multiply', '-composite', tint])
    run([img, tint, '-compose', 'screen', '-composite', img])
    if colorize:
        run([img, '-fill', colorize[0], '-colorize', colorize[1], img])


def dust(img):
    streaks(img, '#e0b080', '99.2%', 168, 40, '0,30%', 5, ('#a07850', '10%'))
    streaks(img, '#f0c890', '99.75%', 172, 60, '0,40%', 9)


def blizzard(img):
    streaks(img, '#ffffff', '99.5%', 120, 10, '0,45%', 13, ('#c8d4e0', '8%'))
    streaks(img, '#ffffff', '99.85%', 125, 18, '0,60%', 17)


def ashfall(img):
    r = tmp('ash')
    run(['-size', f'{W}x{H}', 'xc:white', '-seed', '21', '+noise', 'Random', '-channel', 'G', '-separate', '+channel',
         '-threshold', '0.4%', '-motion-blur', '0x6+100', r])
    run([img, r, '-compose', 'multiply', '-composite', img])
    streaks(img, '#ff7a2a', '99.93%', 80, 6, '0,70%', 23)




CONCEPTS = [
    ('01', 'the-problem', 'The problem', 'At the cinematic camera the range hides the pass, your route and the valley behind it.'),
    ('02', 'zoom-linked-pitch', 'Pitch follows zoom', 'Zoomed out, the camera tilts up to about 55 degrees: the pass, route and hidden valley all read.'),
    ('03', 'fade-what-is-in-the-way', 'Fade what is in the way', 'Peaks between the camera and your route, cursor or known enemies turn into see-through ghosts.'),
    ('04', 'sinking-peaks', 'Sinking peaks', 'Peaks over your route sink into their tiles like a pop-up book, then rise again when you look away.'),
    ('05', 'outlines-through-terrain', 'Outlines through terrain', 'Pieces behind a ridge are drawn as glowing silhouettes, so a hidden enemy is never lost.'),
    ('06', 'banners-above-the-peaks', 'Banners above the peaks', 'Every piece and site flies a banner taller than the highest mountain.'),
    ('07', 'see-through-cursor', 'See-through cursor and ground picking', 'The cursor picks the hex on the ground grid; a circle cuts away rock in front of it.'),
    ('08', 'tactical-view', 'Tactical view', 'Hold a key: scenery flattens and the board becomes your current chart, read top-down.'),
    ('09', 'authored-saddle-pass', 'Design: a saddle pass', 'Peaks drop toward the pass and lean to the near edge of their hex; walkable mountains stay low.'),
    ('10', 'rotate-60-degrees', 'Rotate in 60-degree steps', 'Turning the camera one hex-step lets you look straight down the pass into the valley.'),
    ('11', 'automatic-visibility-check', 'Automatic visibility check', 'A content test casts rays from the default camera and flags every walkable hex it cannot see.'),
    ('12', 'all-together', 'All together', 'Mid pitch, faded peaks on the route, enemy outlines, banners and a hover tooltip.'),
]


def mvg(path, lines):
    with open(path, 'w') as f:
        f.write('\n'.join(lines))
    return path


def chip(img, text, x=None, y=24, color='#efe3c6', sub=None):
    lines = [(text, BOLD, 22, color)] + ([(sub, ITALIC, 16, '#d9ccb0')] if sub else [])
    p, (w, h) = plate(lines, pad=(18, 10))
    x = W - w - 28 if x is None else x
    run([img, p, '-geometry', f'+{int(x)}+{int(y)}', '-composite', img])
    return w, h


def label_at(img, text, x, y, color='#efe3c6', bg='#100e13d8', edge='#c9a35a', size=19):
    p, (w, h) = plate([(text, BOLD, size, color)], bg=bg, edge=edge, pad=(10, 5))
    run([img, p, '-geometry', f'+{int(min(max(x - w / 2, 8), W - w - 8))}+{int(max(y - h, 8))}', '-composite', img])


def outlines(img, prefix, color='#ff5a44', fill_alpha=0.5):
    full, vis = prefix + '_foe_full.png', prefix + '_foe_vis.png'
    if not os.path.exists(full):
        print('no masks', prefix)
        return
    fm, vm, om, em = tmp('fm'), tmp('vm'), tmp('om'), tmp('em')
    run([full, '-alpha', 'extract', '-threshold', '40%', fm])
    run([vis, '-alpha', 'extract', '-threshold', '40%', vm])
    run([fm, '(', vm, '-negate', ')', '-compose', 'Darken', '-composite', om])
    run([fm, '-morphology', 'EdgeOut', 'Disk:2.5', em])
    glow = tmp('glow')
    run([em, '-blur', '0x5', '-level', '0,60%', glow])
    for mask, a in ((om, fill_alpha), (glow, 0.8), (em, 1.0)):
        layer = tmp('layer')
        run(['-size', f'{W}x{H}', f'xc:{color}', '(', mask, '-evaluate', 'multiply', str(a), ')', '-alpha', 'off', '-compose', 'CopyOpacity', '-composite', layer])
        run([img, layer, '-compose', 'over', '-composite', img])


def poly(corners):
    return ' '.join(f'{x:.1f},{y:.1f}' for x, y in corners)


def hover(img, data, tip=True):
    if 'hover_corners' in data:
        p = mvg(tmp('hv') + '.mvg', ["fill none", "stroke '#fff1c9'", "stroke-width 4", f"polygon {poly(data['hover_corners'])}",
                                       "stroke '#c9772b'", "stroke-width 1.5", f"polygon {poly(data['hover_corners'])}"])
        run([img, '-draw', f'@{p}', img])
    cx, cy = data['cursor']
    arrow = [(0, 0), (0, 34), (9, 26), (16, 41), (22, 38), (15, 24), (27, 24)]
    pts = ' '.join(f'{cx + 10 + x:.0f},{cy + 6 + y:.0f}' for x, y in arrow)
    run([img, '-fill', '#fff8e8', '-stroke', '#211e19', '-strokewidth', '2.5', '-draw', f'polygon {pts}', img])
    if tip:
        p, (w, h) = plate([('Highpass Saddle', BOLD, 22, '#efe3c6'), ('Hills · Move 3 · behind the range', ITALIC, 17, '#e6c17c')], pad=(14, 9))
        run([img, p, '-geometry', f'+{int(cx + 48)}+{int(cy + 30)}', '-composite', img])


def zoom_slider(img, level):
    x0, y0, hgt = W - 130, 120, 300
    lines = ["fill '#100e13d0'", "stroke '#c9a35a'", "stroke-width 1.5", f"roundrectangle {x0 - 22},{y0 - 46} {x0 + 104},{y0 + hgt + 30} 8,8",
             "stroke '#8a7146'", "stroke-width 4", f"line {x0},{y0} {x0},{y0 + hgt}"]
    ticks = [(0.0, 'Chart 80°'), (0.45, 'Play 55°'), (1.0, 'Close 17°')]
    for t, _ in ticks:
        yy = y0 + t * hgt
        lines += ["stroke '#c9a35a'", "stroke-width 2", f"line {x0 - 8},{yy} {x0 + 8},{yy}"]
    ym = y0 + level * hgt
    lines += ["stroke none", "fill '#ffd27a'", f"circle {x0},{ym} {x0 + 11},{ym}"]
    run([img, '-draw', f'@{mvg(tmp("zs") + ".mvg", lines)}', img])
    for t, txt in ticks:
        run([img, '-fill', '#efe3c6', '-font', SERIF, '-pointsize', '16', '-annotate', f'+{x0 + 16}+{int(y0 + t * hgt + 6)}', txt, img])
    run([img, '-fill', '#e6c17c', '-font', BOLD, '-pointsize', '18', '-annotate', f'+{x0 - 12}+{y0 - 20}', 'Zoom', img])


def glyph_lines(name, cx, cy, size, color='#211e19', width=2.2):
    s = size / 32.0
    out = ["push graphic-context", f"translate {cx - 16 * s:.1f},{cy - 16 * s:.1f}", f"scale {s:.3f},{s:.3f}",
           f"stroke '{color}'", f"stroke-width {width}", "fill none", "stroke-linecap round", "stroke-linejoin round"]
    for d in SHAPES[name]:
        out.append(f"path '{d}'")
    out.append("pop graphic-context")
    return out


def chart(img, data):
    lines = []
    motif = {'plains': 'motif_plains', 'hills': 'motif_hills', 'forest': 'motif_forest', 'marsh': 'motif_marsh', 'lake': 'motif_lake',
             'mountain': 'motif_mountain', 'peaks': 'motif_mountain'}
    for hx in data['hexes']:
        t = hx['terrain']
        col = WASH.get('mountain' if t == 'peaks' else t, '#a39a8a')
        op = 0.82 if t == 'peaks' else 0.62
        lines += [f"fill '{col}'", f"fill-opacity {op}", "stroke '#211e19'", "stroke-opacity 0.55", "stroke-width 1.4", f"polygon {poly(hx['corners'])}"]
    lines += ["fill-opacity 1", "stroke-opacity 1"]
    for hx in data['hexes']:
        cs = hx['corners']
        size = 0.42 * max(abs(cs[0][0] - cs[3][0]), abs(cs[1][1] - cs[4][1]))
        name = {'village': 'site_village', 'keep': 'site_keep', 'shrine': 'site_ruins'}.get(hx['site']) or motif.get(hx['terrain'])
        if name:
            lines += glyph_lines(name, hx['c'][0], hx['c'][1], size * (1.25 if hx['site'] else 1.0), '#211e19' if not hx['site'] else '#8c2f2a', 2.4)
    pts = data.get('path', [])
    if pts:
        lines += ["push graphic-context", "fill none", "stroke '#8c2f2a'", "stroke-width 4", "stroke-dasharray 14 9", "stroke-linecap round",
                  "polyline " + ' '.join(f'{x:.1f},{y:.1f}' for x, y in pts), "pop graphic-context"]
    for key, (x, y) in data.get('pieces', {}).items():
        r = 30 if key == 'warlord' else 24
        lines += ["stroke none", "fill '#6b2622'", f"circle {x:.1f},{y:.1f} {x + r:.1f},{y:.1f}",
                  "fill none", "stroke '#a63a33'", "stroke-width 3", f"circle {x:.1f},{y:.1f} {x + r - 5:.1f},{y:.1f}"]
    if 'hero' in data:
        hx_, hy_ = data['hero']
        lines += ["stroke none", "fill '#efe3c6'", f"circle {hx_:.1f},{hy_:.1f} {hx_ + 28:.1f},{hy_:.1f}", "fill none", "stroke '#211e19'", "stroke-width 2.5",
                  f"circle {hx_:.1f},{hy_:.1f} {hx_ + 28:.1f},{hy_:.1f}"]
        lines += glyph_lines('pawn', hx_, hy_, 38, '#211e19', 2.6)
    run([img, '-draw', f'@{mvg(tmp("chart") + ".mvg", lines)}', img])
    paper = tmp('paper')
    run(['-size', f'{W}x{H}', 'xc:#e8dcc0', paper])
    run([img, paper, '-compose', 'blend', '-define', 'compose:args=18', '-composite', img])
    grain(img, '47%,53%')


def heat(img, data):
    lines = []
    hidden = partial = 0
    keyset = {tuple(h['key']): h for h in data['hexes']}
    for k, v in data['status'].items():
        q, r = map(int, k.split(','))
        hx = keyset.get((q, r))
        if not hx:
            continue
        if v >= 0.85:
            col, op = '#5fd06a', 0.22
        elif v >= 0.3:
            col, op = '#f0b13a', 0.45
            partial += 1
        else:
            col, op = '#ff4a3a', 0.55
            hidden += 1
        lines += [f"fill '{col}'", f"fill-opacity {op}", f"stroke '{col}'", "stroke-opacity 0.95", "stroke-width 2", f"polygon {poly(hx['corners'])}"]
    run([img, '-draw', f'@{mvg(tmp("heat") + ".mvg", lines)}', img])
    p, (w, h) = plate([('Visibility check · default camera', BOLD, 22, '#efe3c6'),
                       (f'{hidden} walkable hexes hidden · {partial} partly hidden', ITALIC, 17, '#ff8a70')], pad=(16, 10))
    run([img, p, '-geometry', f'+{W - w - 28}+24', '-composite', img])
    leg = mvg(tmp('leg') + '.mvg', ["fill '#100e13d0'", "stroke '#c9a35a'", "stroke-width 1.5", "roundrectangle 1,1 300,118 7,7",
                                     "stroke none", "fill '#5fd06a'", "rectangle 18,18 40,40", "fill '#f0b13a'", "rectangle 18,52 40,74", "fill '#ff4a3a'", "rectangle 18,86 40,108"])
    lg = tmp('legend')
    run(['-size', '302x120', 'xc:none', '-draw', f'@{leg}', '-fill', '#efe3c6', '-font', SERIF, '-pointsize', '18',
         '-annotate', '+52+36', 'Visible', '-annotate', '+52+70', 'Partly hidden', '-annotate', '+52+104', 'Hidden: fails the test', lg])
    run([img, lg, '-geometry', f'+{W - 330}+{H - 150}', '-composite', img])


def finish(num, img, data):
    pre = os.path.join(SRC, f'final_{num}')
    if num == '01':
        chip(img, 'Cinematic camera · pitch 17°', sub='The route disappears into the range')
    elif num == '02':
        zoom_slider(img, 0.45)
    elif num == '03':
        chip(img, f"{data.get('faded', 0)} peaks faded", sub='Only those blocking your route or a known enemy')
    elif num == '04':
        chip(img, f"{data.get('sunk', 0)} peaks lowered", sub='Animated back up when the route changes')
    elif num == '05':
        outlines(img, pre)
        chip(img, 'Hidden pieces drawn through rock', sub='Second render pass, depth test reversed')
    elif num == '06':
        placed = []
        for name, (x, y) in sorted(data.get('banners', {}).items(), key=lambda kv: kv[1][0]):
            w_est = 11 * len(name) + 24
            while any(abs(x - px) < (w_est + pw) / 2 + 6 and abs(y - py) < 34 for px, py, pw in placed):
                y -= 36
            placed.append((x, y, w_est))
            label_at(img, name, x, y - 4, edge='#8c2f2a' if name in ('Eel Hunters', 'The Drowned Regent') else '#c9a35a')
    elif num == '07':
        cx, cy = data['cursor']
        r = data['radius_px']
        run([img, '-fill', 'none', '-stroke', '#fff1c9', '-strokewidth', '2', '-draw', f'circle {cx},{cy} {cx + r},{cy}', img])
        hover(img, data)
    elif num == '08':
        chart(img, data)
        chip(img, 'Tactical view · hold Tab', sub='Same board, read as the chart you have today')
    elif num == '09':
        chip(img, 'Authored range', sub='Saddle at the pass · peaks on the near edge · walkable tier lower')
    elif num == '10':
        chip(img, 'View rotated 60° · Q / E', sub='Looking straight down the pass')
    elif num == '11':
        heat(img, data)
    elif num == '12':
        outlines(img, pre)
        for name, (x, y) in data.get('banners', {}).items():
            label_at(img, name, x, y - 4, edge='#8c2f2a')
        hover(img, data)
        chip(img, 'Recommended: mid pitch 38°', sub='Fade on route · outlines · banners · hover')
    vig(img, 0.18)


only = set(sys.argv[3:])
for num, slug, title, sub in CONCEPTS:
    if only and num not in only:
        continue
    src = os.path.join(SRC, f'final_{num}.png')
    if not os.path.exists(src):
        print('missing', src)
        continue
    data = json.load(open(src.replace('.png', '.json')))
    dst = os.path.join(DEST, f'{num}-{slug}.png')
    run([src, '-strip', '-alpha', 'off', dst])
    finish(num, dst, data)
    caption(dst, num, title, sub)
    print('wrote', dst)
