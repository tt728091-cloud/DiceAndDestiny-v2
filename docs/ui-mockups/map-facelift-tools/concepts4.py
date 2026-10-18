"""Set 4: desert deep-dive in the Storm and Dawn Fog styles. Usage:
blender -b --factory-startup -P concepts4.py -- <id 51-60> <out.png> [preview]
51-55 storm, 56-60 dawn; the same five desert layouts in both styles.
"""
import sys, os, math, random
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import bpy, bmesh
import hexkit as K
import real as R
import biomes as B
import desert as D
from hexkit import lin, Ctx
from mathutils import Vector, Matrix, noise

argv = sys.argv[sys.argv.index('--') + 1:]
CID, OUT = argv[0], argv[1]
PREVIEW = len(argv) > 2 and argv[2] == 'preview'
V = D.V


def palette(**over):
    p = dict(
        plains=(lin('#a07a4a'), lin('#b08a58')), forest=(lin('#5a5030'), lin('#6a5a38')), hills=(lin('#a07048'), lin('#b08050')),
        hill_rock=lin('#8a5a3a'), marsh=(lin('#6a6040'), lin('#7a6a48')), mud=lin('#6a4a30'), bank=lin('#a07850'), sand=lin('#c8a070'),
        deep=lin('#1e4a4a'), mtn_base=(lin('#9a6a48'), lin('#aa7a52')), mtn_rock=(lin('#8a5a3e'), lin('#b07a52')), snow=lin('#e8d8c0'),
        strata=[(0.0, lin('#5a3020')), (0.28, lin('#b0703f')), (0.5, lin('#8a4428')), (0.7, lin('#c88e5a')), (0.86, lin('#d4a46c'))],
        water=lin('#2a8a8a'), marsh_water=lin('#3a5a4a'), sea=lin('#2a5a6a'), seabed=lin('#0a1a1e'),
        trunk=lin('#4a3424'), deadwood=lin('#5a4a3c'),
        conifer=[lin('#3a4a2a')], broad=[lin('#5a6a32'), lin('#4e6a2a')],
        rock=lin('#a8805a'), reed=lin('#8a8a4a'), cattail=lin('#4a3020'), lily=lin('#4e6a2a'),
        field_cols=[lin('#6a8a34'), lin('#7a9a3a'), lin('#5a7a2e'), lin('#8a8a40')], field_soil=lin('#5a3e26'),
        wall=lin('#d8bc94'), roofs=[lin('#c8a070')], roof_dark=lin('#4a3a30'), stone=lin('#caa77a'),
        door=lin('#22160e'), window=lin('#1a120c'), canvas=lin('#d8c8a4'), back=(lin('#2a2220'), lin('#3a3028')), table=lin('#3a2418'),
        beam=lin('#3a2618'), bone=lin('#e6dcc4'),
        sand3=(lin('#e2b46e'), lin('#cc8c46'), lin('#9c5e2e')), rust=lin('#b85a26'),
        redsand3=(lin('#c88352'), lin('#b06a3c'), lin('#80462a')), rock3=(lin('#d29c6a'), lin('#bc7244'), lin('#9c4c2c')),
        varnish=lin('#3a2216'), salt=lin('#e2d8c6'), salt_stain=lin('#b89a76'), oasis_green=lin('#56782a'), oasis_water=lin('#2a8a86'),
        mud_cracks=lin('#a07a52'), adobe=lin('#dcbc8c'), adobe_tints=[lin('#ffffff'), lin('#f2e2d0'), lin('#ead6bc'), lin('#f6e8cc'), lin('#f0d8c8')],
        dome=lin('#2f7d8a'), awnings=[lin('#8c2f2a'), lin('#2f4f86'), lin('#c9a35a'), lin('#5a7a2e'), lin('#9a4a6a')],
        camel=lin('#b88a5a'), tent=lin('#3a2a20'), acacia=[lin('#6a6a3a'), lin('#5c6a34')], palm=[lin('#4e6a2a'), lin('#5a7430'), lin('#46602a')],
    )
    p.update(over)
    return p


GRASS = dict(count={'sandflat': 700, 'oasis': 1100, 'canyon': 120, 'rockland': 250}, children=4, length=0.02,
             cols=[lin('#8a7a48'), lin('#a08c52'), lin('#6e6a3a'), lin('#b49a5a')], radius=0.0011)


