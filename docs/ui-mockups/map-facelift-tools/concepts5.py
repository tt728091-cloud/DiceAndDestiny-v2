"""Set 5: dune canyons (red-sand crescent dunes meet the red canyons). Usage:
blender -b --factory-startup -P concepts5.py -- <id 61-70> <out.png> [preview]
61-65 storm, 66-70 dawn; the same five layouts in both styles.
"""
import sys, os, math, random
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import bpy, bmesh
import hexkit as K
import real as R
import biomes as B
import desert as D
from hexkit import lin, Ctx, smoothstep, lerp, catmull, seg_dist
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


def bolt(x, y, z, col, energy, glossy=False, seed=0, light_y=None):
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
    fl = R.point(Vector((x, light_y if light_y is not None else y - 4, z * 0.6)), energy, (0.8, 0.85, 1.0), 3.0)
    fl.visible_glossy = glossy
    fl.visible_camera = False


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



# ---------------------------------------------------------------------------
# Set 5: dune canyons. Red-sand crescent dunes flowing between mesas/buttes.

B.ALIAS.update({'redune': 'plains'})
_prev_terrain_h = D.terrain_h


def terrain_h(hx, x, y):
    if hx.terrain == 'redune':
        return 0.06 + D.dune(x, y)
    return _prev_terrain_h(hx, x, y)


D.terrain_h = terrain_h
_prev_height = K.Hex.height


def height(self, wx, wy):
    z = _prev_height(self, wx, wy)
    if not getattr(self, 'dz', False):
        return z
    S = Ctx.style
    grid = getattr(Ctx, 'mesa_grid', None)
    if grid:
        q, r = D.axial_round(wx, wy)
        add_ = 0.0
        for (dq, dr) in D.NB7:
            for (mx, my, mr, mh, wide) in grid.get((q + dq, r + dr), ()):
                d = math.hypot(wx - mx, wy - my)
                if d < mr + 0.5:
                    add_ = max(add_, S.get('apron', 0.16) * (1.0 - smoothstep(mr * 0.6, mr + 0.5, d)))
        z += add_
    g = S.get('gorge')
    if g and self.segs:
        depth, inner, outer = g
        rd = self.river_dist(wx, wy) + 0.02 * noise.noise(Vector((wx * 4, wy * 4, 8.8)))
        z = min(z, lerp(-depth, 1.0, smoothstep(inner, outer, rd)))
    return z


K.Hex.height = height


def rock_walls(mat, rock3, varnish, lo=0.42, hi=0.62):
    """Banded sandstone wherever the ground turns into a cliff (gorge walls)."""
    nt = mat.node_tree
    p = nt.nodes['Principled BSDF']
    src = p.inputs['Base Color'].links[0].from_socket
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    sep = nt.nodes.new('ShaderNodeSeparateXYZ')
    nt.links.new(geo.outputs['Normal'], sep.inputs['Vector'])
    fac = K._maprange(nt, sep.outputs['Z'], lo, hi, 1.0, 0.0)
    w = nt.nodes.new('ShaderNodeTexWave')
    w.bands_direction = 'Z'
    w.inputs['Scale'].default_value = 3.2
    w.inputs['Distortion'].default_value = 3.0
    w.inputs['Detail'].default_value = 5.0
    nt.links.new(geo.outputs['Position'], w.inputs['Vector'])
    ramp = nt.nodes.new('ShaderNodeValToRGB')
    els = ramp.color_ramp.elements
    els[0].position, els[0].color = 0.0, rock3[2]
    els[1].position, els[1].color = 1.0, rock3[0]
    e = els.new(0.5)
    e.color = rock3[1]
    nt.links.new(w.outputs['Fac'], ramp.inputs['Fac'])
    vs = nt.nodes.new('ShaderNodeTexNoise')
    vs.inputs['Scale'].default_value = 3.0
    mp = nt.nodes.new('ShaderNodeMapping')
    mp.inputs['Scale'].default_value = (7.0, 7.0, 0.35)
    nt.links.new(geo.outputs['Position'], mp.inputs['Vector'])
    nt.links.new(mp.outputs['Vector'], vs.inputs['Vector'])
    rc = K._mix(nt, K._maprange(nt, vs.outputs['Fac'], 0.6, 0.74, 0.0, 0.5), ramp.outputs['Color'], varnish)
    col = K._mix(nt, fac, src, rc)
    nt.links.new(col, p.inputs['Base Color'])


