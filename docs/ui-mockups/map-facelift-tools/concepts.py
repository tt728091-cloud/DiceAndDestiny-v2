"""Ten Dice and Destiny map facelift concepts. Usage:
blender -b --factory-startup -P concepts.py -- <concept_id> <out.png> [preview]
"""
import sys, os, math
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import importlib
import hexkit as K
importlib.reload(K)
from hexkit import lin, Ctx
from mathutils import Vector, Matrix

argv = sys.argv[sys.argv.index('--') + 1:]
CID, OUT = argv[0], argv[1]
PREVIEW = len(argv) > 2 and argv[2] == 'preview'


def bright():
    return dict(
        plains=(lin('#93ad4f'), lin('#b5be62')), forest=(lin('#4c6a33'), lin('#5d7838')),
        hills=(lin('#a3a056'), lin('#c19a5c')), hill_rock=lin('#8a7a66'),
        marsh=(lin('#6d7a48'), lin('#5b6a40')), mud=lin('#6a5a3e'), bank=lin('#b59a68'), sand=lin('#d9c58f'), deep=lin('#2d5866'),
        mtn_base=(lin('#8c8676'), lin('#a09682')), mtn_rock=(lin('#6a6560'), lin('#8a8278')), snow=lin('#f4f6f8'),
        strata=[(0.0, lin('#6e5a48')), (0.33, lin('#a88b62')), (0.58, lin('#8b6b47')), (0.84, lin('#4a3a28'))],
        water=lin('#79c0d6'), marsh_water=lin('#6e8c78'), sea=lin('#74b6cc'), seabed=lin('#3d8099'),
        trunk=lin('#5a4030'), deadwood=lin('#5e554c'),
        conifer=[lin('#2f5a3a'), lin('#3a6a3c'), lin('#2d5034')], broad=[lin('#5f8a3a'), lin('#7a9c43'), lin('#4f7a34'), lin('#8aa848')],
        rock=lin('#9a958c'), reed=lin('#8f8f4c'), cattail=lin('#5a3a22'), lily=lin('#5f8a3a'),
        field_cols=[lin('#e0b84a'), lin('#d4a83c'), lin('#9ab84e'), lin('#c8a050')], field_soil=lin('#7a5a3a'),
        wall=lin('#ebe0c6'), roofs=[lin('#a8453a'), lin('#b8743a'), lin('#8a5a3a'), lin('#c9a25a')], roof_dark=lin('#4a4a52'),
        stone=lin('#a39d90'), door=lin('#3a2a1e'), window=lin('#2a3a48'), canvas=lin('#d8c8a0'),
        back=(lin('#2b3242'), lin('#363e52')), table=lin('#6a4428'),
    )


def grim():
    """Salt Marches palette pulled toward the game's dark watercolor battle art."""
    return dict(
        plains=(lin('#7f8650'), lin('#9c9566')), forest=(lin('#3c4630'), lin('#4a4f36')),
        hills=(lin('#857252'), lin('#9a8562')), hill_rock=lin('#6e6458'),
        marsh=(lin('#4d5640'), lin('#5c573e')), mud=lin('#4e4434'), bank=lin('#7d6c50'), sand=lin('#b3a682'), deep=lin('#1e3238'),
        mtn_base=(lin('#6e6a62'), lin('#7e776c')), mtn_rock=(lin('#5e5a58'), lin('#7d7770')), snow=lin('#d9dce0'),
        strata=[(0.0, lin('#3a3530'), ), (0.33, lin('#6b5c48')), (0.58, lin('#544636')), (0.84, lin('#2e261e'))],
        water=lin('#5c7f86'), marsh_water=lin('#4d5442'), sea=lin('#3a5c66'), seabed=lin('#0c1a1e'),
        trunk=lin('#3e3228'), deadwood=lin('#4a4540'),
        conifer=[lin('#26352c'), lin('#2e3e30'), lin('#34402e')], broad=[lin('#4e5a34'), lin('#5c6238'), lin('#475230')],
        rock=lin('#7d7870'), reed=lin('#7d7a4a'), cattail=lin('#3e2a1c'), lily=lin('#4e5a34'),
        field_cols=[lin('#a8904e'), lin('#9a8046'), lin('#6a7040'), lin('#8a7a4a')], field_soil=lin('#4a3a2a'),
        wall=lin('#c4b89c'), roofs=[lin('#6e3a32'), lin('#7a5a40'), lin('#5a4a3e'), lin('#8a7448')], roof_dark=lin('#33333a'),
        stone=lin('#8d877c'), door=lin('#221a14'), window=lin('#1e2228'), canvas=lin('#b8aa88'),
        back=(lin('#1f2430'), lin('#2a2f3c')), table=lin('#3a2618'),
    )