def setup(pal, **kw):
    s = dict(gap=0.03, thickness=0.35, bevel=0.015, density=1.0, rivers=True, fields=False, real=True, smooth=True, detail=0.0,
             token_scale=1.5, site_scale=1.45, tree_scale=1.1, dressing=False, roof='slate', snow=False, wind=-1.05,
             dune_lambda=2.4, dune_amp=0.42, mesa_scale=1.0, grass=GRASS)
    s['pal'] = pal
    s.update(kw)
    Ctx.style = s
    K.build_materials()
    R.extra_materials()
    K.build_meshes()
    R.build_meshes_real()
    R.build_real_extras()
    R.install(True)
    B.biome_materials()
    B.build_biome_props()
    D.desert_materials(pal)
    D.build_desert_meshes()


def res_by(target, near=56, mid=40, far=24, r1=5.0, r2=9.0):
    tx, ty = target

    def f(hx):
        d = math.hypot(hx.center.x - tx, hx.center.y - ty)
        return near if d < r1 else mid if d < r2 else far
    return f


def bolt(x, y, z, col, energy, glossy=False, seed=0):
    rnd = random.Random(seed)
    mat = K.m_emit('bolt', col, 60.0)
    pts = [Vector((x, y, z))]
    for i in range(11):
        pts.append(pts[-1] + Vector((rnd.uniform(-0.8, 0.8), rnd.uniform(-0.3, 0.3), -z / 11.5)))
    cu = bpy.data.curves.new('bolt', 'CURVE')
    cu.dimensions = '3D'
    cu.bevel_depth = 0.06
    sp = cu.splines.new('POLY')
    sp.points.add(len(pts) - 1)
    for i, q in enumerate(pts):
        sp.points[i].co = (q.x, q.y, q.z, 1)
    cu.materials.append(mat)
    K.link(bpy.data.objects.new('bolt', cu))
    fl = R.point(Vector((x, y - 4, z * 0.6)), energy, (0.8, 0.85, 1.0), 3.0)
    fl.visible_glossy = glossy


def grid_lines():
    g = K.m_alpha('grid', lin('#5a3a22'), 0.2)
    g['no_overlay'] = True
    for hx in Ctx.hexes.values():
        K.hex_ring(hx, hx.center, 0.988, 1.0, g, z=0.006, n=14)


def cells_from(xr, yr, tf, seed):
    return B.gen_cells(xr, yr, tf, seed)


def setsite(cells, x, y, terrain=None, site=None, allowed=None):
    return B.put(cells, x, y, terrain=terrain, site=site, allowed=allowed)


XR, YR = (-10.5, 10.5), (-5.0, 16.0)


# ---------------------------------------------------------------------------
# The five desert layouts

def lay_erg():
    def tf(w, rnd):
        return 'sandflat' if noise.noise(Vector((w.x * 0.18, w.y * 0.18, 1.0))) < -0.3 else 'dune'
    cells = cells_from(XR, YR, tf, 1)
    oas = setsite(cells, 2.6, 7.0, terrain='oasis')
    core = B.nearest(cells, 4.4, 6.4, exclude=(oas,))
    cells[core][0], cells[core][1] = 'dcity', 'citycore'
    setsite(cells, -4.5, 5.0, terrain='dune', site='ruins')
    setsite(cells, 5.6, 2.0, terrain='dune', site='skeleton')
    setsite(cells, -1.4, 8.8, terrain='sandflat', site='camp')
    extra = dict(caravan=dict(path=[(-6.0, -1.8), (-3.2, 0.4), (-1.2, 2.4), (0.6, 4.4), (1.8, 5.8)], camels=7, wagons=2, spacing=0.17, offset=3.0))
    return cells, [], (0.5, 1.6), [(2.7, 2.3), (3.1, 2.7)], (-3.6, 10.0), extra


def lay_crest():
    cells = cells_from((-5.5, 5.5), (-4.0, 8.0), lambda w, rnd: 'dune', 2)
    setsite(cells, 1.7, 2.4, site='ruins')
    setsite(cells, -1.9, 2.9, site='skeleton')
    extra = dict(crest=(0.0, 0.6))
    return cells, [], None, [(2.4, 4.4), (2.8, 4.8)], None, extra


def lay_canyon():
    def tf(w, rnd):
        return 'rockland' if noise.noise(Vector((w.x * 0.22, w.y * 0.22, 3.0))) > 0.35 else 'canyon'
    cells = cells_from(XR, YR, tf, 3)
    setsite(cells, 4.2, 6.0, terrain='rockland', site='fort')
    setsite(cells, -3.6, 4.6, terrain='canyon', site='temple')
    setsite(cells, -4.6, 9.2, terrain='canyon', site='ruins')
    river = [(1.5, 16.0), (-1.0, 10.5), (1.2, 6.0), (-0.4, 2.0), (0.8, -2.5), (-0.5, -6.0)]
    return cells, [river], (1.05, 0.9), [(-1.8, 2.7), (-1.4, 3.1)], (3.0, 10.6), {}