def canyon_materials(pal):
    M = Ctx.mats
    rd = pal['reddune']
    M['redune'] = D.m_dune('redune', rd[0], rd[1], rd[2], pal['rust'], Ctx.style.get('wind', -1.05), cracks=pal['mud_cracks'])
    rock_walls(M['redune'], pal['rock3'], pal['varnish'])
    M['flood'] = K.m_water('flood2', lin('#6a3e1c'), trans=0.0, rough=0.22, ripple=1.2)
    M['flood']['no_overlay'] = True


def mesa_at(hx, x, y, r, h, rnd):
    zs = [D.ground(x + r * 0.9 * math.cos(a), y + r * 0.9 * math.sin(a)).z for a in [i / 8 * 6.283 for i in range(8)]]
    z0 = min(zs + [D.ground(x, y).z]) - 0.02
    ob = B.mesa_obj(hx, x - hx.center.x, y - hx.center.y, r, h, rnd)
    ob.location.z = z0
    return z0 + h


def plan_mesas(clear, explicit=(), p=0.3, seed=7):
    """Choose mesa/butte footprints before the terrain is built so sand can ramp against them."""
    rnd = random.Random(seed)
    mesas = list(explicit)
    for hx in Ctx.hexes.values():
        if hx.terrain != 'redune' or hx.site or rnd.random() > p:
            continue
        a, d = rnd.uniform(0, 6.28), rnd.uniform(0.0, 0.25)
        x, y = hx.center.x + math.cos(a) * d, hx.center.y + math.sin(a) * d
        wide = rnd.random() < 0.55
        r = rnd.uniform(0.48, 0.7) if wide else rnd.uniform(0.24, 0.36)
        h = rnd.uniform(0.62, 0.9) if wide else rnd.uniform(0.95, 1.5)
        if hx.segs and hx.river_dist(x, y) < r + 0.3:
            continue
        if any(math.hypot(x - cx, y - cy) < r + cr for (cx, cy, cr) in clear):
            continue
        if any(math.hypot(x - m[0], y - m[1]) < r + m[2] + 0.15 for m in mesas):
            continue
        mesas.append((x, y, r, h, wide))
    grid = {}
    for m in mesas:
        grid.setdefault(D.axial_round(m[0], m[1]), []).append(m)
    Ctx.mesa_grid = grid
    Ctx.mesas = mesas
    return mesas


def deco_redune(hx):
    rnd = hx.rnd
    D.shrubs(hx, 2)
    if rnd.random() < 0.12:
        for (x, y) in D.scatter_xy(hx, 1, 0.25, avoid=0.2, seed=7):
            D.place(Ctx.meshes[f'hoodoo{rnd.randrange(3)}'], x, y, rnd.uniform(0, 6), rnd.uniform(1.4, 2.2), sink=0.03)
    if rnd.random() < 0.08:
        for (x, y) in D.scatter_xy(hx, 1, 0.3, avoid=0.35, seed=8):
            D.place(Ctx.meshes[f'arch{rnd.randrange(2)}'], x, y, rnd.uniform(0, 6), rnd.uniform(1.6, 2.2), sink=0.02)
    if rnd.random() < 0.3:
        D.stones(hx, 4, (0.4, 1.0), mat_col=Ctx.style['pal']['rock3'][1])
    if hx.site == 'mesafort':
        top = mesa_at(hx, hx.center.x, hx.center.y, 0.62, 0.62, random.Random(hx.seed))
        me = Ctx.meshes
        c = hx.center
        K.add(me['dtower'], Vector((c.x, c.y, top - 0.01)), 0.0, 1.3, (1, 1, 1, 1))
        for (dx, dy, rot) in ((0, -0.2, 0), (0, 0.2, 0), (-0.2, 0, 1.5708), (0.2, 0, 1.5708)):
            K.add(me['dwall'], Vector((c.x + dx, c.y + dy, top - 0.02)), rot, (0.4, 1.1, 1.0), (1, 1, 1, 1))
        for (dx, dy) in ((-0.2, -0.2), (0.2, -0.2), (0.2, 0.2), (-0.2, 0.2)):
            K.add(me['dtower'], Vector((c.x + dx, c.y + dy, top - 0.02)), 0.0, 0.7, (1, 1, 1, 1))
        R.point(Vector((c.x, c.y - 0.25, top + 0.15)), 6.0 * Ctx.style.get('light_mult', 1.0), (1.0, 0.6, 0.3), 0.05)
    else:
        D.site(hx)