def pastel():
    return dict(
        plains=(lin('#a9c779'), lin('#c3d38a')), forest=(lin('#6f9a5a'), lin('#7fa864')),
        hills=(lin('#d2b47a'), lin('#e0c48e')), hill_rock=lin('#b9a68e'),
        marsh=(lin('#8fae86'), lin('#9fb88e')), mud=lin('#a08a68'), bank=lin('#e0c896'), sand=lin('#f0dfb0'), deep=lin('#5b9cb4'),
        mtn_base=(lin('#b8b0a6'), lin('#c7beb2')), mtn_rock=(lin('#a69ea0'), lin('#c0b8b6')), snow=lin('#ffffff'),
        strata=[(0.0, lin('#c9a98a')), (0.33, lin('#e2c7a2')), (0.58, lin('#d4b48e')), (0.84, lin('#a8846a'))],
        water=lin('#8fd2e4'), marsh_water=lin('#93b8a2'), sea=lin('#8ccbe0'), seabed=lin('#4a8fae'),
        trunk=lin('#8a6a52'), deadwood=lin('#8a7e74'),
        conifer=[lin('#4f8a62'), lin('#5c9a6a'), lin('#4a7e5c')], broad=[lin('#8cbc5c'), lin('#a2c868'), lin('#7aae58'), lin('#e0a86a')],
        rock=lin('#c4bcb4'), reed=lin('#b6b878'), cattail=lin('#8a6a4a'), lily=lin('#8cbc5c'),
        field_cols=[lin('#f2d27a'), lin('#e8c068'), lin('#b6d27a'), lin('#d8b07e')], field_soil=lin('#b89070'),
        wall=lin('#fff6e4'), roofs=[lin('#e07a6a'), lin('#f0a46a'), lin('#7aa4d4'), lin('#e8c070')], roof_dark=lin('#7a7e9a'),
        stone=lin('#d6d0c6'), door=lin('#7a5a46'), window=lin('#7aa4c4'), canvas=lin('#fff0d0'),
        back=(lin('#7a86a8'), lin('#8a96b8')), table=lin('#e8dcc6'),
    )


