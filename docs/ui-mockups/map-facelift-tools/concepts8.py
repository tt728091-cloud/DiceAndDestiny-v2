"""Set 8: a high mountain pass (trail shelf cut into the slopes). Usage:
blender -b --factory-startup -P concepts8.py -- <01-06> <out.png> [preview|json]
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
# Set 8: a HIGH pass. The range stays tall; the trail is a narrow shelf cut
# into the slopes, zigzagging up the mountain face and along high ledges.
import json
from mathutils import kdtree
from hexkit import smoothstep, lerp, hex_world, catmull, SQ3

W, H_ = 1920, 1080
JSON_ONLY = len(argv) > 2 and argv[2] == 'json'


def V3(x, y, z=0.0):
    return Vector((x, y, z))


SPINE = [(-2.7, -0.6), (-2.2, 0.5), (-1.45, 1.4), (-1.05, 2.4), (-0.75, 3.3), (0.35, 3.9), (0.4, 4.6), (0.75, 5.1), (1.45, 5.55),
         (1.9, 6.2), (2.65, 7.1), (3.05, 8.1), (3.2, 9.2), (3.45, 10.2), (3.7, 11.6)]
COL = Vector((1.9, 6.2))
PERP = Vector((-0.85, 0.53)).normalized()


def dense_kd(points, step=0.01):
    pts = catmull([V3(x, y) for (x, y) in points], 40)
    out = []
    for a, b in zip(pts, pts[1:]):
        L = (b - a).length
        n = max(1, int(L / step))
        for i in range(n):
            out.append(a.lerp(b, i / n))
    out.append(pts[-1])
    kd = kdtree.KDTree(len(out))
    for i, p in enumerate(out):
        kd.insert(V3(p.x, p.y), i)
    kd.balance()
    return out, kd


class Spine:
    pts, kd = None, None


class Trail:
    pts, kd, e, turns = None, None, None, None


def spine_x(y):
    return min(Spine.pts, key=lambda p: abs(p.y - y)).x


def ridged(x, y, seed=3.0, oct_=5):
    total, amp, f = 0.0, 0.55, 1.0
    for i in range(oct_):
        n = 1.0 - abs(noise.noise(V3(x * f, y * f, seed + i * 1.7)))
        total += amp * n * n
        amp *= 0.5
        f *= 2.1
    return total


def face(y):
    return max(smoothstep(1.0, 1.4, y) * (1 - smoothstep(2.9, 3.3, y)), smoothstep(7.8, 8.2, y) * (1 - smoothstep(10.0, 10.4, y)))


def mass(y):
    up = smoothstep(1.2, 2.9, y)
    down = 1 - smoothstep(8.2, 10.4, y)
    return max(up * down, smoothstep(12.4, 14.0, y))


def H0(x, y):
    m = mass(y)
    base = 1.25 * m
    co, idx, d = Spine.kd.find(V3(x, y))
    corridor = 0.42 + 0.58 * smoothstep(0.12, 0.75, d)
    fz = face(co.y)
    corridor = lerp(corridor, 0.5 + 0.5 * smoothstep(0.25, 0.9, d), fz)
    r1 = ridged(x * 0.42 + 5.0, y * 0.42)
    r2 = ridged(x * 1.7 + 2.0, y * 1.7, 11.0, 3)
    z = base + m * (1.1 * r1 * corridor + 0.24 * r2 * (0.45 + 0.55 * corridor)) + 0.03 * noise.noise(V3(x * 4.0, y * 4.0, 2.0))
    for s in (-1, 1):
        c = COL + PERP * 0.8 * s
        z += 0.85 * math.exp(-((x - c.x) ** 2 + (y - c.y) ** 2) / 0.3)
    return max(z, 0.05 + 0.025 * noise.noise(V3(x * 1.5, y * 1.5, 9.0)))


def H(x, y):
    z = H0(x, y)
    if Trail.kd is None:
        return z
    co, idx, d = Trail.kd.find(V3(x, y))
    if d > 0.12:
        return z
    e = Trail.e[idx]
    return z + (e - z) * (1.0 - smoothstep(0.03, 0.11, d))


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


def trail_d(x, y):
    return Trail.kd.find(V3(x, y))[2]


def plan_trail():
    Spine.pts, Spine.kd = dense_kd(SPINE)

    def zig(y0, y1, legs, amp, phase=0):
        out = []
        for i in range(legs + 1):
            y = lerp(y0, y1, i / legs)
            out.append((spine_x(y) + (amp if (i + phase) % 2 == 0 else -amp), y))
        return out
    pts = [(-2.65, -0.45), (-2.3, 0.35), (-1.8, 0.95), (-1.5, 1.25)]
    pts += zig(1.4, 2.95, 9, 0.34)
    pts += [(-0.35, 3.3), (-0.25, 3.62), (0.45, 3.95), (0.62, 4.35), (0.25, 4.75), (0.55, 5.1), (1.05, 5.3)]
    pts += zig(5.45, 6.0, 3, 0.2, 1)
    pts += [(1.9, 6.2), (2.35, 6.72), (2.8, 7.3), (3.0, 7.7)]
    pts += zig(7.95, 9.85, 7, 0.46)
    pts += [(3.4, 10.25), (3.55, 10.65), (3.65, 11.1)]
    dense, kd = dense_kd(pts, 0.008)
    raw = [H0(p.x, p.y) for p in dense]
    win = 45
    sm = []
    for i in range(len(raw)):
        a, b = max(0, i - win), min(len(raw), i + win + 1)
        sm.append(sum(raw[a:b]) / (b - a))
    Trail.pts, Trail.kd, Trail.e, Trail.turns = dense, kd, sm, [Vector(p) for p in pts]


# ---------------------------------------------------------------------------
# Materials

def m_massif(name, P):
    import desert as D
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
    zz = nt.nodes.new('ShaderNodeMath')
    zz.operation = 'MULTIPLY_ADD'
    nt.links.new(n.outputs['Fac'], zz.inputs[0])
    zz.inputs[1].default_value = 0.25
    nt.links.new(sep.outputs['Z'], zz.inputs[2])
    col = K._mix(nt, K._maprange(nt, zz.outputs['Value'], 0.45, 0.7, 0.0, 0.7), src, P['alpine'])
    rk = K._maprange(nt, zz.outputs['Value'], 0.75, 1.0, 0.0, 1.0)
    steep = K._maprange(nt, sn.outputs['Z'], 0.55, 0.85, 1.0, 0.0)
    rmask = D._math(nt, 'MAXIMUM', rk, steep)
    vo = nt.nodes.new('ShaderNodeTexVoronoi')
    vo.feature = 'DISTANCE_TO_EDGE'
    vo.inputs['Scale'].default_value = 14.0
    nt.links.new(geo.outputs['Position'], vo.inputs['Vector'])
    rockc = K._mix(nt, K._maprange(nt, vo.outputs['Distance'], 0.0, 0.05, 0.6, 0.0), P['massrock'], (0.03, 0.03, 0.03, 1))
    col = K._mix(nt, rmask, col, rockc)
    snow = K._maprange(nt, zz.outputs['Value'], 2.15, 2.3, 0.0, 0.85)
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

def build_pass(pal, storm=False):
    plan_trail()
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
        if mass(c.y) < 0.2:
            hx.terrain = 'vforest' if noise.noise(V3(c.x * 0.4, c.y * 0.4, 6.0)) > -0.1 else 'vplains'
        Ctx.hexes[(q, r)] = hx
    village = K.nearest_hex(Vector((0.4, -0.2)))
    village.terrain, village.site = 'vplains', 'village'
    keep = K.nearest_hex(Vector((5.6, 11.4)))
    keep.terrain, keep.site = 'vplains', 'keep'
    for hx in Ctx.hexes.values():
        K.tile_mesh(hx)
    trail_mesh()
    for hx in Ctx.hexes.values():
        decorate_pass(hx)
    dress_pass(storm)


def trail_mesh():
    dense, e = Trail.pts, Trail.e
    bm = bmesh.new()
    left, right = [], []
    for i, p in enumerate(dense):
        q = dense[min(i + 1, len(dense) - 1)] - dense[max(i - 1, 0)]
        t = q.normalized() if q.length > 1e-6 else Vector((0, 1, 0))
        nrm = Vector((-t.y, t.x, 0))
        for lst, o in ((left, 0.026), (right, -0.026)):
            lst.append(bm.verts.new((p.x + nrm.x * o, p.y + nrm.y * o, e[i] + 0.006)))
    for i in range(len(left) - 1):
        bm.faces.new((left[i], right[i], right[i + 1], left[i + 1]))
    K.add(K.to_mesh(bm, 'trail', [Ctx.mats['trail']], smooth=True), (0, 0, 0), name='trail')
    # low dry-stone parapet on the drop side wherever the trail is high
    bm = bmesh.new()
    for i in range(0, len(dense) - 1, 7):
        if e[i] < 0.7:
            continue
        p = dense[i]
        q = dense[min(i + 1, len(dense) - 1)] - dense[max(i - 1, 0)]
        t = q.normalized() if q.length > 1e-6 else Vector((0, 1, 0))
        nrm = Vector((-t.y, t.x, 0))
        side = 1 if H0(p.x + nrm.x * 0.09, p.y + nrm.y * 0.09) < H0(p.x - nrm.x * 0.09, p.y - nrm.y * 0.09) else -1
        c = Vector((p.x, p.y, e[i])) + nrm * 0.034 * side
        K.bm_box(bm, (c.x, c.y, c.z + 0.011), (0.05, 0.011, 0.022), 0, rot=math.atan2(t.y, t.x))
    K.add(K.to_mesh(bm, 'parapet', [Ctx.mats['stone']]), (0, 0, 0), name='parapet')


def decorate_pass(hx):
    rnd = random.Random(hx.seed + 5)
    me = Ctx.meshes
    pal = Ctx.style['pal']
    c = hx.center
    if hx.terrain == 'massif':
        Mv = mass(c.y)
        placed = []
        for i in range(18):
            if len(placed) >= 4 + (hx.seed % 2):
                break
            lx, ly = rnd.uniform(-0.6, 0.6), rnd.uniform(-0.6, 0.6)
            x, y = c.x + lx, c.y + ly
            r = rnd.uniform(0.3, 0.52)
            if trail_d(x, y) < r * 0.75 + 0.04:
                continue
            if any((Vector((x, y)) - q).length < 0.4 for q in placed):
                continue
            h = rnd.uniform(0.55, 1.3) * Mv * Ctx.style.get('peak_scale', 1.0)
            if h < 0.25:
                continue
            ob = R.mountain_real(hx, lx, ly, h, r)
            zmin = min(H(x + math.cos(a) * r * 0.85, y + math.sin(a) * r * 0.85) for a in [i_ * 0.785 for i_ in range(8)])
            ob.location.z = min(zmin, H(x, y)) - 0.03
            placed.append(Vector((x, y)))
        ntree = 14
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
        if z > 0.6 or slope(x, y) > 1.2 or trail_d(x, y) < 0.07:
            continue
        if hx.site and math.hypot(lx, ly) < 0.5:
            continue
        ntree -= 1
        s = rnd.uniform(0.9, 1.4) * Ctx.style.get('tree_scale', 1.1)
        if rnd.random() < 0.85:
            K.add(me[f'conifer{rnd.randrange(4)}'], V3(x, y, z - 0.01), rnd.uniform(0, 6), s, K.leaf_color('conifer', rnd))
        else:
            K.add(me[f'broad{rnd.randrange(4)}'], V3(x, y, z - 0.01), rnd.uniform(0, 6), s, K.leaf_color('broad', rnd))
    for i in range(14 if hx.terrain == 'massif' else 5):
        lx, ly = rnd.uniform(-0.8, 0.8), rnd.uniform(-0.8, 0.8)
        x, y = c.x + lx, c.y + ly
        if trail_d(x, y) < 0.06:
            continue
        K.add(me[f'rock{rnd.randrange(3)}'], V3(x, y, H(x, y) - 0.012), rnd.uniform(0, 6), rnd.uniform(0.6, 1.4), K.jitter_col(pal['rock'], rnd))
    if hx.site:
        K.place_site(hx, [])
        R.place_dressing(hx)


def dress_pass(storm):
    me = Ctx.meshes
    rnd = random.Random(9)
    S = Ctx.style
    dense, e = Trail.pts, Trail.e
    acc = 0.0
    for i in range(1, len(dense)):
        acc += (dense[i] - dense[i - 1]).length
        if acc > 0.85:
            acc = 0.0
            p = dense[i]
            q = dense[min(i + 1, len(dense) - 1)] - dense[i - 1]
            t = q.normalized()
            nrm = Vector((-t.y, t.x, 0))
            side = -1 if H0(p.x + nrm.x * 0.09, p.y + nrm.y * 0.09) < H0(p.x - nrm.x * 0.09, p.y - nrm.y * 0.09) else 1
            o = Vector((p.x, p.y, e[i])) + nrm * 0.045 * side
            K.add(me['torch'], o, 0.0, 1.3)
            R.point(o + V3(0, 0, 0.16), S.get('lantern_e', 1.6), (1.0, 0.6, 0.3), 0.02)
    for tpt in Trail.turns[4:-3]:
        co, idx, d = Trail.kd.find(V3(tpt.x, tpt.y))
        g = V3(co.x + 0.05, co.y + 0.02, e[idx])
        for k in range(3):
            K.add(me[f'rock{k}'], g + V3(0, 0, 0.012 * k), rnd.uniform(0, 6), 0.5 - 0.11 * k, K.jitter_col(S['pal']['rock'], rnd))
    co, idx, d = Trail.kd.find(V3(COL.x, COL.y))
    zc = e[idx]
    t = Vector((0.6, 0.85)).normalized()
    nrm = Vector((-t.y, t.x))
    for s in (-1, 1):
        q = COL + nrm * 0.14 * s
        K.add(me['tower'], V3(q.x, q.y, min(zc, H(q.x, q.y)) - 0.03), 0.0, 1.15)
        q2 = COL + nrm * 0.32 * s
        K.add(me['wall'], V3(q2.x, q2.y, min(zc, H(q2.x, q2.y)) - 0.04), math.atan2(nrm.y, nrm.x), (0.32, 1.1, 1.1))
        R.point(V3(q.x, q.y, zc + 0.3), S.get('gate_e', 4.0), (1.0, 0.6, 0.3), 0.04)
    a = COL + nrm * 0.22 + t * 0.3
    b = COL - nrm * 0.22 + t * 0.3
    co_a, ia, _ = Trail.kd.find(V3(a.x, a.y))
    za, zb = H(a.x, a.y) + 0.2, H(b.x, b.y) + 0.2
    bm = bmesh.new()
    for ee in (a, b):
        K.bm_cone(bm, 5, 0.004, 0.004, 0.2, H(ee.x, ee.y) - 0.01, 0, x=ee.x, y=ee.y)
    cols = [lin('#2f4f86'), lin('#efe3c6'), lin('#8c2f2a'), lin('#3f7a3a'), lin('#e6c17c')]
    flag_mats = [K.m_flat(f'pf{i}', c_, 0.7) for i, c_ in enumerate(cols)]
    for k in range(13):
        f = (k + 0.5) / 13
        pp = a.lerp(b, f)
        z = lerp(za, zb, f) - 0.05 * math.sin(math.pi * f)
        vs = [bm.verts.new(v) for v in ((pp.x - 0.011, pp.y, z), (pp.x + 0.011, pp.y, z), (pp.x + 0.011, pp.y, z - 0.03), (pp.x - 0.011, pp.y, z - 0.03))]
        bm.faces.new(vs).material_index = 1 + k % 5
    K.add(K.to_mesh(bm, 'flags', [Ctx.mats['deadwood']] + flag_mats), (0, 0, 0))
    hq = Trail.turns[4 + 8]
    co, idx, d = Trail.kd.find(V3(hq.x - 0.25, hq.y + 0.12))
    K.add(me['house0'], V3(hq.x - 0.25, hq.y + 0.12, H(hq.x - 0.25, hq.y + 0.12) - 0.01), 0.4, 1.3, S['pal']['roofs'][0])


def trail_point(frac):
    i = int((len(Trail.pts) - 1) * frac)
    p = Trail.pts[i]
    return V3(p.x, p.y, Trail.e[i])


def place_pieces(hero_frac=None):
    ts = Ctx.style.get('token_scale', 1.5)
    g = trail_point(hero_frac) if hero_frac is not None else gz(-2.62, -0.35)
    K.add(Ctx.meshes['hero'], g - V3(0, 0, 0.004), 0.4, ts)
    Ctx.hero = g
    foes = [Vector((3.5, 10.4)), Vector((3.85, 10.65))]
    for f in foes:
        K.add(Ctx.meshes['enemy'], gz(f.x, f.y) - V3(0, 0, 0.01), 0.2, ts)
    wl = Vector((1.4, 11.8))
    K.add(Ctx.meshes['warlord'], gz(wl.x, wl.y) - V3(0, 0, 0.01), 0.0, ts * 1.1)
    Ctx.foes = foes


def pass_hex_badges():
    best = {}
    for i, p in enumerate(Trail.pts[::3]):
        hx = K.nearest_hex(Vector((p.x, p.y)))
        if hx.terrain != 'massif':
            continue
        d = (Vector((p.x, p.y)) - hx.center.xy).length
        k = (hx.q, hx.r)
        if k not in best or d < best[k][0]:
            best[k] = (d, V3(p.x, p.y, Trail.e[i * 3]))
    return [v[1] for v in best.values()]


def route_dots(mat=None):
    mat = mat or Ctx.mats['ring_path']
    bm = bmesh.new()
    acc = 0.0
    pts, e = Trail.pts, Trail.e
    for i in range(1, len(pts)):
        acc += (pts[i] - pts[i - 1]).length
        if acc >= 0.1:
            acc = 0.0
            K.bm_uvsphere(bm, 0.024, (pts[i].x, pts[i].y, e[i] + 0.042), 0, 8, 6)
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
    pal = nature(alpine=lin('#6a5e44'), massrock=lin('#5e554c'), trailc=lin('#b49a72'), plains=(lin('#4e5a32'), lin('#646638')))
    pal['roofs'] = [lin('#8a7448'), lin('#7a6440'), lin('#6e5a3a'), lin('#94804e')]
    g = dict(GRASS)
    g['count'] = {'vforest': 900, 'vplains': 1800}
    ov = {'ao': (0.06, 0.5, (0.25, 0.25, 0.3, 1))}
    near = lambda hx: Trail.kd.find(V3(hx.center.x, hx.center.y))[2] < 1.25
    opts = dict(grass=g, gap=0.0, thickness=1.2, bevel=0.0, window_glow=5.0, flame_k=30.0, roof='thatch',
                overlays=ov, ring_k=2.4, ring_col='#ffd27a', snow_z=0.6, peak_scale=1.0, light_mult=1.0, lantern_e=1.8, gate_e=4.0,
                res_fn=lambda hx: 72 if near(hx) else 36 if (hx.center.xy - Vector((0.5, 5.0))).length < 8.5 else 20)
    setup(pal, **opts)
    build_pass(pal, storm)


def dawn_light(valley=1.2, clouds=True, sun_el=8, sun_az=95, haze=True):
    K.fog_box((0, 6, 0.08), (44, 34, 0.5), valley * 0.75, lin('#ece0ea'), 0.0, clumps=0.7, fade_z=(-0.05, 0.28))
    if clouds:
        K.fog_box((0, 7, 2.15), (44, 34, 0.5), 0.45, lin('#f4e6ea'), 0.0, clumps=0.55, fade_z=(1.9, 2.4))
    if haze:
        K.fog_box((0, 10, 1.6), (60, 40, 4.0), 0.012, lin('#f0c8c0'), 0.5)
    R.sky_world(2, 260, strength=0.55, camera_bg=[lin('#f2c2a8'), lin('#6a7aa8')])
    K.sun(sun_az, sun_el, 4.8, (1.0, 0.64, 0.46), 1.0)


CINE = ((0.3, 4.4, 0.9), -10, 21, 12.5, 36)
PLAY = ((0.5, 5.0, 0.9), -4, 55, 15.0, 36)
TOP = ((1.1, 5.3, 0.8), 0, 80, 16.5, 36)
SIDE = ((-0.8, 2.3, 0.7), 38, 9, 6.4, 30)
MID = ((0.4, 5.0, 0.9), -6, 38, 13.5, 36)


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


def badges_px():
    return [K.project(p + V3(0, 0, 0.12)) for p in pass_hex_badges()]


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
    route_dots()
    dawn_light(0.9)
    cam(PLAY)
    dump({'badges': badges_px()})
    render()


def s03():
    setup_pass()
    place_pieces()
    R.apply_overlays()
    route_dots()
    dawn_light(0.7, clouds=False, sun_el=22)
    cam(TOP)
    dump({'badges': badges_px()})
    render()


def s04():
    setup_pass()
    place_pieces(hero_frac=0.36)
    R.apply_overlays()
    dawn_light(0.5, clouds=False, sun_az=175, sun_el=12, haze=False)
    cam(SIDE)
    dump()
    render(samples=192)


def s05():
    setup_pass()
    place_pieces(hero_frac=0.405)
    R.apply_overlays()
    dawn_light(0.5, clouds=False, sun_az=170, sun_el=12, haze=False)
    h = Ctx.hero
    cam(((h.x + 0.15, h.y + 0.4, h.z + 0.05), -48, 19, 3.8, 32))
    dump()
    render(samples=192)


def s06():
    setup_pass()
    place_pieces()
    R.apply_overlays()
    route_dots()
    col_i = Trail.kd.find(V3(COL.x, COL.y))[1]
    seg = [V3(Trail.pts[i].x, Trail.pts[i].y, Trail.e[i] + 0.03) for i in range(max(0, col_i - 140), min(len(Trail.pts), col_i + 90), 3)]
    bm = bmesh.new()
    __import__('desert').tube(bm, seg, [0.017] * len(seg), 8, 0)
    K.add(K.to_mesh(bm, 'hover_trail', [K.m_emit('hover', lin('#fff1c9'), 9.0)], smooth=True), (0, 0, 0))
    cam(MID)
    pts = [V3(p.x, p.y, Trail.e[i * 12] + 0.05) for i, p in enumerate(Trail.pts[::12])]
    pts += [gz(f.x, f.y) + V3(0, 0, 0.15) for f in Ctx.foes]
    occ = occluders(pts)
    ghost(occ, 0.34)
    dawn_light(1.0)
    dump({'faded': len(occ), 'cursor': K.project(V3(COL.x, COL.y, Trail.e[col_i] + 0.05)), 'badges': badges_px()})
    render()
    route = [o for o in bpy.context.scene.objects if o.type == 'MESH' and o.data.name.split('.')[0] in ('dots', 'hover_trail')]
    render_masks({'foe': tokens('foe'), 'route': route})


SCENES = {f'{i:02d}': globals()[f's{i:02d}'] for i in range(1, 7)}
K.reset()
SCENES[CID]()
