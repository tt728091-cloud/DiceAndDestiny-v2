"""Set 3: Storm Coast and Valley Fog at Dawn across ten landscapes. Usage:
blender -b --factory-startup -P concepts3.py -- <id 31-50> <out.png> [preview]
31-40 = storm, 41-50 = dawn; landscapes in the same order for both.
"""
import sys, os, math, random
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import bpy, bmesh
import hexkit as K
import real as R
import biomes as B
from hexkit import lin, Ctx
from mathutils import Vector, Matrix, noise

argv = sys.argv[sys.argv.index('--') + 1:]
CID, OUT = argv[0], argv[1]
PREVIEW = len(argv) > 2 and argv[2] == 'preview'


def nature(**over):
    p = dict(
        plains=(lin('#58642f'), lin('#747342')), forest=(lin('#343c24'), lin('#44462a')),
        hills=(lin('#646436'), lin('#7a6c44')), hill_rock=lin('#68625a'),
        marsh=(lin('#454e2e'), lin('#545636')), mud=lin('#43372a'), bank=lin('#594b37'), sand=lin('#857658'), deep=lin('#18241f'),
        mtn_base=(lin('#5a564c'), lin('#6a6458')), mtn_rock=(lin('#55524d'), lin('#7a756c')), snow=lin('#e9ebef'),
        strata=[(0.0, lin('#26201b')), (0.33, lin('#4a3e32')), (0.58, lin('#3a3128')), (0.84, lin('#221c17'))],
        water=lin('#3e5856'), marsh_water=lin('#3a4230'), sea=lin('#2a454c'), seabed=lin('#0a1214'),
        trunk=lin('#35281e'), deadwood=lin('#463f37'),
        conifer=[lin('#1d2c21'), lin('#243425'), lin('#2a3624')], broad=[lin('#3a4a24'), lin('#46562a'), lin('#525c2c'), lin('#364622')],
        rock=lin('#75706a'), reed=lin('#77714a'), cattail=lin('#38261a'), lily=lin('#46562a'),
        field_cols=[lin('#9c8646'), lin('#8a763c'), lin('#5a6630'), lin('#76663e')], field_soil=lin('#3a2e22'),
        wall=lin('#c4b89e'), roofs=[lin('#4a4c52'), lin('#555860'), lin('#3e4046'), lin('#5a5048')], roof_dark=lin('#2c2c32'),
        stone=lin('#8a8478'), door=lin('#1c1511'), window=lin('#18181c'), canvas=lin('#a49676'),
        back=(lin('#1a1c22'), lin('#22252c')), table=lin('#2a1e16'), beam=lin('#2a1d15'), bone=lin('#d6cbb0'),
    )
    p.update(over)
    return p


COUNTS = {'plains': 1800, 'hills': 1500, 'forest': 500, 'marsh': 900, 'mountain': 300, 'lake': 500, 'farm': 1500, 'moor': 1300,
          'deepforest': 300, 'swamp': 700, 'city': 400, 'peaks': 200}
GRASS = dict(count=COUNTS, children=4, length=0.024, cols=[lin('#46522a'), lin('#5e6834'), lin('#76743e'), lin('#535c2c')], radius=0.0012)

BASE = dict(gap=0.03, thickness=0.35, bevel=0.02, density=1.3, rivers=True, fields=True, sea_trans=0.6, real=True,
            res=30, smooth=True, detail=0.022, token_scale=1.5, site_scale=1.45, tree_scale=1.1, dressing=True,
            roof='slate', conifer_mix=0.65, peak_h=1.0, snow_z=0.62)


def setup(pal, **kw):
    s = dict(BASE)
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


# ---------------------------------------------------------------------------
# Orientation: storm looks north across the board, dawn looks east into the light.

class Orient:
    def __init__(self, style):
        self.storm = style == 'storm'
        self.xr, self.yr = ((-10.5, 10.5), (-8.0, 15.5)) if self.storm else ((-3.5, 17.5), (-7.5, 7.5))

    def dl(self, w):
        return (w.y + 2.0, w.x) if self.storm else (w.x + 0.5, -w.y)

    def world(self, d, l):
        return (l, d - 2.0) if self.storm else (d - 0.5, -l)


