"""Post-process set 3 renders: captions plus per-look finishing passes.
usage: python3 post2.py <out_dir> <dest_dir>
"""
import os, subprocess, sys

SRC, DEST = sys.argv[1], sys.argv[2]
TMP = os.path.join(SRC, 'post_tmp3')
os.makedirs(TMP, exist_ok=True)
os.makedirs(DEST, exist_ok=True)
F = '/System/Library/Fonts/Supplemental/'
BOLD, ITALIC = F + 'Georgia Bold.ttf', F + 'Georgia Italic.ttf'
W, H = 1920, 1080


LANDS = [('mountains', 'Mountains'), ('deep-forest', 'Deep Forest'), ('desert', 'Desert'), ('marshland', 'Marshland'), ('farmland', 'Farmland'),
         ('highland-moor', 'Highland Moor'), ('frozen-north', 'Frozen North'), ('sea-cliffs', 'Sea Cliffs'), ('volcanic-ashlands', 'Volcanic Ashlands'),
         ('castle-and-city', 'Castle and City')]
STORM = ['Jagged peaks under a breaking storm; keeps and villages lit in the passes.',
         'Old-growth woods in driving rain; a ruined chapel hidden among the pines.',
         'A sandstorm over dunes and mesas; a walled oasis town keeps its lamps lit.',
         'Fen, reed beds and a drowned chapel under rain, draining into a grey sea.',
         'Hedged fields, windmills and a river town as the storm rolls in.',
         'Heather, standing stones and a keep on the ridge, lashed by wind and rain.',
         'A blizzard over tundra, frozen lakes and snow-laden forest.',
         'Cliffs, sea stacks and a lighthouse above a heaving, foaming sea.',
         'Volcanic lightning over ash plains, lava rivers and a black keep.',
         'A walled city under siege: camp fires on the plain, lightning over the keep.']
DAWN = ['Mist fills the valleys while peaks and towers catch the first light.',
        'Fog drifting through old-growth woods at sunrise.',
        'Golden haze over dunes and eroded mesas; palms ring the oases.',
        'Low mist over fen pools, reed beds and a drowned chapel.',
        'Morning fog in the furrows; mills, hedgerows and a river town.',
        'Heather moor and standing stones above a sea of fog.',
        'Pink alpenglow on snowfields, frozen lakes and pine.',
        'Sea fog against the cliffs, the lighthouse still burning at dawn.',
        'Steam and smoke pool in the ash plains under a red dawn.',
        'The walled city wakes above the river mist.']
CONCEPTS = []
for i, (slug, name) in enumerate(LANDS):
    CONCEPTS.append((str(31 + i), f'storm-{slug}', f'Storm \u00b7 {name}', STORM[i]))
for i, (slug, name) in enumerate(LANDS):
    CONCEPTS.append((str(41 + i), f'dawn-{slug}', f'Dawn Fog \u00b7 {name}', DAWN[i]))


def run(args):
    subprocess.run(['magick'] + [str(a) for a in args], check=True, stderr=subprocess.DEVNULL)


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


def finish(num, img):
    n = int(num)
    if n <= 40:
        {33: dust, 37: blizzard, 39: ashfall}.get(n, rain)(img)
        vig(img, 0.4)
    else:
        vig(img, 0.2)


only = set(sys.argv[3:])
for num, slug, title, sub in CONCEPTS:
    if only and num not in only:
        continue
    src = os.path.join(SRC, f'final_{num}.png')
    if not os.path.exists(src):
        print('missing', src)
        continue
    dst = os.path.join(DEST, f'{num}-{slug}.png')
    run([src, '-strip', dst])
    finish(num, dst)
    caption(dst, num, title, sub)
    print('wrote', dst)
