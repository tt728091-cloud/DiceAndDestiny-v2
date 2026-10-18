"""Set 2: twenty strongly different map facelift concepts. Usage:
blender -b --factory-startup -P concepts2.py -- <concept_id> <out.png> [preview]
"""
import sys, os, math, random
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import bpy, bmesh
import hexkit as K
import real as R
from hexkit import lin, Ctx
from mathutils import Vector, Matrix

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
        wall=lin('#c4b89e'), roofs=[lin('#8a7448'), lin('#7a6440'), lin('#6e5a3a'), lin('#94804e')], roof_dark=lin('#2c2c32'),
        stone=lin('#8a8478'), door=lin('#1c1511'), window=lin('#18181c'), canvas=lin('#a49676'),
        back=(lin('#1a1c22'), lin('#22252c')), table=lin('#2a1e16'), beam=lin('#2a1d15'),
        bone=lin('#d6cbb0'),
    )
    p.update(over)
    return p


GRASS = dict(count={'plains': 2600, 'hills': 2200, 'forest': 900, 'marsh': 1400, 'mountain': 400, 'lake': 900}, children=4,
             length=0.024, cols=[lin('#46522a'), lin('#5e6834'), lin('#76743e'), lin('#535c2c')], radius=0.0012)

BASE2 = dict(gap=0.03, thickness=0.35, bevel=0.02, density=1.45, rivers=True, fields=True, sea_trans=0.6, real=True,
             res=36, smooth=True, detail=0.022, token_scale=1.5, site_scale=1.45, tree_scale=1.1, dressing=True,
             roof='thatch', conifer_mix=0.65, peak_h=1.0, snow_z=0.62)


def setup(pal, **kw):
    s = dict(BASE2)
    s['pal'] = pal
    s.update(kw)
    Ctx.style = s
    K.build_materials()
    R.extra_materials()
    K.build_meshes()
    if s.get('real'):
        R.build_meshes_real()
        R.build_real_extras()
        R.install(True)


def finish():
    R.apply_overlays()


# ---------------------------------------------------------------------------

def c11():
    """Overcast Marches: realistic, grey-green, wet and quiet."""
    setup(nature(), grass=GRASS, overlays={'ao': (0.06, 0.55, (0.25, 0.22, 0.2, 1)), 'desat': 0.25})
    K.build_region(slots=['start', 'east', 'south', 'west', 'northeast'], warlord=False)
    finish()
    R.sky_world(18, 200, strength=0.5)
    K.sun(-60, 35, 1.8, (0.95, 0.95, 1.0), 18.0)
    K.fog_box((1, 6, 0.6), (40, 16, 2.6), 0.035, lin('#a8acae'), 0.0)
    K.camera((0.8, -0.6, 0), -10, 44, 10.5, lens=50)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.1)


def c12():
    """Sunlit realism: a clear summer afternoon, aerial tilt-shift."""
    pal = nature(plains=(lin('#55622e'), lin('#6c6e3a')), forest=(lin('#2f4220'), lin('#3e4c26')),
                 conifer=[lin('#1e3420'), lin('#28402a'), lin('#2e4226')], broad=[lin('#3e5a22'), lin('#4e6a28'), lin('#5c702c'), lin('#3a5420')])
    g = dict(GRASS)
    g['cols'] = [lin('#4a5a26'), lin('#5e6a2e'), lin('#76763a'), lin('#52602a')]
    setup(pal, grass=g, gap=0.0, thickness=0.6, bevel=0.0, overlays={'ao': (0.06, 0.5, (0.3, 0.27, 0.22, 1)), 'desat': 0.12})
    K.build_region(sea_z=-0.33)
    finish()
    R.sky_world(42, 140, strength=0.45, dust=0.5)
    K.sun(-35, 45, 4.4, (1.0, 0.95, 0.86), 1.2)
    K.camera((1.2, -0.6, 0), 10, 36, 9.0, lens=70, dof={'fstop': 0.9, 'offset': 0.0})
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=-0.1)


def emit_mat(name, col, k):
    return K.m_emit(name, col, k)


def scatter_air(mesh, n, box_min, box_max, seed=1, scale=(0.6, 1.4), lights=0, light_e=1.0, light_col=(0.5, 1.0, 0.6)):
    rnd = random.Random(seed)
    out = []
    for i in range(n):
        p = Vector([rnd.uniform(a, b) for a, b in zip(box_min, box_max)])
        out.append(K.add(mesh, p, rnd.uniform(0, 6.28), rnd.uniform(*scale)))
        if i < lights:
            R.point(p, light_e, light_col, 0.02)
    return out


def chimney_smoke(sites=('village', 'town', 'monastery'), color=(0.55, 0.55, 0.55, 1), density=1.2, height=1.0):
    for hx in Ctx.hexes.values():
        if hx.site in sites:
            R.smoke(hx.ground(0.15, 0.1) + Vector((0, 0, 0.12)), 0.12, height, density, color)


def fires_at(cells, scale=2.0, light=30.0, smoke=True):
    for (q, r) in cells:
        hx = Ctx.hexes.get((q, r))
        if not hx:
            continue
        for (x, y) in ((0.12, 0.2), (-0.22, -0.08), (0.26, -0.2)):
            g = hx.ground(x * 1.45, y * 1.45)
            K.add(Ctx.meshes['fire'], g + Vector((0, 0, 0.04)), 0.0, scale)
            R.point(g + Vector((0, 0, 0.16)), light, (1.0, 0.45, 0.18), 0.08)
        if smoke:
            R.smoke(hx.ground(0, 0) + Vector((0.25, 0.5, 0.2)), 0.4, 3.0, 2.2, (0.05, 0.045, 0.045, 1))


def c13():
    """Blood Moon: crimson moonlight, black silhouettes, burning homes."""
    pal = nature(water=lin('#4a0e0c'), marsh_water=lin('#2e0e0c'), sea=lin('#220807'), seabed=lin('#050101'))
    setup(pal, grass=GRASS, window_glow=10.0, flame_k=60.0, light_mult=2.5,
          overlays={'ao': (0.06, 0.6, (0.2, 0.15, 0.15, 1)), 'desat': 0.6},
          pieces={'enemy': K.m_flat('pe', lin('#0c0c0e'), 0.3, metal=0.7)})
    K.build_region(slots=['start', 'east', 'northeast', 'south', 'far_east'], warlord=False, hero_off=(0.25, -0.2))
    finish()
    fires_at([(-1, 1), (3, -1)])
    moon = emit_mat('moon', lin('#ff4a2e'), 9.0)
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=32, v_segments=16, radius=3.0)
    K.add(K.to_mesh(bm, 'moon', [moon], smooth=True), (2.0, 55.0, 8.0))
    K.sun(-6, 16, 2.6, (1.0, 0.2, 0.12), 2.0)
    K.world(lin('#300808'), 0.7, camera_bg=[lin('#3a0a08'), lin('#0a0202')])
    K.fog_box((2, 14, 1.0), (60, 40, 4.0), 0.02, lin('#c04030'), 0.3)
    K.camera((1.0, 0.5, 0), 0, 21, 11.0, lens=40)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.9, samples=192)