def ash():
    """Ink and ash: the battle screens' charcoal, bone and sickly-green palette."""
    return dict(
        plains=(lin('#7e7b6c'), lin('#9a947f')), forest=(lin('#45463a'), lin('#52523e')),
        hills=(lin('#77705f'), lin('#8f8573')), hill_rock=lin('#55524c'),
        marsh=(lin('#474c3d'), lin('#545643')), mud=lin('#3a352c'), bank=lin('#5e584a'), sand=lin('#8a8270'), deep=lin('#161c1c'),
        mtn_base=(lin('#5a5854'), lin('#6c6862')), mtn_rock=(lin('#45434a'), lin('#64605c')), snow=lin('#c9c8c2'),
        strata=[(0.0, lin('#141210')), (0.33, lin('#36302a')), (0.58, lin('#28231e')), (0.84, lin('#1a1714'))],
        water=lin('#56645e'), marsh_water=lin('#3d4537'), sea=lin('#2a3030'), seabed=lin('#0a0c0c'),
        trunk=lin('#2e2722'), deadwood=lin('#3c3a36'),
        conifer=[lin('#2a3128'), lin('#323a2e'), lin('#3a4232')], broad=[lin('#4a5034'), lin('#555a3a'), lin('#40472e')],
        rock=lin('#6c6a64'), reed=lin('#6c6a46'), cattail=lin('#2e2218'), lily=lin('#4a5034'),
        field_cols=[lin('#8c7c4e'), lin('#7c6e48'), lin('#5c6040'), lin('#6e6044')], field_soil=lin('#2e2620'),
        wall=lin('#a69d8a'), roofs=[lin('#3e2e2a'), lin('#4a3c34'), lin('#5a4636'), lin('#4e2a26')], roof_dark=lin('#26262c'),
        stone=lin('#7d786e'), door=lin('#161210'), window=lin('#1a1814'), canvas=lin('#8a8270'),
        back=(lin('#1a1c22'), lin('#22252c')), table=lin('#1a1412'),
    )


def muted(pal, k=0.35, warm=0.04):
    out = {}
    for key, v in pal.items():
        def m(c):
            if not (isinstance(c, tuple) and len(c) == 4 and all(isinstance(x, float) for x in c)):
                return c
            l = 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]
            return (lerp(c[0], l, k) * (1 + warm), lerp(c[1], l, k), lerp(c[2], l, k) * (1 - warm), 1.0)
        if isinstance(v, list):
            out[key] = [(p[0], m(p[1])) if isinstance(p, tuple) and len(p) == 2 and isinstance(p[0], float) else m(p) for p in v]
        elif isinstance(v, tuple) and len(v) == 2 and isinstance(v[0], tuple):
            out[key] = (m(v[0]), m(v[1]))
        else:
            out[key] = m(v)
    return out


def lerp(a, b, t):
    return a + (b - a) * t


BASE = dict(gap=0.035, thickness=0.3, bevel=0.03, density=1.0, rivers=True, fields=True, sea_trans=0.5)


def style(pal, **kw):
    s = dict(BASE)
    s['pal'] = pal
    s.update(kw)
    Ctx.style = s
    K.build_materials()
    K.build_meshes()


def c01():
    """Tabletop classic: separated chunky tiles, island in a sea, full region."""
    style(bright())
    K.build_region()
    K.camera((4.4, -0.75, 0), 0, 52, 29.5, lens=50)
    K.sun(-40, 38, 4.2, (1.0, 0.93, 0.82), 2.5)
    K.world(lin('#9db8d8'), 0.75)
    K.render(OUT, preview=PREVIEW, look='AgX - Punchy')


def c02():
    """Miniature diorama: low angle, tilt-shift focus on the pawn, wooden table, golden hour."""
    style(bright(), gap=0.05, thickness=0.22, bevel=0.035)
    K.build_region(slots=['start', 'east', 'south', 'west', 'northeast'], warlord=False, sea=False, table=True, hero_off=(0.25, -0.3))
    hero = Ctx.hero
    K.camera((hero.x - 0.2, hero.y + 0.6, 0.1), -14, 31, 7.2, lens=65, dof={'fstop': 0.55, 'offset': -0.55})
    K.sun(-62, 17, 4.8, (1.0, 0.78, 0.55), 1.5)
    K.world(lin('#a8b4c8'), 0.55)
    K.render(OUT, preview=PREVIEW, look='AgX - Punchy', exposure=0.15)


