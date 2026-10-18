"""Post-process the final renders with ImageMagick: captions, labels, HUD, grading.
usage: python3 post.py <out_dir> <dest_dir>
"""
import json, os, subprocess, sys

SRC, DEST = sys.argv[1], sys.argv[2]
TMP = os.path.join(SRC, 'post_tmp')
os.makedirs(TMP, exist_ok=True)
os.makedirs(DEST, exist_ok=True)
F = '/System/Library/Fonts/Supplemental/'
SERIF, BOLD, ITALIC = F + 'Georgia.ttf', F + 'Georgia Bold.ttf', F + 'Georgia Italic.ttf'
W, H = 1920, 1080

CONCEPTS = [
    ('01', 'tabletop-classic', 'Tabletop Classic', 'Chunky separated tiles with layered earth sides. The whole Salt Marches as an island board.'),
    ('02', 'miniature-diorama', 'Miniature Diorama', 'Low camera, tilt-shift focus on the pawn, golden-hour light on a wooden table.'),
    ('03', 'seamless-realm', 'Seamless Realm', 'One continuous landscape with a faint hex grid, cliffs only at the coast, drifting cloud shadows.'),
    ('04', 'ink-and-ash', 'Ink & Ash', "The battle screens' charcoal-and-bone palette: ink outlines, lantern-lit sites, mist under the board."),
    ('05', 'night-march', 'Night March', 'Night rounds as moonlight. Lit windows and lanterns guide the eye; enemy eyes glow in the dark.'),
    ('06', 'the-unexplored', 'The Unexplored', 'Face-down tiles sealed in wax. A revealed tile flips onto its slot while the stack waits.'),
    ('07', 'terraced-columns', 'Terraced Columns', 'Each terrain at its own height on deep rock pillars: lakes sink, hills and mountains rise.'),
    ('08', 'isometric-pastel', 'Isometric Pastel', 'True orthographic isometric view, soft toy palette, clean low-poly shapes.'),
    ('09', 'tile-kit', 'Tile Kit', 'The modular pieces: six terrains, rivers, every site type, and the pawn, enemy and Warlord tokens.'),
    ('10', 'readable-play-layer', 'Readable Play Layer', 'Reachable hexes, the planned path, a threat ring and site labels drawn over the 3D board.'),
]


def run(args):
    subprocess.run(['magick'] + [str(a) for a in args], check=True, stderr=subprocess.DEVNULL)


def size(path):
    out = subprocess.run(['magick', 'identify', '-format', '%w %h', path], capture_output=True, text=True, check=True).stdout
    w, h = out.split()
    return int(w), int(h)


_n = [0]


def text_img(text, font, pt, fill, bg='none', border=(0, 0), stroke=None):
    _n[0] += 1
    p = os.path.join(TMP, f't{_n[0]}.png')
    args = ['-background', bg, '-fill', fill, '-font', font, '-pointsize', pt]
    if stroke:
        args += ['-stroke', stroke[0], '-strokewidth', stroke[1]]
    args += [f'label:{text}']
    if border != (0, 0):
        args += ['-bordercolor', bg, '-border', f'{border[0]}x{border[1]}']
    run(args + [p])
    return p


def plate(lines, bg='#100e13d0', edge='#c9a35a', pad=(20, 12)):
    """lines: [(text, font, pt, fill)] stacked into a framed plate."""
    parts = [text_img(t, f, s, c, bg='none') for (t, f, s, c) in lines]
    _n[0] += 1
    stack = os.path.join(TMP, f'stack{_n[0]}.png')
    run(['-background', 'none'] + parts + ['-gravity', 'West', '-append', stack])
    w, h = size(stack)
    W2, H2 = w + 2 * pad[0], h + 2 * pad[1]
    out = os.path.join(TMP, f'plate{_n[0]}.png')
    run(['-size', f'{W2}x{H2}', 'xc:none', '-fill', bg, '-stroke', edge, '-strokewidth', '1.5',
         '-draw', f'roundrectangle 1,1 {W2 - 2},{H2 - 2} 7,7', stack, '-gravity', 'NorthWest', '-geometry', f'+{pad[0]}+{pad[1]}', '-composite', out])
    return out, (W2, H2)


def composite(base, layer, x, y, out=None):
    run([base, layer, '-geometry', f'+{int(x)}+{int(y)}', '-composite', out or base])


def caption(img, num, title, sub, top=False):
    p, (w, h) = plate([(f'{num} · {title}', BOLD, 28, '#efe3c6'), (sub, ITALIC, 17, '#d9ccb0')], pad=(22, 14))
    composite(img, p, 28, 24 if top else H - h - 26)


def grade_paper(img, strength=7, vignette=True, tint='#e8dcc0'):
    noise = os.path.join(TMP, 'noise.png')
    if not os.path.exists(noise):
        run(['-size', f'{W}x{H}', 'xc:gray50', '-seed', '7', '+noise', 'Gaussian', '-colorspace', 'gray', '-blur', '0x0.7',
             '+level', '44%,56%', noise])
    run([img, noise, '-compose', 'soft-light', '-define', f'compose:args={strength}', '-composite', img])
    run([img, '-fill', tint, '-colorize', '4%', img])
    if vignette:
        vig(img, 0.4)


