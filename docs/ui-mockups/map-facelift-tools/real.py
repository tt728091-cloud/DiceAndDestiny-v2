"""Realism upgrade + theming layer over hexkit (set 2 concepts).

Overrides hexkit functions at runtime (hexkit.py itself is untouched), adds
high-resolution smooth terrain, ridged mountains, realistic trees, grass
strands, timber-framed buildings, medieval dressing, global material overlays
(grime, snow, blight, caustics, tint) and whole-board material re-theming.
"""
import bpy, bmesh, math, random
from mathutils import Vector, Matrix, noise
import hexkit as K
from hexkit import Ctx, lin, smoothstep, lerp, edge_dist, faces_of, to_mesh, bm_cone, bm_box, bm_ico, bm_uvsphere, add, link, WATER

# --------------------------------------------------------------------------
# Terrain

_orig_height = K.Hex.height


def height(self, wx, wy):
    z = _orig_height(self, wx, wy) * Ctx.style.get('relief', 1.0)
    S = Ctx.style
    if not S.get('real'):
        return z
    p = Vector((wx - self.center.x, wy - self.center.y))
    f = smoothstep(0.0, 0.28, edge_dist(p, 1.0 - S['gap']))
    amp = {'hills': 1.8, 'mountain': 1.6, 'forest': 1.0, 'plains': 0.45, 'marsh': 0.35, 'lake': 0.3}.get(self.terrain, 0.5)
    d = noise.fractal(Vector((wx * 2.6, wy * 2.6, (self.seed % 17) * 0.7)), 0.55, 2.1, 5)
    zz = z + S.get('detail', 0.022) * amp * d * f
    if self.segs and S.get('rivers', True):
        rd = self.river_dist(wx, wy)
        if rd < 0.2:
            zz = min(zz, z) if rd < 0.12 else lerp(min(zz, z), zz, (rd - 0.12) / 0.08)
    return zz


K.Hex.height = height


def tile_mesh(hx, n=None):
    S = Ctx.style
    n = n or (S['res_fn'](hx) if S.get('res_fn') else S.get('res', 18))
    rr = 1.0 - S['gap']
    T = S['thickness']
    corners = [Vector((rr * math.cos(math.radians(30 + 60 * i)), rr * math.sin(math.radians(30 + 60 * i)), 0)) for i in range(6)]
    verts, faces, index = [], [], {}

    def vid(p):
        key = (round(p.x, 5), round(p.y, 5))
        if key not in index:
            index[key] = len(verts)
            verts.append(p.copy())
        return index[key]
    C = Vector((0, 0, 0))
    for i in range(6):
        A, B = corners[i], corners[(i + 1) % 6]

        def P(a, b):
            return C + (A - C) * (a / n) + (B - C) * (b / n)
        for a in range(n):
            for b in range(n - a):
                v0, v1, v2 = vid(P(a, b)), vid(P(a + 1, b)), vid(P(a, b + 1))
                faces.append((v0, v1, v2))
                if a + b < n - 1:
                    faces.append((v1, vid(P(a + 1, b + 1)), v2))
    bm = bmesh.new()
    bvs = [bm.verts.new(v) for v in verts]
    for f in faces:
        bm.faces.new([bvs[i] for i in f])
    for v in bm.verts:
        v.co.z = hx.height(hx.center.x + v.co.x, hx.center.y + v.co.y)
    top_count = len(bm.verts)
    boundary = [e for e in bm.edges if e.is_boundary]
    ret = bmesh.ops.extrude_edge_only(bm, edges=boundary)
    newv = [g for g in ret['geom'] if isinstance(g, bmesh.types.BMVert)]
    for v in newv:
        v.co.z = -T - hx.top
    for g in ret['geom']:
        if isinstance(g, bmesh.types.BMFace):
            g.material_index = 1
    newv.sort(key=lambda v: math.atan2(v.co.y, v.co.x))
    bm.faces.new(newv).material_index = 1
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    me = bpy.data.meshes.new(f'tile_{hx.q}_{hx.r}')
    bm.to_mesh(me)
    bm.free()
    me.materials.append(Ctx.mats[hx.terrain])
    me.materials.append(Ctx.mats['side'])
    for poly in me.polygons:
        poly.use_smooth = S.get('smooth', False) and poly.material_index == 0
    ob = add(me, (hx.center.x, hx.center.y, hx.top), name=f'tile_{hx.q}_{hx.r}')
    vg = ob.vertex_groups.new(name='top')
    vg.add(list(range(top_count)), 1.0, 'REPLACE')
    add_grass(ob, hx)
    if S['bevel'] > 0:
        bv = ob.modifiers.new('bevel', 'BEVEL')
        bv.width = S['bevel']
        bv.segments = 2
        bv.limit_method = 'ANGLE'
        bv.angle_limit = math.radians(55)
    need = getattr(hx, 'base', hx.terrain) in ('lake', 'marsh') or (hx.segs and not S.get('dry_rivers'))
    if need:
        bm = bmesh.new()
        bmesh.ops.create_circle(bm, cap_ends=True, segments=6, radius=rr * 0.985)
        wm = Ctx.mats.get(hx.terrain + '_water') or (Ctx.mats['marsh_water'] if getattr(hx, 'base', hx.terrain) == 'marsh' else Ctx.mats['water'])
        add(to_mesh(bm, 'water', [wm]), (hx.center.x, hx.center.y, hx.top + WATER), name='water')
    return ob


K.tile_mesh = tile_mesh


def add_grass(ob, hx):
    G = Ctx.style.get('grass')
    if not G or hx.terrain not in G['count']:
        return
    mat = Ctx.mats.get('grass_hair')
    ob.data.materials.append(mat)
    ob.modifiers.new('grass', 'PARTICLE_SYSTEM')
    psys = ob.particle_systems[-1]
    psys.seed = hx.seed % 1000
    st = psys.settings
    st.type = 'HAIR'
    st.count = G['count'][hx.terrain]
    st.hair_length = G.get('length', 0.03)
    st.use_advanced_hair = False
    st.emit_from = 'FACE'
    st.distribution = 'RAND'
    st.child_type = 'SIMPLE'
    st.rendered_child_count = G.get('children', 3)
    st.child_percent = 10
    st.child_radius = G.get('spread', 0.02)
    st.roughness_1 = G.get('rough', 0.006)
    st.roughness_1_size = 1.0
    st.roughness_endpoint = G.get('tipr', 0.008)
    st.child_length = 1.0
    st.child_length_threshold = 0.4
    st.render_type = 'PATH'
    st.display_step = 2
    st.render_step = 3
    st.material = len(ob.data.materials)
    st.root_radius = 1.0
    st.tip_radius = 0.0
    st.radius_scale = G.get('radius', 0.0018)
    st.use_close_tip = True
    for attr, val in (('length_random', 0.6),):
        if hasattr(st, attr):
            setattr(st, attr, val)
    psys.vertex_group_density = 'top'


def m_grass_hair(cols):
    m, nt, p = K._new('grass_hair')
    hi = nt.nodes.new('ShaderNodeHairInfo')
    ramp = nt.nodes.new('ShaderNodeValToRGB')
    els = ramp.color_ramp.elements
    els[0].color = cols[0]
    els[1].color = cols[-1]
    for i, c in enumerate(cols[1:-1]):
        e = els.new((i + 1) / (len(cols) - 1))
        e.color = c
    nt.links.new(hi.outputs['Random'], ramp.inputs['Fac'])
    tip = nt.nodes.new('ShaderNodeMixRGB')
    tip.blend_type = 'MULTIPLY'
    nt.links.new(hi.outputs['Intercept'], tip.inputs['Fac'])
    nt.links.new(ramp.outputs['Color'], tip.inputs['Color1'])
    tip.inputs['Color2'].default_value = (1.35, 1.3, 1.0, 1.0)
    # darker roots, lighter tips
    root = nt.nodes.new('ShaderNodeMixRGB')
    root.blend_type = 'MULTIPLY'
    inv = nt.nodes.new('ShaderNodeMath')
    inv.operation = 'SUBTRACT'
    inv.inputs[0].default_value = 1.0
    nt.links.new(hi.outputs['Intercept'], inv.inputs[1])
    nt.links.new(inv.outputs['Value'], root.inputs['Fac'])
    nt.links.new(tip.outputs['Color'], root.inputs['Color1'])
    root.inputs['Color2'].default_value = (0.45, 0.45, 0.4, 1.0)
    nt.links.new(root.outputs['Color'], p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.7)
    return m