def c03():
    """Seamless realm: one continuous landscape, faint hex grid, cliffs only at the coast."""
    pal = bright()
    pal['plains'] = (lin('#8fa651'), lin('#a8ae62'))
    pal['hills'] = (lin('#9c9656'), lin('#b0905a'))
    style(pal, gap=0.0, thickness=0.55, bevel=0.0)
    K.build_region(sea_z=-0.32)
    grid = K.m_alpha('grid', lin('#2a2418'), 0.45)
    for hx in Ctx.hexes.values():
        K.hex_ring(hx, hx.center, 0.975, 1.0, grid, z=0.012, n=10)
    K.cloud_shadows(z=9.0, scale=0.16, cover=0.52)
    K.camera((2.6, -0.2, 0), 8, 42, 13.5, lens=35)
    K.sun(-35, 46, 4.6, (1.0, 0.93, 0.8), 2.0)
    K.world(lin('#a9c0dc'), 0.8, camera_bg=[lin('#dfe9f2'), lin('#9fbad6')])
    K.fog_box((4, 6, 0.4), (40, 14, 2.4), 0.05, lin('#dfe9f2'))
    K.render(OUT, preview=PREVIEW, look='AgX - Punchy')


def c04():
    """Ink and ash: the game's dark watercolor battle language applied to 3D tiles."""
    style(ash(), snow_z=0.74, dead_mix=0.35, conifer_mix=0.75, window_glow=6.0, flame_k=30.0, light_mult=1.6, grotto_k=8.0, grotto_light=8.0)
    K.build_region(sea=False)
    K.lanterns(energy=1.4, per_site=3)
    K.fog_box((5, 0, -0.55), (60, 50, 1.3), 1.6, lin('#8c8880'), 0.0, clumps=0.55, fade_z=(-0.6, 0.15))
    K.camera((5.4, -0.9, 0), 0, 50, 22.5, lens=50)
    K.sun(-50, 30, 1.9, (0.95, 0.9, 0.82), 10.0)
    K.world(lin('#6c6c72'), 0.55, camera_bg=[lin('#0e0d10'), lin('#1c1b1e')])
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.35,
             freestyle={'thickness': 1.5, 'color': (0.05, 0.04, 0.035), 'alpha': 0.9})


def c05():
    """Night march: moonlight, lantern-lit sites, the Drowned Regent's eyes in the dark."""
    pal = grim()
    style(pal, window_glow=14.0, flame_k=40.0, light_mult=2.5, grotto_k=14.0, grotto_light=14.0, snow_z=0.7, dead_mix=0.15)
    K.build_region(slots=['far_east', 'strand', 'reach', 'northeast', 'east'], hero_at=(5, 2), hero_off=(0.1, -0.2))
    K.lanterns(energy=2.2, per_site=3)
    K.camera((10.9, -1.7, 0), -4, 50, 14.0, lens=45)
    K.sun(30, 30, 1.4, (0.55, 0.66, 1.0), 1.0)
    K.world(lin('#24325a'), 0.5)
    K.fog_box((9, -1, 0.3), (30, 22, 1.8), 0.05, lin('#7f90b8'))
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=1.35, samples=256)


def c06():
    """Fog of the unexplored: face-down tiles, one flipping into place, the tile stack waiting."""
    style(muted(bright(), 0.32), ring_k=1.4, ring_col="#ffd27a")
    built = K.build_region(slots=['start', 'east', 'northeast', 'south', 'far_east'], warlord=False, hidden=('far_east',), hero_at=(2, 0), hero_off=(0.0, -0.1))
    c = K.hex_world(4, 2)
    M = Matrix.Translation(c + Vector((0.15, 0.35, 1.3))) @ Matrix.Rotation(math.radians(30), 4, Vector((1, -0.45, 0)).normalized()) @ Matrix.Translation(-c)
    K.transform_objects(built['far_east'], M)
    sea_z = -Ctx.style['thickness'] * 0.55
    for (dq, dr), *_ in K.TILES['stillwater']:
        hc = K.hex_world(4 + dq, 2 + dr)
        K.hex_ring(None, hc, 0.86, 0.95, Ctx.mats['ring_move'], z=sea_z + 0.01, flat=True)
    for sid in ('strand', 'reach'):
        K.face_down_flower(K.SLOTS[sid][0])
    T = Ctx.style['thickness']
    for k in range(4):
        objs = K.face_down_flower((5, -1), lift=k * (T + 0.015))
        K.transform_objects(objs, Matrix.Translation((0.25 * k, 0.12 * k, 0)))
    K.camera((7.0, -1.0, 0), 0, 50, 19.0, lens=50)
    K.sun(-55, 30, 3.6, (1.0, 0.86, 0.68), 2.5)
    K.world(lin('#a4aec0'), 0.75)
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast')


