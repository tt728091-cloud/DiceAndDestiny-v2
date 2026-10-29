"""Biome extension for the set-3 concepts: new terrain types, props and a
generic map builder over seven-hex flowers. Layers on hexkit + real."""
import bpy, bmesh, math, random
from mathutils import Vector, Matrix, noise
import hexkit as K
import real as R
from hexkit import Ctx, lin, smoothstep, lerp, edge_dist, faces_of, to_mesh, bm_cone, bm_box, bm_ico, add, link, WATER, hex_world, catmull, seg_dist

ALIAS = {'desert': 'plains', 'mesa': 'plains', 'tundra': 'plains', 'moor': 'hills', 'ash': 'plains', 'beach': 'plains',
         'farm': 'plains', 'deepforest': 'forest', 'peaks': 'mountain', 'volcano': 'mountain', 'city': 'plains',
         'swamp': 'marsh', 'oasis': 'lake', 'icelake': 'lake', 'badlands': 'hills', 'snowforest': 'forest'}
FLOWER = [(0, 0), (1, 0), (1, -1), (0, -1), (-1, 0), (-1, 1), (0, 1)]


def make_hex(q, r, terrain, site=None, enemies=0):
    base = ALIAS.get(terrain, terrain)
    hx = K.Hex(q, r, base, site, enemies, 'biome')
    hx.base = base
    hx.terrain = terrain
    return hx


# --------------------------------------------------------------------------
# Heights

_prev_height = K.Hex.height


def height(self, wx, wy):
    t = self.terrain
    base = getattr(self, 'base', t)
    if base != t:
        self.terrain = base
        try:
            z = _prev_height(self, wx, wy)
        finally:
            self.terrain = t
    else:
        z = _prev_height(self, wx, wy)
    if base == t:
        return z
    S = Ctx.style
    p = Vector((wx - self.center.x, wy - self.center.y))
    ed = edge_dist(p, 1.0 - S['gap'])
    extra = 0.0
    if t in ('desert', 'mesa'):
        th = S.get('wind', 0.6)
        u = (wx * math.cos(th) + wy * math.sin(th)) * 5.5 + 1.4 * noise.noise(Vector((wx * 0.55, wy * 0.55, 5.0)))
        d = (0.5 + 0.5 * math.sin(u)) ** 2.4
        amp = 0.13 if t == 'desert' else 0.04
        extra = amp * d * (0.6 + 0.4 * noise.noise(Vector((wx * 0.9, wy * 0.9, 1.0)))) * smoothstep(0.0, 0.22, ed)
    elif t == 'moor':
        z *= 0.55
    elif t == 'badlands':
        z *= 1.3
    elif t in ('tundra', 'ash'):
        extra = 0.035 * noise.fractal(Vector((wx * 2.0, wy * 2.0, 7.0)), 0.6, 2.0, 4) * smoothstep(0.0, 0.25, ed)
    elif t == 'beach':
        z *= 0.4
    z += extra
    if self.segs and S.get('rivers', True):
        rd = self.river_dist(wx, wy)
        z = min(z, lerp(-0.085, 0.03, smoothstep(0.075, 0.2, rd)))
    return z


K.Hex.height = height


# --------------------------------------------------------------------------
# Materials

