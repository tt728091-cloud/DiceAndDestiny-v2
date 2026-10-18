"""Post-process set 2 renders: captions plus per-look finishing passes.
usage: python3 post2.py <out_dir> <dest_dir>
"""
import os, subprocess, sys

SRC, DEST = sys.argv[1], sys.argv[2]
TMP = os.path.join(SRC, 'post_tmp2')
os.makedirs(TMP, exist_ok=True)
os.makedirs(DEST, exist_ok=True)
F = '/System/Library/Fonts/Supplemental/'
BOLD, ITALIC = F + 'Georgia Bold.ttf', F + 'Georgia Italic.ttf'
W, H = 1920, 1080

CONCEPTS = [
    ('11', 'overcast-marches', 'Overcast Marches', 'Realistic terrain under a flat grey sky: wet grass, craggy rock, timber-framed homes.'),
    ('12', 'sunlit-realism', 'Sunlit Realism', 'A clear summer afternoon, shot like a miniature photograph with shallow focus.'),
    ('13', 'blood-moon', 'Blood Moon', 'Crimson moonlight, black silhouettes, burning homes, rivers running dark red.'),
    ('14', 'winter-of-the-dead', 'Winter of the Dead', 'Snowbound tiles, frozen lakes, bare trees and chimney smoke under a low cold sun.'),
    ('15', 'necromancers-blight', "Necromancer's Blight", "The Regent's rot spreads west: violet dead land, green witchlights, poisoned mist."),
    ('16', 'the-drowned-kingdom', 'The Drowned Kingdom', 'The marches lie under the sea: caustic light, kelp, drifting bubbles, a sunbeam.'),
    ('17', 'storm-coast', 'Storm Coast', 'Greywharf in a gale: heaving sea, lit windows, lightning on the horizon.'),
    ('18', 'autumn-reaping', 'Autumn Reaping', 'Rust and gold woods, haystacks and chimney smoke in a low amber sun.'),
    ('19', 'salt-and-bone', 'Salt and Bone', 'Bleached salt flats, turquoise brine and the ribs of drowned giants.'),
    ('20', 'valley-fog-at-dawn', 'Valley Fog at Dawn', 'Mist pools in the lowlands while peaks and towers catch the first light.'),
    ('21', 'carved-granite', 'Carved Granite', 'The region as a sculpted stone relief, lit by a single torch.'),
    ('22', 'bronze-war-table', 'Bronze War Table', 'A cast-bronze map on an oak war table, with candles, dice and cards.'),
    ('23', 'obsidian-and-ember', 'Obsidian and Ember', 'Black glass tiles with burning seams, floating on a sea of lava.'),
    ('24', 'stained-glass', 'Stained Glass', 'Every hex a glowing pane set in lead; trees and towns as lead silhouettes.'),
    ('25', 'ossuary', 'Ossuary', 'The map carved from yellowed bone, set among skulls and candles in a crypt.'),
    ('26', 'living-parchment', 'Living Parchment', 'The land rises out of an old inked map; pieces in ink and red wax.'),
    ('27', 'woodcut', 'Woodcut', 'Black ink on white paper, with red kept only for the enemy.'),
    ('28', 'spectral-scrying', 'Spectral Scrying', "The map as a seer's vision, glowing in the dark."),
    ('29', 'illuminated-manuscript', 'Illuminated Manuscript', 'Vellum, gold leaf, lapis and vermilion, outlined in brown ink.'),
    ('30', 'wargame-table', 'Wargame Table', 'Painted miniature terrain under studio lights, shot macro.'),
]


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


FINISH = {
    '13': lambda i: vig(i, 0.45), '14': lambda i: vig(i, 0.25), '15': lambda i: vig(i, 0.4), '16': lambda i: vig(i, 0.45),
    '17': lambda i: (rain(i), vig(i, 0.4)), '20': lambda i: vig(i, 0.2), '21': lambda i: vig(i, 0.45),
    '22': lambda i: (bloom(i, '70%', 10, 35), vig(i, 0.45)), '23': lambda i: (bloom(i, '60%', 12, 45), vig(i, 0.35)),
    '24': lambda i: bloom(i, '45%', 16, 55), '25': lambda i: (bloom(i, '75%', 10, 30), vig(i, 0.5)),
    '26': lambda i: (grain(i, '46%,54%'), vig(i, 0.45)), '27': woodcut, '28': lambda i: bloom(i, '35%', 18, 70),
    '29': manuscript_border, '30': lambda i: vig(i, 0.3), '18': lambda i: vig(i, 0.25), '11': lambda i: vig(i, 0.2),
}

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
    if num in FINISH:
        FINISH[num](dst)
    caption(dst, num, title, sub, top=num == '29')
    print('wrote', dst)