def c07():
    """Terraced columns: each terrain at its own height on deep stratified pillars above the clouds."""
    style(bright(), gap=0.025, thickness=1.6, bevel=0.025, rivers=False, peak_h=0.8)
    elev = {'lake': -0.16, 'marsh': -0.08, 'plains': 0.0, 'forest': 0.06, 'hills': 0.2, 'mountain': 0.42}
    K.build_region(slots=['start', 'east', 'south', 'west', 'northeast'], warlord=False, elevation=elev, sea=False, rivers=[])
    K.fog_box((0, -1, -1.45), (40, 30, 1.6), 1.2, lin('#ffffff'), 0.0)
    K.camera((1.2, -1.4, 0), 18, 36, 12.5, lens=45)
    K.sun(-55, 34, 4.4, (1.0, 0.92, 0.8), 2.0)
    K.world(lin('#a4c4e8'), 0.85, camera_bg=[lin('#f4efe6'), lin('#8fb4dc')])
    K.render(OUT, preview=PREVIEW, look='AgX - Punchy')


def c08():
    """Isometric pastel: true orthographic isometric, soft toy palette."""
    style(pastel(), gap=0.03, thickness=0.26, bevel=0.03, snow_z=0.6)
    K.build_region(slots=['start', 'east', 'south', 'west', 'northeast', 'far_east'], warlord=False, sea_z=-0.12)
    K.camera((2.4, -1.0, 0), 45, 35.264, 40.0, ortho=13.5)
    K.sun(-30, 55, 3.6, (1.0, 0.96, 0.9), 6.0)
    K.world(lin('#e8eef6'), 1.1)
    K.render(OUT, preview=PREVIEW, look='AgX - Base Contrast', exposure=0.25)


def c09():
    """Tile kit sheet: every terrain, river, site and token as a separate model."""
    style(bright(), site_scale=1.5)
    Ctx.rivers = []
    sx, sy = 2.25, 2.25
    rows = [
        [('plains', None, 'Plains'), ('hills', None, 'Hills'), ('forest', None, 'Forest'), ('marsh', None, 'Marsh'), ('lake', None, 'Lake'), ('mountain', None, 'Mountain')],
        [('plains', 'river', 'River (straight)'), ('forest', 'river_bend', 'River (bend)'), ('plains', 'village', 'Village'), ('hills', 'keep', 'Keep'), ('plains', 'town', 'Town'), ('plains', 'monastery', 'Monastery')],
        [('plains', 'cairn', 'Ruins: cairn'), ('forest', 'chapel', 'Ruins: chapel'), ('hills', 'grotto', 'Grotto'), ('plains', 'shrine', 'Shrine (event)'), ('plains', 'hermit', 'Camp (event)'), ('plains', 'tokens', 'Pawn, enemy, Warlord')],
    ]
    rivers = {'river': [(-0.87, 0.0), (-0.3, 0.12), (0.3, -0.12), (0.87, 0.0)],
              'river_bend': [(-0.75, 0.43), (-0.2, 0.05), (0.0, -0.4), (0.0, -0.87)]}
    tok = None
    for ri, row in enumerate(rows):
        for ci, (terr, site, label) in enumerate(row):
            x = (ci - 2.5) * sx + (0.0 if ri % 2 == 0 else 0.0)
            y = (1 - ri) * sy
            riv = rivers.get(site)
            real_site = None if site in (None, 'river', 'river_bend', 'tokens') else site
            Ctx.style['fields'] = site != 'tokens'
            hx = K.build_hex_at((x, y), terr, real_site, river=riv, key=(500 + ri * 10 + ci, 0), label=label, decorate_it=site != 'tokens')
            if site == 'tokens':
                tok = hx
    ts = Ctx.style['token_scale'] if 'token_scale' in Ctx.style else 1.6
    K.add(Ctx.meshes['hero'], tok.ground(-0.42, 0.0), 0.4, ts)
    K.add(Ctx.meshes['enemy'], tok.ground(0.02, -0.05), 0.2, ts)
    K.add(Ctx.meshes['warlord'], tok.ground(0.45, 0.05), 0.0, ts * 1.1)
    bm = K.bmesh.new()
    K.bmesh.ops.create_grid(bm, x_segments=1, y_segments=1, size=40)
    K.add(K.to_mesh(bm, 'floor', [K.m_flat('floor', lin('#2a2c33'), 0.8)]), (0, 0, -Ctx.style['thickness'] - 0.001))
    K.camera((0.0, -0.45, 0), 0, 54, 21.0, lens=50)
    K.sun(-40, 40, 4.2, (1.0, 0.93, 0.82), 2.5)
    K.world(lin('#9db8d8'), 0.6)
    K.render(OUT, preview=PREVIEW, look='AgX - Punchy')
    K.dump_labels(OUT.replace('.png', '_labels.json'))