def nz(d, l, s=0.22, seed=0.0):
    return noise.noise(Vector((d * s, l * s, seed)))


# ---------------------------------------------------------------------------
# The ten landscapes (shared by both styles)

def land_mountains(O):
    def tf(w, rnd):
        d, l = O.dl(w)
        v = nz(d, l, 0.2, 1.0) + 0.055 * d
        return 'peaks' if v > 0.62 else 'mountain' if v > 0.4 else 'hills' if v > 0.2 else 'forest' if v > 0.02 else 'plains' if v > -0.25 else 'lake'
    cells = B.gen_cells(O.xr, O.yr, tf, 1)
    hero = B.put(cells, *O.world(1.5, 0.5), allowed={'plains', 'hills', 'forest'})
    B.put(cells, *O.world(4.5, -2.0), site='keep', allowed={'hills'})
    B.put(cells, *O.world(1.0, 3.0), site='village', allowed={'plains', 'forest'})
    B.put(cells, *O.world(7.5, 2.5), site='monastery', allowed={'hills', 'mountain', 'plains'})
    en = [B.put(cells, *O.world(3.5, 1.5), enemies=1, allowed={'hills', 'forest', 'plains'}), B.put(cells, *O.world(6.0, -3.5), enemies=2)]
    river = [O.world(13, 1.0), O.world(9, 2.5), O.world(6, 0.5), O.world(3, 2.0), O.world(-1, 0.8)]
    return cells, [river], hero, en, B.nearest(cells, *O.world(10, 0), {'peaks', 'mountain'})


def land_forest(O):
    def tf(w, rnd):
        d, l = O.dl(w)
        v = nz(d, l, 0.3, 2.0)
        return 'deepforest' if v > 0.05 else 'forest' if v > -0.15 else 'hills' if v > -0.28 else 'plains' if v > -0.4 else 'swamp'
    cells = B.gen_cells(O.xr, O.yr, tf, 2)
    hero = B.put(cells, *O.world(1.5, 0.0), terrain='plains')
    B.put(cells, *O.world(4.0, 1.5), terrain='deepforest', site='chapel')
    B.put(cells, *O.world(2.5, -2.5), terrain='plains', site='village')
    B.put(cells, *O.world(7.0, -1.0), terrain='hills', site='keep')
    B.put(cells, *O.world(5.5, 3.5), terrain='forest', site='hermit')
    en = [B.put(cells, *O.world(3.5, -0.8), enemies=2), B.put(cells, *O.world(6.0, 2.0), enemies=1)]
    river = [O.world(14, -3.0), O.world(9, -1.5), O.world(5, -3.0), O.world(2, -0.8), O.world(-1, -2.0)]
    return cells, [river], hero, en, None


def land_desert(O):
    def tf(w, rnd):
        d, l = O.dl(w)
        v = nz(d, l, 0.24, 3.0)
        return 'mesa' if v > 0.42 else 'badlands' if v > 0.25 else 'desert'
    cells = B.gen_cells(O.xr, O.yr, tf, 3)
    for (d, l) in ((3.5, 2.0), (8.0, -2.5)):
        B.put(cells, *O.world(d, l), terrain='oasis')
    B.put(cells, *O.world(4.2, 0.8), terrain='desert', site='town')
    B.put(cells, *O.world(6.5, 3.5), site='cairn')
    B.put(cells, *O.world(2.0, -3.0), site='shrine')
    hero = B.put(cells, *O.world(1.5, 0.3), terrain='desert')
    en = [B.put(cells, *O.world(3.5, -1.5), enemies=2), B.put(cells, *O.world(7.5, 1.0), enemies=1)]
    return cells, [], hero, en, B.nearest(cells, *O.world(10, 2))