# --------------------------------------------------------------------------
# Realistic mountains, rocks and trees

def mountain_real(hx, lx, ly, h, r, segs=None, rings=None):
    rnd = hx.rnd
    rings, segs = 24, 52
    seed = Vector((rnd.uniform(0, 50), rnd.uniform(0, 50), rnd.uniform(0, 50)))
    bm = bmesh.new()
    center = bm.verts.new((0, 0, h))
    grid = []
    for k in range(1, rings + 1):
        t = k / rings
        ring = []
        for i in range(segs):
            a = 2 * math.pi * i / segs
            wob = 1.0 + 0.22 * noise.noise(Vector((math.cos(a) * 1.3, math.sin(a) * 1.3, 0)) + seed)
            rad = r * t * wob
            x, y = math.cos(a) * rad, math.sin(a) * rad
            prof = (1.0 - t) ** 1.05
            rid = noise.ridged_multi_fractal(Vector((x * 3.6, y * 3.6, 0)) + seed, 0.85, 2.1, 6, 1.0, 2.2)
            z = h * prof * (0.35 + 0.42 * rid) + 0.035 * noise.noise(Vector((x * 9, y * 9, 1)) + seed)
            if k == rings:
                z = -0.04
            ring.append(bm.verts.new((x, y, z)))
        grid.append(ring)
    for i in range(segs):
        bm.faces.new((center, grid[0][i], grid[0][(i + 1) % segs]))
    for k in range(rings - 1):
        A, B = grid[k], grid[k + 1]
        for i in range(segs):
            j = (i + 1) % segs
            bm.faces.new((A[i], B[i], B[j], A[j]))
    bm.faces.new(list(reversed(grid[-1])))
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    me = to_mesh(bm, 'mountain', [Ctx.mats['mtn']], smooth=Ctx.style.get('mtn_smooth', False))
    g = hx.ground(lx, ly)
    return add(me, g - Vector((0, 0, 0.01)), rnd.uniform(0, 6.28), name='mountain')


def m_mountain_real(name, rock1, rock2, snow, snow_z=0.55, snow_on=True, band=0.14):
    m, nt, p = K._new(name)
    n = K._noise(nt, 4.0, 6.0)
    ramp = nt.nodes.new('ShaderNodeValToRGB')
    ramp.color_ramp.elements[0].color = rock1
    ramp.color_ramp.elements[1].color = rock2
    nt.links.new(n.outputs['Fac'], ramp.inputs['Fac'])
    col = ramp.outputs['Color']
    # sedimentary bands
    tc = nt.nodes.new('ShaderNodeTexCoord')
    w = nt.nodes.new('ShaderNodeTexWave')
    w.bands_direction = 'Z'
    w.inputs['Scale'].default_value = 5.0
    w.inputs['Distortion'].default_value = 9.0
    w.inputs['Detail'].default_value = 3.0
    nt.links.new(tc.outputs['Object'], w.inputs['Vector'])
    bf = K._maprange(nt, w.outputs['Fac'], 0.2, 0.9, 0.0, band)
    col = K._mix(nt, bf, col, (rock1[0] * 0.5, rock1[1] * 0.5, rock1[2] * 0.5, 1))
    if snow_on:
        z = K._objz(nt)
        n2 = K._noise(nt, 7.0, 3.0)
        a = nt.nodes.new('ShaderNodeMath')
        a.operation = 'MULTIPLY_ADD'
        nt.links.new(n2.outputs['Fac'], a.inputs[0])
        a.inputs[1].default_value = 0.22
        nt.links.new(z, a.inputs[2])
        fz = K._maprange(nt, a.outputs['Value'], snow_z, snow_z + 0.05)
        geo = nt.nodes.new('ShaderNodeNewGeometry')
        sep = nt.nodes.new('ShaderNodeSeparateXYZ')
        nt.links.new(geo.outputs['Normal'], sep.inputs['Vector'])
        fs = K._maprange(nt, sep.outputs['Z'], 0.45, 0.7)
        mul = nt.nodes.new('ShaderNodeMath')
        mul.operation = 'MULTIPLY'
        nt.links.new(fz, mul.inputs[0])
        nt.links.new(fs, mul.inputs[1])
        col = K._mix(nt, mul.outputs['Value'], col, snow)
    nt.links.new(col, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.88)
    K._bump(nt, p, 26.0, 0.5, 0.02)
    return m


def rock_mesh(rnd, r=0.05):
    bm = bmesh.new()
    vs = bmesh.ops.create_icosphere(bm, subdivisions=3, radius=r)['verts']
    off = Vector((rnd.uniform(0, 40), rnd.uniform(0, 40), rnd.uniform(0, 40)))
    for v in vs:
        d = noise.fractal(v.co * 30 + off, 0.6, 2.0, 4)
        v.co += v.co.normalized() * d * r * 0.35
        v.co.z *= 0.62
        if v.co.z < -r * 0.15:
            v.co.z = -r * 0.15
    for f in bm.faces:
        f.material_index = 0
    return to_mesh(bm, 'rock_real', [Ctx.mats['rock']], smooth=True)


def star_tier(bm, z, r, h, mi, spikes, rnd, droop=0.012):
    ring = []
    for i in range(spikes * 2):
        a = i / (spikes * 2) * 6.283 + rnd.uniform(-0.05, 0.05)
        rad = r * (1.0 if i % 2 == 0 else 0.58) * rnd.uniform(0.9, 1.08)
        dz = -droop if i % 2 == 0 else 0.0
        ring.append(bm.verts.new((math.cos(a) * rad, math.sin(a) * rad, z + dz)))
    tip = bm.verts.new((rnd.uniform(-0.004, 0.004), rnd.uniform(-0.004, 0.004), z + h))
    base = bm.verts.new((0, 0, z + h * 0.15))
    for i in range(len(ring)):
        j = (i + 1) % len(ring)
        bm.faces.new((ring[i], ring[j], tip)).material_index = mi
        bm.faces.new((ring[j], ring[i], base)).material_index = mi


def branch(bm, base, d, length, radius, depth, rnd, mi=0):
    tip = base + d * length
    rot = d.to_track_quat('Z', 'Y').to_matrix().to_4x4()
    mat = Matrix.Translation(base) @ rot @ Matrix.Translation((0, 0, length / 2))
    ret = bmesh.ops.create_cone(bm, cap_ends=False, cap_tris=False, segments=5, radius1=radius, radius2=radius * 0.62, depth=length, matrix=mat)
    for f in faces_of(ret['verts']):
        f.material_index = mi
    if depth > 0:
        for i in range(rnd.randint(2, 3)):
            ax = Vector((rnd.uniform(-1, 1), rnd.uniform(-1, 1), rnd.uniform(-0.2, 0.2))).cross(d).normalized()
            nd = (Matrix.Rotation(rnd.uniform(0.45, 0.85), 3, ax) @ d).normalized()
            nd = (nd + Vector((0, 0, 0.15))).normalized()
            branch(bm, tip - d * length * rnd.uniform(0.0, 0.25), nd, length * rnd.uniform(0.62, 0.78), radius * 0.62, depth - 1, rnd, mi)