def c14():
    """Winter of the Dead: snowbound, frozen lakes, bare trees, low cold sun."""
    ice = K.m_flat('ice', lin('#c9d8e2'), 0.22, spec=0.6)
    pal = nature(conifer=[lin('#1a2620'), lin('#1f2c24'), lin('#24302a')], broad=[lin('#3a3a30'), lin('#45433a')])
    setup(pal, grass=None, dead_mix=0.35, conifer_mix=0.85, window_glow=6.0, roof='slate', snow_z=0.3,
          overlays={'ao': (0.05, 0.5, (0.3, 0.32, 0.38, 1)), 'snow': (lin('#eef2f6'), 0.42), 'desat': 0.35})
    Ctx.mats['water'] = ice
    Ctx.mats['marsh_water'] = ice
    Ctx.mats['sea'] = K.m_water('sea', lin('#2a3a44'), trans=0.4)
    K.build_region(slots=['start', 'east', 'northeast', 'south'], warlord=False, hero_at=(2, -1), hero_off=(0.1, -0.1))
    finish()
    chimney_smoke(color=(0.7, 0.72, 0.75, 1), density=1.0, height=1.3)
    bm = bmesh.new()
    bmesh.ops.create_icosphere(bm, subdivisions=1, radius=0.007)
    flake = K.to_mesh(bm, 'flake', [K.m_flat('flake', (1, 1, 1, 1), 0.5)])
    scatter_air(flake, 2600, (-1.5, -3.5, 0.1), (7.5, 4.5, 3.5), seed=4)
    R.sky_world(8, 160, strength=0.55)
    K.sun(160, 11, 3.2, (1.0, 0.88, 0.72), 1.5)
    K.fog_box((3, 2, 0.7), (40, 30, 3.0), 0.03, lin('#cdd8e4'), 0.1)
    K.camera((2.6, 0.6, 0), 12, 38, 9.5, lens=45)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.15)


def c15():
    """Necromancer's Blight: the Regent's rot spreads west, green witchlight in a violet fog."""
    pal = nature(water=lin('#3c5a2a'), marsh_water=lin('#2e3f22'), sea=lin('#1e2a26'))
    setup(pal, grass=GRASS, dead_mix=0.3, window_glow=4.0, bones=True, wisp=lin('#8dff9a'), wisp_k=40.0,
          overlays={'ao': (0.06, 0.6, (0.22, 0.2, 0.25, 1)), 'blight': ((12.5, -0.5), 6.5, lin('#9a7ab8')), 'desat': 0.1})
    wm = K.m_water('water_blight', lin('#1c2a18'), trans=0.15, rough=0.04)
    wm['no_overlay'] = True
    Ctx.mats['water'] = wm
    K.build_region(slots=['far_east', 'strand', 'reach', 'northeast', 'east', 'start'], hero_at=(2, 0), hero_off=(0.0, -0.1))
    finish()
    scatter_air(Ctx.meshes['wisp'], 70, (6.0, -6.0, 0.15), (15.0, 3.0, 0.9), seed=8, lights=14, light_e=3.0, light_col=(0.45, 1.0, 0.5))
    for hx in Ctx.hexes.values():
        if hx.center.x > 8.0 and hx.terrain in ('plains', 'hills'):
            for i in range(6):
                p = Vector((hx.rnd.uniform(-0.5, 0.5), hx.rnd.uniform(-0.5, 0.3)))
                K.add(Ctx.meshes['headstone'], hx.ground(p.x, p.y) - Vector((0, 0, 0.004)), hx.rnd.uniform(-0.4, 0.4), 1.4)
    K.world(lin('#3a3048'), 0.9, camera_bg=[lin('#3a2c48'), lin('#120e18')])
    K.sun(-30, 30, 2.6, (0.9, 0.95, 1.0), 4.0)
    K.fog_box((13, 2, 0.15), (12, 12, 0.7), 0.5, lin('#7aa070'), 0.0, clumps=0.9, fade_z=(-0.1, 0.3))
    K.fog_box((8, 4, 1.0), (40, 26, 4.0), 0.02, lin('#7a6a9a'), 0.2)
    K.camera((7.6, -1.0, 0), 0, 44, 14.0, lens=45)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.7, samples=192)


def c16():
    """The Drowned Kingdom: the map lies under the sea; caustics, kelp and god rays."""
    pal = nature(plains=(lin('#4e6050'), lin('#627058')), forest=(lin('#2e3e34'), lin('#3a4a3a')), hills=(lin('#5e6450'), lin('#6c6c56')))
    setup(pal, grass=None, dressing=True, window_glow=0.0,
          overlays={'ao': (0.06, 0.6, (0.15, 0.25, 0.28, 1)), 'tint': (0.7, 0.92, 1.0, 1), 'caustics': (2.6, 0.12, lin('#9ff0e0'))})
    K.build_region(slots=['start', 'east', 'south', 'northeast', 'west'], warlord=False, sea=False)
    finish()
    sb = K.m_flat('seabed2', lin('#3a4a40'), 1.0)
    R.plane((1, 0, -Ctx.style['thickness'] - 0.01), 60, sb)
    scatter_air(Ctx.meshes['kelp'], 0, (0, 0, 0), (1, 1, 1))
    rnd = random.Random(3)
    for hx in Ctx.hexes.values():
        for i in range(5 if hx.terrain in ('marsh', 'lake', 'plains') else 2):
            p = Vector((rnd.uniform(-0.6, 0.6), rnd.uniform(-0.6, 0.6)))
            if K.edge_dist(p, 0.95) > 0.08:
                K.add(Ctx.meshes['kelp'], hx.ground(p.x, p.y), rnd.uniform(0, 6), rnd.uniform(1.4, 2.6))
    bm = bmesh.new()
    bmesh.ops.create_icosphere(bm, subdivisions=2, radius=0.01)
    bub = K.to_mesh(bm, 'bubble', [K.m_flat('bubble', (0.9, 1, 1, 1), 0.02, spec=1.0)], smooth=True)
    scatter_air(bub, 400, (-3, -5, 0.2), (6, 4, 3.0), seed=9, scale=(0.4, 1.6))
    for (x, y) in ((-2.5, 3.0), (1.0, 4.0), (4.0, 2.5)):
        R.spot((x, y, 9.0), (x + 0.5, y - 3.0, 0), 9000.0, (0.7, 1.0, 0.95), angle=14, blend=0.4, size=0.3)
    K.world(lin('#0e2a30'), 0.6, camera_bg=[lin('#1e5a62'), lin('#06161a')])
    K.sun(-20, 70, 2.0, (0.7, 1.0, 0.95), 6.0)
    fog = K.fog_box((1, 0, 2.0), (40, 30, 5.0), 0.045, lin('#4aa8a8'), 0.5)
    K.camera((0.6, -0.6, 0), -8, 34, 9.5, lens=40)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.9, samples=192)