def land_marsh(O):
    def tf(w, rnd):
        d, l = O.dl(w)
        v = nz(d, l, 0.32, 4.0)
        return 'lake' if v > 0.35 else 'swamp' if v > 0.08 else 'marsh' if v > -0.12 else 'plains' if v > -0.3 else 'forest'
    cells = B.gen_cells(O.xr, O.yr, tf, 4)
    hero = B.put(cells, *O.world(1.2, 0.0), terrain='plains')
    B.put(cells, *O.world(3.0, 2.2), terrain='plains', site='village')
    B.put(cells, *O.world(5.0, -1.5), terrain='marsh', site='chapel')
    B.put(cells, *O.world(7.5, 2.0), terrain='hills', site='grotto')
    en = [B.put(cells, *O.world(3.0, -0.8), enemies=2), B.put(cells, *O.world(6.0, 1.0), enemies=1)]
    rivers = [[O.world(13, 2.0), O.world(8, 0.5), O.world(4, 1.2), O.world(0, -0.5)], [O.world(10, -4.0), O.world(6, -2.5), O.world(3, -3.5)]]
    return cells, rivers, hero, en, None


def land_farm(O):
    def tf(w, rnd):
        d, l = O.dl(w)
        v = nz(d, l, 0.28, 5.0)
        return 'forest' if v > 0.38 else 'hills' if v > 0.25 else 'farm' if v > -0.3 else 'plains'
    cells = B.gen_cells(O.xr, O.yr, tf, 5)
    hero = B.put(cells, *O.world(1.4, 0.0), terrain='plains')
    B.put(cells, *O.world(3.0, 1.8), terrain='plains', site='village')
    B.put(cells, *O.world(6.5, -1.0), terrain='plains', site='town')
    B.put(cells, *O.world(5.0, 3.5), terrain='hills', site='monastery')
    B.put(cells, *O.world(9.0, 2.0), terrain='plains', site='village')
    en = [B.put(cells, *O.world(3.2, -1.6), enemies=1)]
    river = [O.world(15, 0.0), O.world(10, -2.0), O.world(6.5, 0.4), O.world(3.5, -0.5), O.world(-1, 1.5)]
    return cells, [river], hero, en, None


def land_moor(O):
    def tf(w, rnd):
        d, l = O.dl(w)
        v = nz(d, l, 0.26, 6.0) + 0.03 * d
        return 'mountain' if v > 0.55 else 'hills' if v > 0.3 else 'moor' if v > -0.2 else 'lake' if v > -0.32 else 'marsh'
    cells = B.gen_cells(O.xr, O.yr, tf, 6)
    hero = B.put(cells, *O.world(1.4, 0.4), terrain='moor')
    B.put(cells, *O.world(3.5, -1.5), terrain='moor', site='cairn')
    B.put(cells, *O.world(5.5, 2.0), terrain='hills', site='keep')
    B.put(cells, *O.world(2.5, 3.0), terrain='moor', site='hermit')
    en = [B.put(cells, *O.world(4.0, 0.5), enemies=2)]
    river = [O.world(12, -1.0), O.world(8, 1.0), O.world(4.5, -0.3), O.world(0, 1.2)]
    return cells, [river], hero, en, B.nearest(cells, *O.world(9, -2))


def land_frozen(O):
    def tf(w, rnd):
        d, l = O.dl(w)
        v = nz(d, l, 0.24, 7.0) + 0.045 * d
        return 'peaks' if v > 0.6 else 'mountain' if v > 0.42 else 'snowforest' if v > 0.12 else 'tundra' if v > -0.2 else 'icelake'
    cells = B.gen_cells(O.xr, O.yr, tf, 7)
    hero = B.put(cells, *O.world(1.4, 0.0), terrain='tundra')
    B.put(cells, *O.world(3.0, 2.0), terrain='tundra', site='village')
    B.put(cells, *O.world(5.5, -1.8), terrain='hills', site='keep')
    B.put(cells, *O.world(3.5, -3.2), terrain='tundra', site='cairn')
    en = [B.put(cells, *O.world(3.5, 0.0), enemies=2), B.put(cells, *O.world(6.5, 1.5), enemies=1)]
    return cells, [], hero, en, B.nearest(cells, *O.world(9.5, 0.5), {'peaks', 'mountain'})