def build_meshes_real():
    M, me = Ctx.mats, Ctx.meshes
    rnd = random.Random(11)
    for k in range(4):
        bm = bmesh.new()
        bm_cone(bm, 6, 0.011, 0.006, 0.09, 0.0, 0)
        tiers = 8 + k % 3
        for t in range(tiers):
            f = t / tiers
            star_tier(bm, 0.03 + f * 0.2, (0.072 * (1 - f) ** 0.95 + 0.012) * rnd.uniform(0.92, 1.06), 0.055, 1, 9, rnd)
        me[f'conifer{k}'] = to_mesh(bm, f'conifer{k}', [M['trunk'], M['leaf_conifer']])
    for k in range(4):
        bm = bmesh.new()
        branch(bm, Vector((0, 0, -0.01)), Vector((0, 0, 1)), 0.09, 0.013, 1, rnd)
        cx = Vector((0, 0, 0.13))
        for j in range(7):
            c = cx + Vector((rnd.uniform(-0.045, 0.045), rnd.uniform(-0.045, 0.045), rnd.uniform(-0.03, 0.035)))
            vs = bmesh.ops.create_icosphere(bm, subdivisions=2, radius=rnd.uniform(0.03, 0.048), matrix=Matrix.Translation(c))['verts']
            off = Vector((rnd.uniform(0, 30), rnd.uniform(0, 30), 0))
            for v in vs:
                v.co += (v.co - c).normalized() * noise.noise((v.co * 70) + off) * 0.008
            for fc in faces_of(vs):
                fc.material_index = 1
        me[f'broad{k}'] = to_mesh(bm, f'broad{k}', [M['trunk'], M['leaf_broad']], smooth=True)
    for k in range(3):
        bm = bmesh.new()
        branch(bm, Vector((0, 0, -0.01)), Vector((rnd.uniform(-0.1, 0.1), rnd.uniform(-0.1, 0.1), 1)).normalized(), 0.1, 0.012, 3, rnd)
        me[f'dead{k}'] = to_mesh(bm, f'dead{k}', [M['deadwood']], smooth=True)
    for k in range(3):
        me[f'rock{k}'] = rock_mesh(rnd)
    bm = bmesh.new()
    for j in range(3):
        c = Vector((rnd.uniform(-0.02, 0.02), rnd.uniform(-0.02, 0.02), 0.022))
        vs = bmesh.ops.create_icosphere(bm, subdivisions=2, radius=rnd.uniform(0.022, 0.032), matrix=Matrix.Translation(c))['verts']
        for v in vs:
            v.co += (v.co - c).normalized() * noise.noise(v.co * 80) * 0.006
    me['bush'] = to_mesh(bm, 'bush', [M['leaf_broad']], smooth=True)
    build_houses_real()
    build_dressing()


def build_houses_real():
    M, me = Ctx.mats, Ctx.meshes
    for k, (w, d, h, rh) in enumerate([(0.09, 0.062, 0.055, 0.055), (0.13, 0.066, 0.05, 0.055), (0.075, 0.06, 0.075, 0.06)]):
        bm = bmesh.new()
        bm_box(bm, (0, 0, h / 2), (w, d, h), 0)
        o = 0.01
        vs = [bm.verts.new(c) for c in [(-w / 2 - o, -d / 2 - o, h - 0.004), (w / 2 + o, -d / 2 - o, h - 0.004), (w / 2 + o, d / 2 + o, h - 0.004), (-w / 2 - o, d / 2 + o, h - 0.004), (-w / 2 - o * 0.5, 0, h + rh), (w / 2 + o * 0.5, 0, h + rh)]]
        for fv in ((vs[0], vs[1], vs[5], vs[4]), (vs[2], vs[3], vs[4], vs[5]), (vs[3], vs[0], vs[4]), (vs[1], vs[2], vs[5]), (vs[3], vs[2], vs[1], vs[0])):
            bm.faces.new(fv).material_index = 1
        # timber frame on long faces
        for s in (-1, 1):
            y = s * (d / 2 + 0.0015)
            for x in (-w / 2 + 0.003, -w / 6, w / 6, w / 2 - 0.003):
                bm_box(bm, (x, y, h / 2), (0.005, 0.003, h), 4)
            bm_box(bm, (0, y, h * 0.52), (w, 0.003, 0.005), 4)
            bm_box(bm, (0, y, h - 0.003), (w, 0.003, 0.005), 4)
        bm_box(bm, (w * 0.25, -d / 2 - 0.002, 0.016), (0.016, 0.005, 0.032), 2)
        bm_box(bm, (-w * 0.3, -d / 2 - 0.0025, h * 0.72), (0.012, 0.004, 0.012), 3)
        bm_box(bm, (w / 2 + 0.002, 0.0, h * 0.7), (0.004, 0.012, 0.012), 3)
        bm_box(bm, (-w * 0.32, 0.012, h + rh * 0.75), (0.014, 0.014, 0.05), 5)
        me[f'house{k}'] = to_mesh(bm, f'house{k}', [M['wall'], M['roof'], M['door'], M['window'], M['beam'], M['stone']])


def build_dressing():
    M, me = Ctx.mats, Ctx.meshes
    rnd = random.Random(5)
    bm = bmesh.new()
    for i in range(12):
        x = -0.11 + i * 0.02
        hgt = rnd.uniform(0.055, 0.07)
        bm_cone(bm, 5, 0.008, 0.008, hgt, 0.0, 0, x=x)
        bm_cone(bm, 5, 0.008, 0.0, 0.014, hgt, 0, x=x)
    bm_box(bm, (0, -0.006, 0.035), (0.24, 0.004, 0.006), 0)
    me['palisade'] = to_mesh(bm, 'palisade', [M['deadwood']])
    bm = bmesh.new()
    bm_box(bm, (-0.04, 0, 0.06), (0.008, 0.008, 0.12), 0)
    bm_box(bm, (0.04, 0, 0.06), (0.008, 0.008, 0.12), 0)
    bm_box(bm, (0, 0, 0.118), (0.1, 0.009, 0.008), 0)
    bm_cone(bm, 4, 0.0012, 0.0012, 0.04, 0.074, 0, x=0.012)
    bm_box(bm, (0.012, 0, 0.07), (0.01, 0.006, 0.012), 0)
    me['gallows'] = to_mesh(bm, 'gallows', [M['deadwood']])
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.022), (0.006, 0.005, 0.044), 0)
    bm_box(bm, (0, 0, 0.032), (0.024, 0.005, 0.006), 0)
    bm_box(bm, (0, 0.012, 0.002), (0.018, 0.03, 0.006), 1)
    me['grave'] = to_mesh(bm, 'grave', [M['deadwood'], M['mud_mound']])
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.018), (0.016, 0.008, 0.036), 0)
    bm_cone(bm, 8, 0.008, 0.008, 0.008, 0.036, 0, tilt=(math.pi / 2, 0))
    me['headstone'] = to_mesh(bm, 'headstone', [M['stone']])
    bm = bmesh.new()
    bm_cone(bm, 5, 0.003, 0.003, 0.1, 0.0, 0)
    bm_cone(bm, 6, 0.007, 0.004, 0.012, 0.1, 0)
    bm_cone(bm, 6, 0.006, 0.0, 0.022, 0.108, 1)
    me['torch'] = to_mesh(bm, 'torch', [M['deadwood'], M['flame']])
    bm = bmesh.new()
    bm_cone(bm, 5, 0.003, 0.003, 0.16, 0.0, 0)
    vs = [bm.verts.new(c) for c in [(0.002, 0, 0.158), (0.002, 0, 0.1), (0.055, 0, 0.11), (0.045, 0, 0.13), (0.06, 0, 0.152)]]
    bm.faces.new(vs).material_index = 1
    me['banner_pole'] = to_mesh(bm, 'banner_pole', [M['deadwood'], M['banner_dark']])
    bm = bmesh.new()
    vs = bmesh.ops.create_uvsphere(bm, u_segments=12, v_segments=8, radius=0.03, matrix=Matrix.Diagonal((1, 1, 1.15, 1)))['verts']
    for v in vs:
        if v.co.z < 0:
            v.co.z *= 0.3
    me['haystack'] = to_mesh(bm, 'haystack', [M['hay']], smooth=True)
    bm = bmesh.new()
    bm_uvsphere(bm, 0.012, (0, 0, 0.012), 0, 12, 8)
    bm_box(bm, (0, -0.008, 0.004), (0.014, 0.012, 0.008), 0)
    for s in (-1, 1):
        bm_ico(bm, 0.0035, (s * 0.0045, -0.0105, 0.013), 1, sub=1)
    me['skull'] = to_mesh(bm, 'skull', [M['bone'], M['door']], smooth=True)
    bm = bmesh.new()
    for i in range(6):
        a = (i - 2.5) * 0.028
        branch(bm, Vector((a, 0, 0)), Vector((0, 0.1, 1)).normalized(), 0.08, 0.005, 0, rnd)
    branch(bm, Vector((-0.08, 0.01, 0.07)), Vector((1, 0, 0)), 0.16, 0.007, 0, rnd)
    me['ribs'] = to_mesh(bm, 'ribs', [M['bone']], smooth=True)
    bm = bmesh.new()
    for i in range(5):
        a = rnd.uniform(0, 6.28)
        rr = rnd.uniform(0, 0.02)
        branch(bm, Vector((math.cos(a) * rr, math.sin(a) * rr, 0)), Vector((rnd.uniform(-0.2, 0.2), rnd.uniform(-0.2, 0.2), 1)).normalized(), rnd.uniform(0.1, 0.22), 0.004, 0, rnd)
    me['kelp'] = to_mesh(bm, 'kelp', [M['kelp']], smooth=True)
    bm = bmesh.new()
    bm_ico(bm, 0.012, (0, 0, 0), 0, sub=2)
    me['wisp'] = to_mesh(bm, 'wisp', [M['wisp']], smooth=True)
    # watchtower
    bm = bmesh.new()
    for (x, y) in ((-0.03, -0.03), (0.03, -0.03), (0.03, 0.03), (-0.03, 0.03)):
        bm_cone(bm, 5, 0.005, 0.005, 0.2, 0.0, 0, x=x, y=y)
    bm_box(bm, (0, 0, 0.19), (0.085, 0.085, 0.012), 0)
    bm_box(bm, (0, -0.042, 0.21), (0.085, 0.004, 0.03), 0)
    bm_box(bm, (0, 0.042, 0.21), (0.085, 0.004, 0.03), 0)
    bm_cone(bm, 4, 0.07, 0.0, 0.06, 0.235, 1, rot=math.pi / 4)
    me['watchtower'] = to_mesh(bm, 'watchtower', [M['deadwood'], M['roof']])