B.CUSTOM['redune'] = deco_redune


def bridge(near, half=0.32, n=22):
    poly = Ctx.rivers[0]
    i = min(range(1, len(poly) - 1), key=lambda k: (poly[k].x - near[0]) ** 2 + (poly[k].y - near[1]) ** 2)
    p, tan = poly[i], (poly[i + 1] - poly[i - 1]).normalized()
    nrm = Vector((-tan.y, tan.x))
    a, b = p - nrm * half, p + nrm * half
    za, zb = D.ground(a.x, a.y).z, D.ground(b.x, b.y).z
    wood = Ctx.mats['deadwood']
    bm = bmesh.new()
    rails = ([], [])
    for k in range(n):
        t = k / (n - 1)
        q = a.lerp(b, t)
        z = lerp(za, zb, t) - 0.06 * math.sin(math.pi * t) + 0.005
        K.bm_box(bm, (q.x, q.y, z), (0.012, 0.07, 0.004), 0, rot=math.atan2(nrm.y, nrm.x))
        for j, s in enumerate((-1, 1)):
            rails[j].append(Vector((q.x, q.y, z + 0.035)) + Vector((tan.x, tan.y, 0)) * 0.034 * s)
    for rl in rails:
        D.tube(bm, rl, [0.0022] * len(rl), 5, 0)
    for end, z in ((a, za), (b, zb)):
        for s in (-1, 1):
            q = end + tan * 0.034 * s
            K.bm_cone(bm, 5, 0.004, 0.004, 0.06, z - 0.01, 0, x=q.x, y=q.y)
    K.add(K.to_mesh(bm, 'bridge', [wood]), (0, 0, 0))
    return p


def flood_sheets(z):
    rr = 1.0 - Ctx.style['gap']
    for hx in Ctx.hexes.values():
        if hx.segs:
            bm = bmesh.new()
            bmesh.ops.create_circle(bm, cap_ends=True, segments=6, radius=rr * 0.985)
            K.add(K.to_mesh(bm, 'flood', [Ctx.mats['flood']]), (hx.center.x, hx.center.y, hx.top + z))


def build_canyon(cells, rivers, hero, enemies, warlord, explicit=(), p=0.3, clear_extra=()):
    S = Ctx.style
    Ctx.rivers = [catmull([Vector((x, y, 0)) for (x, y) in r]) for r in rivers]
    for (q, r), (t, st, en) in cells.items():
        hx = B.make_hex(q, r, t, st, en)
        hx.dz = True
        Ctx.hexes[(q, r)] = hx
    for hx in Ctx.hexes.values():
        for poly in Ctx.rivers:
            for a, b in zip(poly, poly[1:]):
                if seg_dist(Vector((hx.center.x, hx.center.y)), a, b) < 1.3:
                    hx.segs.append((a, b))
    clear = [(x, y, 0.35) for (x, y) in ([hero] if hero and hero != 'crest' else []) + list(enemies) + ([warlord] if warlord else [])]
    clear += list(clear_extra)
    plan_mesas(clear, explicit, p)
    for hx in Ctx.hexes.values():
        K.tile_mesh(hx)
        B.decorate(hx)
    for (x, y, r, h, wide) in Ctx.mesas:
        hx = K.nearest_hex(Vector((x, y)))
        mesa_at(hx, x, y, r, h, random.Random(int(x * 1000 + y * 7)))
    ts = S.get('token_scale', 1.5)
    if hero and hero != 'crest':
        g = D.ground(*hero)
        K.add(Ctx.meshes['hero'], g - Vector((0, 0, 0.01)), 0.4, ts)
        Ctx.hero = g
    for (x, y) in enemies:
        D.place(Ctx.meshes['enemy'], x, y, random.uniform(-0.3, 0.3), ts, sink=0.01)
    if warlord:
        D.place(Ctx.meshes['warlord'], warlord[0], warlord[1], 0.0, ts * 1.1, sink=0.01)