def land_coast(O):
    def tf(w, rnd):
        d, l = O.dl(w)
        v = nz(d, l, 0.22, 8.0)
        return 'hills' if v > 0.3 else 'plains' if v > 0.05 else 'forest' if v > -0.1 else 'moor' if v > -0.25 else 'beach'
    cells = B.gen_cells(O.xr, O.yr, tf, 8)
    keep = {}
    for k, c in cells.items():
        w = K.hex_world(*k)
        d, l = O.dl(w)
        coast = 2.8 * math.sin(d * 0.35) + 0.5 * d
        if l < coast - 1.6:
            continue
        keep[k] = c
        if l < coast - 0.4:
            c[0] = 'beach'
    cells = keep
    hero = B.put(cells, *O.world(1.6, 2.0), allowed={'plains', 'hills', 'moor', 'forest'})
    B.put(cells, *O.world(3.5, 0.5), site='town', allowed={'plains', 'hills', 'beach'})
    lh = B.put(cells, *O.world(6.0, -1.0), terrain='hills', allowed={'hills', 'plains', 'beach', 'moor', 'forest'})
    B.put(cells, *O.world(5.5, 4.0), site='keep', allowed={'hills', 'plains', 'moor'})
    en = [B.put(cells, *O.world(4.5, 2.5), enemies=2, allowed={'plains', 'hills', 'moor', 'forest'})]
    return cells, [], hero, en, None, lh


def land_volcanic(O):
    def tf(w, rnd):
        d, l = O.dl(w)
        v = nz(d, l, 0.26, 9.0) + 0.03 * d
        return 'mountain' if v > 0.5 else 'badlands' if v > 0.3 else 'ash' if v > -0.15 else 'forest' if v > -0.3 else 'lake'
    cells = B.gen_cells(O.xr, O.yr, tf, 9)
    B.put(cells, *O.world(9.0, 0.5), terrain='volcano')
    B.put(cells, *O.world(6.5, 4.5), terrain='volcano')
    hero = B.put(cells, *O.world(1.4, 0.0), terrain='ash')
    B.put(cells, *O.world(4.5, -1.8), terrain='badlands', site='keep')
    B.put(cells, *O.world(3.0, 2.5), terrain='ash', site='ruins' if False else 'chapel')
    en = [B.put(cells, *O.world(3.2, -0.2), enemies=2), B.put(cells, *O.world(5.5, 1.5), enemies=1)]
    river = [O.world(9.0, 0.5), O.world(6.0, -0.5), O.world(3.0, 1.0), O.world(-1, -0.5)]
    return cells, [river], hero, en, B.nearest(cells, *O.world(7.0, -3.0))


def land_city(O):
    def tf(w, rnd):
        d, l = O.dl(w)
        v = nz(d, l, 0.28, 10.0)
        return 'forest' if v > 0.35 else 'hills' if v > 0.22 else 'farm' if v > -0.2 else 'plains'
    cells = B.gen_cells(O.xr, O.yr, tf, 10)
    center = O.world(4.6, 0.0)
    core = B.nearest(cells, *center)
    for k in list(cells.keys()):
        w = K.hex_world(*k)
        c = K.hex_world(*core)
        if (w - c).length < 1.9:
            cells[k][0] = 'city'
            cells[k][1] = None
    B.put(cells, *O.world(7.8, -2.2), terrain='hills', site='keep')
    B.put(cells, *O.world(2.0, 3.0), terrain='plains', site='village')
    B.put(cells, *O.world(8.5, 3.0), terrain='hills', site='monastery')
    hero = B.put(cells, *O.world(1.5, -0.8), terrain='plains')
    en = [B.put(cells, *O.world(2.8, -2.5), enemies=2)]
    river = [O.world(15, 2.5), O.world(9, 1.8), O.world(6.2, 1.6), O.world(3.0, 1.4), O.world(-1, 2.5)]
    return cells, [river], hero, en, B.nearest(cells, *O.world(10, -0.5))


LANDS = ['mountains', 'forest', 'desert', 'marsh', 'farm', 'moor', 'frozen', 'coast', 'volcanic', 'city']
LANDFN = {'mountains': land_mountains, 'forest': land_forest, 'desert': land_desert, 'marsh': land_marsh, 'farm': land_farm,
          'moor': land_moor, 'frozen': land_frozen, 'coast': land_coast, 'volcanic': land_volcanic, 'city': land_city}