def place_dressing(hx):
    """Medieval set dressing around sites: palisades, torches, graves, gallows."""
    S = Ctx.style
    if not S.get('dressing', True):
        return
    rnd = random.Random(hx.seed + 3)
    me = Ctx.meshes
    site = hx.site
    if site in ('village',):
        for i in range(5):
            a = i / 5 * 6.283 + 0.8
            p = Vector((math.cos(a) * 0.5, math.sin(a) * 0.5))
            if hx.river_dist(hx.center.x + p.x, hx.center.y + p.y) > 0.2:
                add(me['palisade'], hx.ground(p.x, p.y) - Vector((0, 0, 0.01)), a + math.pi / 2, 1.3)
        for (x, y) in ((0.22, -0.38), (-0.3, -0.25)):
            add(me['torch'], hx.ground(x, y), 0.0, 1.4)
        for (x, y) in ((0.42, 0.1), (0.38, 0.2)):
            add(me['haystack'], hx.ground(x, y), 0, rnd.uniform(1.2, 1.6), (1, 1, 1, 1))
        add(me['watchtower'], hx.ground(-0.45, 0.2), 0.3, 1.3, (1, 1, 1, 1))
    elif site in ('town',):
        add(me['gallows'], hx.ground(0.52, -0.35), 0.4, 1.5)
        for (x, y) in ((0.4, -0.5), (-0.5, -0.38), (0.6, 0.1)):
            add(me['torch'], hx.ground(x, y), 0.0, 1.5)
        add(me['banner_pole'], hx.ground(0.1, 0.0) + Vector((0, 0, 0.24)), 0.2, 1.5)
    elif site in ('keep',):
        for (x, y) in ((-0.4, -0.45), (0.45, -0.4)):
            add(me['torch'], hx.ground(x, y), 0.0, 1.5)
        add(me['gallows'], hx.ground(-0.5, -0.1), 0.2, 1.4)
        add(me['banner_pole'], hx.ground(0, 0) + Vector((0, 0, 0.3)), 0.3, 1.6)
    elif site in ('monastery', 'chapel', 'cairn'):
        for i in range(9):
            p = Vector((rnd.uniform(0.3, 0.55), rnd.uniform(-0.55, -0.25)))
            m = me['grave'] if i % 2 else me['headstone']
            add(m, hx.ground(p.x, p.y) - Vector((0, 0, 0.004)), rnd.uniform(-0.3, 0.3), 1.4)
    elif site in ('hermit', 'shrine'):
        add(me['torch'], hx.ground(-0.2, -0.3), 0.0, 1.4)
    if hx.enemies and S.get('bones', False):
        for i in range(4):
            p = Vector((rnd.uniform(-0.4, 0.4), rnd.uniform(-0.4, 0.2)))
            add(me['skull'], hx.ground(p.x, p.y), rnd.uniform(0, 6), 1.3)


_orig_decorate = K.decorate


def decorate(hx):
    _orig_decorate(hx)
    if Ctx.style.get('real'):
        place_dressing(hx)


K.decorate = decorate


def install(real=True):
    """Swap in the realistic geometry builders for this run."""
    if real:
        K.mountain_obj = mountain_real


def extra_materials():
    S = Ctx.style
    P = S['pal']
    M = Ctx.mats
    M['beam'] = K.m_flat('beam', P.get('beam', lin('#2e2219')), 0.9)
    M['hay'] = K.m_terrain('hay', P.get('hay', lin('#b8964e')), P.get('hay2', lin('#8e7038')), nscale=40.0, speck=0.2, bump=0.5)
    M['bone'] = K.m_flat('bone', P.get('bone', lin('#d8cdb2')), 0.6)
    M['mud_mound'] = K.m_flat('mud_mound', P.get('mud', lin('#4a3c2c')), 0.95)
    M['kelp'] = K.m_flat('kelp', P.get('kelp', lin('#3e5a2a')), 0.6)
    M['wisp'] = K.m_emit('wisp', P.get('wisp', lin('#9dffb0')), S.get('wisp_k', 30.0))
    M['wax'] = K.m_flat('wax', P.get('wax', lin('#e8dcc0')), 0.5)
    if S.get('grass'):
        M['grass_hair'] = m_grass_hair(S['grass']['cols'])
    piece_materials(*[S.get('pieces', {}).get(k) for k in ('hero', 'trim', 'flag', 'enemy', 'crown')])
    if S.get('real'):
        real_terrain_materials()
        M['wall'] = m_plaster('wall', P['wall'])
        roof = S.get('roof', 'thatch')
        M['roof'] = m_thatch('roof') if roof == 'thatch' else m_slate('roof')
        M['stone'] = m_stone('stone', P['stone'])
    for nm in ('water', 'marsh_water', 'sea', 'seabed', 'grid', 'clouds'):
        if nm in bpy.data.materials:
            bpy.data.materials[nm]['no_overlay'] = True


def m_plaster(name, col):
    m, nt, p = K._new(name)
    n = K._noise(nt, 30.0, 4.0)
    c = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.35, 0.75, 0.0, 0.35), col, (col[0] * 0.6, col[1] * 0.56, col[2] * 0.5, 1))
    nt.links.new(c, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.95)
    K._bump(nt, p, 80.0, 0.25, 0.005)
    return m