def lay_salt():
    def tf(w, rnd):
        d = math.hypot(w.x - 0.3, w.y - 5.5) + 1.2 * noise.noise(Vector((w.x * 0.3, w.y * 0.3, 4.0)))
        return 'saltpan' if d < 4.2 else 'sandflat' if d < 5.6 else 'dune'
    cells = cells_from(XR, YR, tf, 4)
    setsite(cells, -1.4, 3.6, terrain='saltpan', site='skeleton')
    setsite(cells, 2.4, 7.0, terrain='saltpan', site='skeleton')
    setsite(cells, 0.8, 9.4, site='obelisk')
    setsite(cells, -5.2, 1.2, terrain='sandflat', site='camp')
    extra = dict(caravan=dict(path=[(-8.0, 2.0), (-6.4, 4.8), (-5.4, 7.6), (-5.0, 10.0)], camels=6, wagons=1, spacing=0.17, offset=1.0))
    return cells, [], (0.3, 1.6), [(3.2, 3.0), (3.6, 3.4)], (-3.8, 10.5), extra


def lay_city():
    def tf(w, rnd):
        v = noise.noise(Vector((w.x * 0.2, w.y * 0.2, 5.0)))
        if w.y > 11.0:
            return 'canyon' if v > 0.1 else 'dune'
        return 'dune' if v > -0.15 else 'sandflat'
    cells = cells_from(XR, YR, tf, 5)
    oas = setsite(cells, -1.2, 5.6, terrain='oasis')
    oas2 = setsite(cells, -4.0, 8.2, terrain='oasis')
    core = B.nearest(cells, 2.5, 4.4, exclude=(oas, oas2))
    cells[core][0], cells[core][1] = 'dcity', 'citycore'
    cc = K.hex_world(*core)
    for k in list(cells.keys()):
        w = K.hex_world(*k)
        if k != core and (w - cc).length < 1.9 and w.y >= cc.y - 0.2 and cells[k][0] != 'oasis':
            cells[k][0], cells[k][1] = 'dcity', None
    for k in list(cells.keys()):
        w = K.hex_world(*k)
        if cells[k][0] in ('dune', 'sandflat') and min((w - K.hex_world(*oas)).length, (w - K.hex_world(*oas2)).length) < 1.9:
            cells[k][0] = 'sandflat'
            cells[k][1] = 'fields'
    setsite(cells, -4.4, 2.0, terrain='sandflat', site='camp')
    extra = dict(caravan=dict(path=[(-7.5, -1.8), (-4.8, 0.0), (-2.2, 1.4), (0.4, 2.4), (1.8, 3.2)], camels=8, wagons=2, spacing=0.16, offset=1.4),
                 oases=(oas, oas2))
    return cells, [], (0.2, 1.4), [(4.9, 1.6)], (-5.6, 11.2), extra


LAYOUTS = [lay_erg, lay_crest, lay_canyon, lay_salt, lay_city]


def fields_near_oasis():
    pal = Ctx.style['pal']
    for hx in Ctx.hexes.values():
        if hx.site != 'fields':
            continue
        rnd = random.Random(hx.seed + 17)
        ang = rnd.uniform(0, math.pi)
        for (x, y) in D.scatter_xy(hx, 6, 0.28, margin=0.16, seed=21):
            D.place(Ctx.meshes['furrow_patch'], x, y, ang + rnd.uniform(-0.08, 0.08), (rnd.uniform(0.3, 0.4), rnd.uniform(0.2, 0.26), 1.0),
                    K.jitter_col(rnd.choice(pal['field_cols']), rnd, 0.08), sink=0.002)
        for (x, y) in D.scatter_xy(hx, 4, 0.15, seed=23):
            D.place(Ctx.meshes[f'palm{rnd.randrange(3)}'], x, y, rnd.uniform(0, 6), rnd.uniform(1.2, 1.6), K.jitter_col(rnd.choice(pal['palm']), rnd))