def c17():
    """Storm Coast: Greywharf in a gale, heaving sea, lightning on the horizon."""
    pal = nature(plains=(lin('#4a5240'), lin('#5c6048')), wall=lin('#9a958a'))
    setup(pal, grass=GRASS, window_glow=12.0, roof='slate', flame_k=40.0, light_mult=2.0,
          overlays={'ao': (0.06, 0.6, (0.2, 0.22, 0.25, 1)), 'desat': 0.4, 'tint': (0.85, 0.9, 0.96, 1)})
    K.build_region(slots=['strand', 'reach', 'far_east'], sea=False)
    finish()
    R.ocean(-0.16, R.m_ocean('oc', lin('#0e1a1e'), lin('#2a4a50'), lin('#c8d4d6')), size=2.6, spatial=18, scale=0.5, chop=1.6, wind=18.0, res=12, foam=0.55, loc=(10.0, -2.0))
    bolt = emit_mat('bolt', lin('#d8e8ff'), 60.0)
    rnd = random.Random(2)
    pts = [Vector((17.0, 20.0, 9.0))]
    for i in range(10):
        pts.append(pts[-1] + Vector((rnd.uniform(-0.8, 0.8), rnd.uniform(-0.3, 0.3), -0.95)))
    cu = bpy.data.curves.new('bolt', 'CURVE')
    cu.dimensions = '3D'
    cu.bevel_depth = 0.05
    sp = cu.splines.new('POLY')
    sp.points.add(len(pts) - 1)
    for i, q in enumerate(pts):
        sp.points[i].co = (q.x, q.y, q.z, 1)
    cu.materials.append(bolt)
    ob = bpy.data.objects.new('bolt', cu)
    K.link(ob)
    R.point(Vector((17, 18, 6)), 30000.0, (0.75, 0.82, 1.0), 2.0)
    K.world(lin('#3a4248'), 0.6, camera_bg=[lin('#5a6268'), lin('#1a1e22')])
    K.sun(40, 20, 0.9, (0.8, 0.85, 0.95), 15.0)
    K.fog_box((12, 4, 1.0), (40, 30, 4.0), 0.045, lin('#8a9298'), 0.2)
    K.camera((12.4, -3.2, 0.25), -22, 17, 6.5, lens=32)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.6, samples=192)


def c18():
    """Autumn Reaping: rust and gold woods, harvested fields, smoke in a low amber sun."""
    pal = nature(plains=(lin('#6e6a34'), lin('#8a7a40')), forest=(lin('#4a3a22'), lin('#5a4428')),
                 broad=[lin('#a8501a'), lin('#8a2a14'), lin('#c08a26'), lin('#7a4a1a'), lin('#9a6a20')],
                 field_cols=[lin('#c0a050'), lin('#a88a40'), lin('#8a6a34'), lin('#6a5a2e')])
    g = dict(GRASS)
    g['cols'] = [lin('#7a6a30'), lin('#94823c'), lin('#a88e44'), lin('#6a5a2a')]
    setup(pal, grass=g, conifer_mix=0.3, window_glow=3.0, overlays={'ao': (0.06, 0.55, (0.3, 0.22, 0.15, 1))})
    K.build_region(slots=['start', 'south', 'west', 'east'], warlord=False, hero_off=(0.3, -0.35))
    finish()
    for hx in Ctx.hexes.values():
        if hx.terrain == 'plains' and not hx.site:
            for i in range(4):
                p = Vector((hx.rnd.uniform(-0.5, 0.5), hx.rnd.uniform(-0.5, 0.5)))
                if K.edge_dist(p, 0.95) > 0.1 and hx.river_dist(hx.center.x + p.x, hx.center.y + p.y) > 0.2:
                    K.add(Ctx.meshes['haystack'], hx.ground(p.x, p.y), 0, hx.rnd.uniform(1.2, 1.7))
    chimney_smoke(color=(0.62, 0.6, 0.58, 1), density=1.1, height=1.4)
    R.sky_world(5, 250, strength=0.5, dust=2.0)
    K.sun(-72, 8, 4.2, (1.0, 0.66, 0.38), 1.2)
    K.fog_box((-1, 0, 0.5), (40, 30, 2.4), 0.03, lin('#e0b07a'), 0.4)
    K.camera((-0.7, -0.9, 0), 24, 34, 6.8, lens=50, dof={'fstop': 2.8, 'offset': 0.0})
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.1)


def c19():
    """Salt and Bone: bleached salt flats, turquoise brine, the ribs of drowned giants."""
    pal = nature(plains=(lin('#d6d0c0'), lin('#e6e1d4')), hills=(lin('#c4baa2'), lin('#d4cab4')), forest=(lin('#9a9480'), lin('#aaa48e')),
                 marsh=(lin('#b6b4a4'), lin('#c4c0ae')), mtn_base=(lin('#b0a894'), lin('#c0b8a4')), mtn_rock=(lin('#9a9284'), lin('#b8b0a2')),
                 hill_rock=lin('#a49a88'), mud=lin('#9a907c'), bank=lin('#c8c0ac'), sand=lin('#e0dccc'), deep=lin('#1e6a6a'),
                 water=lin('#3ec0b4'), marsh_water=lin('#5ab8a8'), sea=lin('#6ccabe'), seabed=lin('#bfe6dc'),
                 strata=[(0.0, lin('#8a8270')), (0.33, lin('#c4bca8')), (0.58, lin('#a89e8a')), (0.84, lin('#d8d2c2'))],
                 broad=[lin('#6a6a50'), lin('#7a7656')], conifer=[lin('#4a5040'), lin('#555a48')],
                 field_cols=[lin('#cfc6ae'), lin('#bdb49c')], wall=lin('#e2dccc'), bone=lin('#efe8d6'))
    setup(pal, grass=None, dead_mix=0.75, conifer_mix=0.4, snow=False, fields=False, bones=True,
          overlays={'ao': (0.05, 0.5, (0.45, 0.42, 0.38, 1)), 'cracks': (9.0, 0.7)})
    K.build_region(slots=['start', 'east', 'south', 'west', 'northeast', 'far_east'], warlord=False, sea_z=-0.12)
    finish()
    for (q, r, rot) in ((1, 1, 0.4), (-2, 3, 1.6), (3, 1, 2.6), (-3, 2, 0.9)):
        hx = Ctx.hexes.get((q, r))
        if hx:
            K.add(Ctx.meshes['ribs'], hx.ground(0.1, -0.2) - Vector((0, 0, 0.03)), rot, 5.5)
            K.add(Ctx.meshes['skull'], hx.ground(-0.4, -0.1), rot, 7.0)
    R.sky_world(62, 120, strength=0.7, dust=3.0)
    K.sun(-20, 64, 6.0, (1.0, 0.97, 0.92), 0.8)
    K.fog_box((2, 4, 0.6), (40, 20, 2.6), 0.03, lin('#f4f0e6'), 0.0)
    K.camera((1.8, -1.0, 0), 0, 64, 14.5, lens=50)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=-0.35)