def c10():
    """Gameplay readability: reachable hexes, path, threat, labels over the 3D board."""
    style(bright(), ring_k=2.2, ring_col='#f0c860')
    K.build_region(hero_off=(0.1, -0.15))
    hero_hex = (0, 0)
    reach = [(1, 0), (0, 1), (-1, 1), (1, -1), (0, -1), (1, 1), (2, 0), (-1, 2), (0, 2), (2, -1), (2, -2), (-2, 2)]
    for h in reach:
        hx = Ctx.hexes.get(h)
        if hx and hx.terrain not in ('lake', 'mountain'):
            K.hex_ring(hx, hx.center, 0.83, 0.88, Ctx.mats['ring_move'], z=0.02)
    tgt = Ctx.hexes[(3, 0)]
    K.hex_ring(tgt, tgt.center, 0.74, 0.88, Ctx.mats['ring_path'], z=0.025)
    for h in ((2, 1),):
        hx = Ctx.hexes[h]
        K.hex_ring(hx, hx.center, 0.8, 0.9, Ctx.mats['ring_enemy'], z=0.02)
    pts = [Ctx.hero, K.hex_world(1, 0) + Vector((0.0, 0.15, 0)), K.hex_world(2, 0) + Vector((0.0, 0.0, 0)), K.hex_world(3, 0) + Vector((-0.25, -0.1, 0))]
    K.path_dots(pts, Ctx.mats['ring_path'], spacing=0.11, r=0.028)
    K.camera((1.6, -0.9, 0), 0, 55, 11.5, lens=45)
    K.sun(-40, 38, 4.0, (1.0, 0.93, 0.82), 2.5)
    K.world(lin('#9db8d8'), 0.75)
    K.render(OUT, preview=PREVIEW, look='AgX - Punchy')
    extra = {'path': [K.project(K.hex_world(*h) + Vector((0, 0, 0.05))) for h in ((1, 0), (2, 0), (3, 0))],
             'enemy': K.project(Ctx.hexes[(2, 1)].ground(0, 0) + Vector((0, 0, 0.55))),
             'target': K.project(Ctx.hexes[(3, 0)].ground(0, 0) + Vector((0, 0, 0.6)))}
    K.dump_screen(OUT.replace('.png', '.json'), extra)


CONCEPTS = {'01': c01, '02': c02, '03': c03, '04': c04, '05': c05, '06': c06, '07': c07, '08': c08, '09': c09, '10': c10}


K.reset()
CONCEPTS[CID]()
if CID != '10':
    K.dump_screen(OUT.replace('.png', '.json'))