def build(idx, style):
    cells, rivers, hero, enemies, warlord, extra = LAYOUTS[idx]()
    if 'crest' in extra:
        D.build_desert(cells, rivers, None, enemies, warlord)
        cx, cy = D.crest_point(*extra['crest'], r=0.55)
        g = D.ground(cx, cy)
        K.add(Ctx.meshes['hero'], g - Vector((0, 0, 0.01)), 0.4, Ctx.style['token_scale'])
        Ctx.hero = g
        D.footprints([(cx, cy), (cx - 0.2, cy - 0.5), (cx - 0.5, cy - 1.0), (cx - 0.6, cy - 1.8)])
    else:
        D.build_desert(cells, rivers, (None, hero) if hero else None, enemies, warlord)
    if 'caravan' in extra:
        D.caravan(**extra['caravan'])
    fields_near_oasis()
    R.apply_overlays()
    if style == 'dawn':
        grid_lines()
    return extra


# ---------------------------------------------------------------------------
# Storm style

STORM = {
    0: dict(cam=((0.0, 2.6, 0.15), -10, 19, 8.6, 32), haboob=1.3, drift=0.7, haze=0.025),
    1: dict(cam=None, haboob=1.5, drift=0.45, haze=0.015),
    2: dict(cam=((0.2, 3.2, 0.2), -6, 20, 8.8, 32), haboob=0.0, drift=0.0, haze=0.03),
    3: dict(cam=((0.3, 2.8, 0.1), -8, 17, 8.6, 32), haboob=1.0, drift=0.4, haze=0.025),
    4: dict(cam=((1.0, 3.0, 0.15), -12, 19, 8.2, 32), haboob=1.6, drift=1.0, haze=0.07),
}


def storm(idx):
    pal = palette()
    kw = dict(window_glow=14.0, flame_k=40.0, light_mult=2.0, overlays={'ao': (0.06, 0.55, (0.3, 0.24, 0.2, 1)), 'desat': 0.05})
    tgt = (STORM[idx]['cam'] or ((0, 0, 0),))[0]
    kw['res_fn'] = res_by(tgt[:2]) if idx != 1 else res_by((0.0, 0.8), 84, 48, 32, 3.0, 6.0)
    kw.update(gap=0.012, bevel=0.006)
    if idx == 1:
        kw['dune_amp'] = 0.5
    if idx == 2:
        kw.update(dry_rivers=False, overlays={'ao': (0.06, 0.6, (0.25, 0.22, 0.22, 1)), 'desat': 0.12, 'tint': (0.94, 0.92, 0.94, 1)}, mesa_scale=1.3)
    if idx == 3:
        kw['wet'] = 0.85
    setup(pal, **kw)
    if idx == 2:
        mud = K.m_water('flood', lin('#a07040'), trans=0.05, rough=0.4, ripple=0.8)
        mud['no_overlay'] = True
        Ctx.mats['water'] = mud
    extra = build(idx, 'storm')
    T = Ctx.style['thickness']
    R.plane((0, 5, -T * 0.6), 90, K.m_flat('void', lin('#3a2a1c'), 0.95))
    S = STORM[idx]
    if idx == 1:
        h = Ctx.hero
        cam = ((h.x + 0.1, h.y + 0.5, h.z + 0.05), -20, 20, 3.5, 45)
    else:
        cam = S['cam']
    if idx == 2:
        K.world(lin('#4a4e58'), 1.0, camera_bg=[lin('#6a6a70'), lin('#1a1c22')])
        sn = K.sun(-150, 8, 2.6, (1.0, 0.7, 0.45), 2.0)
        bolt(-3.0, 22.0, 10.0, lin('#d8e8ff'), 26000.0, seed=3)
        K.fog_box((0, 10, 1.5), (50, 34, 4.0), S['haze'], lin('#8a8e96'), 0.2)
    elif idx == 3:
        K.world(lin('#4a4252'), 1.0, camera_bg=[lin('#7a6a70'), lin('#1c1820')])
        sn = K.sun(-150, 7, 2.4, (1.0, 0.66, 0.42), 2.0)
        bolt(2.5, 24.0, 10.0, lin('#e0e8ff'), 30000.0, glossy=True, seed=5)
        bolt(-6.0, 26.0, 10.0, lin('#e0e8ff'), 12000.0, glossy=True, seed=8)
    else:
        K.world(lin('#5a4634'), 0.8, camera_bg=[lin('#7a5a3c'), lin('#140f0c')])
        sn = K.sun(-125, 9, 3.4 if idx != 4 else 2.0, (1.0, 0.72, 0.45), 2.0)
        bolt(3.5, 26.0, 9.0, lin('#ffe2c0'), 20000.0, seed=idx)
    sn.visible_glossy = idx == 3
    if S['haboob']:
        K.fog_box((0, 26, 4.0), (90, 14, 9.0), S['haboob'] * 2.4, lin('#3e2614'), 0.1, clumps=0.2, fade_z=(1.0, 8.5))
    if S['haze'] and idx != 2:
        K.fog_box((0, 9, 0.9), (60, 34, 1.8), S['haze'], lin('#a07a50'), 0.25)
    if S['drift']:
        K.fog_box((0, 5, 0.18), (60, 34, 0.5), S['drift'], lin('#dab27c'), 0.0, clumps=1.4, fade_z=(0.04, 0.4))
    dof = {'fstop': 1.6, 'offset': 0.0} if idx == 1 else None
    K.camera(cam[0], cam[1], cam[2], cam[3], lens=cam[4], dof=dof)
    exp = {0: 0.45, 1: 0.35, 2: 0.4, 3: 0.3, 4: 0.6}[idx]
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=exp, samples=192)