def m_sand(name, c1, c2, c3, ripple=0.5, low=None):
    m = R.m_ground(name, c1, c2, c3, (c1[0] * 0.8, c1[1] * 0.75, c1[2] * 0.65, 1), dirt_amt=0.15, low=low, micro=0.25)
    nt = m.node_tree
    p = nt.nodes['Principled BSDF']
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    w = nt.nodes.new('ShaderNodeTexWave')
    w.inputs['Scale'].default_value = 60.0
    w.inputs['Distortion'].default_value = 3.0
    w.inputs['Detail'].default_value = 2.0
    nt.links.new(geo.outputs['Position'], w.inputs['Vector'])
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = ripple
    b.inputs['Distance'].default_value = 0.003
    nt.links.new(w.outputs['Fac'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


def m_sandstone(name, c1, c2):
    m, nt, p = K._new(name)
    tc = nt.nodes.new('ShaderNodeTexCoord')
    w = nt.nodes.new('ShaderNodeTexWave')
    w.bands_direction = 'Z'
    w.inputs['Scale'].default_value = 2.6
    w.inputs['Distortion'].default_value = 7.0
    w.inputs['Detail'].default_value = 6.0
    nt.links.new(tc.outputs['Object'], w.inputs['Vector'])
    c = K._mix(nt, K._maprange(nt, w.outputs['Fac'], 0.25, 0.85, 0.0, 0.65), c1, c2)
    n = K._noise(nt, 40.0, 6.0)
    c = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.4, 0.7, 0.0, 0.3), c, (c1[0] * 0.5, c1[1] * 0.45, c1[2] * 0.4, 1))
    nt.links.new(c, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.92)
    h = nt.nodes.new('ShaderNodeMath')
    h.operation = 'ADD'
    nt.links.new(w.outputs['Fac'], h.inputs[0])
    nt.links.new(n.outputs['Fac'], h.inputs[1])
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = 0.6
    b.inputs['Distance'].default_value = 0.01
    nt.links.new(h.outputs['Value'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


def m_ash(name, c1, c2, ember=None, k=4.0):
    m = R.m_ground(name, c1, c2, (c1[0] * 1.4, c1[1] * 1.3, c1[2] * 1.3, 1), (0.02, 0.02, 0.02, 1), dirt_amt=0.4, micro=0.6)
    if ember:
        nt = m.node_tree
        p = nt.nodes['Principled BSDF']
        geo = nt.nodes.new('ShaderNodeNewGeometry')
        vo = nt.nodes.new('ShaderNodeTexVoronoi')
        vo.feature = 'DISTANCE_TO_EDGE'
        vo.inputs['Scale'].default_value = 5.0
        nt.links.new(geo.outputs['Position'], vo.inputs['Vector'])
        line = K._maprange(nt, vo.outputs['Distance'], 0.0, 0.025, 1.0, 0.0)
        mask = K._maprange(nt, K._noise(nt, 1.1, 2.0).outputs['Fac'], 0.58, 0.66)
        mul = nt.nodes.new('ShaderNodeMath')
        mul.operation = 'MULTIPLY'
        nt.links.new(line, mul.inputs[0])
        nt.links.new(mask, mul.inputs[1])
        em = K._mix(nt, mul.outputs['Value'], (0, 0, 0, 1), ember)
        nt.links.new(em, p.inputs['Emission Color'])
        p.inputs['Emission Strength'].default_value = k
    return m


def biome_materials():
    S = Ctx.style
    P = S['pal']
    M = Ctx.mats
    sand = P.get('sand_desert', (lin('#b8945e'), lin('#c8a46c'), lin('#a07e4c')))
    M['desert'] = m_sand('desert', *sand)
    red = P.get('sand_red', (lin('#a8683e'), lin('#b8784a'), lin('#8e5634')))
    M['mesa'] = m_sand('mesa_ground', *red, ripple=0.3)
    M['badlands'] = R.m_ground('badlands', red[0], red[1], red[2], P['mud'], rock=lin('#7a4a30'), slope=(0.55, 0.78))
    M['sandstone'] = m_sandstone('sandstone', P.get('sandstone', (lin('#a05a34'), lin('#c88a56')))[0], P.get('sandstone', (lin('#a05a34'), lin('#c88a56')))[1])
    snow = P.get('snowground', (lin('#e6eaee'), lin('#f4f6f8'), lin('#cfd8e2')))
    M['tundra'] = R.m_ground('tundra', snow[0], snow[1], snow[2], lin('#a8b4c0'), dirt_amt=0.2, rock=lin('#6a6e72'), slope=(0.5, 0.7), micro=0.3)
    M['icelake'] = R.m_ground('icelake', snow[0], snow[1], snow[2], lin('#a8b4c0'), dirt_amt=0.2, low=lin('#b8c4cc'), micro=0.3)
    M['icelake_water'] = K.m_flat('ice2', lin('#b6ccd8'), 0.18, spec=0.7)
    M['icelake_water']['no_overlay'] = True
    heather = P.get('heather', (lin('#5a4a3e'), lin('#6a5442'), lin('#5e3e52')))
    M['moor'] = R.m_ground('moor', heather[0], heather[1], heather[2], P['mud'], rock=P['hill_rock'], slope=(0.6, 0.8))
    ash = P.get('ashground', (lin('#1e1c1c'), lin('#2c2826')))
    M['ash'] = m_ash('ash', ash[0], ash[1], P.get('ember'), S.get('ember_k', 4.0))
    M['volcano'] = M['ash']
    beach = P.get('beachsand', (lin('#b8aa86'), lin('#c8ba96'), lin('#a49674')))
    M['beach'] = m_sand('beach', *beach, ripple=0.2)
    M['oasis'] = R.m_ground('oasis', sand[0], sand[1], sand[2], P['mud'], low=lin('#5a6a3a'), low_z=(-0.08, -0.02), deep=lin('#1e4a4a'))
    M['oasis_water'] = K.m_water('oasis_water', lin('#2e8a86'), trans=0.6)
    M['oasis_water']['no_overlay'] = True
    M['farm'] = M['plains']
    M['city'] = M['plains']
    M['deepforest'] = R.m_ground('deepforest', P['forest'][0], P['forest'][1], lin('#2e3a22'), P['mud'], dirt_amt=0.6)
    M['snowforest'] = M['tundra']
    M['peaks'] = M['mountain']
    M['swamp'] = M['marsh']
    M['palm_trunk'] = K.m_flat('palm_trunk', lin('#6a5238'), 0.9)
    M['frond'] = K.m_flat('frond', (1, 1, 1, 1), 0.7, obj_color=True)
    M['ice_shard'] = K.m_flat('ice_shard', lin('#c8e0ec'), 0.1, spec=0.8)
    M['lamp'] = K.m_emit('lamp', lin('#ffd890'), 40.0)
    M['sail'] = K.m_flat('sail', lin('#c8bca0'), 0.9)
    M['lava'] = K.m_emit('lava_b', lin('#ff4a10'), 5.0)


# --------------------------------------------------------------------------
# Props

def build_biome_props():
    M, me = Ctx.mats, Ctx.meshes
    rnd = random.Random(31)
    for k in range(3):
        bm = bmesh.new()
        lean = Vector((rnd.uniform(-0.25, 0.25), rnd.uniform(-0.25, 0.25), 1)).normalized()
        pos = Vector((0, 0, 0))
        r = 0.009
        for i in range(7):
            d = (lean + Vector((0, 0, 0.12 * i))).normalized()
            R.branch(bm, pos, d, 0.032, r, 0, rnd)
            pos = pos + d * 0.03
            r *= 0.92
        for j in range(8):
            a = j / 8 * 6.283 + rnd.uniform(-0.2, 0.2)
            dirv = Vector((math.cos(a), math.sin(a), 0))
            side = Vector((-dirv.y, dirv.x, 0)) * 0.012
            pts = [pos]
            for s in range(1, 5):
                t = s / 4
                pts.append(pos + dirv * 0.085 * t + Vector((0, 0, 0.03 * t - 0.07 * t * t)))
            vs_l = [bm.verts.new(p + side * math.sin(math.pi * i / 4)) for i, p in enumerate(pts)]
            vs_r = [bm.verts.new(p - side * math.sin(math.pi * i / 4)) for i, p in enumerate(pts)]
            for i in range(4):
                f = bm.faces.new((vs_l[i], vs_l[i + 1], vs_r[i + 1], vs_r[i]))
                f.material_index = 1
        me[f'palm{k}'] = to_mesh(bm, f'palm{k}', [M['palm_trunk'], M['frond']])
    bm = bmesh.new()
    bm_cone(bm, 8, 0.045, 0.03, 0.18, 0.0, 0)
    bm_cone(bm, 8, 0.04, 0.0, 0.05, 0.18, 1)
    for j in range(4):
        a = j / 4 * 6.283 + 0.4
        c = Vector((0, -0.045, 0.16))
        mat = Matrix.Translation(c) @ Matrix.Rotation(a, 4, 'Y') @ Matrix.Translation((0, 0, 0.055))
        ret = bmesh.ops.create_cube(bm, size=1.0, matrix=mat @ Matrix.Diagonal((0.018, 0.003, 0.11, 1)))
        for f in faces_of(ret['verts']):
            f.material_index = 2
    me['windmill'] = to_mesh(bm, 'windmill', [M['wall'], M['roof'], M['sail']])
    bm = bmesh.new()
    bm_cone(bm, 12, 0.05, 0.035, 0.42, 0.0, 0)
    bm_cone(bm, 12, 0.045, 0.045, 0.04, 0.42, 1)
    bm_cone(bm, 12, 0.05, 0.0, 0.05, 0.46, 2)
    for b_ in range(3):
        bm_box(bm, (0, 0, 0.08 + b_ * 0.13), (0.1, 0.1, 0.03), 2)
    me['lighthouse'] = to_mesh(bm, 'lighthouse', [M['wall'], M['lamp'], M['roof_dark']])
    for k in range(3):
        bm = bmesh.new()
        n = rnd.randint(3, 5)
        for i in range(n):
            a = rnd.uniform(0, 6.28)
            rr = rnd.uniform(0, 0.03)
            bm_cone(bm, 5, rnd.uniform(0.012, 0.02), 0.0, rnd.uniform(0.06, 0.13), -0.01, 0, x=math.cos(a) * rr, y=math.sin(a) * rr,
                    tilt=(rnd.uniform(-0.35, 0.35), rnd.uniform(-0.35, 0.35)))
        me[f'shard{k}'] = to_mesh(bm, f'shard{k}', [M['ice_shard']])
    bm = bmesh.new()
    R.branch(bm, Vector((-0.06, 0, 0.006)), Vector((1, 0.2, 0.05)).normalized(), 0.12, 0.006, 1, rnd)
    me['driftwood'] = to_mesh(bm, 'driftwood', [M['deadwood']], smooth=True)
    for k in range(2):
        me[f'stack{k}'] = R.rock_mesh(rnd, 0.1)


def mesa_obj(hx, lx, ly, r, h, rnd):
    bm = bmesh.new()
    n = 16
    pts = []
    for i in range(n):
        a = i / n * 6.283
        rr = r * (0.75 + 0.35 * noise.noise(Vector((math.cos(a) * 1.7, math.sin(a) * 1.7, rnd.uniform(0, 9)))) + rnd.uniform(-0.05, 0.05))
        pts.append(bm.verts.new((math.cos(a) * rr, math.sin(a) * rr, -0.06)))
    f = bm.faces.new(pts)
    ext = bmesh.ops.extrude_face_region(bm, geom=[f])
    top = [g for g in ext['geom'] if isinstance(g, bmesh.types.BMVert)]
    for v in top:
        v.co.z = h * 0.62
        v.co.x *= 0.9
        v.co.y *= 0.9
    topf = [g for g in ext['geom'] if isinstance(g, bmesh.types.BMFace)]
    capf = [fc for fc in bm.faces if all(v in top for v in fc.verts)]
    ins = bmesh.ops.inset_region(bm, faces=capf, thickness=r * 0.22, depth=0.0)
    ext2 = bmesh.ops.extrude_face_region(bm, geom=capf)
    for v in [g for g in ext2['geom'] if isinstance(g, bmesh.types.BMVert)]:
        v.co.z = h
        v.co.x *= 0.95
        v.co.y *= 0.95
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    for fc in bm.faces:
        fc.material_index = 1 if fc.normal.z > 0.8 else 0
    bmesh.ops.subdivide_edges(bm, edges=[e for e in bm.edges if abs(e.verts[0].co.z - e.verts[1].co.z) > 0.05], cuts=4)
    for v in bm.verts:
        if v.co.z > -0.05:
            k = 0.035 * (0.5 + v.co.z)
            v.co.x += noise.noise(v.co * 11) * k
            v.co.y += noise.noise(v.co * 11 + Vector((3, 3, 3))) * k
    me = to_mesh(bm, 'mesa', [Ctx.mats['sandstone'], Ctx.mats['mesa']])
    return add(me, hx.ground(lx, ly), rnd.uniform(0, 6.28), name='mesa')


def volcano_obj(hx, h=1.5, r=0.85):
    rnd = hx.rnd
    rings, segs = 26, 56
    seed = Vector((rnd.uniform(0, 40), rnd.uniform(0, 40), 0))
    bm = bmesh.new()
    grid = []
    crater = 0.16
    for k in range(rings + 1):
        t = k / rings
        ring = []
        for i in range(segs):
            a = 2 * math.pi * i / segs
            rad = r * (crater + (1 - crater) * t) * (1 + 0.12 * noise.noise(Vector((math.cos(a) * 2, math.sin(a) * 2, 0)) + seed))
            x, y = math.cos(a) * rad, math.sin(a) * rad
            z = h * (1 - t) ** 1.35 * (0.85 + 0.25 * noise.ridged_multi_fractal(Vector((x * 3, y * 3, 0)) + seed, 0.9, 2.0, 5, 1.0, 2.0) / 2.0)
            if k == rings:
                z = -0.04
            ring.append(bm.verts.new((x, y, z)))
        grid.append(ring)
    inner = [bm.verts.new((v.co.x * 0.7, v.co.y * 0.7, v.co.z - 0.12)) for v in grid[0]]
    for i in range(segs):
        j = (i + 1) % segs
        bm.faces.new((inner[i], inner[j], grid[0][j], grid[0][i]))
    bm.faces.new(list(inner))
    for k in range(rings):
        A, B = grid[k], grid[k + 1]
        for i in range(segs):
            j = (i + 1) % segs
            bm.faces.new((A[i], B[i], B[j], A[j]))
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    for fc in bm.faces:
        fc.material_index = 1 if all(v in inner for v in fc.verts) else 0
    me = to_mesh(bm, 'volcano', [Ctx.mats['mtn'], Ctx.mats['lava']], smooth=True)
    g = hx.ground(0, 0)
    ob = add(me, g, 0.0, name='volcano')
    top = g + Vector((0, 0, h - 0.05))
    R.point(top + Vector((0, 0, 0.2)), 400.0, (1.0, 0.4, 0.12), 0.3)
    R.smoke(top + Vector((0, 0, 0.1)), 0.45, 3.5, 2.5, (0.08, 0.07, 0.07, 1), emit=(lin('#ff5a14'), 0.6))
    return ob


# --------------------------------------------------------------------------
# Decoration per biome terrain

def _site(hx, taken):
    if hx.site:
        K.place_site(hx, taken)
    R.place_dressing(hx)


def deco_desert(hx):
    rnd = hx.rnd
    taken = []
    K.rocks(hx, 3, taken, scale=0.9)
    for g in K.scatter(hx, 4, 0.12, taken=taken, avoid_center=0.3 if hx.site else 0.0):
        add(Ctx.meshes[f'dead{rnd.randrange(3)}'], g, rnd.uniform(0, 6), rnd.uniform(0.35, 0.55))
    if rnd.random() < 0.25:
        add(Ctx.meshes['skull'], hx.ground(rnd.uniform(-0.4, 0.4), rnd.uniform(-0.4, 0.4)), rnd.uniform(0, 6), 1.6)
    _site(hx, taken)


def deco_mesa(hx):
    rnd = hx.rnd
    taken = [Vector((0, 0))]
    for i in range(rnd.randint(1, 2)):
        a = rnd.uniform(0, 6.28)
        d = rnd.uniform(0.0, 0.3) if i == 0 else 0.45
        mesa_obj(hx, math.cos(a) * d, math.sin(a) * d, rnd.uniform(0.2, 0.32), rnd.uniform(0.35, 0.75), rnd)
    K.rocks(hx, 4, taken, scale=1.1)
    _site(hx, taken)


def deco_oasis(hx):
    rnd = hx.rnd
    rr = 1.0 - Ctx.style['gap']
    for i in range(9):
        a = rnd.uniform(0, 6.28)
        d = rnd.uniform(0.5, 0.66)
        p = Vector((math.cos(a) * d, math.sin(a) * d))
        if edge_dist(p, rr) < 0.08:
            continue
        add(Ctx.meshes[f'palm{rnd.randrange(3)}'], hx.ground(p.x, p.y) - Vector((0, 0, 0.01)), rnd.uniform(0, 6), rnd.uniform(1.0, 1.5), K.leaf_color('broad', rnd))
    for i in range(6):
        a = rnd.uniform(0, 6.28)
        p = Vector((math.cos(a) * 0.48, math.sin(a) * 0.48))
        add(Ctx.meshes['reeds'], hx.ground(p.x, p.y) - Vector((0, 0, 0.01)), rnd.uniform(0, 6), 1.1)
    _site(hx, [])


def deco_tundra(hx):
    rnd = hx.rnd
    taken = []
    K.rocks(hx, 4, taken, scale=1.0)
    K.plant_trees(hx, 4, mix=0.95, taken=taken, avoid=0.35 if hx.site else 0.0)
    for g in K.scatter(hx, 3, 0.15, taken=taken):
        add(Ctx.meshes[f'shard{rnd.randrange(3)}'], g, rnd.uniform(0, 6), rnd.uniform(1.0, 2.0))
    _site(hx, taken)


def deco_snowforest(hx):
    taken = []
    K.plant_trees(hx, int(34 * Ctx.style.get('density', 1.0)), mix=0.95, taken=taken, avoid=0.36 if hx.site else 0.0, min_dist=0.085)
    _site(hx, taken)


def deco_icelake(hx):
    rnd = hx.rnd
    for i in range(6):
        a = rnd.uniform(0, 6.28)
        p = Vector((math.cos(a) * 0.62, math.sin(a) * 0.62))
        add(Ctx.meshes[f'rock{rnd.randrange(3)}'], hx.ground(p.x, p.y) - Vector((0, 0, 0.012)), rnd.uniform(0, 6), rnd.uniform(0.8, 1.4), K.jitter_col(Ctx.style['pal']['rock'], rnd))
    for i in range(3):
        a = rnd.uniform(0, 6.28)
        p = Vector((math.cos(a) * 0.25, math.sin(a) * 0.25))
        add(Ctx.meshes[f'shard{rnd.randrange(3)}'], hx.ground(p.x, p.y) + Vector((0, 0, 0.0)), rnd.uniform(0, 6), rnd.uniform(1.0, 1.8))


def deco_moor(hx):
    rnd = hx.rnd
    taken = []
    P = Ctx.style['pal']
    for g in K.scatter(hx, int(22 * Ctx.style.get('density', 1.0)), 0.06, taken=taken, avoid_center=0.3 if hx.site else 0.0):
        add(Ctx.meshes['bush'], g - Vector((0, 0, 0.006)), rnd.uniform(0, 6), rnd.uniform(0.7, 1.3), K.jitter_col(rnd.choice(P.get('heather_bush', [lin('#5a2e4a'), lin('#6a3a56'), lin('#4a3a2a')])), rnd))
    K.rocks(hx, 3, taken)
    if rnd.random() < 0.35 and not hx.site:
        for i in range(7):
            a = i / 7 * 6.283
            p = Vector((math.cos(a) * 0.22, math.sin(a) * 0.22))
            add(Ctx.meshes['menhir'], hx.ground(p.x, p.y) - Vector((0, 0, 0.02)), a, (1.3, 1.3, rnd.uniform(1.0, 1.7)), tilt=(rnd.uniform(-0.1, 0.1), rnd.uniform(-0.1, 0.1)))
    elif rnd.random() < 0.4:
        K.plant_trees(hx, 2, mix=0.2, taken=taken, dead=0.7)
    _site(hx, taken)


def deco_ash(hx):
    rnd = hx.rnd
    taken = []
    K.rocks(hx, 6, taken, scale=1.2)
    for g in K.scatter(hx, 3, 0.15, taken=taken):
        add(Ctx.meshes[f'dead{rnd.randrange(3)}'], g, rnd.uniform(0, 6), rnd.uniform(0.8, 1.3))
    if rnd.random() < 0.18:
        g = hx.ground(rnd.uniform(-0.4, 0.4), rnd.uniform(-0.4, 0.4))
        R.smoke(g, 0.16, 1.2, 1.0, (0.75, 0.75, 0.75, 1))
    _site(hx, taken)


def deco_beach(hx):
    rnd = hx.rnd
    taken = []
    for g in K.scatter(hx, 3, 0.15, taken=taken):
        add(Ctx.meshes['driftwood'], g, rnd.uniform(0, 6), rnd.uniform(1.0, 1.6))
    K.rocks(hx, 4, taken)
    if rnd.random() < 0.3:
        add(Ctx.meshes['boat'], hx.ground(0.1, -0.1) - Vector((0, 0, 0.02)), rnd.uniform(0, 6), 1.6, tilt=(0.15, 0.1))
    _site(hx, taken)


def deco_peaks(hx):
    rnd = hx.rnd
    peaks = [(0.0, 0.08, 1.75, 0.66), (-0.38, -0.15, 1.05, 0.44), (0.36, -0.22, 0.95, 0.42), (0.25, 0.42, 0.85, 0.38)]
    for (x, y, h, r) in peaks[: 3 + (hx.seed % 2)]:
        if hx.river_dist(hx.center.x + x, hx.center.y + y) < r * 0.7:
            continue
        R.mountain_real(hx, x, y, h * rnd.uniform(0.85, 1.12) * Ctx.style.get('peak_scale', 1.0), r)
    K.rocks(hx, 6, [], scale=1.3)


def deco_volcano(hx):
    volcano_obj(hx, Ctx.style.get('volcano_h', 1.6))
    K.rocks(hx, 6, [], scale=1.4)


def deco_city(hx):
    rnd = hx.rnd
    me = Ctx.meshes
    P = Ctx.style['pal']
    ring = 0.62
    for i in range(10):
        if i == 7:
            continue
        a0, a1 = i / 10 * 6.283, (i + 1) / 10 * 6.283
        p0 = Vector((math.cos(a0) * ring, math.sin(a0) * ring))
        p1 = Vector((math.cos(a1) * ring, math.sin(a1) * ring))
        m = (p0 + p1) / 2
        seg = p1 - p0
        add(me['wall'], hx.ground(m.x, m.y) - Vector((0, 0, 0.02)), math.atan2(seg.y, seg.x), (seg.length, 1.4, 1.5))
        add(me['tower'], hx.ground(p0.x, p0.y) - Vector((0, 0, 0.02)), 0.0, 1.0)
    if hx.site in (None, 'town'):
        add(me['keep'], hx.ground(0.12, 0.18) - Vector((0, 0, 0.02)), 0.2, 1.1)
        add(me['church'], hx.ground(-0.25, -0.1), 0.5, 1.6)
    for i in range(34):
        a = rnd.uniform(0, 6.28)
        d = rnd.uniform(0.12, 0.55)
        lp = Vector((math.cos(a) * d, math.sin(a) * d))
        if (lp - Vector((0.12, 0.18))).length < 0.2 or (lp - Vector((-0.25, -0.1))).length < 0.15:
            continue
        add(me[f'house{rnd.randrange(3)}'], hx.ground(lp.x, lp.y) - Vector((0, 0, 0.01)), a + math.pi / 2 + rnd.uniform(-0.2, 0.2), rnd.uniform(1.15, 1.45), rnd.choice(P['roofs']))
    R.place_dressing(hx)


def deco_farm(hx):
    rnd = hx.rnd
    me = Ctx.meshes
    taken = []
    ang = rnd.uniform(0, math.pi)
    for g in K.scatter(hx, 6, 0.26, taken=taken, margin=0.16, river_clear=0.26, avoid_center=0.3 if hx.site else 0.0):
        add(me['furrow_patch'], g - Vector((0, 0, 0.002)), ang + rnd.uniform(-0.1, 0.1), (rnd.uniform(0.3, 0.4), rnd.uniform(0.2, 0.26), 1.0), K.jitter_col(rnd.choice(Ctx.style['pal']['field_cols']), rnd, 0.08))
    for k in range(2):
        a = ang + math.pi / 2 * k
        d = Vector((math.cos(a), math.sin(a)))
        for i in range(-6, 7):
            p = d * i * 0.07 + Vector((-d.y, d.x)) * rnd.uniform(-0.3, 0.3) * 0
            if K.edge_dist(p, 0.95) > 0.06 and hx.river_dist(hx.center.x + p.x, hx.center.y + p.y) > 0.16:
                add(me['bush'], hx.ground(p.x, p.y) - Vector((0, 0, 0.006)), rnd.uniform(0, 6), rnd.uniform(0.9, 1.2), K.leaf_color('broad', rnd))
    if not hx.site and rnd.random() < 0.3:
        add(me['windmill'], hx.ground(0.35, 0.3), rnd.uniform(0, 6), 1.4)
        add(me['house0'], hx.ground(0.15, 0.4), 0.4, 1.3, rnd.choice(Ctx.style['pal']['roofs']))
    for g in K.scatter(hx, 3, 0.1, taken=taken):
        add(me['haystack'], g, 0, rnd.uniform(1.1, 1.5))
    _site(hx, taken)


def deco_deepforest(hx):
    taken = []
    S = Ctx.style
    K.plant_trees(hx, int(46 * S.get('density', 1.0)), mix=S.get('conifer_mix', 0.6), taken=taken, avoid=0.36 if hx.site else 0.0,
                  dead=S.get('dead_mix', 0.05), min_dist=0.075)
    K.rocks(hx, 3, taken, scale=0.8)
    _site(hx, taken)


CUSTOM = {'desert': deco_desert, 'mesa': deco_mesa, 'oasis': deco_oasis, 'tundra': deco_tundra, 'icelake': deco_icelake,
          'moor': deco_moor, 'ash': deco_ash, 'beach': deco_beach, 'peaks': deco_peaks, 'volcano': deco_volcano,
          'city': deco_city, 'farm': deco_farm, 'deepforest': deco_deepforest, 'snowforest': deco_snowforest,
          'badlands': deco_mesa}

_prev_decorate = K.decorate


def decorate(hx):
    t = hx.terrain
    if t in CUSTOM:
        CUSTOM[t](hx)
        return
    base = getattr(hx, 'base', t)
    if base != t:
        hx.terrain = base
        try:
            _prev_decorate(hx)
        finally:
            hx.terrain = t
    else:
        _prev_decorate(hx)
    if t == 'swamp':
        K.plant_trees(hx, 5, mix=0.1, dead=0.8)


K.decorate = decorate


# --------------------------------------------------------------------------
# Map building

def lattice(xr, yr):
    out = []
    for a in range(-8, 9):
        for b in range(-6, 7):
            q, r = 2 * a - b, a + 3 * b
            w = hex_world(q, r)
            if xr[0] <= w.x <= xr[1] and yr[0] <= w.y <= yr[1]:
                out.append((q, r))
    return out


def gen_cells(xr, yr, terrain_fn, seed=1):
    rnd = random.Random(seed)
    cells = {}
    for (cq, cr) in lattice(xr, yr):
        for (dq, dr) in FLOWER:
            q, r = cq + dq, cr + dr
            cells[(q, r)] = [terrain_fn(hex_world(q, r), rnd), None, 0]
    return cells


def nearest(cells, x, y, allowed=None, exclude=()):
    best, bd = None, 1e9
    for (q, r), c in cells.items():
        if allowed and c[0] not in allowed:
            continue
        if (q, r) in exclude:
            continue
        w = hex_world(q, r)
        d = (w.x - x) ** 2 + (w.y - y) ** 2
        if d < bd:
            best, bd = (q, r), d
    return best


def put(cells, x, y, terrain=None, site=None, enemies=None, allowed=None):
    k = nearest(cells, x, y, allowed)
    if k is None:
        return None
    if terrain:
        cells[k][0] = terrain
    if site is not None:
        cells[k][1] = site
    if enemies is not None:
        cells[k][2] = enemies
    return k


def build_map(cells, rivers=(), hero=None, enemies=(), warlord=None, sea=True, sea_z=None):
    S = Ctx.style
    Ctx.rivers = [catmull([Vector((x, y, 0)) for (x, y) in r]) for r in rivers] if S.get('rivers', True) else []
    for (q, r), (t, site, en) in cells.items():
        Ctx.hexes[(q, r)] = make_hex(q, r, t, site, en)
    for hx in Ctx.hexes.values():
        for poly in Ctx.rivers:
            for a, b in zip(poly, poly[1:]):
                if seg_dist(Vector((hx.center.x, hx.center.y)), a, b) < 1.3:
                    hx.segs.append((a, b))
    for hx in Ctx.hexes.values():
        K.tile_mesh(hx)
        decorate(hx)
    ts = S.get('token_scale', 1.5)
    if hero:
        hx = Ctx.hexes[hero]
        g = hx.ground(0.15, -0.2)
        add(Ctx.meshes['hero'], g, 0.4, ts)
        Ctx.hero = g
    for k in enemies:
        hx = Ctx.hexes[k]
        for (x, y) in ((-0.15, -0.05), (0.15, -0.1)):
            add(Ctx.meshes['enemy'], hx.ground(x, y), hx.rnd.uniform(-0.3, 0.3), ts)
    if warlord:
        hx = Ctx.hexes[warlord]
        add(Ctx.meshes['warlord'], hx.ground(-0.05, -0.1), 0.0, ts * 1.1)
    T = S['thickness']
    if sea:
        bm = bmesh.new()
        bmesh.ops.create_grid(bm, x_segments=1, y_segments=1, size=80)
        add(to_mesh(bm, 'sea', [Ctx.mats['sea']]), (0, 0, sea_z if sea_z is not None else -T * 0.55), name='sea')
        bm = bmesh.new()
        bmesh.ops.create_grid(bm, x_segments=1, y_segments=1, size=80)
        add(to_mesh(bm, 'seabed', [Ctx.mats['seabed']]), (0, 0, -T - 0.6), name='seabed')


def stacks(n, xr, yr, z, seed=3, scale=(1.0, 2.0)):
    rnd = random.Random(seed)
    for i in range(n):
        p = Vector((rnd.uniform(*xr), rnd.uniform(*yr), z - 0.35))
        if any((Vector((h.center.x, h.center.y)) - p.xy).length < 1.2 for h in Ctx.hexes.values()):
            continue
        s = rnd.uniform(*scale)
        add(Ctx.meshes[f'stack{rnd.randrange(2)}'], p, rnd.uniform(0, 6), (s * 2.4, s * 2.4, s * 9.0), K.jitter_col(Ctx.style['pal']['rock'], rnd))


def siege_camp(cx, cy, n=10, seed=4):
    rnd = random.Random(seed)
    for i in range(n):
        a = rnd.uniform(-2.6, -0.5)
        d = rnd.uniform(1.1, 1.9)
        x, y = cx + math.cos(a) * d, cy + math.sin(a) * d
        hx = K.nearest_hex(Vector((x, y)))
        g = hx.ground(x - hx.center.x, y - hx.center.y)
        add(Ctx.meshes['tent'], g, rnd.uniform(0, 6), 1.5)
        if i % 3 == 0:
            add(Ctx.meshes['fire'], g + Vector((0.08, -0.06, 0.0)), 0, 1.6)
            R.point(g + Vector((0.08, -0.06, 0.12)), 12.0, (1.0, 0.5, 0.2), 0.05)