# ---------------------------------------------------------------------------
# Per-landscape palette and effect tweaks

def land_style(land):
    pal, kw, grass_cols = {}, {}, None
    if land == 'desert':
        kw.update(grass=None, broad=None)
        pal.update(broad=[lin('#4e6a2a'), lin('#5a7430'), lin('#46602a')], rock=lin('#8a6a4e'), mtn_rock=(lin('#7a5038'), lin('#a07050')),
                   wall=lin('#d6c4a0'), roofs=[lin('#b89a6a'), lin('#a8865a'), lin('#c8aa7a')], stone=lin('#b8a07a'))
    elif land == 'frozen':
        kw.update(grass=None, snow=True, snow_z=0.3, dead_mix=0.2, conifer_mix=0.9)
    elif land == 'moor':
        grass_cols = [lin('#5a4a3a'), lin('#6a5440'), lin('#5c3e4c'), lin('#4e4a32')]
        pal.update(broad=[lin('#4a4a2a'), lin('#55502e')])
    elif land == 'volcanic':
        kw.update(grass=None, dead_mix=0.85, ember_k=5.0)
        pal.update(ember=lin('#ff5a14'), mtn_rock=(lin('#2a2624'), lin('#3e3834')), rock=lin('#3a3634'), hill_rock=lin('#2e2a28'),
                   hills=(lin('#3a3230'), lin('#4a3e38')), forest=(lin('#2a2622'), lin('#36302a')), plains=(lin('#3a3a2e'), lin('#4a4434')),
                   mtn_base=(lin('#2a2624'), lin('#3a3430')), wall=lin('#5a5450'), sand_red=(lin('#4a2e22'), lin('#5a3a2a'), lin('#3a2620')),
                   sandstone=(lin('#3a2a24'), lin('#5a3e30')), snow=lin('#5a5450'))
    elif land == 'forest':
        kw.update(conifer_mix=0.55, density=1.5)
    elif land == 'farm':
        pal.update(field_cols=[lin('#a89048'), lin('#8a7a3e'), lin('#5e6a30'), lin('#7a6a40'), lin('#4e3a26')])
    return pal, kw, grass_cols


# ---------------------------------------------------------------------------
# Styles