# ---------------------------------------------------------------------------
# Dawn style

DAWN = {
    0: dict(cam=((0.2, 3.4, 0.1), 8, 21, 10.5, 36), fog=(0.21, 1.6), sky=('#f2c4a2', '#6a78ac')),
    1: dict(cam=None, fog=(0.18, 1.0), sky=('#f4c8a6', '#6e7cae')),
    2: dict(cam=((0.3, 3.4, 0.2), 10, 28, 10.0, 38), fog=(0.36, 2.2), sky=('#f0bca0', '#6676aa')),
    3: dict(cam=((0.6, 3.6, 0.1), 6, 24, 10.0, 36), fog=(0.13, 1.2), sky=('#f6c8b8', '#7480b4')),
    4: dict(cam=((0.8, 3.2, 0.15), 8, 25, 9.4, 38), fog=(0.21, 1.6), sky=('#f2c2a4', '#6a78ac')),
}


def dawn(idx):
    pal = palette()
    kw = dict(gap=0.0, thickness=0.6, bevel=0.0, window_glow=5.0, flame_k=30.0, overlays={'ao': (0.06, 0.5, (0.32, 0.26, 0.24, 1))})
    D_ = DAWN[idx]
    tgt = (D_['cam'] or ((0, 0, 0),))[0]
    kw['res_fn'] = res_by(tgt[:2]) if idx != 1 else res_by((0.0, 0.8), 84, 48, 32, 3.0, 6.0)
    if idx == 1:
        kw['dune_amp'] = 0.5
    if idx == 2:
        kw.update(dry_rivers=True, mesa_scale=1.3)
    if idx == 3:
        kw['wet'] = 0.5
    setup(pal, **kw)
    extra = build(idx, 'dawn')
    if idx == 1:
        h = Ctx.hero
        cam = ((h.x - 0.1, h.y + 0.5, h.z + 0.05), 18, 23, 3.7, 45)
    else:
        cam = D_['cam']
    top, dens = D_['fog']
    K.fog_box((0, 5, top * 0.5), (60, 40, top + 0.1), dens, lin('#f2d4c4'), 0.0, clumps=0.8, fade_z=(0.0, top))
    K.fog_box((0, 12, 1.2), (60, 36, 4.0), 0.016, lin('#f4d2bc'), 0.5)
    if idx == 4:
        for k in extra.get('oases', ()):
            c = K.hex_world(*k)
            K.fog_box((c.x, c.y, 0.25), (2.2, 2.2, 0.5), 2.0, lin('#f6e2d8'), 0.0, clumps=1.6, fade_z=(-0.05, 0.45))
        for hx in Ctx.hexes.values():
            if hx.site == 'citycore':
                for (dx, dy) in ((0.2, 0.3), (-0.3, -0.1), (0.4, -0.3)):
                    R.smoke(D.ground(hx.center.x + dx, hx.center.y + dy) + Vector((0, 0, 0.15)), 0.12, 1.4, 0.8, (0.7, 0.66, 0.64, 1))
    R.sky_world(4, 250, strength=0.3, dust=3.0, camera_bg=[lin(D_['sky'][0]), lin(D_['sky'][1])])
    K.sun(40, 6, 5.6, (1.0, 0.62, 0.36), 1.0)
    dof = {'fstop': 1.8, 'offset': 0.0} if idx == 1 else None
    K.camera(cam[0], cam[1], cam[2], cam[3], lens=cam[4], dof=dof)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.15)


i = int(CID)
K.reset()
if 51 <= i <= 55:
    storm(i - 51)
else:
    dawn(i - 56)