def c20():
    """Valley Fog at Dawn: mist pools in the lowlands, peaks and keep towers rise into first light."""
    pal = nature(plains=(lin('#4e5a36'), lin('#62653e')))
    setup(pal, grass=GRASS, gap=0.0, thickness=0.6, bevel=0.0, peak_h=1.25, window_glow=5.0,
          overlays={'ao': (0.06, 0.5, (0.25, 0.25, 0.3, 1))})
    K.build_region(slots=['start', 'east', 'northeast', 'south', 'far_east', 'reach'], warlord=True, sea_z=-0.33)
    finish()
    K.fog_box((5, 0, 0.0), (40, 30, 0.8), 3.5, lin('#e8dcea'), 0.0, clumps=0.7, fade_z=(-0.05, 0.3))
    K.fog_box((5, 8, 1.2), (60, 30, 4.0), 0.03, lin('#f0c8c0'), 0.5)
    R.sky_world(2, 260, strength=0.55, camera_bg=[lin('#f2c2a8'), lin('#6a7aa8')])
    K.sun(-80, 4, 4.5, (1.0, 0.62, 0.46), 1.0)
    K.camera((4.2, 0.2, 0), -96, 21, 11.5, lens=42)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.15)


# ---------------------------------------------------------------------------
# Material themes

CATS = {
    'terrain': ['plains', 'forest', 'hills', 'marsh', 'lake', 'mountain'],
    'side': ['side'],
    'water': ['water', 'marsh_water', 'sea'],
    'foliage': ['leaf_conifer', 'leaf_broad', 'reed', 'cattail', 'lily', 'kelp', 'hay', 'field', 'field_soil', 'furrow', 'grass_hair'],
    'wood': ['trunk', 'deadwood', 'beam', 'door', 'canvas'],
    'build': ['wall', 'roof', 'roof_dark', 'stone', 'window', 'rock', 'bone', 'mud_mound', 'wax'],
    'mtn': ['mtn'],
    'hero': ['piece_hero', 'piece_trim', 'piece_flag'],
    'enemy': ['piece_enemy', 'piece_crown'],
}


def theme(cat_mats, names=None):
    names = names or {}
    lookup = {}
    for cat, lst in CATS.items():
        for n in lst:
            if cat in cat_mats:
                lookup[n] = cat_mats[cat]
    lookup.update(names)
    R.retheme(lambda n: lookup.get(n.split('.')[0]))


def _p(name):
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    nt = m.node_tree
    return m, nt, nt.nodes['Principled BSDF']


def m_granite(name, base, dark, rough=0.6, speck=0.5, scale=140.0):
    m, nt, p = _p(name)
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    vo = nt.nodes.new('ShaderNodeTexVoronoi')
    vo.inputs['Scale'].default_value = scale
    nt.links.new(geo.outputs['Position'], vo.inputs['Vector'])
    n = nt.nodes.new('ShaderNodeTexNoise')
    n.inputs['Scale'].default_value = 3.0
    nt.links.new(geo.outputs['Position'], n.inputs['Vector'])
    c = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.35, 0.65, 0.0, 0.4), base, dark)
    c = K._mix(nt, K._maprange(nt, vo.outputs['Distance'], 0.05, 0.25, speck, 0.0), c, (0.02, 0.02, 0.02, 1))
    nt.links.new(c, p.inputs['Base Color'])
    K._in(p, 'Roughness', rough)
    K._bump(nt, p, 90.0, 0.25, 0.004)
    return m


def m_bronze(name, base=None, patina=None, rough=0.35):
    base = base or lin('#a8743a')
    patina = patina or lin('#4f8a76')
    m, nt, p = _p(name)
    ao = nt.nodes.new('ShaderNodeAmbientOcclusion')
    ao.samples = 8
    ao.inputs['Distance'].default_value = 0.08
    n = K._noise(nt, 18.0, 5.0)
    f = nt.nodes.new('ShaderNodeMath')
    f.operation = 'MULTIPLY'
    inv = K._maprange(nt, ao.outputs['AO'], 0.45, 0.95, 1.0, 0.0)
    nt.links.new(inv, f.inputs[0])
    nt.links.new(K._maprange(nt, n.outputs['Fac'], 0.3, 0.6, 0.3, 1.0), f.inputs[1])
    c = K._mix(nt, f.outputs['Value'], base, patina)
    nt.links.new(c, p.inputs['Base Color'])
    met = K._maprange(nt, f.outputs['Value'], 0.0, 1.0, 1.0, 0.0)
    nt.links.new(met, p.inputs['Metallic'])
    ro = K._maprange(nt, f.outputs['Value'], 0.0, 1.0, rough, 0.85)
    nt.links.new(ro, p.inputs['Roughness'])
    K._bump(nt, p, 60.0, 0.2, 0.004)
    m['no_overlay'] = True
    return m


def m_obsidian(name, ember=lin('#ff5a14'), k=6.0, scale=6.0):
    m, nt, p = _p(name)
    K._in(p, 'Base Color', lin('#0a0a0c'))
    K._in(p, 'Roughness', 0.12)
    K._in(p, ('Specular IOR Level', 'Specular'), 0.7)
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    vo = nt.nodes.new('ShaderNodeTexVoronoi')
    vo.feature = 'DISTANCE_TO_EDGE'
    vo.inputs['Scale'].default_value = scale
    n = K._noise(nt, 2.0, 3.0)
    av = nt.nodes.new('ShaderNodeVectorMath')
    av.operation = 'MULTIPLY_ADD'
    nt.links.new(n.outputs['Color'], av.inputs[0])
    av.inputs[1].default_value = (0.15, 0.15, 0.15)
    nt.links.new(geo.outputs['Position'], av.inputs[2])
    nt.links.new(av.outputs['Vector'], vo.inputs['Vector'])
    line = K._maprange(nt, vo.outputs['Distance'], 0.0, 0.03, 1.0, 0.0)
    mask = K._maprange(nt, K._noise(nt, 1.3, 2.0).outputs['Fac'], 0.56, 0.64, 0.0, 1.0)
    mul = nt.nodes.new('ShaderNodeMath')
    mul.operation = 'MULTIPLY'
    nt.links.new(line, mul.inputs[0])
    nt.links.new(mask, mul.inputs[1])
    em = K._mix(nt, mul.outputs['Value'], (0, 0, 0, 1), ember)
    nt.links.new(em, p.inputs['Emission Color'])
    p.inputs['Emission Strength'].default_value = k
    m['no_overlay'] = True
    return m