def storm_scene(land):
    O = Orient('storm')
    pal_over, kw, grass_cols = land_style(land)
    pal = nature(**pal_over)
    g = dict(GRASS)
    if grass_cols:
        g['cols'] = grass_cols
    opts = dict(grass=g, window_glow=12.0, flame_k=40.0, light_mult=2.0,
                overlays={'ao': (0.06, 0.6, (0.2, 0.22, 0.25, 1)), 'desat': 0.4, 'tint': (0.85, 0.9, 0.96, 1)})
    if land == 'desert':
        opts['overlays'] = {'ao': (0.06, 0.6, (0.3, 0.24, 0.2, 1)), 'desat': 0.25, 'tint': (1.0, 0.9, 0.78, 1)}
    if land == 'frozen':
        opts['overlays'] = {'ao': (0.05, 0.5, (0.3, 0.32, 0.38, 1)), 'snow': (lin('#e6ecf2'), 0.42), 'desat': 0.45, 'tint': (0.88, 0.92, 0.98, 1)}
    if land == 'volcanic':
        opts['overlays'] = {'ao': (0.06, 0.6, (0.2, 0.18, 0.18, 1)), 'desat': 0.3}
    opts.update(kw)
    setup(pal, **opts)
    if land == 'volcanic':
        lava = K.m_emit('lava_w', lin('#e8380c'), 2.6)
        Ctx.mats['water'] = lava
        Ctx.mats['marsh_water'] = lava
    elif land != 'frozen':
        sw = K.m_water('water_storm', lin('#22343a'), trans=0.25, rough=0.22, ripple=0.4)
        sw['no_overlay'] = True
        Ctx.mats['water'] = sw
        Ctx.mats['oasis_water'] = sw
        mw = K.m_water('marsh_storm', lin('#262e22'), trans=0.15, rough=0.25, ripple=0.4)
        mw['no_overlay'] = True
        Ctx.mats['marsh_water'] = mw
    res = LANDFN[land](O)
    cells, rivers, hero, en, war = res[:5]
    coastal = land in ('coast', 'marsh', 'city', 'mountains', 'farm', 'forest', 'moor')
    B.build_map(cells, rivers, hero, [e for e in en if e], war, sea=False)
    R.apply_overlays()
    T = Ctx.style['thickness']
    if coastal:
        oc = R.m_ocean('oc', lin('#0e1a1e'), lin('#2a4a50'), lin('#c8d4d6'))
        oc.node_tree.nodes['Principled BSDF'].inputs['Roughness'].default_value = 0.2
        R.ocean(-0.3, oc, size=3.2, spatial=20, scale=0.22, chop=1.4, wind=16.0, res=12, foam=0.35, loc=(0.0, 4.0))
    else:
        ground = {'desert': lin('#5a4630'), 'frozen': lin('#b8c4cc'), 'volcanic': lin('#120e0c')}.get(land, lin('#1a1e22'))
        R.plane((0, 5, -T * 0.6), 80, K.m_flat('void', ground, 0.95))
    if land == 'coast':
        B.stacks(14, (-10, 4), (-6, 16), -0.3, seed=5)
        lh = res[5]
        hx = Ctx.hexes[lh]
        K.add(Ctx.meshes['lighthouse'], hx.ground(0, 0), 0, 2.2)
        R.point(hx.ground(0, 0) + Vector((0, 0, 1.0)), 200.0, (1.0, 0.85, 0.55), 0.1)
    if land == 'city':
        c = Ctx.hexes[B.nearest(cells, *O.world(4.6, 0.0))].center
        B.siege_camp(c.x, c.y, 14)
    if land == 'volcanic':
        for hx in Ctx.hexes.values():
            if hx.terrain == 'ash' and hx.rnd.random() < 0.3:
                R.point(hx.ground(0, 0) + Vector((0, 0, 0.2)), 8.0, (1.0, 0.4, 0.15), 0.3)
    # sky, bolt, haze
    sky = {'desert': ('#8a6e52', '#2e2218', '#7a6048', '#b89a78', 0.06), 'frozen': ('#9aa4ac', '#3a4248', '#7a848a', '#c8d0d8', 0.07),
           'volcanic': ('#4a2a22', '#120806', '#3a2420', '#7a4a3a', 0.05)}.get(land, ('#5a6268', '#1a1e22', '#3a4248', '#8a9298', 0.045))
    rnd = random.Random(LANDS.index(land))
    bx = rnd.uniform(-6, 6)
    bolt = K.m_emit('bolt', lin('#ffd2a0') if land == 'volcanic' else lin('#d8e8ff'), 60.0)
    pts = [Vector((bx, 30.0, 10.0))]
    for i in range(11):
        pts.append(pts[-1] + Vector((rnd.uniform(-0.9, 0.9), rnd.uniform(-0.3, 0.3), -0.92)))
    cu = bpy.data.curves.new('bolt', 'CURVE')
    cu.dimensions = '3D'
    cu.bevel_depth = 0.06
    sp = cu.splines.new('POLY')
    sp.points.add(len(pts) - 1)
    for i, q in enumerate(pts):
        sp.points[i].co = (q.x, q.y, q.z, 1)
    cu.materials.append(bolt)
    K.link(bpy.data.objects.new('bolt', cu))
    fl = R.point(Vector((bx, 34, 16)), 26000.0, (0.75, 0.82, 1.0) if land != 'volcanic' else (1.0, 0.6, 0.35), 3.0)
    fl.visible_glossy = False
    lift = 1.0 if land in ('desert', 'frozen', 'volcanic') else 1.9
    K.world(lin(sky[2]), 0.6 * lift, camera_bg=[lin(sky[0]), lin(sky[1])])
    sn = K.sun(40, 20, 0.9 * lift, (0.8, 0.85, 0.95), 15.0)
    sn.visible_glossy = False
    K.fog_box((0, 8, 1.2), (40, 34, 4.4), sky[4], lin(sky[3]), 0.2)
    tgt = Vector(O.world(2.6, 0.3) + (0.25,))
    K.camera(tuple(tgt), -14, 18, 8.4, lens=32)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=(0.65 if land != 'frozen' else 0.35) + (0.35 if lift > 1 else 0.0), samples=192)