def redune_tf(rock_p=-9.0):
    return lambda w, rnd: 'redune'


# ---------------------------------------------------------------------------
# The five dune-canyon layouts

def lay_sea():
    cells = cells_from(XR, YR, redune_tf(), 11)
    setsite(cells, -3.6, 4.6, site='temple')
    setsite(cells, -4.8, 9.4, site='ruins')
    setsite(cells, 4.4, 6.4, site='mesafort')
    setsite(cells, 6.2, 2.0, site='skeleton')
    river = [(1.5, 16.0), (-1.2, 10.5), (1.4, 6.2), (-0.8, 2.2), (0.6, -2.5), (-0.8, -6.0)]
    cara = dict(path=[(-6.5, -1.6), (-4.0, 0.6), (-2.6, 2.6), (-1.6, 4.6)], camels=7, wagons=2, spacing=0.17, offset=1.2)
    clear = [(x, y, 0.3) for (x, y) in cara['path']] + [(0.3, -0.9, 1.0)]
    return dict(cells=cells, rivers=[river], hero=(0.9, 1.0), enemies=[(-1.9, 2.6), (-1.5, 3.0)], warlord=(3.0, 10.6), p=0.32,
                caravan=cara, clear=clear)


def lay_crest():
    cells = cells_from((-5.5, 5.5), (-4.0, 9.0), redune_tf(), 12)
    sk = (-0.6, 2.8) if STYLE[0] == 'storm' else (-1.8, 2.3)
    setsite(cells, sk[0], sk[1], site='skeleton')
    explicit = [(-2.0, 3.8, 0.36, 1.55, False), (2.8, 5.4, 0.66, 0.85, True), (-3.8, 6.6, 0.6, 0.8, True), (0.6, 7.4, 0.3, 1.4, False)]
    return dict(cells=cells, rivers=[], hero='crest', enemies=[(2.2, 3.4), (2.6, 3.8)], warlord=None, p=0.0, explicit=explicit,
                clear=[(0.0, 0.6, 1.0), (sk[0], sk[1], 0.6)], arch=(1.7, 2.0))


def lay_monument():
    cells = cells_from((-12.5, 12.5), (-5.0, 20.0), redune_tf(), 13)
    rnd = random.Random(5)
    explicit = []
    for (x, y) in ((-4.2, 5.0), (-1.4, 7.6), (2.0, 5.8), (4.8, 8.8), (-6.2, 10.4), (0.8, 11.8), (6.8, 4.4), (-3.0, 14.0), (4.0, 14.6)):
        x += rnd.uniform(-0.4, 0.4)
        y += rnd.uniform(-0.4, 0.4)
        if rnd.random() < 0.6:
            explicit.append((x, y, rnd.uniform(0.28, 0.4), rnd.uniform(1.3, 2.0), False))
        else:
            explicit.append((x, y, rnd.uniform(0.6, 0.85), rnd.uniform(0.8, 1.1), True))
    setsite(cells, 3.4, 2.6, site='obelisk')
    return dict(cells=cells, rivers=[], hero='crest', crest=(0.2, 1.0), enemies=[(-2.2, 2.4), (-1.8, 2.8)], warlord=(1.2, 9.0), p=0.0,
                explicit=explicit, clear=[(0.2, 1.0, 1.0)])