def m_lava(name, k=9.0):
    m, nt, p = _p(name)
    n = K._noise(nt, 5.0, 6.0)
    ramp = nt.nodes.new('ShaderNodeValToRGB')
    els = ramp.color_ramp.elements
    els[0].position, els[0].color = 0.5, (0.0, 0.0, 0.0, 1)
    els[1].position, els[1].color = 0.82, lin('#ffb030')
    e = els.new(0.64)
    e.color = lin('#e0300a')
    nt.links.new(n.outputs['Fac'], ramp.inputs['Fac'])
    nt.links.new(ramp.outputs['Color'], p.inputs['Emission Color'])
    p.inputs['Emission Strength'].default_value = k
    K._in(p, 'Base Color', lin('#1a0a04'))
    K._in(p, 'Roughness', 0.5)
    m['no_overlay'] = True
    return m


def m_glass(name, col, k=3.0, scale=7.0, lead=0.04):
    m, nt, p = _p(name)
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    vo = nt.nodes.new('ShaderNodeTexVoronoi')
    vo.inputs['Scale'].default_value = scale
    nt.links.new(geo.outputs['Position'], vo.inputs['Vector'])
    ve = nt.nodes.new('ShaderNodeTexVoronoi')
    ve.feature = 'DISTANCE_TO_EDGE'
    ve.inputs['Scale'].default_value = scale
    nt.links.new(geo.outputs['Position'], ve.inputs['Vector'])
    hsv = nt.nodes.new('ShaderNodeHueSaturation')
    hsv.inputs['Color'].default_value = col
    hue = K._maprange(nt, vo.outputs['Color'], 0.0, 1.0, 0.46, 0.54)
    nt.links.new(hue, hsv.inputs['Hue'])
    val = K._maprange(nt, vo.outputs['Distance'], 0.0, 1.0, 1.25, 0.6)
    nt.links.new(val, hsv.inputs['Value'])
    n = K._noise(nt, 40.0, 4.0)
    grain = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.3, 0.7, 0.0, 0.35), hsv.outputs['Color'], (0, 0, 0, 1))
    leadm = K._maprange(nt, ve.outputs['Distance'], 0.0, lead, 1.0, 0.0)
    em = K._mix(nt, leadm, grain, (0, 0, 0, 1))
    nt.links.new(em, p.inputs['Emission Color'])
    p.inputs['Emission Strength'].default_value = k
    K._in(p, 'Base Color', (0.01, 0.01, 0.01, 1))
    K._in(p, 'Roughness', 0.08)
    K._in(p, ('Specular IOR Level', 'Specular'), 0.8)
    m['no_overlay'] = True
    return m


def m_bonemat(name, col, dirt=0.5):
    m, nt, p = _p(name)
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    vo = nt.nodes.new('ShaderNodeTexVoronoi')
    vo.inputs['Scale'].default_value = 220.0
    nt.links.new(geo.outputs['Position'], vo.inputs['Vector'])
    n = K._noise(nt, 6.0, 5.0)
    c = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.35, 0.7, 0.0, dirt), col, lin('#6a5638'))
    c = K._mix(nt, K._maprange(nt, vo.outputs['Distance'], 0.0, 0.12, 0.5, 0.0), c, lin('#4a3a28'))
    nt.links.new(c, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.55)
    K._in(p, ('Subsurface Weight', 'Subsurface'), 0.15)
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = 0.35
    b.inputs['Distance'].default_value = 0.003
    nt.links.new(vo.outputs['Distance'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


def m_parchment(name, col, stain=0.45):
    m, nt, p = _p(name)
    n = K._noise(nt, 2.2, 6.0)
    n2 = K._noise(nt, 0.6, 3.0)
    c = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.45, 0.75, 0.0, stain), col, lin('#8a6a3a'))
    c = K._mix(nt, K._maprange(nt, n2.outputs['Fac'], 0.55, 0.7, 0.0, stain * 0.6), c, lin('#6a4a24'))
    nt.links.new(c, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.9)
    K._bump(nt, p, 120.0, 0.15, 0.002)
    return m


def m_ghost(name, col, k=4.0, alpha=0.08):
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.remove(nt.nodes['Principled BSDF'])
    lw = nt.nodes.new('ShaderNodeLayerWeight')
    lw.inputs['Blend'].default_value = 0.35
    em = nt.nodes.new('ShaderNodeEmission')
    em.inputs['Color'].default_value = col
    st = K._maprange(nt, lw.outputs['Facing'], 0.0, 1.0, k * 0.08, k)
    nt.links.new(st, em.inputs['Strength'])
    tr = nt.nodes.new('ShaderNodeBsdfTransparent')
    mix = nt.nodes.new('ShaderNodeMixShader')
    fac = K._maprange(nt, lw.outputs['Facing'], 0.0, 1.0, alpha, 0.9)
    nt.links.new(fac, mix.inputs['Fac'])
    nt.links.new(tr.outputs['BSDF'], mix.inputs[1])
    nt.links.new(em.outputs['Emission'], mix.inputs[2])
    nt.links.new(mix.outputs['Shader'], nt.nodes['Material Output'].inputs['Surface'])
    m.blend_method = 'BLEND'
    return m


def m_paint(name, col, rough=0.85, metal=0.0, crinkle=0.0):
    m, nt, p = _p(name)
    n = K._noise(nt, 25.0, 4.0)
    c = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.3, 0.7, 0.0, 0.18), col, (col[0] * 0.6, col[1] * 0.6, col[2] * 0.6, 1))
    nt.links.new(c, p.inputs['Base Color'])
    K._in(p, 'Roughness', rough)
    K._in(p, 'Metallic', metal)
    K._bump(nt, p, 60.0 if not crinkle else 18.0, 0.15 + crinkle, 0.004)
    return m


def floor(mat, z=None, size=60, loc=(1, 0)):
    z = -Ctx.style['thickness'] - 0.002 if z is None else z
    return R.plane((loc[0], loc[1], z), size, mat)


def c21():
    """Carved Granite: the region as a sculpted stone relief, lit by one torch."""
    setup(nature(), grass=None, gap=0.012, bevel=0.012, thickness=0.4, roof='slate')
    K.build_region(slots=['start', 'east', 'northeast', 'south', 'west'], sea=False, warlord=False)
    gr = m_granite('granite', lin('#8e8a84'), lin('#6a6660'))
    grd = m_granite('granite_dark', lin('#4a4846'), lin('#3a3836'))
    pol = m_granite('granite_pol', lin('#141416'), lin('#0a0a0c'), rough=0.08, speck=0.25)
    theme({'terrain': gr, 'side': grd, 'water': pol, 'foliage': gr, 'wood': gr, 'build': gr, 'mtn': gr,
           'hero': K.m_flat('marble', lin('#e8e4dc'), 0.2, spec=0.6), 'enemy': pol},
          {'piece_trim': K.m_flat('gt', lin('#c9a35a'), 0.3, metal=1.0)})
    floor(grd)
    K.world((0.0, 0.0, 0.0, 1), 0.0)
    R.spot((-7.0, -4.0, 1.6), (1.0, 0.0, 0.0), 17000.0, (1.0, 0.62, 0.35), angle=70, blend=0.8, size=0.4)
    R.spot((8.0, 9.0, 4.0), (1.0, 0.0, 0.0), 6000.0, (0.55, 0.65, 1.0), angle=60, blend=0.9, size=1.0)
    K.camera((0.5, -0.6, 0), 0, 52, 11.5, lens=45)
    K.render(OUT, preview=PREVIEW, look='AgX - High Contrast', exposure=0.2)