def dawn_scene(land):
    O = Orient('dawn')
    pal_over, kw, grass_cols = land_style(land)
    pal = nature(**pal_over)
    pal['roofs'] = [lin('#8a7448'), lin('#7a6440'), lin('#6e5a3a'), lin('#94804e')] if land not in ('desert', 'frozen', 'volcanic') else pal['roofs']
    g = dict(GRASS)
    if grass_cols:
        g['cols'] = grass_cols
    opts = dict(grass=g, gap=0.0, thickness=0.6, bevel=0.0, peak_h=1.25, window_glow=5.0, roof='thatch',
                overlays={'ao': (0.06, 0.5, (0.25, 0.25, 0.3, 1))})
    if land == 'frozen':
        opts['overlays'] = {'ao': (0.05, 0.5, (0.3, 0.32, 0.38, 1)), 'snow': (lin('#eef2f6'), 0.42)}
        opts['roof'] = 'slate'
    if land == 'desert':
        opts['roof'] = 'slate'
    opts.update(kw)
    setup(pal, **opts)
    if land == 'volcanic':
        lava = K.m_emit('lava_w', lin('#e8380c'), 2.4)
        Ctx.mats['water'] = lava
        Ctx.mats['marsh_water'] = lava
    res = LANDFN[land](O)
    cells, rivers, hero, en, war = res[:5]
    B.build_map(cells, rivers, hero, [e for e in en if e], war, sea=True, sea_z=-0.33)
    R.apply_overlays()
    if land == 'coast':
        B.stacks(12, (-4, 18), (-12, 6), -0.33, seed=6)
        hx = Ctx.hexes[res[5]]
        K.add(Ctx.meshes['lighthouse'], hx.ground(0, 0), 0, 2.2)
        R.point(hx.ground(0, 0) + Vector((0, 0, 1.0)), 120.0, (1.0, 0.85, 0.55), 0.1)
    if land == 'city':
        R.smoke(K.hex_world(*B.nearest(cells, *O.world(4.6, 0.0))) + Vector((0.3, 0.2, 0.3)), 0.25, 1.6, 1.0, (0.6, 0.58, 0.56, 1))
    look = {'desert': ('#f0c890', '#e8b080', '#7a7ab0', (1.0, 0.7, 0.42), '#f0d0a0', 0.9),
            'frozen': ('#f8e0e8', '#f2b8c8', '#6a80b8', (1.0, 0.62, 0.62), '#f4e4f0', 3.5),
            'volcanic': ('#c88878', '#d07858', '#3a3a5a', (1.0, 0.5, 0.35), '#b8a0a0', 3.0)}.get(
        land, ('#e8dcea', '#f2c2a8', '#6a7aa8', (1.0, 0.62, 0.46), '#f0c8c0', 3.5))
    fog_col, bg0, bg1, sun_col, haze, dens = look
    K.fog_box((7, 0, 0.0), (44, 34, 0.8), dens * (0.25 if land == 'coast' else 1.0), lin(fog_col), 0.0, clumps=0.7, fade_z=(-0.05, 0.3))
    K.fog_box((7, 0, 1.2), (60, 40, 4.0), 0.03, lin(haze), 0.5)
    R.sky_world(2, 260, strength=0.55, camera_bg=[lin(bg0), lin(bg1)])
    K.sun(-80, 4, 4.5, sun_col, 1.0)
    K.camera((4.2, 0.2, 0), -96, 21, 11.5, lens=42)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.15)


i = int(CID)
K.reset()
if 31 <= i <= 40:
    storm_scene(LANDS[i - 31])
else:
    dawn_scene(LANDS[i - 41])
