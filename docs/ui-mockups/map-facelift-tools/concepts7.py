"""Set 7: a narrow mountain pass through a solid range. Usage:
blender -b --factory-startup -P concepts7.py -- <01-06> <out.png> [preview|json]
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
# Set 7: a narrow mountain pass through a solid range.
import json
from mathutils import kdtree
from hexkit import smoothstep, lerp, hex_world, catmull, SQ3

W, H_ = 1920, 1080
JSON_ONLY = len(argv) > 2 and argv[2] == 'json'


def V3(x, y, z=0.0):
    return Vector((x, y, z))


SPINE = [(-2.7, -0.6), (-2.3, 0.4), (-1.5, 1.2), (-1.05, 2.2), (-0.8, 3.3), (0.4, 4.3), (1.35, 5.3), (1.9, 6.2), (2.6, 7.2),
         (3.1, 8.2), (3.25, 9.2), (3.5, 10.3), (3.7, 11.6)]


def _ev(y):
    if y < 1.3:
        return 0.07
    if y < 3.4:
        return lerp(0.07, 0.55, smoothstep(1.3, 3.4, y))
    if y < 6.2:
        return lerp(0.55, 0.9, smoothstep(3.4, 6.2, y))
    if y < 7.3:
        return lerp(0.9, 0.76, smoothstep(6.2, 7.3, y))
    if y < 9.3:
        return lerp(0.76, 0.1, smoothstep(7.3, 9.3, y))
    return 0.08


def _w(y):
    head = max(smoothstep(1.1, 1.6, y) * (1 - smoothstep(3.2, 3.6, y)), smoothstep(7.2, 7.6, y) * (1 - smoothstep(9.0, 9.4, y)))
    col = 1 - smoothstep(0.0, 0.9, abs(y - 6.2))
    valley = max(1 - smoothstep(0.5, 1.2, y), smoothstep(9.6, 10.3, y))
    return 0.1 + 0.36 * head - 0.03 * col + 0.9 * valley


class Spine:
    pts, kd, ys = None, None, None


def build_spine():
    pts = catmull([V3(x, y) for (x, y) in SPINE], 40)
    dense = []
    for a, b in zip(pts, pts[1:]):
        L = (b - a).length
        n = max(1, int(L / 0.01))
        for i in range(n):
            dense.append(a.lerp(b, i / n))
    dense.append(pts[-1])
    kd = kdtree.KDTree(len(dense))
    for i, p in enumerate(dense):
        kd.insert(V3(p.x, p.y), i)
    kd.balance()
    Spine.pts, Spine.kd = dense, kd


def spine_x(y):
    best = min(Spine.pts, key=lambda p: abs(p.y - y))
    return best.x


def ridged(x, y, seed=3.0, oct_=5):
    total, amp, f = 0.0, 0.55, 1.0
    for i in range(oct_):
        n = 1.0 - abs(noise.noise(V3(x * f, y * f, seed + i * 1.7)))
        total += amp * n * n
        amp *= 0.5
        f *= 2.1
    return total


COL = Vector((1.9, 6.2))
PERP = Vector((-0.85, 0.53)).normalized()


def mass(y):
    front = smoothstep(0.9, 2.4, y) * (1 - smoothstep(9.4, 10.6, y))
    back = smoothstep(12.6, 14.2, y)
    return max(front, back)


def H(x, y):
    M = mass(y)
    m = M * (0.42 + 1.25 * ridged(x * 0.42 + 5.0, y * 0.42) + 0.3 * ridged(x * 1.7 + 2.0, y * 1.7, 11.0, 3)) + 0.035 * noise.noise(V3(x * 4.0, y * 4.0, 2.0))
    for s in (-1, 1):
        c = COL + PERP * 0.95 * s
        d2 = (x - c.x) ** 2 + (y - c.y) ** 2
        m += 0.75 * math.exp(-d2 / 0.35)
    valley = 0.05 + 0.025 * noise.noise(V3(x * 1.5, y * 1.5, 9.0))
    zm = max(m, valley)
    co, idx, d = Spine.kd.find(V3(x, y))
    if d < 2.0:
        ys = co.y
        w = _w(ys)
        floor = _ev(ys) + 0.12 * (min(d, w) / max(w, 1e-3)) ** 2 * smoothstep(0.3, 0.6, w)
        t = smoothstep(w, w + 0.38, d) ** 0.7
        return floor + (zm - floor) * t
    return zm


_prev_h = K.Hex.height


def hex_height(self, wx, wy):
    if getattr(self, 'passmap', False):
        return H(wx, wy)
    return _prev_h(self, wx, wy)


K.Hex.height = hex_height


def gz(x, y):
    return V3(x, y, H(x, y))


def slope(x, y, e=0.03):
    return math.hypot(H(x + e, y) - H(x - e, y), H(x, y + e) - H(x, y - e)) / (2 * e)


def spine_d(x, y):
    co, idx, d = Spine.kd.find(V3(x, y))
    return d, co.y


# ---------------------------------------------------------------------------
# Materials

def m_massif(name, P):
    m = R.m_ground(name, P['plains'][0], P['plains'][1], lin('#6a6a38'), P['mud'], rock=P['hill_rock'], slope=(0.6, 0.8), micro=0.5)
    nt = m.node_tree
    p = nt.nodes['Principled BSDF']
    src = p.inputs['Base Color'].links[0].from_socket
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    sep = nt.nodes.new('ShaderNodeSeparateXYZ')
    nt.links.new(geo.outputs['Position'], sep.inputs['Vector'])
    sn = nt.nodes.new('ShaderNodeSeparateXYZ')
    nt.links.new(geo.outputs['Normal'], sn.inputs['Vector'])
    n = K._noise(nt, 3.0, 4.0)
    import desert as D
    zj = D._math(nt, 'MULTIPLY_ADD', n.outputs['Fac'], 0.25) if False else None
    zz = nt.nodes.new('ShaderNodeMath')
    zz.operation = 'MULTIPLY_ADD'
    nt.links.new(n.outputs['Fac'], zz.inputs[0])
    zz.inputs[1].default_value = 0.25
    nt.links.new(sep.outputs['Z'], zz.inputs[2])
    alp = K._maprange(nt, zz.outputs['Value'], 0.45, 0.7, 0.0, 0.7)
    col = K._mix(nt, alp, src, P['alpine'])
    rk = K._maprange(nt, zz.outputs['Value'], 0.75, 1.0, 0.0, 1.0)
    steep = K._maprange(nt, sn.outputs['Z'], 0.55, 0.85, 1.0, 0.0)
    rmask = D._math(nt, 'MAXIMUM', rk, steep)
    vo = nt.nodes.new('ShaderNodeTexVoronoi')
    vo.feature = 'DISTANCE_TO_EDGE'
    vo.inputs['Scale'].default_value = 14.0
    nt.links.new(geo.outputs['Position'], vo.inputs['Vector'])
    rockc = K._mix(nt, K._maprange(nt, vo.outputs['Distance'], 0.0, 0.05, 0.6, 0.0), P['massrock'], (0.03, 0.03, 0.03, 1))
    col = K._mix(nt, rmask, col, rockc)
    snow = K._maprange(nt, zz.outputs['Value'], 1.78, 1.92, 0.0, 0.85)
    flat = K._maprange(nt, sn.outputs['Z'], 0.6, 0.8, 0.0, 1.0)
    col = K._mix(nt, D._math(nt, 'MULTIPLY', snow, flat), col, P['snow'])
    nt.links.new(col, p.inputs['Base Color'])
    return m


def m_trail(name, c):
    m, nt, p = K._new(name)
    n = K._noise(nt, 60.0, 5.0)
    col = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.35, 0.7, 0.0, 0.5), c, (c[0] * 0.55, c[1] * 0.5, c[2] * 0.45, 1))
    nt.links.new(col, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.95)
    K._bump(nt, p, 120.0, 0.4, 0.003)
    return m


# ---------------------------------------------------------------------------
# Map

FLOWER = [(0, 0), (1, 0), (1, -1), (0, -1), (-1, 0), (-1, 1), (0, 1)]


def build_pass(pal, storm=False):
    build_spine()
    M = Ctx.mats
    mat = m_massif('massif', pal)
    for k in ('massif', 'vforest', 'vplains'):
        M[k] = mat
    M['trail'] = m_trail('trail', pal['trailc'])
    cells = B.gen_cells((-9.5, 10.5), (-3.2, 15.5), lambda w, rnd: 'massif', 7)
    for (q, r) in cells:
        hx = B.make_hex(q, r, 'massif', None, 0)
        hx.passmap = True
        c = hx.center
        if mass(c.y) < 0.25:
            hx.terrain = 'vforest' if noise.noise(V3(c.x * 0.4, c.y * 0.4, 6.0)) > -0.1 else 'vplains'
        Ctx.hexes[(q, r)] = hx
    village = K.nearest_hex(Vector((0.4, -0.2)))
    village.terrain, village.site = 'vplains', 'village'
    keep = K.nearest_hex(Vector((5.6, 11.2)))
    keep.terrain, keep.site = 'vplains', 'keep'
    for hx in Ctx.hexes.values():
        K.tile_mesh(hx)
    trail = build_trail()
    for hx in Ctx.hexes.values():
        decorate_pass(hx, trail)
    dress_pass(trail, storm)
    return trail


def build_trail():
    pts = [(-2.65, -0.45), (-2.3, 0.35), (-1.75, 1.0), (-1.35, 1.35)]
    def zig(y0, y1, legs, amp):
        out = []
        for i in range(legs + 1):
            y = lerp(y0, y1, i / legs)
            out.append((spine_x(y) + (amp if i % 2 == 0 else -amp), y))
        return out
    pts += zig(1.6, 3.3, 6, 0.46)
    pts += [(-0.15, 3.85), (0.55, 4.45), (1.2, 5.2), (1.6, 5.75), (1.9, 6.2), (2.3, 6.75), (2.75, 7.35)]
    pts += zig(7.6, 9.2, 5, 0.44)
    pts += [(3.4, 9.75), (3.55, 10.4), (3.65, 11.0)]
    curve = catmull([V3(x, y) for (x, y) in pts], 24)
    dense = []
    for a, b in zip(curve, curve[1:]):
        L = (b - a).length
        n = max(1, int(L / 0.012))
        for i in range(n):
            dense.append(a.lerp(b, i / n))
    bm = bmesh.new()
    left, right = [], []
    for i, p in enumerate(dense):
        q = dense[min(i + 1, len(dense) - 1)] - dense[max(i - 1, 0)]
        t = q.normalized() if q.length > 1e-6 else Vector((0, 1, 0))
        nrm = Vector((-t.y, t.x, 0))
        for lst, o in ((left, 0.03), (right, -0.03)):
            x, y = p.x + nrm.x * o, p.y + nrm.y * o
            lst.append(bm.verts.new((x, y, H(x, y) + 0.006)))
    for i in range(len(left) - 1):
        bm.faces.new((left[i], right[i], right[i + 1], left[i + 1]))
    K.add(K.to_mesh(bm, 'trail', [Ctx.mats['trail']], smooth=True), (0, 0, 0), name='trail')
    kd = kdtree.KDTree(len(dense))
    for i, p in enumerate(dense):
        kd.insert(V3(p.x, p.y), i)
    kd.balance()
    Ctx.trail_kd = kd
    Ctx.trail = dense
    Ctx.trail_turns = [Vector(p) for p in pts]
    return dense


def trail_d(x, y):
    return Ctx.trail_kd.find(V3(x, y))[2]


def decorate_pass(hx, trail):
    rnd = random.Random(hx.seed + 5)
    me = Ctx.meshes
    pal = Ctx.style['pal']
    c = hx.center
    if hx.terrain == 'massif':
        Mv = mass(c.y)
        placed = []
        for i in range(16):
            if len(placed) >= 4 + (hx.seed % 2):
                break
            lx, ly = rnd.uniform(-0.55, 0.55), rnd.uniform(-0.55, 0.55)
            x, y = c.x + lx, c.y + ly
            r = rnd.uniform(0.32, 0.55)
            d, ys = spine_d(x, y)
            if d < _w(ys) + r * 0.55 + 0.04 or trail_d(x, y) < r * 0.85 + 0.06:
                continue
            if any((Vector((x, y)) - q).length < 0.4 for q in placed):
                continue
            h = rnd.uniform(0.55, 1.25) * Mv * Ctx.style.get('peak_scale', 1.0)
            if h < 0.25:
                continue
            ob = R.mountain_real(hx, lx, ly, h, r)
            zmin = min(H(x + math.cos(a) * r * 0.85, y + math.sin(a) * r * 0.85) for a in [i_ * 0.785 for i_ in range(8)])
            ob.location.z = min(zmin, H(x, y)) - 0.03
            placed.append(Vector((x, y)))
        ntree = 16
    else:
        ntree = 34 if hx.terrain == 'vforest' else 8
    for i in range(ntree * 3):
        if ntree <= 0:
            break
        lx, ly = rnd.uniform(-0.85, 0.85), rnd.uniform(-0.85, 0.85)
        if K.edge_dist(Vector((lx, ly)), 1.0) < 0.02:
            continue
        x, y = c.x + lx, c.y + ly
        z = H(x, y)
        if z > 0.62 or slope(x, y) > 1.1 or trail_d(x, y) < 0.07:
            continue
        if hx.site and math.hypot(lx, ly) < 0.5:
            continue
        ntree -= 1
        s = rnd.uniform(0.9, 1.4) * Ctx.style.get('tree_scale', 1.1)
        if rnd.random() < 0.85:
            K.add(me[f'conifer{rnd.randrange(4)}'], V3(x, y, z - 0.01), rnd.uniform(0, 6), s, K.leaf_color('conifer', rnd))
        else:
            K.add(me[f'broad{rnd.randrange(4)}'], V3(x, y, z - 0.01), rnd.uniform(0, 6), s, K.leaf_color('broad', rnd))
    for i in range(5):
        lx, ly = rnd.uniform(-0.8, 0.8), rnd.uniform(-0.8, 0.8)
        x, y = c.x + lx, c.y + ly
        if trail_d(x, y) < 0.07:
            continue
        K.add(me[f'rock{rnd.randrange(3)}'], V3(x, y, H(x, y) - 0.012), rnd.uniform(0, 6), rnd.uniform(0.6, 1.4), K.jitter_col(pal['rock'], rnd))
    if hx.site:
        K.place_site(hx, [])
        R.place_dressing(hx)


def dress_pass(trail, storm):
    me = Ctx.meshes
    rnd = random.Random(9)
    S = Ctx.style
    # lantern posts every ~0.9 along the trail
    acc, last = 0.0, trail[0]
    for p in trail[1:]:
        acc += (p - last).length
        last = p
        if acc > 0.9:
            acc = 0.0
            q = Vector((p.x + 0.06, p.y + 0.02))
            g = gz(q.x, q.y)
            K.add(me['torch'], g - V3(0, 0, 0.005), 0.0, 1.3)
            R.point(g + V3(0, 0, 0.16), S.get('lantern_e', 1.6), (1.0, 0.6, 0.3), 0.02)
    # cairns at the hairpins
    for t in Ctx.trail_turns[4:-3]:
        q = Vector((t.x + 0.07, t.y))
        g = gz(q.x, q.y)
        for k in range(3):
            K.add(me[f'rock{k}'], g + V3(0, 0, 0.012 * k), rnd.uniform(0, 6), 0.55 - 0.12 * k, K.jitter_col(S['pal']['rock'], rnd))
    # gatehouse at the col
    t = Vector((0.6, 0.9)).normalized()
    nrm = Vector((-t.y, t.x))
    for s in (-1, 1):
        q = COL + nrm * 0.16 * s
        K.add(me['tower'], gz(q.x, q.y) - V3(0, 0, 0.03), 0.0, 1.25)
        q2 = COL + nrm * 0.36 * s
        K.add(me['wall'], gz(q2.x, q2.y) - V3(0, 0, 0.04), math.atan2(nrm.y, nrm.x), (0.36, 1.2, 1.2))
        R.point(gz(COL.x + nrm.x * 0.16 * s, COL.y + nrm.y * 0.16 * s) + V3(0, 0, 0.3), S.get('gate_e', 4.0), (1.0, 0.6, 0.3), 0.04)
    # waystation hut at the top of the headwall
    hq = Vector((-0.1, 3.7))
    K.add(me['house0'], gz(hq.x, hq.y) - V3(0, 0, 0.01), 0.4, 1.4, S['pal']['roofs'][0])
    # prayer flags across the col
    a = COL + nrm * 0.25 + t * 0.35
    b = COL - nrm * 0.25 + t * 0.35
    za, zb = H(a.x, a.y) + 0.22, H(b.x, b.y) + 0.22
    bm = bmesh.new()
    for e in (a, b):
        K.bm_cone(bm, 5, 0.004, 0.004, 0.22, H(e.x, e.y) - 0.01, 0, x=e.x, y=e.y)
    cols = [lin('#2f4f86'), lin('#efe3c6'), lin('#8c2f2a'), lin('#3f7a3a'), lin('#e6c17c')]
    flag_mats = [K.m_flat(f'pf{i}', c_, 0.7) for i, c_ in enumerate(cols)]
    for k in range(13):
        f = (k + 0.5) / 13
        p = a.lerp(b, f)
        z = lerp(za, zb, f) - 0.05 * math.sin(math.pi * f)
        vs = [bm.verts.new(v) for v in ((p.x - 0.012, p.y, z), (p.x + 0.012, p.y, z), (p.x + 0.012, p.y, z - 0.032), (p.x - 0.012, p.y, z - 0.032))]
        bm.faces.new(vs).material_index = 1 + k % 5
    K.add(K.to_mesh(bm, 'flags', [Ctx.mats['deadwood']] + flag_mats), (0, 0, 0))


def place_pieces(hero_at=None):
    ts = Ctx.style.get('token_scale', 1.5)
    hp = hero_at or Vector((-2.62, -0.35))
    g = gz(hp.x, hp.y)
    K.add(Ctx.meshes['hero'], g - V3(0, 0, 0.01), 0.4, ts)
    Ctx.hero = g
    foes = [Vector((3.45, 10.15)), Vector((3.8, 10.4))]
    for f in foes:
        K.add(Ctx.meshes['enemy'], gz(f.x, f.y) - V3(0, 0, 0.01), 0.2, ts)
    wl = Vector((1.4, 11.6))
    K.add(Ctx.meshes['warlord'], gz(wl.x, wl.y) - V3(0, 0, 0.01), 0.0, ts * 1.1)
    Ctx.foes = foes


def pass_hexes():
    keys = set()
    for p in Ctx.trail[::4]:
        k = K.nearest_hex(Vector((p.x, p.y)))
        keys.add((k.q, k.r))
    return [Ctx.hexes[k] for k in keys]


def grid(alpha=0.22, col='#2a2418'):
    g = K.m_alpha('grid', lin(col), alpha)
    g['no_overlay'] = True
    for hx in Ctx.hexes.values():
        K.hex_ring(hx, hx.center, 0.975, 1.0, g, z=0.012, n=16)


def route_rings():
    for hx in pass_hexes():
        K.hex_ring(hx, hx.center, 0.9, 0.95, Ctx.mats['ring_move'], z=0.02, n=16)


def route_dots(start_frac=0.0):
    pts = Ctx.trail
    i0 = int(len(pts) * start_frac)
    mat = Ctx.mats['ring_path']
    bm = bmesh.new()
    acc = 0.0
    for a, b in zip(pts[i0:], pts[i0 + 1:]):
        acc += (b - a).length
        if acc >= 0.1:
            acc = 0.0
            K.bm_uvsphere(bm, 0.026, (b.x, b.y, H(b.x, b.y) + 0.045), 0, 8, 6)
    K.add(K.to_mesh(bm, 'dots', [mat], smooth=True), (0, 0, 0))


def occluders(points, names=('mountain',)):
    sc = bpy.context.scene
    bpy.context.view_layer.update()
    dg = bpy.context.evaluated_depsgraph_get()
    o = sc.camera.matrix_world.translation.copy()
    hits = set()
    for t in points:
        d = t - o
        L = d.length
        d.normalize()
        origin, travelled = o.copy(), 0.0
        for _ in range(60):
            ok, loc, nrm, idx, ob, mat = sc.ray_cast(dg, origin, d, distance=max(L - travelled - 0.05, 0.001))
            if not ok:
                break
            if ob.name.startswith(names):
                hits.add(ob.name)
            step = (loc - origin).length + 0.003
            travelled += step
            origin = loc + d * 0.003
            if travelled >= L:
                break
    return [bpy.data.objects[n] for n in hits]


def m_ghost_of(mat, opacity=0.25, rim=(0.8, 0.9, 1.0, 1)):
    m = mat.copy()
    nt = m.node_tree
    out = nt.nodes['Material Output']
    src = out.inputs['Surface'].links[0].from_socket
    tr = nt.nodes.new('ShaderNodeBsdfTransparent')
    mix = nt.nodes.new('ShaderNodeMixShader')
    mix.inputs['Fac'].default_value = opacity
    nt.links.new(tr.outputs['BSDF'], mix.inputs[1])
    nt.links.new(src, mix.inputs[2])
    lw = nt.nodes.new('ShaderNodeLayerWeight')
    lw.inputs['Blend'].default_value = 0.25
    em = nt.nodes.new('ShaderNodeEmission')
    em.inputs['Color'].default_value = rim
    rim_k = K._maprange(nt, lw.outputs['Facing'], 0.55, 1.0, 0.0, 0.9)
    nt.links.new(rim_k, em.inputs['Strength'])
    add = nt.nodes.new('ShaderNodeAddShader')
    nt.links.new(mix.outputs['Shader'], add.inputs[0])
    nt.links.new(em.outputs['Emission'], add.inputs[1])
    nt.links.new(add.outputs['Shader'], out.inputs['Surface'])
    return m


def ghost(objs, opacity=0.25):
    cache = {}
    for ob in objs:
        for i, mt in enumerate(ob.data.materials):
            if mt.name not in cache:
                cache[mt.name] = m_ghost_of(mt, opacity)
            ob.data.materials[i] = cache[mt.name]


def render_masks(groups):
    """Silhouette masks per token group: full silhouette, and the part actually visible."""
    if JSON_ONLY:
        return
    sc = bpy.context.scene
    sc.render.film_transparent = True
    sc.cycles.samples = 6
    sc.cycles.use_denoising = False
    sc.cycles.use_adaptive_sampling = False
    for o in sc.objects:
        if o.type == 'MESH' and (o.name.startswith('fog') or o.name.startswith('smoke')):
            o.hide_render = True
    for gname, objs in groups.items():
        names = {o.name for o in objs}
        others = [o for o in sc.objects if o.type in ('MESH', 'CURVE') and o.name not in names and not o.hide_render]
        for o in others:
            o.is_holdout = True
        sc.render.filepath = OUT.replace('.png', f'_{gname}_vis.png')
        bpy.ops.render.render(write_still=True)
        for o in others:
            o.is_holdout = False
            o.hide_render = True
        sc.render.filepath = OUT.replace('.png', f'_{gname}_full.png')
        bpy.ops.render.render(write_still=True)
        for o in others:
            o.hide_render = False




def tokens(kind):
    pref = {'foe': ('enemy', 'warlord'), 'hero': ('hero',)}[kind]
    return [o for o in bpy.context.scene.objects if o.type == 'MESH' and o.data and o.data.name.split('.')[0] in pref]


def mountains():
    return [o for o in bpy.context.scene.objects if o.name.startswith('mountain')]



def cutout(center_px, radius, target_dist, mats=None):
    """Screen-space see-through circle: geometry nearer than the target, inside the circle, is cut away."""
    cx, cy = center_px[0] / W, 1.0 - center_px[1] / H_
    for mt in (mats if mats is not None else {mt for ob in mountains() for mt in ob.data.materials}):
        nt = mt.node_tree
        out = nt.nodes['Material Output']
        src = out.inputs['Surface'].links[0].from_socket
        tc = nt.nodes.new('ShaderNodeTexCoord')
        sep = nt.nodes.new('ShaderNodeSeparateXYZ')
        nt.links.new(tc.outputs['Window'], sep.inputs['Vector'])
        dx = K._math if hasattr(K, '_math') else None
        import desert as D
        mx_ = D._math(nt, 'MULTIPLY', D._math(nt, 'SUBTRACT', sep.outputs['X'], cx), W / H_)
        my_ = D._math(nt, 'SUBTRACT', sep.outputs['Y'], cy)
        dist = D._math(nt, 'SQRT', D._math(nt, 'ADD', D._math(nt, 'MULTIPLY', mx_, mx_), D._math(nt, 'MULTIPLY', my_, my_)), 0.0)
        inside = K._maprange(nt, dist, radius - 0.004, radius, 1.0, 0.0)
        cd = nt.nodes.new('ShaderNodeCameraData')
        nearer = D._math(nt, 'LESS_THAN', cd.outputs['View Distance'], target_dist - 0.4)
        cut = D._math(nt, 'MULTIPLY', inside, nearer)
        tr = nt.nodes.new('ShaderNodeBsdfTransparent')
        mix = nt.nodes.new('ShaderNodeMixShader')
        nt.links.new(cut, mix.inputs['Fac'])
        nt.links.new(src, mix.inputs[1])
        nt.links.new(tr.outputs['BSDF'], mix.inputs[2])
        nt.links.new(mix.outputs['Shader'], out.inputs['Surface'])



def setup_pass(storm=False):
    pal = nature(alpine=lin('#6a5e44'), massrock=lin('#5e554c'), trailc=lin('#a88e68'), plains=(lin('#4e5a32'), lin('#646638')))
    pal['roofs'] = [lin('#8a7448'), lin('#7a6440'), lin('#6e5a3a'), lin('#94804e')]
    if storm:
        pal['trailc'] = lin('#8a7a62')
    g = dict(GRASS)
    g['count'] = {'vforest': 900, 'vplains': 1800}
    ov = {'ao': (0.06, 0.5, (0.25, 0.25, 0.3, 1))}
    if storm:
        ov = {'ao': (0.06, 0.6, (0.2, 0.22, 0.25, 1)), 'desat': 0.4, 'tint': (0.85, 0.9, 0.96, 1)}
    opts = dict(grass=g, gap=0.0, thickness=0.9, bevel=0.0, window_glow=12.0 if storm else 5.0, flame_k=40.0 if storm else 30.0, roof='slate' if storm else 'thatch',
                overlays=ov, ring_k=2.4, ring_col='#ffd27a', snow_z=0.6, peak_scale=1.0, light_mult=2.0 if storm else 1.0,
                lantern_e=5.0 if storm else 1.6, gate_e=14.0 if storm else 4.0,
                res_fn=lambda hx: 60 if (hx.center.xy - Vector((0.5, 5.0))).length < 4.6 else 36 if (hx.center.xy - Vector((0.5, 5.0))).length < 8.5 else 20)
    setup(pal, **opts)
    build_pass(pal, storm)


def dawn_light(valley=1.4, clouds=True, sun_el=8):
    K.fog_box((0, 6, 0.08), (44, 34, 0.5), valley * 0.75, lin('#ece0ea'), 0.0, clumps=0.7, fade_z=(-0.05, 0.28))
    if clouds:
        K.fog_box((0, 7, 1.55), (44, 34, 0.5), 0.5, lin('#f4e6ea'), 0.0, clumps=0.55, fade_z=(1.3, 1.8))
    K.fog_box((0, 10, 1.4), (60, 40, 4.0), 0.014, lin('#f0c8c0'), 0.5)
    R.sky_world(2, 260, strength=0.55, camera_bg=[lin('#f2c2a8'), lin('#6a7aa8')])
    K.sun(95, sun_el, 4.8, (1.0, 0.64, 0.46), 1.0)


def storm_light():
    rnd = random.Random(4)
    bolt = K.m_emit('bolt', lin('#d8e8ff'), 60.0)
    pts = [Vector((-3.0, 26.0, 9.0))]
    for i in range(11):
        pts.append(pts[-1] + Vector((rnd.uniform(-0.8, 0.8), rnd.uniform(-0.3, 0.3), -0.85)))
    cu = bpy.data.curves.new('bolt', 'CURVE')
    cu.dimensions = '3D'
    cu.bevel_depth = 0.06
    sp = cu.splines.new('POLY')
    sp.points.add(len(pts) - 1)
    for i, q in enumerate(pts):
        sp.points[i].co = (q.x, q.y, q.z, 1)
    cu.materials.append(bolt)
    K.link(bpy.data.objects.new('bolt', cu))
    fl = R.point(Vector((-3.0, 30.0, 14.0)), 26000.0, (0.75, 0.82, 1.0), 3.0)
    fl.visible_glossy = False
    K.world(lin('#3a4248'), 0.75, camera_bg=[lin('#5a6268'), lin('#141820')])
    sn = K.sun(70, 14, 2.4, (0.85, 0.88, 1.0), 6.0)
    sn.visible_glossy = False
    K.fog_box((0, 12, 1.2), (40, 30, 4.4), 0.03, lin('#8a9298'), 0.2)
    K.fog_box((0, 7, 1.6), (44, 34, 0.6), 0.6, lin('#9aa2a8'), 0.0, clumps=0.55, fade_z=(1.3, 1.9))


CINE = ((0.3, 4.6, 0.6), -8, 20, 12.0, 36)
PLAY = ((0.5, 5.0, 0.5), -4, 55, 14.5, 36)
TOP = ((1.1, 5.3, 0.4), 0, 80, 16.5, 36)
COLV = ((2.05, 6.55, 0.9), -32, 34, 4.2, 35)
MID = ((0.4, 5.0, 0.5), -6, 38, 13.5, 36)


def cam(c):
    ob = K.camera(c[0], c[1], c[2], c[3], lens=c[4])
    sc = bpy.context.scene
    sc.render.resolution_x, sc.render.resolution_y, sc.render.resolution_percentage = W, H_, 100
    bpy.context.view_layer.update()
    return ob


def render(exposure=0.15, samples=160, look='AgX - Medium High Contrast'):
    if JSON_ONLY:
        return
    K.render(OUT, preview=PREVIEW, look=look, exposure=exposure, samples=samples)


def dump(extra=None):
    data = {}
    if extra:
        data.update(extra)
    with open(OUT.replace('.png', '.json'), 'w') as f:
        json.dump(data, f)


def hex_corners_px(hx):
    return [K.project(V3(*(hx.center.xy + Vector((math.cos(math.radians(30 + 60 * i)), math.sin(math.radians(30 + 60 * i)))) * 0.92), 0) + V3(0, 0, H(*(hx.center.xy + Vector((math.cos(math.radians(30 + 60 * i)), math.sin(math.radians(30 + 60 * i)))) * 0.92)))) for i in range(6)]


def s01():
    setup_pass()
    place_pieces()
    R.apply_overlays()
    dawn_light()
    cam(CINE)
    dump()
    render()


def s02():
    setup_pass()
    place_pieces()
    R.apply_overlays()
    grid(0.32, '#3a2a1a')
    route_dots()
    dawn_light(0.9)
    cam(PLAY)
    dump()
    render()


def s03():
    setup_pass()
    place_pieces()
    R.apply_overlays()
    grid(0.18)
    route_dots()
    dawn_light(0.8, clouds=False, sun_el=22)
    cam(TOP)
    dump()
    render()


def s04():
    setup_pass()
    hp = Vector((1.62, 5.78))
    place_pieces(hero_at=hp)
    R.apply_overlays()
    dawn_light(1.6)
    cam(COLV)
    dump()
    render(samples=192)


def s05():
    setup_pass()
    place_pieces()
    R.apply_overlays()
    route_dots()
    hx = K.nearest_hex(COL)
    seg = [p for p in Ctx.trail if K.nearest_hex(Vector((p.x, p.y))) is hx]
    bm = bmesh.new()
    D_ = __import__('desert')
    D_.tube(bm, [V3(p.x, p.y, H(p.x, p.y) + 0.03) for p in seg[::3]], [0.018] * len(seg[::3]), 8, 0)
    K.add(K.to_mesh(bm, 'hover_trail', [K.m_emit('hover', lin('#fff1c9'), 9.0)], smooth=True), (0, 0, 0))
    cam(MID)
    pts = [V3(p.x, p.y, H(p.x, p.y) + 0.05) for p in Ctx.trail[::12]]
    pts += [gz(f.x, f.y) + V3(0, 0, 0.15) for f in Ctx.foes]
    occ = occluders(pts)
    ghost(occ, 0.34)
    c = gz(COL.x, COL.y)
    dawn_light(1.0)
    dump({'faded': len(occ), 'cursor': K.project(c + V3(0, 0, 0.05))})
    render()
    route = [o for o in bpy.context.scene.objects if o.type == 'MESH' and o.data.name.split('.')[0] in ('dots', 'hover_trail')]
    print('ROUTE_OBJECTS', len(route))
    render_masks({'foe': tokens('foe'), 'route': route})


def s06():
    setup_pass(storm=True)
    place_pieces()
    R.apply_overlays()
    storm_light()
    cam(CINE)
    dump()
    render(exposure=0.55, samples=192)


SCENES = {f'{i:02d}': globals()[f's{i:02d}'] for i in range(1, 7)}
K.reset()
SCENES[CID]()