def c22():
    """Bronze War Table: a cast-bronze map on an oak table, candles, dice and cards."""
    setup(nature(), grass=None, gap=0.02, bevel=0.015, thickness=0.18, roof='slate')
    K.build_region(slots=['start', 'east', 'northeast'], sea=False, warlord=False)
    br = m_bronze('bronze')
    gold = K.m_flat('polished', lin('#d8a850'), 0.12, metal=1.0)
    theme({'terrain': br, 'side': m_bronze('bronze_side', lin('#6a4a28'), lin('#3a5a4a'), 0.5), 'water': gold, 'foliage': br, 'wood': br,
           'build': br, 'mtn': br, 'hero': K.m_flat('silver', lin('#c8c8cc'), 0.2, metal=1.0), 'enemy': K.m_flat('iron', lin('#1e1e20'), 0.45, metal=1.0)})
    Ctx.mats['wax'] = K.m_flat('wax2', lin('#e6d8b8'), 0.5)
    floor(K.m_table('oak', lin('#5a3418')))
    for (x, y, h) in ((-3.3, -2.9, 0.55), (-2.8, -3.5, 0.38), (6.6, -2.6, 0.6), (-2.6, 3.4, 0.45)):
        R.candle(Vector((x, y, -0.18)), h, 0.07, light=7.0)
    ivory = K.m_flat('ivorydie', lin('#e6dcc4'), 0.35)
    for (x, y, rot) in ((-1.6, -3.9, 0.4), (-1.1, -4.2, 1.1), (-0.8, -3.6, 2.2)):
        R.die(Vector((x, y, -0.18)), 0.26, rot, ivory)
    card = K.m_flat('cardback', lin('#4a1414'), 0.6)
    for i in range(4):
        bm = bmesh.new()
        K.bm_box(bm, (0, 0, 0.002), (0.62, 0.88, 0.004), 0)
        K.add(K.to_mesh(bm, 'card', [card]), (4.0 + i * 0.22, -3.7 + i * 0.05, -0.18 + i * 0.004), 0.3 - i * 0.12)
    K.world(lin('#2a1c10'), 0.35)
    R.spot((-1.0, -5.0, 7.0), (1.5, 0.0, 0), 5200.0, (1.0, 0.8, 0.58), angle=60, blend=0.7, size=0.6)
    R.spot((7.0, 6.0, 4.0), (1.5, 0.0, 0), 1500.0, (0.7, 0.8, 1.0), angle=60, blend=0.9, size=1.0)
    K.camera((1.6, -0.6, 0), 8, 38, 9.6, lens=45, dof={'fstop': 2.4, 'offset': -0.6})
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.5, samples=192)


def c23():
    """Obsidian and Ember: black glass tiles with burning seams over a sea of lava."""
    setup(nature(), grass=None, gap=0.05, bevel=0.015, thickness=0.35, window_glow=20.0, flame_k=40.0, roof='slate')
    K.build_region(slots=['start', 'east', 'northeast', 'south', 'west', 'far_east'], sea=False, warlord=False)
    ob = m_obsidian('obsidian', k=5.0, scale=4.0)
    theme({'terrain': ob, 'side': m_obsidian('obs_side', k=3.0, scale=3.0), 'water': m_lava('lava'),
           'foliage': K.m_flat('char', lin('#141210'), 0.85), 'wood': K.m_flat('char2', lin('#1a1612'), 0.9),
           'build': K.m_flat('basalt', lin('#222024'), 0.6), 'mtn': m_obsidian('obs_mtn', k=4.0, scale=4.0),
           'hero': K.m_flat('goldp', lin('#d8a850'), 0.25, metal=1.0), 'enemy': K.m_flat('obsp', lin('#0a0a0c'), 0.1, spec=0.8)},
          {'window': K.m_emit('winlava', lin('#ff8a2a'), 18.0), 'water': m_lava('lava', k=3.5), 'marsh_water': m_lava('lava2', k=3.0)})
    floor(m_lava('lava_floor', k=0.9), z=-0.3)
    K.world(lin('#1a0804'), 0.4, camera_bg=[lin('#3a1206'), lin('#060202')])
    K.sun(-40, 40, 2.2, (1.0, 0.75, 0.6), 4.0)
    K.fog_box((1, 2, 0.8), (40, 30, 3.0), 0.008, lin('#ff9060'), 0.3)
    K.camera((0.8, -0.4, 0), -6, 44, 11.5, lens=45)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.0, samples=192)


def c24():
    """Stained Glass: every hex a glowing pane in lead; trees and towns as lead silhouettes."""
    setup(nature(), grass=None, gap=0.0, bevel=0.0, thickness=0.15, relief=0.2, real=True, dressing=False, fields=False)
    K.build_region(slots=['start', 'east', 'northeast', 'south', 'west'], sea=False, warlord=False)
    lead = K.m_flat('lead', lin('#1a1a1e'), 0.45, metal=0.8)
    glass = {'plains': m_glass('g_plains', lin('#9ab83a')), 'forest': m_glass('g_forest', lin('#1e8a4a')), 'hills': m_glass('g_hills', lin('#d89a2a')),
             'marsh': m_glass('g_marsh', lin('#6a8a2a')), 'lake': m_glass('g_lake', lin('#2a5ad8')), 'mountain': m_glass('g_mtnbase', lin('#8a7ab8')),
             'mtn': m_glass('g_mtn', lin('#c8c0e8'), k=2.5, scale=12.0), 'water': m_glass('g_water', lin('#2a4ae0'), k=3.5, scale=10.0)}
    names = dict(glass)
    names['water'] = glass['water']
    names['marsh_water'] = glass['water']
    theme({'side': lead, 'foliage': lead, 'wood': lead, 'build': lead, 'hero': K.m_flat('gp', lin('#e6c17c'), 0.25, metal=1.0),
           'enemy': K.m_emit('ruby', lin('#e0203a'), 4.0)}, names)
    for hx in Ctx.hexes.values():
        K.hex_ring(hx, hx.center, 0.955, 1.0, lead, z=0.006, n=10)
    floor(K.m_flat('floorstone', lin('#1a1816'), 0.8))
    K.world((0.0, 0.0, 0.0, 1), 0.0)
    K.sun(-30, 50, 0.6, (1.0, 0.95, 0.9), 3.0)
    K.camera((0.6, -0.4, 0), 0, 72, 11.0, lens=45)
    K.render(OUT, preview=PREVIEW, look='AgX - Punchy', exposure=-0.2)