def lay_gorge():
    cells = cells_from(XR, YR, redune_tf(), 14)
    setsite(cells, 4.6, 8.4, site='mesafort')
    setsite(cells, -5.0, 1.6, site='camp')
    river = [(-11.0, 9.5), (-6.0, 7.6), (-2.6, 4.8), (0.4, 3.6), (3.2, 5.0), (6.4, 5.6), (11.0, 3.4)]
    return dict(cells=cells, rivers=[river], hero=(0.4, 2.0), enemies=[(0.9, 5.2), (1.3, 5.5)], warlord=(-3.4, 10.0), p=0.24,
                bridge=(0.4, 3.6), clear=[(0.4, 3.6, 1.0)])


def lay_town():
    cells = cells_from(XR, YR, redune_tf(), 15)
    oas = setsite(cells, -1.0, 5.6, terrain='oasis')
    core = B.nearest(cells, 2.6, 5.0, exclude=(oas,))
    cells[core][0], cells[core][1] = 'dcity', 'citycore'
    cc = K.hex_world(*core)
    n = 0
    for k in sorted(cells.keys(), key=lambda k: (K.hex_world(*k) - cc).length):
        w = K.hex_world(*k)
        if k != core and cells[k][0] == 'redune' and (w - cc).length < 1.9 and w.y >= cc.y - 0.2 and n < 2:
            cells[k][0], cells[k][1] = 'dcity', None
            n += 1
    for k in list(cells.keys()):
        if cells[k][0] == 'redune' and (K.hex_world(*k) - K.hex_world(*oas)).length < 1.9:
            cells[k][1] = 'fields'
    setsite(cells, -4.6, 2.2, site='camp')
    cara = dict(path=[(-7.5, -1.6), (-4.8, 0.2), (-2.0, 1.8), (0.6, 2.8), (1.9, 3.6)], camels=8, wagons=2, spacing=0.16, offset=1.4)
    clear = [(x, y, 0.3) for (x, y) in cara['path']] + [(cc.x, cc.y, 1.6), (0.6, -0.6, 1.3), (2.6, 0.2, 1.0), (-1.4, -0.4, 0.9)]
    return dict(cells=cells, rivers=[], hero=(0.2, 1.4), enemies=[(4.9, 1.8)], warlord=(-5.6, 11.2), p=0.3, caravan=cara, clear=clear)


STYLE = ['storm']
LAYOUTS5 = [lay_sea, lay_crest, lay_monument, lay_gorge, lay_town]


def build5(idx, style):
    STYLE[0] = style
    L = LAYOUTS5[idx]()
    hero = L['hero']
    build_canyon(L['cells'], L['rivers'], hero, L['enemies'], L['warlord'], L.get('explicit', ()), L['p'], L.get('clear', ()))
    if hero == 'crest':
        cx, cy = D.crest_point(*L.get('crest', (0.0, 0.6)), r=0.55)
        g = D.ground(cx, cy)
        K.add(Ctx.meshes['hero'], g - Vector((0, 0, 0.01)), 0.4, Ctx.style['token_scale'])
        Ctx.hero = g
        D.footprints([(cx, cy), (cx - 0.2, cy - 0.5), (cx - 0.5, cy - 1.0), (cx - 0.6, cy - 1.8)])
    if 'arch' in L:
        D.place(Ctx.meshes['arch1'], L['arch'][0], L['arch'][1], 0.6, 2.2, sink=0.02)
    if 'caravan' in L:
        D.caravan(**L['caravan'])
    if 'bridge' in L:
        L['bridge_at'] = bridge(L['bridge'], half=0.72, n=44)
    fields_near_oasis()
    R.apply_overlays()
    if style == 'dawn':
        grid_lines()
    return L