def m_thatch(name):
    m, nt, p = K._new(name)
    tc = nt.nodes.new('ShaderNodeTexCoord')
    w = nt.nodes.new('ShaderNodeTexWave')
    w.bands_direction = 'Z'
    w.inputs['Scale'].default_value = 140.0
    w.inputs['Distortion'].default_value = 4.0
    w.inputs['Detail'].default_value = 4.0
    nt.links.new(tc.outputs['Object'], w.inputs['Vector'])
    oi = nt.nodes.new('ShaderNodeObjectInfo')
    f = K._maprange(nt, w.outputs['Fac'], 0.2, 0.9, 0.0, 0.45)
    c = K._mix(nt, f, oi.outputs['Color'], (0.04, 0.03, 0.02, 1))
    nt.links.new(c, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.95)
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = 0.6
    nt.links.new(w.outputs['Fac'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


def m_slate(name):
    m, nt, p = K._new(name)
    tc = nt.nodes.new('ShaderNodeTexCoord')
    br = nt.nodes.new('ShaderNodeTexBrick')
    br.inputs['Scale'].default_value = 60.0
    br.inputs['Mortar Size'].default_value = 0.02
    oi = nt.nodes.new('ShaderNodeObjectInfo')
    nt.links.new(tc.outputs['Object'], br.inputs['Vector'])
    nt.links.new(oi.outputs['Color'], br.inputs['Color1'])
    mul = nt.nodes.new('ShaderNodeMixRGB')
    mul.blend_type = 'MULTIPLY'
    mul.inputs['Fac'].default_value = 1.0
    nt.links.new(oi.outputs['Color'], mul.inputs['Color1'])
    mul.inputs['Color2'].default_value = (0.7, 0.7, 0.72, 1)
    nt.links.new(mul.outputs['Color'], br.inputs['Color2'])
    br.inputs['Mortar'].default_value = (0.02, 0.02, 0.02, 1)
    nt.links.new(br.outputs['Color'], p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.6)
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = 0.4
    nt.links.new(br.outputs['Fac'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


def m_stone(name, col):
    m, nt, p = K._new(name)
    tc = nt.nodes.new('ShaderNodeTexCoord')
    br = nt.nodes.new('ShaderNodeTexBrick')
    br.inputs['Scale'].default_value = 45.0
    br.inputs['Mortar Size'].default_value = 0.025
    br.inputs['Color1'].default_value = col
    br.inputs['Color2'].default_value = (col[0] * 0.75, col[1] * 0.73, col[2] * 0.7, 1)
    br.inputs['Mortar'].default_value = (col[0] * 0.4, col[1] * 0.4, col[2] * 0.38, 1)
    nt.links.new(tc.outputs['Object'], br.inputs['Vector'])
    n = K._noise(nt, 50.0, 4.0)
    c = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.4, 0.7, 0.0, 0.3), br.outputs['Color'], (0.05, 0.05, 0.04, 1))
    nt.links.new(c, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.92)
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = 0.45
    nt.links.new(br.outputs['Fac'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


# --------------------------------------------------------------------------
# Global material overlays and whole-board re-theming

def apply_overlays():
    O = Ctx.style.get('overlays')
    if not O:
        return
    for m in list(bpy.data.materials):
        if not m.use_nodes or m.get('no_overlay'):
            continue
        nt = m.node_tree
        p = nt.nodes.get('Principled BSDF')
        if p is None:
            continue
        bc = p.inputs['Base Color']
        if bc.is_linked:
            src = bc.links[0].from_socket
        else:
            rgb = nt.nodes.new('ShaderNodeRGB')
            rgb.outputs[0].default_value = bc.default_value
            src = rgb.outputs[0]
        col = src
        geo = nt.nodes.new('ShaderNodeNewGeometry')
        if 'ao' in O:
            dist, k, dirt = O['ao']
            ao = nt.nodes.new('ShaderNodeAmbientOcclusion')
            ao.samples = 8
            ao.inputs['Distance'].default_value = dist
            fac = K._maprange(nt, ao.outputs['AO'], 0.0, 1.0, k, 0.0)
            mx = nt.nodes.new('ShaderNodeMixRGB')
            mx.blend_type = 'MULTIPLY'
            nt.links.new(fac, mx.inputs['Fac'])
            nt.links.new(col, mx.inputs['Color1'])
            mx.inputs['Color2'].default_value = dirt
            col = mx.outputs['Color']
        if 'desat' in O:
            bw = nt.nodes.new('ShaderNodeRGBToBW')
            nt.links.new(col, bw.inputs['Color'])
            mx = nt.nodes.new('ShaderNodeMixRGB')
            mx.inputs['Fac'].default_value = O['desat']
            nt.links.new(col, mx.inputs['Color1'])
            nt.links.new(bw.outputs['Val'], mx.inputs['Color2'])
            col = mx.outputs['Color']
        if 'tint' in O:
            mx = nt.nodes.new('ShaderNodeMixRGB')
            mx.blend_type = 'MULTIPLY'
            mx.inputs['Fac'].default_value = 1.0
            nt.links.new(col, mx.inputs['Color1'])
            mx.inputs['Color2'].default_value = O['tint']
            col = mx.outputs['Color']
        if 'blight' in O:
            (cx, cy), radius, bcol = O['blight']
            vm = nt.nodes.new('ShaderNodeVectorMath')
            vm.operation = 'MULTIPLY'
            vm.inputs[1].default_value = (1, 1, 0)
            nt.links.new(geo.outputs['Position'], vm.inputs[0])
            dist = nt.nodes.new('ShaderNodeVectorMath')
            dist.operation = 'DISTANCE'
            dist.inputs[1].default_value = (cx, cy, 0)
            nt.links.new(vm.outputs['Vector'], dist.inputs[0])
            n = K._noise(nt, 0.9, 3.0)
            wob = nt.nodes.new('ShaderNodeMath')
            wob.operation = 'MULTIPLY_ADD'
            nt.links.new(n.outputs['Fac'], wob.inputs[0])
            wob.inputs[1].default_value = radius * 0.5
            nt.links.new(dist.outputs['Value'], wob.inputs[2])
            fac = K._maprange(nt, wob.outputs['Value'], radius, radius * 0.65, 0.0, 1.0)
            bw = nt.nodes.new('ShaderNodeRGBToBW')
            nt.links.new(col, bw.inputs['Color'])
            tintb = nt.nodes.new('ShaderNodeMixRGB')
            tintb.blend_type = 'MULTIPLY'
            tintb.inputs['Fac'].default_value = 1.0
            nt.links.new(bw.outputs['Val'], tintb.inputs['Color1'])
            tintb.inputs['Color2'].default_value = bcol
            col = K._mix(nt, fac, col, tintb.outputs['Color'])
        if 'snow' in O:
            scol, th = O['snow']
            sep = nt.nodes.new('ShaderNodeSeparateXYZ')
            nt.links.new(geo.outputs['Normal'], sep.inputs['Vector'])
            fn = K._maprange(nt, sep.outputs['Z'], th, th + 0.18)
            n = K._noise(nt, 14.0, 4.0)
            fnz = K._maprange(nt, n.outputs['Fac'], 0.32, 0.5)
            mul = nt.nodes.new('ShaderNodeMath')
            mul.operation = 'MULTIPLY'
            nt.links.new(fn, mul.inputs[0])
            nt.links.new(fnz, mul.inputs[1])
            col = K._mix(nt, mul.outputs['Value'], col, scol)
        if 'cracks' in O:
            scale, dark = O['cracks']
            vo = nt.nodes.new('ShaderNodeTexVoronoi')
            vo.feature = 'DISTANCE_TO_EDGE'
            vo.inputs['Scale'].default_value = scale
            n = K._noise(nt, 30.0, 3.0)
            av = nt.nodes.new('ShaderNodeVectorMath')
            av.operation = 'MULTIPLY_ADD'
            nt.links.new(n.outputs['Color'], av.inputs[0])
            av.inputs[1].default_value = (0.04, 0.04, 0.04)
            nt.links.new(geo.outputs['Position'], av.inputs[2])
            nt.links.new(av.outputs['Vector'], vo.inputs['Vector'])
            line = K._maprange(nt, vo.outputs['Distance'], 0.0, 0.035, dark, 0.0)
            col = K._mix(nt, line, col, (0.25, 0.22, 0.2, 1))
        if 'caustics' in O:
            scale, strength, ccol = O['caustics']
            n = K._noise(nt, 1.5, 2.0)
            add_v = nt.nodes.new('ShaderNodeVectorMath')
            add_v.operation = 'ADD'
            nt.links.new(geo.outputs['Position'], add_v.inputs[0])
            nt.links.new(n.outputs['Color'], add_v.inputs[1])
            vo = nt.nodes.new('ShaderNodeTexVoronoi')
            vo.feature = 'DISTANCE_TO_EDGE'
            vo.inputs['Scale'].default_value = scale
            nt.links.new(add_v.outputs['Vector'], vo.inputs['Vector'])
            line = K._maprange(nt, vo.outputs['Distance'], 0.0, 0.025, 1.0, 0.0)
            em = nt.nodes.new('ShaderNodeMixRGB')
            em.inputs['Color1'].default_value = (0, 0, 0, 1)
            em.inputs['Color2'].default_value = ccol
            nt.links.new(line, em.inputs['Fac'])
            if not p.inputs['Emission Strength'].is_linked and p.inputs['Emission Strength'].default_value == 0.0:
                nt.links.new(em.outputs['Color'], p.inputs['Emission Color'])
                p.inputs['Emission Strength'].default_value = strength
        nt.links.new(col, bc)


def retheme(fn):
    """Replace every mesh material slot through fn(name) -> material or None."""
    cache = {}
    for me in bpy.data.meshes:
        for i, m in enumerate(me.materials):
            if m is None:
                continue
            if m.name not in cache:
                cache[m.name] = fn(m.name)
            if cache[m.name] is not None:
                me.materials[i] = cache[m.name]
    for ps in bpy.data.particles:
        pass


# --------------------------------------------------------------------------
# Sky, ocean, props

def sky_world(sun_el, sun_az, strength=0.35, air=1.0, dust=1.0, ozone=1.0, camera_bg=None, tint=None):
    w = bpy.data.worlds.new('sky')
    bpy.context.scene.world = w
    w.use_nodes = True
    nt = w.node_tree
    bg = nt.nodes['Background']
    sk = nt.nodes.new('ShaderNodeTexSky')
    sk.sky_type = 'NISHITA'
    sk.sun_disc = False
    sk.sun_elevation = math.radians(sun_el)
    sk.sun_rotation = math.radians(sun_az)
    sk.air_density = air
    sk.dust_density = dust
    sk.ozone_density = ozone
    src = sk.outputs['Color']
    if tint:
        mx = nt.nodes.new('ShaderNodeMixRGB')
        mx.blend_type = 'MULTIPLY'
        mx.inputs['Fac'].default_value = 1.0
        nt.links.new(src, mx.inputs['Color1'])
        mx.inputs['Color2'].default_value = tint
        src = mx.outputs['Color']
    nt.links.new(src, bg.inputs['Color'])
    bg.inputs['Strength'].default_value = strength
    if camera_bg is not None:
        out = nt.nodes['World Output']
        bg2 = nt.nodes.new('ShaderNodeBackground')
        if isinstance(camera_bg, list):
            tc = nt.nodes.new('ShaderNodeTexCoord')
            sep = nt.nodes.new('ShaderNodeSeparateXYZ')
            nt.links.new(tc.outputs['Window'], sep.inputs['Vector'])
            ramp = nt.nodes.new('ShaderNodeValToRGB')
            ramp.color_ramp.elements[0].color = camera_bg[0]
            ramp.color_ramp.elements[1].color = camera_bg[1]
            nt.links.new(sep.outputs['Y'], ramp.inputs['Fac'])
            nt.links.new(ramp.outputs['Color'], bg2.inputs['Color'])
        else:
            bg2.inputs['Color'].default_value = camera_bg
        lp = nt.nodes.new('ShaderNodeLightPath')
        mix = nt.nodes.new('ShaderNodeMixShader')
        nt.links.new(lp.outputs['Is Camera Ray'], mix.inputs['Fac'])
        nt.links.new(bg.outputs['Background'], mix.inputs[1])
        nt.links.new(bg2.outputs['Background'], mix.inputs[2])
        nt.links.new(mix.outputs['Shader'], out.inputs['Surface'])


def ocean(z, mat, size=3.0, spatial=16, scale=1.4, chop=1.6, wind=12.0, res=14, foam=None, loc=(5, 0)):
    me = bpy.data.meshes.new('ocean')
    ob = bpy.data.objects.new('ocean', me)
    link(ob)
    ob.location = (loc[0], loc[1], z)
    md = ob.modifiers.new('ocean', 'OCEAN')
    md.geometry_mode = 'GENERATE'
    md.resolution = res
    md.spatial_size = spatial
    md.size = size
    md.wave_scale = scale
    md.choppiness = chop
    md.wind_velocity = wind
    md.repeat_x = 1
    md.repeat_y = 1
    md.random_seed = 7
    if foam:
        md.use_foam = True
        md.foam_layer_name = 'foam'
        md.foam_coverage = foam
    me.materials.append(mat)
    ob.location = (loc[0], loc[1], z)
    return ob


def m_ocean(name, deep, shallow, foam_col=None):
    m, nt, p = K._new(name)
    lw = nt.nodes.new('ShaderNodeLayerWeight')
    lw.inputs['Blend'].default_value = 0.3
    col = K._mix(nt, lw.outputs['Facing'], deep, shallow)
    if foam_col:
        at = nt.nodes.new('ShaderNodeAttribute')
        at.attribute_name = 'foam'
        f = K._maprange(nt, at.outputs['Fac'], 0.1, 0.8)
        col = K._mix(nt, f, col, foam_col)
    nt.links.new(col, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.08)
    K._in(p, ('Specular IOR Level', 'Specular'), 0.6)
    m['no_overlay'] = True
    return m


def smoke(loc, radius=0.25, height=1.2, density=3.0, color=(0.2, 0.2, 0.2, 1), emit=None):
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=16, v_segments=10, radius=1.0)
    m = bpy.data.materials.new('smoke')
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.remove(nt.nodes['Principled BSDF'])
    v = nt.nodes.new('ShaderNodeVolumePrincipled')
    v.inputs['Color'].default_value = color
    tc = nt.nodes.new('ShaderNodeTexCoord')
    n = nt.nodes.new('ShaderNodeTexNoise')
    n.inputs['Scale'].default_value = 3.0
    n.inputs['Detail'].default_value = 5.0
    nt.links.new(tc.outputs['Object'], n.inputs['Vector'])
    grad = nt.nodes.new('ShaderNodeTexGradient')
    grad.gradient_type = 'SPHERICAL'
    nt.links.new(tc.outputs['Object'], grad.inputs['Vector'])
    mul = nt.nodes.new('ShaderNodeMath')
    mul.operation = 'MULTIPLY'
    nt.links.new(n.outputs['Fac'], mul.inputs[0])
    nt.links.new(grad.outputs['Fac'], mul.inputs[1])
    d = K._maprange(nt, mul.outputs['Value'], 0.12, 0.4, 0.0, density)
    nt.links.new(d, v.inputs['Density'])
    if emit:
        v.inputs['Emission Color'].default_value = emit[0]
        em = K._maprange(nt, grad.outputs['Fac'], 0.3, 0.9, 0.0, emit[1])
        nt.links.new(em, v.inputs['Emission Strength'])
    nt.links.new(v.outputs['Volume'], nt.nodes['Material Output'].inputs['Volume'])
    ob = add(to_mesh(bm, 'smoke', [m]), loc + Vector((0, 0, height * 0.5)), 0.0, (radius, radius, height * 0.5), name='smoke')
    return ob


def point(loc, energy, color, size=0.05):
    L = bpy.data.lights.new('pt', 'POINT')
    L.energy = energy
    L.color = color
    L.shadow_soft_size = size
    lo = bpy.data.objects.new('pt', L)
    lo.location = loc
    link(lo)
    return lo


def spot(loc, target, energy, color, angle=40, blend=0.6, size=0.2):
    L = bpy.data.lights.new('spot', 'SPOT')
    L.energy = energy
    L.color = color
    L.spot_size = math.radians(angle)
    L.spot_blend = blend
    L.shadow_soft_size = size
    lo = bpy.data.objects.new('spot', L)
    lo.location = loc
    lo.rotation_euler = (Vector(target) - Vector(loc)).to_track_quat('-Z', 'Y').to_euler()
    link(lo)
    return lo


def area(loc, target, energy, color, size=4.0):
    L = bpy.data.lights.new('area', 'AREA')
    L.energy = energy
    L.color = color
    L.size = size
    lo = bpy.data.objects.new('area', L)
    lo.location = loc
    lo.rotation_euler = (Vector(target) - Vector(loc)).to_track_quat('-Z', 'Y').to_euler()
    link(lo)
    return lo


def plane(loc, size, mat, name='plane'):
    bm = bmesh.new()
    bmesh.ops.create_grid(bm, x_segments=1, y_segments=1, size=size)
    return add(to_mesh(bm, name, [mat]), loc, name=name)


def candle(loc, h=0.35, r=0.06, light=6.0):
    M = Ctx.mats
    bm = bmesh.new()
    bm_cone(bm, 16, r, r * 0.97, h, 0.0, 0)
    bm_cone(bm, 4, 0.004, 0.004, 0.03, h, 1)
    bm_cone(bm, 8, 0.016, 0.0, 0.07, h + 0.02, 2)
    ob = add(to_mesh(bm, 'candle', [M['wax'], M['deadwood'], M['flame']], smooth=True, auto=40), loc)
    point(Vector(loc) + Vector((0, 0, h + 0.07)), light, (1.0, 0.62, 0.3), 0.02)
    return ob


def die(loc, size=0.28, rot=0.0, mat=None, pip=None):
    bm = bmesh.new()
    bmesh.ops.create_cube(bm, size=size)
    me = to_mesh(bm, 'die', [mat or Ctx.mats['ivory']])
    ob = add(me, loc + Vector((0, 0, size / 2)), rot)
    bv = ob.modifiers.new('bv', 'BEVEL')
    bv.width = size * 0.12
    bv.segments = 4
    s = size / 2 + 0.001
    pipm = pip or Ctx.mats['door']
    R2 = Matrix.Rotation(rot, 2)
    for (x, y) in ((-0.25, -0.25), (0.25, 0.25), (0, 0), (-0.25, 0.25), (0.25, -0.25)):
        bm = bmesh.new()
        bmesh.ops.create_circle(bm, cap_ends=True, segments=12, radius=size * 0.08)
        o = R2 @ Vector((x * size, y * size))
        add(to_mesh(bm, 'pip', [pipm]), loc + Vector((o.x, o.y, size + 0.002)), 0.0)
    return ob


def m_ground(name, c1, c2, c3, dirt, dirt_amt=0.35, rock=None, slope=(0.6, 0.8), low=None, low_z=(-0.05, -0.012), deep=None, rough=0.92, micro=0.55):
    """Realistic ground: macro/meso/micro colour variation, dirt patches, pebbles, fine bump."""
    m, nt, p = K._new(name)
    geo = nt.nodes.new('ShaderNodeNewGeometry')

    def nz(scale, detail=4.0, rough_=0.55):
        n = nt.nodes.new('ShaderNodeTexNoise')
        n.inputs['Scale'].default_value = scale
        n.inputs['Detail'].default_value = detail
        n.inputs['Roughness'].default_value = rough_
        nt.links.new(geo.outputs['Position'], n.inputs['Vector'])
        return n
    macro, meso, micro_n = nz(1.6, 3.0), nz(11.0, 5.0, 0.6), nz(95.0, 6.0, 0.7)
    col = K._mix(nt, K._maprange(nt, macro.outputs['Fac'], 0.35, 0.65), c1, c2)
    col = K._mix(nt, K._maprange(nt, meso.outputs['Fac'], 0.4, 0.7, 0.0, 0.6), col, c3)
    col = K._mix(nt, K._maprange(nt, micro_n.outputs['Fac'], 0.3, 0.7, 0.0, 0.35), col, (c1[0] * 0.5, c1[1] * 0.5, c1[2] * 0.45, 1))
    patch = nz(4.5, 6.0, 0.65)
    col = K._mix(nt, K._maprange(nt, patch.outputs['Fac'], 0.58, 0.64, 0.0, dirt_amt), col, dirt)
    vo = nt.nodes.new('ShaderNodeTexVoronoi')
    vo.inputs['Scale'].default_value = 160.0
    nt.links.new(geo.outputs['Position'], vo.inputs['Vector'])
    peb = K._maprange(nt, vo.outputs['Distance'], 0.08, 0.16, 0.25, 0.0)
    col = K._mix(nt, peb, col, (0.32, 0.3, 0.27, 1))
    if rock:
        sep = nt.nodes.new('ShaderNodeSeparateXYZ')
        nt.links.new(geo.outputs['Normal'], sep.inputs['Vector'])
        f = K._maprange(nt, sep.outputs['Z'], slope[0], slope[1], 1.0, 0.0)
        col = K._mix(nt, f, col, rock)
    z = K._objz(nt)
    if low:
        col = K._mix(nt, K._maprange(nt, z, low_z[0], low_z[1], 1.0, 0.0), col, low)
    if deep:
        col = K._mix(nt, K._maprange(nt, z, -0.24, -0.08, 1.0, 0.0), col, deep)
    nt.links.new(col, p.inputs['Base Color'])
    K._in(p, 'Roughness', rough)
    K._in(p, ('Specular IOR Level', 'Specular'), 0.25)
    h = nt.nodes.new('ShaderNodeMath')
    h.operation = 'ADD'
    nt.links.new(micro_n.outputs['Fac'], h.inputs[0])
    nt.links.new(peb, h.inputs[1])
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = micro
    b.inputs['Distance'].default_value = 0.004
    nt.links.new(h.outputs['Value'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


def m_crag(name, rock1, rock2, snow, snow_z=0.6, snow_on=True):
    """Craggy rock: voronoi cracks, strata, slope-aware snow."""
    m = m_mountain_real(name, rock1, rock2, snow, snow_z, snow_on)
    nt = m.node_tree
    p = nt.nodes['Principled BSDF']
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    vo = nt.nodes.new('ShaderNodeTexVoronoi')
    vo.feature = 'DISTANCE_TO_EDGE'
    vo.inputs['Scale'].default_value = 22.0
    nt.links.new(geo.outputs['Position'], vo.inputs['Vector'])
    n = nt.nodes.new('ShaderNodeTexNoise')
    n.inputs['Scale'].default_value = 60.0
    n.inputs['Detail'].default_value = 8.0
    nt.links.new(geo.outputs['Position'], n.inputs['Vector'])
    h = nt.nodes.new('ShaderNodeMath')
    h.operation = 'MULTIPLY_ADD'
    nt.links.new(vo.outputs['Distance'], h.inputs[0])
    h.inputs[1].default_value = 3.0
    nt.links.new(n.outputs['Fac'], h.inputs[2])
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = 0.8
    b.inputs['Distance'].default_value = 0.01
    nt.links.new(h.outputs['Value'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    bc = p.inputs['Base Color'].links[0].from_socket
    crack = K._maprange(nt, vo.outputs['Distance'], 0.0, 0.05, 0.6, 0.0)
    c = K._mix(nt, crack, bc, (0.02, 0.02, 0.02, 1))
    nt.links.new(c, p.inputs['Base Color'])
    return m


def real_terrain_materials():
    S = Ctx.style
    P = S['pal']
    M = Ctx.mats
    G = P.get('ground', {})
    def gcol(key, default):
        return G.get(key, default)
    pl, fo, hi, ma, mb = P['plains'], P['forest'], P['hills'], P['marsh'], P['mtn_base']
    M['plains'] = m_ground('plains', pl[0], pl[1], gcol('plains3', (pl[0][0] * 1.25, pl[0][1] * 1.1, pl[0][2] * 0.8, 1)), P['mud'], low=P['bank'])
    M['forest'] = m_ground('forest', fo[0], fo[1], gcol('forest3', P['mud']), P['mud'], dirt_amt=0.55, low=P['bank'])
    M['hills'] = m_ground('hills', hi[0], hi[1], gcol('hills3', P['bank']), P['mud'], rock=P['hill_rock'], slope=(0.55, 0.78), low=P['bank'])
    M['marsh'] = m_ground('marsh', ma[0], ma[1], gcol('marsh3', P['mud']), P['mud'], dirt_amt=0.6, low=P['mud'], low_z=(-0.05, -0.015))
    M['lake'] = m_ground('lake', pl[0], pl[1], P['bank'], P['mud'], low=P['sand'], low_z=(-0.08, -0.025), deep=P['deep'])
    M['mountain'] = m_ground('mountain', mb[0], mb[1], P['hill_rock'], P['mud'], dirt_amt=0.3, low=P['bank'])
    M['mtn'] = m_crag('mtn', P['mtn_rock'][0], P['mtn_rock'][1], P['snow'], S.get('snow_z', 0.6), S.get('snow', True))


# --------------------------------------------------------------------------
# Realistic fields, foliage and turned playing pieces

def m_furrow(name):
    m, nt, p = K._new(name)
    tc = nt.nodes.new('ShaderNodeTexCoord')
    w = nt.nodes.new('ShaderNodeTexWave')
    w.inputs['Scale'].default_value = 7.0
    w.inputs['Distortion'].default_value = 0.6
    w.inputs['Detail'].default_value = 2.0
    nt.links.new(tc.outputs['Object'], w.inputs['Vector'])
    oi = nt.nodes.new('ShaderNodeObjectInfo')
    n = K._noise(nt, 70.0, 5.0)
    f = K._maprange(nt, w.outputs['Fac'], 0.3, 0.8, 0.0, 0.55)
    c = K._mix(nt, f, oi.outputs['Color'], (0.06, 0.045, 0.03, 1))
    c = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.4, 0.7, 0.0, 0.3), c, (0.1, 0.08, 0.05, 1))
    nt.links.new(c, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.95)
    h = nt.nodes.new('ShaderNodeMath')
    h.operation = 'ADD'
    nt.links.new(w.outputs['Fac'], h.inputs[0])
    nt.links.new(n.outputs['Fac'], h.inputs[1])
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = 0.7
    b.inputs['Distance'].default_value = 0.006
    nt.links.new(h.outputs['Value'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


def fields_real(hx, taken, avoid=0.0):
    rnd = hx.rnd
    P = Ctx.style['pal']
    ang = rnd.uniform(0, math.pi)
    for g in K.scatter(hx, rnd.randint(2, 4), 0.3, avoid_center=avoid, taken=taken, margin=0.18, river_clear=0.26):
        c = rnd.choice(P['field_cols'])
        sx, sy = rnd.uniform(0.26, 0.36), rnd.uniform(0.17, 0.24)
        add(Ctx.meshes['furrow_patch'], g - Vector((0, 0, 0.002)), ang + rnd.uniform(-0.1, 0.1), (sx, sy, 1.0), K.jitter_col(c, rnd, 0.08))


def leaf_canopy(bm, center, radius, rnd, mi, n=26):
    for j in range(n):
        d = Vector((rnd.gauss(0, 1), rnd.gauss(0, 1), rnd.gauss(0, 0.8))).normalized() * radius * rnd.uniform(0.35, 1.0)
        c = center + Vector((d.x, d.y, d.z * 0.85))
        vs = bmesh.ops.create_icosphere(bm, subdivisions=1, radius=rnd.uniform(0.012, 0.021), matrix=Matrix.Translation(c))['verts']
        for v in vs:
            v.co += Vector((rnd.uniform(-1, 1), rnd.uniform(-1, 1), rnd.uniform(-1, 1))) * 0.004
        for fc in faces_of(vs):
            fc.material_index = mi


def lathe(bm, profile, segs, mi):
    """profile: list of (r, z) from bottom to top; returns created verts."""
    rings = []
    for (r, z) in profile:
        ring = []
        for i in range(segs):
            a = 2 * math.pi * i / segs
            ring.append(bm.verts.new((math.cos(a) * r, math.sin(a) * r, z)))
        rings.append(ring)
    for k in range(len(rings) - 1):
        A, B = rings[k], rings[k + 1]
        for i in range(segs):
            j = (i + 1) % segs
            bm.faces.new((A[i], A[j], B[j], B[i])).material_index = mi
    bm.faces.new(list(reversed(rings[0]))).material_index = mi
    bm.faces.new(rings[-1]).material_index = mi


def build_real_extras():
    M, me = Ctx.mats, Ctx.meshes
    rnd = random.Random(23)
    M['furrow'] = m_furrow('furrow')
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.002), (1.0, 1.0, 0.008), 0)
    me['furrow_patch'] = to_mesh(bm, 'furrow_patch', [M['furrow']])
    K.fields = fields_real
    for k in range(4):
        bm = bmesh.new()
        branch(bm, Vector((0, 0, -0.01)), Vector((rnd.uniform(-0.1, 0.1), rnd.uniform(-0.1, 0.1), 1)).normalized(), 0.085, 0.012, 2, rnd)
        leaf_canopy(bm, Vector((0, 0, 0.125 + 0.01 * k)), 0.062 + 0.006 * k, rnd, 1, n=30)
        me[f'broad{k}'] = to_mesh(bm, f'broad{k}', [M['trunk'], M['leaf_broad']])
    bm = bmesh.new()
    leaf_canopy(bm, Vector((0, 0, 0.025)), 0.03, rnd, 0, n=10)
    me['bush'] = to_mesh(bm, 'bush', [M['leaf_broad']])
    # turned playing pieces
    hero = [(0.12, 0.0), (0.12, 0.022), (0.105, 0.03), (0.078, 0.04), (0.06, 0.06), (0.05, 0.11), (0.046, 0.15),
            (0.066, 0.165), (0.066, 0.175), (0.04, 0.185), (0.044, 0.2), (0.05, 0.22), (0.048, 0.245), (0.036, 0.265), (0.018, 0.278), (0.001, 0.282)]
    bm = bmesh.new()
    lathe(bm, hero, 32, 0)
    bm_cone(bm, 8, 0.005, 0.005, 0.36, 0.02, 1, x=0.09, y=0.03)
    vs = [bm.verts.new(c) for c in [(0.093, 0.03, 0.375), (0.093, 0.03, 0.285), (0.215, 0.03, 0.34), (0.17, 0.03, 0.33)]]
    bm.faces.new((vs[0], vs[1], vs[3], vs[2])).material_index = 2
    me['hero'] = to_mesh(bm, 'hero', [M['piece_hero'], M['piece_trim'], M['piece_flag']], smooth=True, auto=50)
    enemy = [(0.09, 0.0), (0.09, 0.02), (0.075, 0.03), (0.06, 0.05), (0.052, 0.1), (0.05, 0.13), (0.058, 0.15), (0.05, 0.17),
             (0.044, 0.19), (0.04, 0.215), (0.028, 0.235), (0.012, 0.245), (0.001, 0.248)]
    bm = bmesh.new()
    lathe(bm, enemy, 28, 0)
    for s_ in (-1, 1):
        bm_ico(bm, 0.009, (s_ * 0.016, -0.036, 0.2), 1, sub=2)
    me['enemy'] = to_mesh(bm, 'enemy', [M['piece_enemy'], M['enemy_eye']], smooth=True, auto=50)
    war = [(0.13, 0.0), (0.13, 0.03), (0.11, 0.045), (0.085, 0.07), (0.07, 0.15), (0.066, 0.22), (0.085, 0.24), (0.085, 0.255),
           (0.06, 0.27), (0.064, 0.3), (0.07, 0.33), (0.06, 0.36), (0.05, 0.37), (0.05, 0.372)]
    bm = bmesh.new()
    lathe(bm, war, 32, 0)
    for j in range(8):
        a = j / 8 * 6.283
        bm_cone(bm, 4, 0.012, 0.0, 0.05, 0.37, 2, x=math.cos(a) * 0.042, y=math.sin(a) * 0.042)
    for s_ in (-1, 1):
        bm_ico(bm, 0.011, (s_ * 0.022, -0.06, 0.315), 1, sub=2)
    me['warlord'] = to_mesh(bm, 'warlord', [M['piece_enemy'], M['warlord_eye'], M['piece_crown']], smooth=True, auto=50)


def piece_materials(hero=None, trim=None, flag=None, enemy=None, crown=None):
    M = Ctx.mats
    M['piece_hero'] = hero or K.m_flat('piece_hero', lin('#8d8e96'), 0.32, metal=1.0)
    M['piece_trim'] = trim or K.m_flat('piece_trim', lin('#c9a35a'), 0.3, metal=1.0)
    M['piece_flag'] = flag or K.m_flat('piece_flag', lin('#8c2f2a'), 0.75)
    M['piece_enemy'] = enemy or K.m_flat('piece_enemy', lin('#1a1a1c'), 0.25, metal=0.6)
    M['piece_crown'] = crown or K.m_flat('piece_crown', lin('#6b2622'), 0.3, metal=0.8)