def c25():
    """Ossuary: the map carved from yellowed bone, set among skulls and candles in a crypt."""
    setup(nature(), grass=None, gap=0.025, bevel=0.02, thickness=0.3, roof='slate', dead_mix=0.8, bones=True)
    K.build_region(slots=['start', 'east', 'northeast', 'south', 'west'], sea=False, warlord=False)
    bone = m_bonemat('bonemat', lin('#d8cbaa'))
    old = m_bonemat('bone_old', lin('#a89676'), dirt=0.7)
    theme({'terrain': bone, 'side': old, 'water': K.m_flat('blackblood', lin('#1a0404'), 0.05, spec=0.8), 'foliage': old, 'wood': old,
           'build': bone, 'mtn': bone, 'hero': K.m_flat('silver2', lin('#b8b8bc'), 0.25, metal=1.0), 'enemy': K.m_flat('ebony', lin('#0c0a0a'), 0.3)})
    Ctx.mats['wax'] = K.m_flat('wax3', lin('#d8cca8'), 0.5)
    flag = K.m_flat('flag', lin('#2a2424'), 0.5)
    fl = bpy.data.materials.new('flagstone')
    fl.use_nodes = True
    nt = fl.node_tree
    p = nt.nodes['Principled BSDF']
    br = nt.nodes.new('ShaderNodeTexBrick')
    br.inputs['Scale'].default_value = 0.9
    br.inputs['Color1'].default_value = lin('#3a3632')
    br.inputs['Color2'].default_value = lin('#2a2724')
    br.inputs['Mortar'].default_value = lin('#0e0c0a')
    nt.links.new(br.outputs['Color'], p.inputs['Base Color'])
    p.inputs['Roughness'].default_value = 0.85
    floor(fl)
    rnd = random.Random(4)
    for i in range(70):
        a = rnd.uniform(0, 6.283)
        d = rnd.uniform(6.2, 7.5)
        K.add(Ctx.meshes['skull'], (0.2 + math.cos(a) * d, -0.6 + math.sin(a) * d * 0.75, -0.3), rnd.uniform(0, 6.28), rnd.uniform(9, 13))
    for (x, y, h) in ((-3.6, -3.3, 0.8), (-3.1, -3.8, 0.55), (-4.0, -2.6, 0.45), (4.4, -3.6, 0.7), (5.0, -3.0, 0.5), (-3.8, 2.4, 0.9)):
        R.candle(Vector((x, y, -0.3)), h, 0.08, light=14.0)
    R.spot((-2.0, -4.0, 6.0), (0.5, -0.5, 0), 4200.0, (1.0, 0.72, 0.45), angle=55, blend=0.8, size=0.8)
    K.world(lin('#20160e'), 0.3)
    K.camera((0.3, -1.4, 0), -10, 38, 10.5, lens=45, dof={'fstop': 2.4, 'offset': -0.4})
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.8, samples=192)


def c26():
    """Living Parchment: the land rises out of an old map; ink, wax, quill and lamp."""
    setup(nature(), grass=None, gap=0.0, bevel=0.0, thickness=0.02, roof='slate', relief=0.8)
    K.build_region(slots=['start', 'east', 'northeast', 'south', 'west'], sea=False, warlord=False)
    pa = m_parchment('parch', lin('#dcc9a0'))
    ink = m_parchment('inkwash', lin('#7a8a90'), 0.3)
    theme({'terrain': pa, 'side': pa, 'water': ink, 'foliage': m_parchment('parch_g', lin('#b8b48a'), 0.3), 'wood': pa, 'build': pa,
           'mtn': pa, 'hero': K.m_flat('inkp', lin('#141210'), 0.2, spec=0.6), 'enemy': K.m_flat('waxp', lin('#7a1a14'), 0.3)},
          {'piece_flag': K.m_flat('pf', lin('#7a1a14'), 0.4), 'piece_trim': K.m_flat('pt', lin('#a88a4a'), 0.3, metal=1.0)})
    floor(m_parchment('sheet', lin('#d8c49a'), 0.55), z=-0.025, size=26, loc=(0.6, -0.6))
    floor(K.m_table('desk', lin('#3a2416')), z=-0.03)
    blk = K.m_flat('inkwell', lin('#0a0a0c'), 0.08, spec=0.8)
    bm = bmesh.new()
    K.bm_cone(bm, 24, 0.28, 0.22, 0.3, 0.0, 0)
    K.bm_cone(bm, 24, 0.12, 0.12, 0.08, 0.3, 0)
    K.add(K.to_mesh(bm, 'inkwell', [blk], smooth=True, auto=40), (-5.2, -3.8, -0.025))
    feather = K.m_flat('feather', lin('#e8e2d4'), 0.6)
    bm = bmesh.new()
    vs = [bm.verts.new(c) for c in [(0, 0, 0), (0.25, 0.9, 0), (0.0, 2.4, 0), (-0.22, 0.9, 0)]]
    bm.faces.new(vs)
    K.add(K.to_mesh(bm, 'quill', [feather]), (-4.9, -3.7, 0.3), 0.6, 1.0, tilt=(0.5, 0.2))
    seal = K.m_flat('sealr', lin('#7a1a14'), 0.3, spec=0.6)
    bm = bmesh.new()
    K.bm_cone(bm, 24, 0.32, 0.3, 0.06, 0.0, 0)
    K.add(K.to_mesh(bm, 'seal', [seal], smooth=True, auto=40), (5.6, -4.6, -0.025))
    R.area((-4, -6, 8), (0.5, -0.5, 0), 900.0, (1.0, 0.84, 0.66), 6.0)
    K.world(lin('#3a2a1a'), 0.35)
    K.camera((0.4, -1.0, 0), 4, 47, 11.0, lens=45)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.2,
             freestyle={'thickness': 1.2, 'color': (0.18, 0.11, 0.06), 'alpha': 0.9, 'crease': True})


def c27():
    """Woodcut: black ink on white, red only for the enemy."""
    setup(nature(), grass=None, gap=0.03, bevel=0.0, thickness=0.25, roof='slate')
    K.build_region(slots=['start', 'east', 'northeast', 'south', 'west'], sea=False, warlord=False)
    white = K.m_flat('white', (0.82, 0.8, 0.76, 1), 0.9)
    red = K.m_flat('woodred', lin('#b0221a'), 0.6)
    theme({'terrain': white, 'side': white, 'water': K.m_flat('ink_water', (0.55, 0.55, 0.53, 1), 0.9), 'foliage': white, 'wood': white,
           'build': white, 'mtn': white, 'hero': white, 'enemy': red}, {'piece_flag': red})
    floor(K.m_flat('paper', (0.85, 0.83, 0.78, 1), 0.95))
    K.world((1, 1, 1, 1), 0.35)
    K.sun(-45, 32, 4.0, (1, 1, 1), 1.0)
    K.camera((0.5, -0.6, 0), 0, 50, 11.0, lens=45)
    K.render(OUT, preview=PREVIEW, look='AgX - High Contrast', exposure=0.0,
             freestyle={'thickness': 2.0, 'color': (0.0, 0.0, 0.0), 'alpha': 1.0, 'crease': True})