def pal5():
    return palette(sand3=(lin('#e2a062'), lin('#c87a40'), lin('#8e4826')), reddune=(lin('#e0985a'), lin('#c8703a'), lin('#8a3e22')),
                   rust=lin('#a8461e'), redsand3=(lin('#d08850'), lin('#b86a3a'), lin('#80442a')))


STORM5 = {
    0: dict(cam=((0.0, 2.8, 0.2), -10, 19, 8.8, 32), haboob=1.3, drift=0.6, haze=0.022),
    1: dict(cam=None, haboob=1.5, drift=0.45, haze=0.015),
    2: dict(cam=((0.4, 4.4, 0.35), -6, 10, 9.2, 35), haboob=1.4, drift=0.5, haze=0.02),
    3: dict(cam=((0.3, 3.4, 0.0), -8, 50, 8.4, 35), haboob=0.0, drift=0.0, haze=0.03),
    4: dict(cam=((1.0, 3.4, 0.2), -12, 19, 8.4, 32), haboob=1.6, drift=0.9, haze=0.06),
}


def storm5(idx):
    pal = pal5()
    kw = dict(window_glow=14.0, flame_k=40.0, light_mult=2.0, dry_rivers=True, overlays={'ao': (0.06, 0.55, (0.3, 0.22, 0.18, 1)), 'desat': 0.05},
              gap=0.012, bevel=0.006)
    S = STORM5[idx]
    tgt = (S['cam'] or ((0.0, 0.8, 0),))[0]
    kw['res_fn'] = res_by(tgt[:2]) if idx not in (1, 2) else (res_by((0.0, 0.8), 84, 48, 32, 3.0, 6.0) if idx == 1 else res_by((0.3, 2.0), 64, 40, 22, 5.0, 10.0))
    if idx in (1, 2):
        kw['dune_amp'] = 0.5
    if idx == 3:
        kw.update(gorge=(0.4, 0.45, 0.58), overlays={'ao': (0.06, 0.6, (0.25, 0.22, 0.22, 1)), 'desat': 0.12, 'tint': (0.94, 0.92, 0.94, 1)})
    setup(pal, **kw)
    canyon_materials(pal)
    L = build5(idx, 'storm')
    if idx == 3:
        flood_sheets(-0.26)
    T = Ctx.style['thickness']
    R.plane((0, 5, -T * 0.6 - 0.3), 120, K.m_flat('void', lin('#3a2216'), 0.95))
    if idx in (1, 2):
        h = Ctx.hero
        cam = ((h.x + 0.1, h.y + 0.5, h.z + 0.05), -20, 20, 3.5, 45) if idx == 1 else S['cam']
    else:
        cam = S['cam']
    if idx == 3:
        K.world(lin('#4a4e58'), 1.0, camera_bg=[lin('#6a6a70'), lin('#1a1c22')])
        sn = K.sun(-150, 9, 2.8, (1.0, 0.7, 0.45), 2.0)
        bolt(-3.0, 22.0, 10.0, lin('#d8e8ff'), 26000.0, seed=3)
        K.fog_box((0, 10, 1.5), (50, 34, 4.0), S['haze'], lin('#8a8e96'), 0.2)
    else:
        K.world(lin('#5a4634'), 0.8, camera_bg=[lin('#7a5a3c'), lin('#140f0c')])
        sn = K.sun(-125, 9, 3.4 if idx != 4 else 2.2, (1.0, 0.72, 0.45), 2.0)
        bolt(3.5 if idx != 2 else -2.0, 26.0, 9.0, lin('#ffe2c0'), 20000.0 if idx != 2 else 9000.0, seed=idx + 2, light_y=None if idx != 2 else 17.0)
    sn.visible_glossy = False
    if idx == 2:
        K.fog_box((0, 25, 1.5), (120, 10, 3.0), 0.9, lin('#b88a5c'), 0.2, clumps=0.25, fade_z=(0.2, 3.0))
    if S['haboob']:
        K.fog_box((0, 28, 4.0), (100, 14, 9.0), S['haboob'] * 2.4, lin('#3e2614'), 0.1, clumps=0.2, fade_z=(1.0, 8.5))
    if S['haze'] and idx not in (2, 3):
        K.fog_box((0, 10, 0.9), (70, 38, 1.8), S['haze'], lin('#a07a50'), 0.25)
    if S['drift']:
        K.fog_box((0, 6, 0.2), (70, 38, 0.6), S['drift'], lin('#d8a070'), 0.0, clumps=1.4, fade_z=(0.04, 0.45))
    dof = {'fstop': 1.6, 'offset': 0.0} if idx == 1 else None
    K.camera(cam[0], cam[1], cam[2], cam[3], lens=cam[4], dof=dof)
    exp = {0: 0.45, 1: 0.35, 2: 0.4, 3: 0.45, 4: 0.6}[idx]
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=exp, samples=192)