def vig(img, amount=0.45):
    v = os.path.join(TMP, f'vig{int(amount * 100)}.png')
    if not os.path.exists(v):
        run(['-size', f'{W}x{H}', 'radial-gradient:white-black', '-level', '0%,135%', '-evaluate', 'pow', '1.6',
             '-fill', 'white', '-colorize', f'{int((1 - amount) * 100)}%', v])
    run([img, v, '-compose', 'multiply', '-composite', img])


def tile_labels(img, data):
    for name, (x, y) in data['labels']:
        p, (w, h) = plate([(name, SERIF, 21, '#efe3c6')], bg='#100e13c8', edge='#8a7146', pad=(12, 6))
        composite(img, p, x - w / 2, y + 6)


def play_layer(img, scr):
    # site labels
    for name, (x, y) in scr['sites'].items():
        if name != 'Drowned Cairn' and 30 < x < W - 30 and 60 < y < H - 160:
            p, (w, h) = plate([(name, SERIF, 19, '#efe3c6')], bg='#100e13c0', edge='#8a7146', pad=(10, 5))
            composite(img, p, x - w / 2, y - h)
    # move-cost chips along the path
    costs = ['2', '3', '2']
    for c, (x, y) in zip(costs, scr['path']):
        chip = os.path.join(TMP, f'chip{c}.png')
        run(['-size', '46x46', 'xc:none', '-fill', '#1d1a1ce6', '-stroke', '#e6c17c', '-strokewidth', '2.5',
             '-draw', 'circle 23,23 23,3', '-fill', '#ffdb78', '-stroke', 'none', '-font', BOLD, '-pointsize', '24',
             '-gravity', 'center', '-annotate', '+0+1', c, chip])
        composite(img, chip, x - 23, y - 23)
    # target and enemy callouts
    tx, ty = scr['target']
    p, (w, h) = plate([('Drowned Cairn', BOLD, 22, '#efe3c6'), ('Ruins · path costs 7 Move', ITALIC, 17, '#e6c17c')], pad=(14, 9))
    composite(img, p, tx - w / 2, ty - h - 18)
    ex, ey = scr['enemy']
    p, (w, h) = plate([('Eel Hunter', BOLD, 21, '#f4d6cf'), ('Enemy group · fight to clear', ITALIC, 16, '#e8826f')], bg='#2a0f0ee0', edge='#a63a33', pad=(13, 8))
    composite(img, p, ex - w / 2 + 40, ey - h + 10)
    # top HUD
    p, (w, h) = plate([('The Salt Marches  ·  Day 2', BOLD, 26, '#efe3c6')], pad=(22, 10))
    composite(img, p, 28, 24)
    p, (w2, h2) = plate([('Move 7   ·   Influence 2   ·   Hand 5', SERIF, 22, '#ffdb78')], pad=(20, 12))
    composite(img, p, W - w2 - 28, 24)
    # legend
    leg = os.path.join(TMP, 'legend.png')
    run(['-size', '380x118', 'xc:none', '-fill', '#100e13d0', '-stroke', '#c9a35a', '-strokewidth', '1.5',
         '-draw', 'roundrectangle 1,1 378,116 7,7',
         '-stroke', '#f0c860', '-strokewidth', '4', '-fill', 'none', '-draw', 'polygon 30,22 44,14 58,22 58,38 44,46 30,38',
         '-stroke', '#ff5a44', '-draw', 'polygon 30,66 44,58 58,66 58,82 44,90 30,82',
         '-stroke', 'none', '-fill', '#fff1c9', '-draw', 'circle 36,104 36,100 circle 48,104 48,100 circle 60,104 60,100',
         '-fill', '#efe3c6', '-font', SERIF, '-pointsize', '18',
         '-annotate', '+76+36', 'Reachable this turn', '-annotate', '+76+80', 'Enemy group (threat)', '-annotate', '+76+110', 'Planned path', leg])
    composite(img, leg, W - 380 - 28, H - 118 - 26)


for num, slug, title, sub in CONCEPTS:
    src = os.path.join(SRC, f'final_{num}.png')
    if not os.path.exists(src):
        print('missing', src)
        continue
    dst = os.path.join(DEST, f'{num}-{slug}.png')
    run([src, '-strip', dst])
    if num == '04':
        grade_paper(dst)
    elif num == '05':
        vig(dst, 0.35)
    elif num in ('02', '06'):
        vig(dst, 0.3)
    if num == '09':
        tile_labels(dst, json.load(open(os.path.join(SRC, 'final_09_labels.json'))))
    if num == '10':
        play_layer(dst, json.load(open(os.path.join(SRC, 'final_10.json'))))
    caption(dst, num, title, sub, top=num == '09')
    print('wrote', dst)