def c28():
    """Spectral Scrying: the map as a vision, glowing in the dark of a seer's bowl."""
    setup(nature(), grass=None, gap=0.03, bevel=0.0, thickness=0.2, roof='slate')
    K.build_region(slots=['start', 'east', 'northeast', 'south', 'west', 'far_east'], sea=False, warlord=False)
    g = m_ghost('ghost', lin('#5ad8ff'), 2.2)
    gw = m_ghost('ghost_w', lin('#2a6aff'), 3.0, alpha=0.2)
    theme({'terrain': m_ghost('ghost_t', lin('#3ab8e8'), 1.2, alpha=0.05), 'side': g, 'water': gw, 'foliage': g, 'wood': g, 'build': m_ghost('ghost_b', lin('#a8f0ff'), 3.0),
           'mtn': g, 'hero': m_ghost('ghost_h', lin('#ffd27a'), 6.0, alpha=0.3), 'enemy': m_ghost('ghost_e', lin('#ff3a6a'), 6.0, alpha=0.3)})
    K.world((0, 0, 0, 1), 0.0)
    K.fog_box((1, 0, 0.5), (30, 24, 2.5), 0.02, lin('#3ab8e8'), 0.0)
    K.camera((0.6, -0.6, 0), 0, 44, 12.0, lens=45)
    K.render(OUT, preview=PREVIEW, look='AgX - Punchy', exposure=0.0,
             freestyle={'thickness': 0.9, 'color': (0.4, 0.85, 1.0), 'alpha': 0.55})


def c29():
    """Illuminated Manuscript: vellum, gold leaf, lapis and vermilion, ink-outlined."""
    setup(nature(), grass=None, gap=0.025, bevel=0.0, thickness=0.12, roof='slate', relief=0.7)
    K.build_region(slots=['start', 'east', 'northeast', 'south'], sea=False, warlord=False)
    gold = m_paint('goldleaf', lin('#d8a848'), 0.3, 1.0, crinkle=0.25)
    names = {'plains': m_paint('p_plains', lin('#8aa04a')), 'forest': m_paint('p_forest', lin('#2e6a3a')), 'hills': m_paint('p_hills', lin('#c89a4a')),
             'marsh': m_paint('p_marsh', lin('#6a7a3a')), 'lake': m_paint('p_lake', lin('#8aa04a')), 'mountain': m_paint('p_mtnb', lin('#b8b4a8')),
             'roof': m_paint('verm', lin('#c0321e')), 'roof_dark': m_paint('verm2', lin('#a02818')), 'wall': m_paint('lead_white', lin('#f0e8d8')),
             'mtn': m_paint('p_mtn', lin('#e8e4dc'), 0.6), 'leaf_conifer': m_paint('malachite', lin('#1e5a34')), 'leaf_broad': m_paint('verdigris', lin('#3a8a4a'))}
    lapis = m_paint('lapis', lin('#1e3aa0'), 0.5)
    names['water'] = lapis
    names['marsh_water'] = lapis
    theme({'side': gold, 'wood': m_paint('umber', lin('#5a3a22')), 'build': m_paint('p_stone', lin('#d8d0c0')), 'foliage': m_paint('sap', lin('#4a7a2a')),
           'hero': gold, 'enemy': m_paint('p_enemy', lin('#1a1a22'), 0.4)}, names)
    floor(m_parchment('vellum', lin('#ece0c4'), 0.25))
    K.world((1, 0.97, 0.92, 1), 1.0)
    K.sun(-30, 55, 2.2, (1, 0.97, 0.9), 6.0)
    K.camera((1.4, -0.8, 0), 0, 62, 14.5, lens=45)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.1,
             freestyle={'thickness': 1.3, 'color': (0.16, 0.1, 0.06), 'alpha': 0.85})


def c30():
    """Wargame Table: painted miniature terrain under studio light, shot macro."""
    pal = nature(plains=(lin('#5a6a2a'), lin('#6e7634')), side=None)
    g = dict(GRASS)
    g['cols'] = [lin('#5a7a28'), lin('#76922e'), lin('#a0a040'), lin('#c8b860')]
    g['count'] = {'plains': 5200, 'hills': 4400, 'forest': 1800, 'marsh': 2600, 'mountain': 800, 'lake': 1800}
    g['length'] = 0.028
    setup(pal, grass=g, gap=0.06, bevel=0.012, thickness=0.12, roof='thatch',
          pieces={'hero': K.m_flat('ph', lin('#d8d4cc'), 0.3, metal=1.0), 'flag': K.m_flat('pf2', lin('#b0281a'), 0.6),
                  'enemy': K.m_flat('pe2', lin('#26222a'), 0.4, metal=0.3)},
          overlays={'ao': (0.05, 0.45, (0.3, 0.27, 0.22, 1))})
    Ctx.mats['side'] = K.m_flat('mdf', lin('#3a2a1e'), 0.85)
    K.build_region(slots=['start', 'east', 'northeast', 'south'], sea=False, warlord=False, hero_at=(1, 0), hero_off=(0.15, -0.1))
    finish()
    felt = bpy.data.materials.new('felt')
    felt.use_nodes = True
    nt = felt.node_tree
    p = nt.nodes['Principled BSDF']
    p.inputs['Base Color'].default_value = lin('#1c1e22')
    p.inputs['Roughness'].default_value = 1.0
    K._bump(nt, p, 400.0, 0.3, 0.002)
    floor(felt)
    R.area((-5.0, -4.0, 5.0), (1.5, 0.5, 0), 1400.0, (1.0, 0.97, 0.92), 5.0)
    R.area((6.0, 7.0, 4.0), (1.5, 0.5, 0), 900.0, (0.8, 0.88, 1.0), 4.0)
    K.world(lin('#3a3c40'), 0.5)
    hx = Ctx.hexes[(1, 0)]
    tgt = hx.ground(0.3, -0.05) + Vector((0, 0, 0.08))
    K.camera(tuple(tgt), -20, 17, 3.3, lens=60, dof={'fstop': 1.0, 'offset': 0.0})
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.2, samples=192)


def c99():
    setup(nature(), grass=GRASS, overlays={'ao': (0.06, 0.55, (0.25, 0.22, 0.2, 1))})
    K.build_region(slots=['start', 'east'], warlord=False)
    finish()
    R.sky_world(30, 200, strength=0.5)
    K.sun(-50, 35, 3.0, (1.0, 0.95, 0.88), 3.0)
    K.camera((0.6, -0.4, 0), -10, 38, 3.2, lens=50)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast')


CONCEPTS = {k[1:]: v for k, v in globals().items() if k.startswith('c') and k[1:].isdigit()}

K.reset()
CONCEPTS[CID]()
K.dump_screen(OUT.replace('.png', '.json'))