DAWN5 = {
    0: dict(cam=((0.2, 3.4, 0.1), 8, 21, 10.5, 36), fog=(0.21, 1.6), sky=('#f2c4a2', '#6a78ac')),
    1: dict(cam=None, fog=(0.18, 1.0), sky=('#f4c8a6', '#6e7cae')),
    2: dict(cam=((0.6, 4.6, 0.35), 6, 11, 9.6, 35), fog=(0.2, 1.6), sky=('#f6c2a0', '#6a76aa')),
    3: dict(cam=((0.3, 3.6, 0.0), 8, 50, 8.8, 35), fog=(0.16, 1.2), sky=('#f2c4a2', '#6a78ac')),
    4: dict(cam=((0.9, 3.4, 0.15), 8, 25, 9.6, 38), fog=(0.21, 1.6), sky=('#f2c2a4', '#6a78ac')),
}


def dawn5(idx):
    pal = pal5()
    kw = dict(gap=0.0, thickness=0.6, bevel=0.0, window_glow=5.0, flame_k=30.0, dry_rivers=True, overlays={'ao': (0.06, 0.5, (0.32, 0.24, 0.2, 1))})
    D_ = DAWN5[idx]
    tgt = (D_['cam'] or ((0.0, 0.8, 0),))[0]
    kw['res_fn'] = res_by(tgt[:2]) if idx not in (1, 2) else (res_by((0.0, 0.8), 84, 48, 32, 3.0, 6.0) if idx == 1 else res_by((0.3, 2.0), 64, 40, 22, 5.0, 10.0))
    if idx in (1, 2):
        kw['dune_amp'] = 0.5
    if idx == 3:
        kw['gorge'] = (0.4, 0.45, 0.58)
    setup(pal, **kw)
    canyon_materials(pal)
    L = build5(idx, 'dawn')
    if idx in (1, 2):
        h = Ctx.hero
        cam = ((h.x - 0.1, h.y + 0.9, h.z + 0.15), 18, 15, 4.4, 45) if idx == 1 else D_['cam']
    else:
        cam = D_['cam']
    top, dens = D_['fog']
    K.fog_box((0, 6, top * 0.5), (70, 44, top + 0.1), dens, lin('#f2d4c4'), 0.0, clumps=0.8, fade_z=(0.0, top))
    if idx == 3:
        K.fog_box((0, 5, -0.2), (70, 44, 0.5), 3.0, lin('#f6dccc'), 0.0, clumps=1.2, fade_z=(-0.3, 0.02))
    K.fog_box((0, 12, 1.2), (70, 40, 4.0), 0.016, lin('#f4d2bc'), 0.5)
    if idx == 2:
        K.fog_box((0, 25, 1.5), (120, 10, 3.0), 0.9, lin('#f2d2c0'), 0.3, clumps=0.25, fade_z=(0.2, 3.0))
    if idx == 4:
        for hx in Ctx.hexes.values():
            if hx.terrain == 'oasis':
                K.fog_box((hx.center.x, hx.center.y, 0.25), (2.2, 2.2, 0.5), 2.0, lin('#f6e2d8'), 0.0, clumps=1.6, fade_z=(-0.05, 0.45))
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
if 61 <= i <= 65:
    storm5(i - 61)
else:
    dawn5(i - 66)
