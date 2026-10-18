"""Procedural 2.5D hex-map diorama kit for Dice and Destiny concept renders.

Blender 4.0 / Cycles. Builds the Salt Marches region (real tile data) as
3D hex tiles with raised terrain, carved rivers, props, sites and tokens.
Concept art only; nothing here is game code.
"""
import bpy, bmesh, math, random, json
from mathutils import Vector, Matrix, noise
from bpy_extras.object_utils import world_to_camera_view

SQ3 = math.sqrt(3.0)
INR = SQ3 / 2.0
WATER = -0.035
EDGE_NORMALS = [Vector((math.cos(math.radians(a)), math.sin(math.radians(a)))) for a in (0, 60, 120)]

# --------------------------------------------------------------------------
# Real region data (dice-and-destiny-server/content/expedition_v1)
# (offset, terrain, site, enemy count)
TILES = {
    'meadow_crossing': [((0, 0), 'plains', None, 0), ((1, 0), 'plains', None, 0), ((1, -1), 'forest', None, 0), ((0, -1), 'hills', None, 0), ((-1, 0), 'lake', None, 0), ((-1, 1), 'plains', 'village', 0), ((0, 1), 'plains', None, 0)],
    'cairn_hills': [((0, 0), 'hills', None, 1), ((1, 0), 'hills', None, 0), ((1, -1), 'plains', 'cairn', 0), ((0, -1), 'forest', None, 0), ((-1, 0), 'mountain', None, 0), ((-1, 1), 'plains', None, 0), ((0, 1), 'hills', None, 0)],
    'saltreed_fen': [((0, 0), 'plains', None, 1), ((1, 0), 'plains', 'barge', 0), ((1, -1), 'marsh', None, 0), ((0, -1), 'forest', None, 0), ((-1, 0), 'lake', None, 0), ((-1, 1), 'plains', 'village', 0), ((0, 1), 'hills', None, 0)],
    'reed_shallows': [((0, 0), 'marsh', None, 1), ((1, 0), 'plains', None, 0), ((1, -1), 'plains', None, 0), ((0, -1), 'forest', None, 0), ((-1, 0), 'marsh', None, 0), ((-1, 1), 'plains', 'shrine', 0), ((0, 1), 'hills', None, 0)],
    'brinehold_march': [((0, 0), 'plains', None, 1), ((1, 0), 'hills', None, 0), ((1, -1), 'hills', 'keep', 0), ((0, -1), 'mountain', None, 0), ((-1, 0), 'forest', 'chapel', 0), ((-1, 1), 'marsh', None, 0), ((0, 1), 'plains', 'hermit', 0)],
    'stillwater': [((0, 0), 'forest', None, 2), ((1, 0), 'plains', None, 0), ((1, -1), 'forest', None, 0), ((0, -1), 'hills', None, 0), ((-1, 0), 'lake', None, 0), ((-1, 1), 'marsh', None, 0), ((0, 1), 'plains', 'monastery', 0)],
    'greywharf_strand': [((0, 0), 'plains', 'town', 0), ((1, 0), 'plains', None, 0), ((1, -1), 'forest', None, 0), ((0, -1), 'hills', None, 0), ((-1, 0), 'lake', None, 0), ((-1, 1), 'marsh', None, 0), ((0, 1), 'hills', 'grotto', 0)],
    'drowned_reach': [((0, 0), 'marsh', None, 2), ((1, 0), 'mountain', None, 0), ((1, -1), 'hills', None, 0), ((0, -1), 'forest', None, 0), ((-1, 0), 'plains', None, 0), ((-1, 1), 'lake', None, 0), ((0, 1), 'plains', None, 0)],
}
SLOTS = {
    'start': ((0, 0), 'meadow_crossing'), 'east': ((2, 1), 'cairn_hills'), 'south': ((-1, 3), 'saltreed_fen'),
    'west': ((-3, 2), 'reed_shallows'), 'northeast': ((3, -2), 'brinehold_march'), 'far_east': ((4, 2), 'stillwater'),
    'strand': ((6, 3), 'greywharf_strand'), 'reach': ((7, 0), 'drowned_reach'),
}
EMPTY_SLOTS = {'gap_ne': (5, -1), 'gap_n': (1, -3), 'gap_nw': (-2, -1), 'gap_s': (1, 4), 'gap_se': (3, 5), 'gap_w': (-5, 1), 'gap_far_e': (8, 4), 'gap_far_w': (-4, -2)}
SITE_NAMES = {(-1, 1): 'Hollow Ford', (-2, 4): 'Saltreed Village', (3, 0): 'Drowned Cairn', (0, 3): 'Wrecked Barge',
              (-4, 3): 'Wayside Shrine', (4, -3): 'Brinehold Keep', (2, -2): 'Sunken Chapel', (3, -1): "Hermit's Fire",
              (4, 3): 'Stillwater Monastery', (6, 3): 'Greywharf', (6, 4): 'Kelp Grotto'}


def hex_world(q, r):
    return Vector((SQ3 * (q + r / 2.0), -1.5 * r, 0.0))


def mid(a, b):
    return (hex_world(*a) + hex_world(*b)) / 2.0


def at(h, dx=0.0, dy=0.0):
    return hex_world(*h) + Vector((dx, dy, 0))


RIVERS = [
    # Hollow Ford: Cairn mountain foot -> meadow -> the ford village -> lake
    [at((1, 1), -0.30, -0.28), mid((1, 1), (0, 1)), at((0, 1), 0.05, 0.10), mid((0, 1), (-1, 1)), at((-1, 1), 0.28, 0.30), mid((-1, 1), (-1, 0)), at((-1, 0), 0.15, -0.10)],
    # Brinehold: mountain -> chapel woods -> marsh -> through the meadow -> lake
    [at((3, -3), -0.32, -0.30), mid((3, -3), (2, -2)), at((2, -2), 0.30, -0.12), mid((2, -2), (2, -1)), at((2, -1), 0.0, 0.05), mid((2, -1), (1, 0)), at((1, 0), -0.15, 0.20), mid((1, 0), (0, 0)), at((0, 0), -0.05, 0.30), mid((0, 0), (-1, 0)), at((-1, 0), 0.25, 0.05)],
    # Reach: Regent's mountain -> plains -> hills gorge -> strand lake
    [at((8, 0), -0.25, -0.35), mid((8, 0), (7, 1)), at((7, 1), 0.0, 0.0), mid((7, 1), (6, 2)), at((6, 2), 0.1, -0.05), mid((6, 2), (5, 3)), at((5, 3), 0.15, 0.1)],
]

# --------------------------------------------------------------------------
# Helpers


def lin(h, a=1.0):
    h = h.lstrip('#')
    c = [int(h[i:i + 2], 16) / 255.0 for i in (0, 2, 4)]
    return tuple([(x / 12.92 if x <= 0.04045 else ((x + 0.055) / 1.055) ** 2.4) for x in c] + [a])


def jitter_col(c, rnd, amt=0.12):
    k = 1.0 + rnd.uniform(-amt, amt)
    return (c[0] * k * (1 + rnd.uniform(-amt, amt) * 0.5), c[1] * k, c[2] * k * (1 + rnd.uniform(-amt, amt) * 0.5), 1.0)


def smoothstep(a, b, x):
    if b == a:
        return 0.0 if x < a else 1.0
    t = max(0.0, min(1.0, (x - a) / (b - a)))
    return t * t * (3 - 2 * t)


def lerp(a, b, t):
    return a + (b - a) * t


def edge_dist(p, rr=1.0):
    return INR * rr - max(abs(p.x * n.x + p.y * n.y) for n in EDGE_NORMALS)


def catmull(points, steps=14):
    pts = [points[0]] + points + [points[-1]]
    out = []
    for i in range(1, len(pts) - 2):
        p0, p1, p2, p3 = pts[i - 1], pts[i], pts[i + 1], pts[i + 2]
        for s in range(steps):
            t = s / steps
            t2, t3 = t * t, t * t * t
            out.append(0.5 * ((2 * p1) + (-p0 + p2) * t + (2 * p0 - 5 * p1 + 4 * p2 - p3) * t2 + (-p0 + 3 * p1 - 3 * p2 + p3) * t3))
    out.append(points[-1])
    return [Vector((p.x, p.y)) for p in out]


def seg_dist(p, a, b):
    ab = b - a
    t = max(0.0, min(1.0, (p - a).dot(ab) / max(ab.length_squared, 1e-9)))
    return (a + ab * t - p).length


# --------------------------------------------------------------------------
# Scene state

class Ctx:
    style = None
    mats = {}
    meshes = {}
    created = []
    rivers = []          # list of polylines (Vector2)
    hexes = {}           # (q,r) -> Hex
    labels = []


def link(ob):
    bpy.context.scene.collection.objects.link(ob)
    Ctx.created.append(ob)
    return ob


def add(mesh, loc, rot=0.0, scale=1.0, color=None, name='prop', tilt=None):
    ob = bpy.data.objects.new(name, mesh)
    ob.location = loc
    ob.rotation_euler = (tilt[0] if tilt else 0.0, tilt[1] if tilt else 0.0, rot)
    ob.scale = (scale, scale, scale) if not isinstance(scale, tuple) else scale
    if color is not None:
        ob.color = color
    return link(ob)


def reset():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    Ctx.mats, Ctx.meshes, Ctx.created, Ctx.rivers, Ctx.hexes, Ctx.labels = {}, {}, [], [], {}, []


# --------------------------------------------------------------------------
# Materials

def _in(node, names, value):
    for n in (names if isinstance(names, (list, tuple)) else (names,)):
        if n in node.inputs:
            node.inputs[n].default_value = value
            return node.inputs[n]


def _new(name):
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    nt = m.node_tree
    return m, nt, nt.nodes['Principled BSDF']


def m_flat(name, col, rough=0.75, metal=0.0, obj_color=False, emit=None, emit_k=0.0, spec=0.35):
    m, nt, p = _new(name)
    _in(p, 'Base Color', col)
    _in(p, 'Roughness', rough)
    _in(p, 'Metallic', metal)
    _in(p, ('Specular IOR Level', 'Specular'), spec)
    if obj_color:
        oi = nt.nodes.new('ShaderNodeObjectInfo')
        mix = nt.nodes.new('ShaderNodeMixRGB')
        mix.blend_type = 'MULTIPLY'
        mix.inputs['Fac'].default_value = 1.0
        mix.inputs['Color1'].default_value = col
        nt.links.new(oi.outputs['Color'], mix.inputs['Color2'])
        nt.links.new(mix.outputs['Color'], p.inputs['Base Color'])
    if emit is not None:
        _in(p, ('Emission Color', 'Emission'), emit)
        _in(p, 'Emission Strength', emit_k)
    return m


def _noise(nt, scale, detail=4.0, coord='Position'):
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    n = nt.nodes.new('ShaderNodeTexNoise')
    n.inputs['Scale'].default_value = scale
    n.inputs['Detail'].default_value = detail
    nt.links.new(geo.outputs[coord], n.inputs['Vector'])
    return n


def _mix(nt, fac_socket, a, b, fac_value=None):
    mx = nt.nodes.new('ShaderNodeMixRGB')
    if fac_socket is not None:
        nt.links.new(fac_socket, mx.inputs['Fac'])
    else:
        mx.inputs['Fac'].default_value = fac_value
    for sock, val in ((mx.inputs['Color1'], a), (mx.inputs['Color2'], b)):
        if isinstance(val, tuple):
            sock.default_value = val
        else:
            nt.links.new(val, sock)
    return mx.outputs['Color']


def _maprange(nt, src, fmin, fmax, tmin=0.0, tmax=1.0):
    mr = nt.nodes.new('ShaderNodeMapRange')
    nt.links.new(src, mr.inputs['Value'])
    mr.inputs['From Min'].default_value = fmin
    mr.inputs['From Max'].default_value = fmax
    mr.inputs['To Min'].default_value = tmin
    mr.inputs['To Max'].default_value = tmax
    mr.interpolation_type = 'SMOOTHSTEP'
    return mr.outputs['Result']


def _objz(nt):
    tc = nt.nodes.new('ShaderNodeTexCoord')
    sep = nt.nodes.new('ShaderNodeSeparateXYZ')
    nt.links.new(tc.outputs['Object'], sep.inputs['Vector'])
    return sep.outputs['Z']


def _bump(nt, p, scale, strength, distance=0.02):
    n = _noise(nt, scale, 6.0)
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = strength
    b.inputs['Distance'].default_value = distance
    nt.links.new(n.outputs['Fac'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])


def m_terrain(name, c1, c2, rock=None, slope=(0.70, 0.88), low=None, low_z=(-0.075, -0.02), deep=None,
              nscale=1.8, rough=0.9, speck=0.12, bump=0.25):
    m, nt, p = _new(name)
    n = _noise(nt, nscale, 3.0)
    ramp = nt.nodes.new('ShaderNodeValToRGB')
    ramp.color_ramp.elements[0].color = c1
    ramp.color_ramp.elements[0].position = 0.38
    ramp.color_ramp.elements[1].color = c2
    ramp.color_ramp.elements[1].position = 0.64
    nt.links.new(n.outputs['Fac'], ramp.inputs['Fac'])
    col = ramp.outputs['Color']
    if speck:
        sn = _noise(nt, 38.0, 2.0)
        f = _maprange(nt, sn.outputs['Fac'], 0.55, 0.75, 0.0, speck)
        dark = (c1[0] * 0.55, c1[1] * 0.55, c1[2] * 0.55, 1.0)
        col = _mix(nt, f, col, dark)
    if rock:
        geo = nt.nodes.new('ShaderNodeNewGeometry')
        sep = nt.nodes.new('ShaderNodeSeparateXYZ')
        nt.links.new(geo.outputs['Normal'], sep.inputs['Vector'])
        f = _maprange(nt, sep.outputs['Z'], slope[0], slope[1], 1.0, 0.0)
        col = _mix(nt, f, col, rock)
    z = _objz(nt)
    if low:
        f = _maprange(nt, z, low_z[0], low_z[1], 1.0, 0.0)
        col = _mix(nt, f, col, low)
    if deep:
        f = _maprange(nt, z, -0.24, -0.08, 1.0, 0.0)
        col = _mix(nt, f, col, deep)
    nt.links.new(col, p.inputs['Base Color'])
    _in(p, 'Roughness', rough)
    _in(p, ('Specular IOR Level', 'Specular'), 0.25)
    if bump:
        _bump(nt, p, 45.0, bump, 0.01)
    return m


def m_strata(name, bands, thickness, wobble=0.035):
    """bands: list of (position 0=bottom..1=top, color) constant ramp."""
    m, nt, p = _new(name)
    z = _objz(nt)
    n = _noise(nt, 5.0, 3.0)
    add = nt.nodes.new('ShaderNodeMath')
    add.operation = 'MULTIPLY_ADD'
    nt.links.new(n.outputs['Fac'], add.inputs[0])
    add.inputs[1].default_value = wobble
    nt.links.new(z, add.inputs[2])
    zr = _maprange(nt, add.outputs['Value'], -thickness, 0.0)
    ramp = nt.nodes.new('ShaderNodeValToRGB')
    ramp.color_ramp.interpolation = 'CONSTANT'
    els = ramp.color_ramp.elements
    els[0].position, els[0].color = bands[0]
    els[1].position, els[1].color = bands[1]
    for pos, c in bands[2:]:
        e = els.new(pos)
        e.color = c
    nt.links.new(zr, ramp.inputs['Fac'])
    sn = _noise(nt, 30.0, 2.0)
    f = _maprange(nt, sn.outputs['Fac'], 0.5, 0.8, 0.0, 0.25)
    col = _mix(nt, f, ramp.outputs['Color'], (0.02, 0.018, 0.015, 1.0))
    nt.links.new(col, p.inputs['Base Color'])
    _in(p, 'Roughness', 0.95)
    _bump(nt, p, 22.0, 0.45, 0.02)
    return m


def m_water(name, tint, rough=0.06, trans=0.9, ripple=0.18, emit=None):
    m, nt, p = _new(name)
    _in(p, 'Base Color', tint)
    _in(p, 'Roughness', rough)
    _in(p, ('Transmission Weight', 'Transmission'), trans)
    _in(p, 'IOR', 1.333)
    n = _noise(nt, 9.0, 3.0)
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = ripple
    b.inputs['Distance'].default_value = 0.01
    nt.links.new(n.outputs['Fac'], b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    out = nt.nodes['Material Output']
    lp = nt.nodes.new('ShaderNodeLightPath')
    tr = nt.nodes.new('ShaderNodeBsdfTransparent')
    mix = nt.nodes.new('ShaderNodeMixShader')
    nt.links.new(lp.outputs['Is Shadow Ray'], mix.inputs['Fac'])
    nt.links.new(p.outputs['BSDF'], mix.inputs[1])
    nt.links.new(tr.outputs['BSDF'], mix.inputs[2])
    nt.links.new(mix.outputs['Shader'], out.inputs['Surface'])
    if emit:
        _in(p, ('Emission Color', 'Emission'), emit[0])
        _in(p, 'Emission Strength', emit[1])
    return m


def m_stripes(name, base, stripe_dark=0.72, scale=55.0):
    m, nt, p = _new(name)
    tc = nt.nodes.new('ShaderNodeTexCoord')
    w = nt.nodes.new('ShaderNodeTexWave')
    w.inputs['Scale'].default_value = scale
    w.inputs['Distortion'].default_value = 1.5
    nt.links.new(tc.outputs['Object'], w.inputs['Vector'])
    f = _maprange(nt, w.outputs['Fac'], 0.3, 0.9, 0.0, 1.0)
    oi = nt.nodes.new('ShaderNodeObjectInfo')
    darker = nt.nodes.new('ShaderNodeMixRGB')
    darker.blend_type = 'MULTIPLY'
    darker.inputs['Fac'].default_value = 1.0
    nt.links.new(oi.outputs['Color'], darker.inputs['Color1'])
    darker.inputs['Color2'].default_value = (stripe_dark, stripe_dark, stripe_dark, 1)
    col = _mix(nt, f, darker.outputs['Color'], oi.outputs['Color'])
    nt.links.new(col, p.inputs['Base Color'])
    _in(p, 'Roughness', 0.85)
    return m


def m_emit(name, col, k):
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.remove(nt.nodes['Principled BSDF'])
    e = nt.nodes.new('ShaderNodeEmission')
    e.inputs['Color'].default_value = col
    e.inputs['Strength'].default_value = k
    nt.links.new(e.outputs['Emission'], nt.nodes['Material Output'].inputs['Surface'])
    return m


def m_mountain(name, rock1, rock2, snow, snow_z=0.48, snow_on=True):
    m, nt, p = _new(name)
    n = _noise(nt, 3.5, 4.0)
    ramp = nt.nodes.new('ShaderNodeValToRGB')
    ramp.color_ramp.elements[0].color = rock1
    ramp.color_ramp.elements[1].color = rock2
    nt.links.new(n.outputs['Fac'], ramp.inputs['Fac'])
    col = ramp.outputs['Color']
    if snow_on:
        z = _objz(nt)
        n2 = _noise(nt, 6.0, 2.0)
        add = nt.nodes.new('ShaderNodeMath')
        add.operation = 'MULTIPLY_ADD'
        nt.links.new(n2.outputs['Fac'], add.inputs[0])
        add.inputs[1].default_value = 0.18
        nt.links.new(z, add.inputs[2])
        fz = _maprange(nt, add.outputs['Value'], snow_z, snow_z + 0.04)
        geo = nt.nodes.new('ShaderNodeNewGeometry')
        sep = nt.nodes.new('ShaderNodeSeparateXYZ')
        nt.links.new(geo.outputs['Normal'], sep.inputs['Vector'])
        fs = _maprange(nt, sep.outputs['Z'], 0.25, 0.45)
        mul = nt.nodes.new('ShaderNodeMath')
        mul.operation = 'MULTIPLY'
        nt.links.new(fz, mul.inputs[0])
        nt.links.new(fs, mul.inputs[1])
        col = _mix(nt, mul.outputs['Value'], col, snow)
    nt.links.new(col, p.inputs['Base Color'])
    _in(p, 'Roughness', 0.85)
    _bump(nt, p, 18.0, 0.35, 0.02)
    return m


# --------------------------------------------------------------------------
# Mesh building

def faces_of(verts):
    return {f for v in verts for f in v.link_faces}


def to_mesh(bm, name, mats, smooth=False, auto=None):
    me = bpy.data.meshes.new(name)
    bm.to_mesh(me)
    bm.free()
    for m in mats:
        me.materials.append(m)
    for poly in me.polygons:
        poly.use_smooth = smooth
    if auto is not None:
        me.use_auto_smooth = True
        me.auto_smooth_angle = math.radians(auto)
    return me


def bm_cone(bm, segs, r1, r2, h, z0, mi, x=0.0, y=0.0, rot=0.0, tilt=None):
    mat = Matrix.Translation((x, y, z0))
    if tilt:
        mat = mat @ Matrix.Rotation(tilt[0], 4, 'X') @ Matrix.Rotation(tilt[1], 4, 'Y')
    mat = mat @ Matrix.Rotation(rot, 4, 'Z') @ Matrix.Translation((0, 0, h / 2))
    ret = bmesh.ops.create_cone(bm, cap_ends=True, cap_tris=False, segments=segs, radius1=r1, radius2=r2, depth=h, matrix=mat)
    for f in faces_of(ret['verts']):
        f.material_index = mi
    return ret['verts']


def bm_box(bm, center, size, mi, rot=0.0):
    mat = Matrix.Translation(center) @ Matrix.Rotation(rot, 4, 'Z') @ Matrix.Diagonal((size[0], size[1], size[2], 1.0))
    ret = bmesh.ops.create_cube(bm, size=1.0, matrix=mat)
    for f in faces_of(ret['verts']):
        f.material_index = mi
    return ret['verts']


def bm_ico(bm, r, center, mi, sub=1, squash=(1, 1, 1), rnd=None, jit=0.0):
    mat = Matrix.Translation(center) @ Matrix.Diagonal((squash[0], squash[1], squash[2], 1.0))
    ret = bmesh.ops.create_icosphere(bm, subdivisions=sub, radius=r, matrix=mat)
    if rnd and jit:
        for v in ret['verts']:
            v.co += Vector((rnd.uniform(-jit, jit), rnd.uniform(-jit, jit), rnd.uniform(-jit, jit)))
    for f in faces_of(ret['verts']):
        f.material_index = mi
    return ret['verts']


def bm_uvsphere(bm, r, center, mi, u=12, v=8):
    ret = bmesh.ops.create_uvsphere(bm, u_segments=u, v_segments=v, radius=r, matrix=Matrix.Translation(center))
    for f in faces_of(ret['verts']):
        f.material_index = mi
    return ret['verts']


def build_meshes():
    S, M = Ctx.style, Ctx.mats
    rnd = random.Random(7)
    me = Ctx.meshes
    # Trees -------------------------------------------------------------
    for k in range(4):
        bm = bmesh.new()
        bm_cone(bm, 5, 0.013, 0.010, 0.06, 0.0, 0)
        tiers = [(0.078, 0.11, 0.04), (0.06, 0.10, 0.09), (0.043, 0.09, 0.14)]
        if k % 2:
            tiers = [(0.07, 0.12, 0.035), (0.05, 0.12, 0.11)]
        for (r, h, z) in tiers:
            bm_cone(bm, 6 + k % 2, r * rnd.uniform(0.9, 1.1), 0.0, h, z, 1, rot=rnd.uniform(0, 1))
        me[f'conifer{k}'] = to_mesh(bm, f'conifer{k}', [M['trunk'], M['leaf_conifer']])
    for k in range(4):
        bm = bmesh.new()
        bm_cone(bm, 6, 0.016, 0.011, 0.08, 0.0, 0)
        bm_ico(bm, 0.07, (0, 0, 0.125), 1, sub=1, squash=(1, 1, 0.85), rnd=rnd, jit=0.012)
        if k >= 2:
            bm_ico(bm, 0.045, (0.04, 0.02, 0.1), 1, sub=1, rnd=rnd, jit=0.008)
        me[f'broad{k}'] = to_mesh(bm, f'broad{k}', [M['trunk'], M['leaf_broad']])
    for k in range(3):
        bm = bmesh.new()
        bm_cone(bm, 5, 0.012, 0.004, 0.17, 0.0, 0, tilt=(rnd.uniform(-0.1, 0.1), rnd.uniform(-0.1, 0.1)))
        for j in range(3):
            a = rnd.uniform(0, 6.28)
            bm_cone(bm, 4, 0.006, 0.001, 0.08, 0.06 + j * 0.03, 0, x=0, y=0, tilt=(math.cos(a) * 0.9, math.sin(a) * 0.9))
        me[f'dead{k}'] = to_mesh(bm, f'dead{k}', [M['deadwood']])
    bm = bmesh.new()
    bm_ico(bm, 0.038, (0, 0, 0.018), 0, sub=1, squash=(1, 1, 0.75), rnd=rnd, jit=0.006)
    me['bush'] = to_mesh(bm, 'bush', [M['leaf_broad']])
    for k in range(3):
        bm = bmesh.new()
        bm_ico(bm, 0.05, (0, 0, 0.01), 0, sub=1, squash=(1.0, 0.8, 0.6), rnd=rnd, jit=0.014)
        me[f'rock{k}'] = to_mesh(bm, f'rock{k}', [M['rock']])
    bm = bmesh.new()
    for j in range(8):
        a = rnd.uniform(0, 6.28)
        rr = rnd.uniform(0, 0.025)
        h = rnd.uniform(0.07, 0.13)
        bm_cone(bm, 3, 0.006, 0.0, h, 0.0, 0, x=math.cos(a) * rr, y=math.sin(a) * rr, tilt=(rnd.uniform(-0.25, 0.25), rnd.uniform(-0.25, 0.25)))
        if j % 3 == 0:
            bm_cone(bm, 5, 0.006, 0.006, 0.022, h * 0.7, 1, x=math.cos(a) * rr, y=math.sin(a) * rr)
    me['reeds'] = to_mesh(bm, 'reeds', [M['reed'], M['cattail']])
    for k in range(2):
        bm = bmesh.new()
        bm_box(bm, (0, 0, -0.004), (0.3, 0.2, 0.014), 0)
        for i in range(10):
            for j in range(6):
                x = -0.135 + i * 0.03 + rnd.uniform(-0.004, 0.004)
                y = -0.075 + j * 0.03 + rnd.uniform(-0.004, 0.004)
                bm_cone(bm, 4, 0.013, 0.002, 0.03 + rnd.uniform(-0.006, 0.006) + 0.01 * k, 0.0, 1, x=x, y=y, rot=rnd.uniform(0, 1))
        me[f'field{k}'] = to_mesh(bm, f'field{k}', [M['field_soil'], M['field']])
    bm = bmesh.new()
    bmesh.ops.create_circle(bm, cap_ends=True, segments=10, radius=0.025)
    me['lily'] = to_mesh(bm, 'lily', [M['lily']])
    # Houses --------------------------------------------------------------
    for k, (w, d, h, rh) in enumerate([(0.09, 0.065, 0.055, 0.045), (0.12, 0.065, 0.05, 0.045), (0.075, 0.06, 0.07, 0.05)]):
        bm = bmesh.new()
        bm_box(bm, (0, 0, h / 2), (w, d, h), 0)
        o = 0.012
        vs = [bm.verts.new(c) for c in [(-w / 2 - o, -d / 2 - o, h), (w / 2 + o, -d / 2 - o, h), (w / 2 + o, d / 2 + o, h), (-w / 2 - o, d / 2 + o, h), (-w / 2 - o, 0, h + rh), (w / 2 + o, 0, h + rh)]]
        for fv in ((vs[0], vs[1], vs[5], vs[4]), (vs[2], vs[3], vs[4], vs[5]), (vs[3], vs[0], vs[4]), (vs[1], vs[2], vs[5]), (vs[3], vs[2], vs[1], vs[0])):
            f = bm.faces.new(fv)
            f.material_index = 1
        bm_box(bm, (w * 0.18, -d / 2 - 0.002, 0.016), (0.018, 0.006, 0.032), 2)
        bm_box(bm, (-w * 0.22, -d / 2 - 0.002, h * 0.62), (0.014, 0.006, 0.014), 3)
        bm_box(bm, (w / 2 + 0.002, 0.0, h * 0.62), (0.006, 0.014, 0.014), 3)
        if k == 1:
            bm_box(bm, (-w * 0.3, 0.01, h + rh * 0.8), (0.014, 0.014, 0.04), 0)
        me[f'house{k}'] = to_mesh(bm, f'house{k}', [M['wall'], M['roof'], M['door'], M['window']])
    # Towers, walls, church ------------------------------------------------
    bm = bmesh.new()
    bm_cone(bm, 8, 0.05, 0.046, 0.26, 0.0, 0)
    bm_cone(bm, 8, 0.066, 0.0, 0.11, 0.26, 1)
    bm_box(bm, (0, -0.048, 0.18), (0.012, 0.01, 0.022), 3)
    me['tower'] = to_mesh(bm, 'tower', [M['stone'], M['roof_dark'], M['door'], M['window']])
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.04), (1.0, 0.03, 0.08), 0)
    for j in range(7):
        bm_box(bm, (-0.45 + j * 0.15, 0, 0.09), (0.06, 0.034, 0.02), 0)
    me['wall'] = to_mesh(bm, 'wall', [M['stone']])
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.17), (0.2, 0.2, 0.34), 0)
    for j in range(4):
        for s in (-1, 1):
            bm_box(bm, (-0.075 + j * 0.05, s * 0.098, 0.355), (0.028, 0.012, 0.03), 0)
            bm_box(bm, (s * 0.098, -0.075 + j * 0.05, 0.355), (0.012, 0.028, 0.03), 0)
    bm_box(bm, (0, -0.101, 0.045), (0.05, 0.006, 0.09), 2)
    for (x, z) in ((-0.05, 0.22), (0.05, 0.22), (0, 0.29)):
        bm_box(bm, (x, -0.101, z), (0.016, 0.006, 0.028), 3)
    me['keep'] = to_mesh(bm, 'keep', [M['stone'], M['roof_dark'], M['door'], M['window']])
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.045), (0.22, 0.09, 0.09), 0)
    o = 0.012
    w, d, h, rh = 0.22, 0.09, 0.09, 0.06
    vs = [bm.verts.new(c) for c in [(-w / 2 - o, -d / 2 - o, h), (w / 2 + o, -d / 2 - o, h), (w / 2 + o, d / 2 + o, h), (-w / 2 - o, d / 2 + o, h), (-w / 2 - o, 0, h + rh), (w / 2 + o, 0, h + rh)]]
    for fv in ((vs[0], vs[1], vs[5], vs[4]), (vs[2], vs[3], vs[4], vs[5]), (vs[3], vs[0], vs[4]), (vs[1], vs[2], vs[5])):
        bm.faces.new(fv).material_index = 1
    bm_box(bm, (0.13, 0, 0.11), (0.07, 0.07, 0.22), 0)
    bm_cone(bm, 4, 0.06, 0.0, 0.12, 0.22, 1, x=0.13, rot=math.pi / 4)
    for x in (-0.07, -0.02, 0.03):
        bm_box(bm, (x, -0.046, 0.055), (0.014, 0.006, 0.03), 3)
    bm_box(bm, (0.13, -0.036, 0.17), (0.02, 0.006, 0.03), 3)
    bm_box(bm, (-0.111, 0, 0.03), (0.006, 0.026, 0.06), 2)
    me['church'] = to_mesh(bm, 'church', [M['wall'], M['roof_dark'], M['door'], M['window']])
    # Ruins, menhirs, columns ---------------------------------------------
    bm = bmesh.new()
    bm_cone(bm, 4, 0.03, 0.02, 0.14, 0.0, 0, rot=0.4)
    me['menhir'] = to_mesh(bm, 'menhir', [M['stone']])
    bm = bmesh.new()
    bm_cone(bm, 8, 0.02, 0.019, 0.12, 0.0, 0)
    bm_box(bm, (0, 0, 0.0), (0.05, 0.05, 0.016), 0)
    me['column'] = to_mesh(bm, 'column', [M['stone']])
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.05), (0.18, 0.025, 0.10), 0)
    bm_box(bm, (0.05, 0, 0.12), (0.08, 0.025, 0.05), 0)
    bm_box(bm, (-0.03, -0.013, 0.06), (0.03, 0.006, 0.05), 2)
    me['ruinwall'] = to_mesh(bm, 'ruinwall', [M['stone'], M['roof_dark'], M['door']])
    # Grotto mound -----------------------------------------------------------
    bm = bmesh.new()
    bm_ico(bm, 0.22, (0, 0, 0.0), 0, sub=2, squash=(1.1, 0.9, 0.75), rnd=rnd, jit=0.03)
    bm_ico(bm, 0.07, (0.0, -0.17, 0.035), 1, sub=2, squash=(1.2, 0.5, 1.2))
    me['grotto'] = to_mesh(bm, 'grotto', [M['rock'], M['grotto_glow']])
    # Boat (wrecked barge) ---------------------------------------------------
    bm = bmesh.new()
    L, W, H = 0.30, 0.09, 0.05
    prof = [(-L / 2, 0.0), (-L / 2 + 0.05, W / 2), (L / 2 - 0.06, W / 2), (L / 2, 0.0)]
    top = [bm.verts.new((x, y, H)) for (x, y) in prof] + [bm.verts.new((x, -y, H)) for (x, y) in reversed(prof[1:-1])]
    bot = [bm.verts.new((x * 0.85, y * 0.5, 0.0)) for (x, y) in prof] + [bm.verts.new((x * 0.85, -y * 0.5, 0.0)) for (x, y) in reversed(prof[1:-1])]
    n = len(top)
    for i in range(n):
        j = (i + 1) % n
        bm.faces.new((bot[i], bot[j], top[j], top[i])).material_index = 0
    bm.faces.new(list(reversed(bot))).material_index = 0
    for j in range(5):
        bm_cone(bm, 4, 0.005, 0.004, 0.07, H - 0.01, 0, x=-0.1 + j * 0.05, y=0.04, tilt=(0.3, 0))
    bm_cone(bm, 5, 0.007, 0.005, 0.2, 0.02, 0, x=0.02, tilt=(0.35, 0.2))
    me['boat'] = to_mesh(bm, 'boat', [M['deadwood']])
    # Shrine, tent, fire -------------------------------------------------
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.015), (0.12, 0.12, 0.03), 0)
    bm_cone(bm, 4, 0.03, 0.02, 0.15, 0.03, 0, rot=math.pi / 4)
    bm_box(bm, (0, -0.022, 0.12), (0.022, 0.006, 0.022), 1)
    for (x, y) in ((-0.045, -0.045), (0.045, -0.045), (0.045, 0.045)):
        bm_cone(bm, 5, 0.006, 0.006, 0.02, 0.03, 2, x=x, y=y)
        bm_cone(bm, 4, 0.004, 0.0, 0.012, 0.05, 1, x=x, y=y)
    me['shrine'] = to_mesh(bm, 'shrine', [M['stone'], M['flame'], M['wall']])
    bm = bmesh.new()
    bm_cone(bm, 4, 0.1, 0.0, 0.12, 0.0, 0, rot=math.pi / 4)
    bm_box(bm, (0, -0.072, 0.03), (0.04, 0.004, 0.06), 1)
    me['tent'] = to_mesh(bm, 'tent', [M['canvas'], M['door']])
    bm = bmesh.new()
    for a in (0.0, 1.05, 2.1):
        bm_cone(bm, 5, 0.007, 0.007, 0.07, 0.006, 0, x=0, y=0, tilt=(math.pi / 2, 0), rot=a)
    bm_cone(bm, 6, 0.022, 0.0, 0.06, 0.0, 1)
    me['fire'] = to_mesh(bm, 'fire', [M['deadwood'], M['flame']])
    # Tokens -------------------------------------------------------------
    bm = bmesh.new()
    bm_cone(bm, 24, 0.12, 0.115, 0.03, 0.0, 0)
    bm_cone(bm, 14, 0.072, 0.03, 0.17, 0.03, 3)
    bm_cone(bm, 12, 0.045, 0.03, 0.12, 0.05, 1)
    bm_cone(bm, 12, 0.05, 0.042, 0.025, 0.165, 0)
    bm_uvsphere(bm, 0.036, (0, 0, 0.215), 1, 16, 10)
    bm_cone(bm, 12, 0.046, 0.0, 0.085, 0.2, 3, y=0.008, tilt=(0.12, 0))
    bm_cone(bm, 6, 0.006, 0.006, 0.38, 0.03, 0, x=0.085, y=0.02)
    bm_cone(bm, 8, 0.012, 0.0, 0.03, 0.41, 0, x=0.085, y=0.02)
    vs = [bm.verts.new(c) for c in [(0.088, 0.02, 0.40), (0.088, 0.02, 0.30), (0.21, 0.02, 0.36), (0.165, 0.02, 0.35)]]
    bm.faces.new((vs[0], vs[1], vs[3], vs[2])).material_index = 2
    bm_box(bm, (-0.07, -0.02, 0.1), (0.012, 0.012, 0.14), 0, rot=0.2)
    me['hero'] = to_mesh(bm, 'hero', [M['brass'], M['ivory'], M['banner'], M['cloak']], smooth=True, auto=40)
    bm = bmesh.new()
    bm_cone(bm, 20, 0.085, 0.082, 0.025, 0.0, 0)
    bm_cone(bm, 9, 0.055, 0.016, 0.15, 0.025, 1, tilt=(-0.25, 0))
    bm_ico(bm, 0.036, (0, -0.035, 0.155), 1, sub=1, squash=(1, 1.2, 0.9))
    for s in (-1, 1):
        bm_ico(bm, 0.008, (s * 0.014, -0.066, 0.16), 2, sub=1)
    bm_cone(bm, 4, 0.004, 0.002, 0.25, 0.025, 1, x=0.05, y=-0.01, tilt=(0.15, 0.1))
    bm_cone(bm, 4, 0.012, 0.0, 0.03, 0.262, 1, x=0.088, y=-0.05, tilt=(0.15, 0.1))
    me['enemy'] = to_mesh(bm, 'enemy', [M['enemy_rim'], M['enemy_body'], M['enemy_eye']], smooth=True, auto=35)
    bm = bmesh.new()
    bm_cone(bm, 24, 0.13, 0.126, 0.03, 0.0, 0)
    bm_cone(bm, 11, 0.095, 0.03, 0.25, 0.03, 1)
    bm_ico(bm, 0.05, (0, -0.01, 0.30), 1, sub=2)
    for j in range(7):
        a = j / 7 * 6.283
        bm_cone(bm, 4, 0.01, 0.0, 0.04, 0.33, 3, x=math.cos(a) * 0.035, y=math.sin(a) * 0.035 - 0.01)
    for s in (-1, 1):
        bm_ico(bm, 0.009, (s * 0.018, -0.055, 0.305), 2, sub=1)
    bm_cone(bm, 5, 0.006, 0.006, 0.52, 0.03, 3, x=-0.09, y=0.02)
    vs = [bm.verts.new(c) for c in [(-0.093, 0.02, 0.54), (-0.093, 0.02, 0.40), (-0.25, 0.02, 0.43), (-0.21, 0.02, 0.48), (-0.26, 0.02, 0.52)]]
    bm.faces.new((vs[0], vs[1], vs[2], vs[3], vs[4])).material_index = 4
    me['warlord'] = to_mesh(bm, 'warlord', [M['enemy_rim'], M['warlord_body'], M['warlord_eye'], M['brass'], M['banner_dark']], smooth=True, auto=35)


def build_materials():
    S = Ctx.style
    P = S['pal']
    M = Ctx.mats
    T = S['thickness']
    M['plains'] = m_terrain('plains', P['plains'][0], P['plains'][1], low=P['bank'], low_z=(-0.045, -0.012), speck=0.15)
    M['forest'] = m_terrain('forest', P['forest'][0], P['forest'][1], low=P['bank'], low_z=(-0.045, -0.012), speck=0.2)
    M['hills'] = m_terrain('hills', P['hills'][0], P['hills'][1], rock=P['hill_rock'], slope=(0.55, 0.75), low=P['bank'], low_z=(-0.045, -0.012))
    M['marsh'] = m_terrain('marsh', P['marsh'][0], P['marsh'][1], low=P['mud'], low_z=(-0.05, -0.015), speck=0.25)
    M['lake'] = m_terrain('lake', P['plains'][0], P['plains'][1], low=P['sand'], low_z=(-0.08, -0.025), deep=P['deep'])
    M['mountain'] = m_terrain('mountain', P['mtn_base'][0], P['mtn_base'][1], low=P['bank'], low_z=(-0.045, -0.012), speck=0.2)
    M['side'] = m_strata('side', P['strata'], T)
    M['water'] = m_water('water', P['water'], trans=S.get('water_trans', 0.6))
    M['marsh_water'] = m_water('marsh_water', P['marsh_water'], trans=0.2, rough=0.07)
    M['sea'] = m_water('sea', P['sea'], trans=S.get('sea_trans', 0.75), ripple=0.25)
    M['seabed'] = m_flat('seabed', P['seabed'], 1.0)
    M['mtn'] = m_mountain('mtn', P['mtn_rock'][0], P['mtn_rock'][1], P['snow'], snow_z=S.get('snow_z', 0.66), snow_on=S.get('snow', True))
    M['trunk'] = m_flat('trunk', P['trunk'], 0.9)
    M['leaf_conifer'] = m_flat('leaf_conifer', (1, 1, 1, 1), 0.8, obj_color=True)
    M['leaf_broad'] = m_flat('leaf_broad', (1, 1, 1, 1), 0.8, obj_color=True)
    M['deadwood'] = m_flat('deadwood', P['deadwood'], 0.95)
    M['rock'] = m_flat('rock', (1, 1, 1, 1), 0.9, obj_color=True)
    M['reed'] = m_flat('reed', P['reed'], 0.85)
    M['cattail'] = m_flat('cattail', P['cattail'], 0.9)
    M['field'] = m_flat('field', (1, 1, 1, 1), 0.85, obj_color=True)
    M['field_soil'] = m_flat('field_soil', P['field_soil'], 0.95)
    M['lily'] = m_flat('lily', P['lily'], 0.6)
    M['wall'] = m_flat('wall', P['wall'], 0.9)
    M['roof'] = m_flat('roof', (1, 1, 1, 1), 0.85, obj_color=True)
    M['roof_dark'] = m_flat('roof_dark', P['roof_dark'], 0.85)
    M['stone'] = m_flat('stone', P['stone'], 0.9)
    M['door'] = m_flat('door', P['door'], 0.9)
    win_k = S.get('window_glow', 0.0)
    M['window'] = m_flat('window', P['window'], 0.4, emit=lin('#ffb45a'), emit_k=win_k) if win_k else m_flat('window', P['window'], 0.3)
    M['grotto_glow'] = m_emit('grotto_glow', lin('#3fd1c0'), S.get('grotto_k', 6.0))
    M['flame'] = m_emit('flame', lin('#ff9a3c'), S.get('flame_k', 18.0))
    M['canvas'] = m_flat('canvas', P['canvas'], 0.9)
    M['brass'] = m_flat('brass', lin('#c9a35a'), 0.35, metal=1.0)
    M['ivory'] = m_flat('ivory', lin('#efe3c6'), 0.45, spec=0.5)
    M['banner'] = m_flat('banner', lin('#e6c17c'), 0.6)
    M['cloak'] = m_flat('cloak', lin('#2f4f86'), 0.6)
    M['banner_dark'] = m_flat('banner_dark', lin('#3a1414'), 0.8)
    M['enemy_rim'] = m_flat('enemy_rim', lin('#a63a33'), 0.5)
    M['enemy_body'] = m_flat('enemy_body', lin('#2a2c2e'), 0.55)
    M['enemy_eye'] = m_emit('enemy_eye', lin('#ff4a36'), 20.0)
    M['warlord_body'] = m_flat('warlord_body', lin('#1d2224'), 0.5)
    M['warlord_eye'] = m_emit('warlord_eye', lin('#56f0c8'), 25.0)
    M['tile_back'] = m_terrain('tile_back', P['back'][0], P['back'][1], speck=0.1, nscale=4.0, bump=0.15)
    M['gold'] = m_flat('gold', lin('#c9a35a'), 0.3, metal=1.0)
    M['seal'] = m_flat('seal', lin('#6b2622'), 0.35, spec=0.6)
    M['ring_move'] = m_emit('ring_move', lin(S.get('ring_col', '#f3d98a')), S.get('ring_k', 5.0))
    M['ring_enemy'] = m_emit('ring_enemy', lin('#ff5a44'), 5.0)
    M['ring_path'] = m_emit('ring_path', lin('#fff1c9'), 7.0)
    M['table'] = m_table('table', P.get('table', lin('#5a3a22')))


def m_table(name, col):
    m, nt, p = _new(name)
    tc = nt.nodes.new('ShaderNodeTexCoord')
    w = nt.nodes.new('ShaderNodeTexWave')
    w.wave_type = 'BANDS'
    w.inputs['Scale'].default_value = 0.6
    w.inputs['Distortion'].default_value = 6.0
    w.inputs['Detail'].default_value = 3.0
    mp = nt.nodes.new('ShaderNodeMapping')
    mp.inputs['Scale'].default_value = (0.15, 1.0, 1.0)
    nt.links.new(tc.outputs['Object'], mp.inputs['Vector'])
    nt.links.new(mp.outputs['Vector'], w.inputs['Vector'])
    f = _maprange(nt, w.outputs['Fac'], 0.2, 1.0, 0.0, 1.0)
    dark = (col[0] * 0.55, col[1] * 0.55, col[2] * 0.55, 1)
    c = _mix(nt, f, dark, col)
    nt.links.new(c, p.inputs['Base Color'])
    _in(p, 'Roughness', 0.55)
    return m


# --------------------------------------------------------------------------
# Hex terrain

class Hex:
    def __init__(self, q, r, terrain, site, enemies, slot):
        self.q, self.r, self.terrain, self.site, self.enemies, self.slot = q, r, terrain, site, enemies, slot
        self.seed = (q * 92821 + r * 68917 + 1234567) & 0xffffff
        self.rnd = random.Random(self.seed)
        self.center = hex_world(q, r)
        self.top = 0.0
        self.mounds, self.pools, self.segs = [], [], []
        rnd = self.rnd
        if terrain == 'hills':
            k = 3 if not site else 1
            for i in range(k):
                if site and i == 0:
                    self.mounds.append((0.0, 0.0, 0.5, 0.2))
                    continue
                a = rnd.uniform(0, 6.28)
                d = rnd.uniform(0.15, 0.4)
                self.mounds.append((math.cos(a) * d, math.sin(a) * d, rnd.uniform(0.32, 0.48), rnd.uniform(0.13, 0.24)))
        if terrain == 'marsh':
            for i in range(6):
                a = rnd.uniform(0, 6.28)
                d = rnd.uniform(0.0, 0.55)
                self.pools.append((math.cos(a) * d, math.sin(a) * d, rnd.uniform(0.17, 0.3)))

    def river_dist(self, wx, wy):
        if not self.segs:
            return 9.0
        p = Vector((wx, wy))
        return min(seg_dist(p, a, b) for a, b in self.segs)

    def height(self, wx, wy):
        S = Ctx.style
        rr = 1.0 - S['gap']
        p = Vector((wx - self.center.x, wy - self.center.y))
        ed = edge_dist(p, rr)
        t = self.terrain
        fw = 0.14 if t == 'lake' else 0.3
        f = smoothstep(0.0, fw, ed)
        n1 = noise.noise(Vector((wx * 1.9, wy * 1.9, self.seed % 97 * 0.31)))
        n2 = noise.noise(Vector((wx * 5.0, wy * 5.0, 3.3)))
        if t == 'plains':
            z = 0.014 * n1 + 0.004 * n2
        elif t == 'forest':
            z = 0.03 * n1 + 0.008 * n2
        elif t == 'hills':
            z = 0.02 * n1
            for (mx, my, mr, mh) in self.mounds:
                d = math.hypot(p.x - mx, p.y - my)
                if d < mr:
                    z += mh * 0.5 * (1 + math.cos(math.pi * d / mr))
            z += 0.012 * n2
        elif t == 'mountain':
            z = 0.04 * n1 + 0.07 * (1 - smoothstep(0.15, 0.75, p.length)) + 0.012 * n2
        elif t == 'marsh':
            z = 0.012 * n1 + 0.004 * n2
            for (mx, my, mr) in self.pools:
                d = math.hypot(p.x - mx, p.y - my) + 0.05 * n2
                z = min(z, lerp(-0.075, z, smoothstep(mr * 0.45, mr, d)))
        elif t == 'lake':
            d = p.length + 0.1 * n1
            z = -0.24 * (1 - smoothstep(0.42, 0.74, d)) + 0.01 * n2
        else:
            z = 0.0
        z *= f
        if self.segs and S.get('rivers', True):
            d = self.river_dist(wx, wy) + 0.015 * n2
            carve = lerp(-0.085, 0.03, smoothstep(0.075, 0.2, d))
            z = min(z, carve)
        return z

    def ground(self, lx, ly):
        wx, wy = self.center.x + lx, self.center.y + ly
        return Vector((wx, wy, self.top + self.height(wx, wy)))


def tile_mesh(hx, n=18):
    S = Ctx.style
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
    boundary = [e for e in bm.edges if e.is_boundary]
    ret = bmesh.ops.extrude_edge_only(bm, edges=boundary)
    newv = [g for g in ret['geom'] if isinstance(g, bmesh.types.BMVert)]
    for v in newv:
        v.co.z = -T - hx.top
    for g in ret['geom']:
        if isinstance(g, bmesh.types.BMFace):
            g.material_index = 1
    newv.sort(key=lambda v: math.atan2(v.co.y, v.co.x))
    bot = bm.faces.new(newv)
    bot.material_index = 1
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    me = to_mesh(bm, f'tile_{hx.q}_{hx.r}', [Ctx.mats[hx.terrain], Ctx.mats['side']])
    ob = add(me, (hx.center.x, hx.center.y, hx.top), name=f'tile_{hx.q}_{hx.r}')
    if S['bevel'] > 0:
        bv = ob.modifiers.new('bevel', 'BEVEL')
        bv.width = S['bevel']
        bv.segments = 2
        bv.limit_method = 'ANGLE'
        bv.angle_limit = math.radians(55)
        bv.harden_normals = False
    # water sheet under the terrain: shows in lakes, pools and carved rivers
    need = hx.terrain in ('lake', 'marsh') or hx.segs
    if need:
        bm = bmesh.new()
        bmesh.ops.create_circle(bm, cap_ends=True, segments=6, radius=rr * 0.985)
        wm = Ctx.mats['marsh_water'] if hx.terrain == 'marsh' else Ctx.mats['water']
        me = to_mesh(bm, 'water', [wm])
        add(me, (hx.center.x, hx.center.y, hx.top + WATER), name='water')
    return ob


# --------------------------------------------------------------------------
# Scatter and props

def scatter(hx, count, min_dist, avoid_center=0.0, river_clear=0.17, margin=0.1, water_ok=False, taken=None, center_only=None):
    rnd = hx.rnd
    rr = 1.0 - Ctx.style['gap']
    pts = []
    taken = taken if taken is not None else []
    tries = 0
    while len(pts) < count and tries < count * 40:
        tries += 1
        p = Vector((rnd.uniform(-1, 1), rnd.uniform(-1, 1)))
        if center_only is not None and p.length > center_only:
            continue
        if edge_dist(p, rr) < margin or p.length < avoid_center:
            continue
        wx, wy = hx.center.x + p.x, hx.center.y + p.y
        if hx.river_dist(wx, wy) < river_clear:
            continue
        z = hx.height(wx, wy)
        if not water_ok and z < WATER + 0.012:
            continue
        if any((p - q).length < min_dist for q in pts + taken):
            continue
        pts.append(p)
    taken.extend(pts)
    return [hx.ground(p.x, p.y) for p in pts]


def leaf_color(kind, rnd):
    P = Ctx.style['pal']
    base = rnd.choice(P[kind])
    return jitter_col(base, rnd, 0.1)


def plant_trees(hx, count, mix=0.55, taken=None, avoid=0.0, dead=0.0, min_dist=0.1):
    rnd = hx.rnd
    for g in scatter(hx, count, min_dist, avoid_center=avoid, taken=taken):
        s = rnd.uniform(0.85, 1.35) * Ctx.style.get('tree_scale', 1.25)
        roll = rnd.random()
        if roll < dead:
            add(Ctx.meshes[f'dead{rnd.randrange(3)}'], g - Vector((0, 0, 0.01)), rnd.uniform(0, 6.28), s)
        elif roll < dead + mix:
            add(Ctx.meshes[f'conifer{rnd.randrange(4)}'], g - Vector((0, 0, 0.01)), rnd.uniform(0, 6.28), s, leaf_color('conifer', rnd))
        else:
            add(Ctx.meshes[f'broad{rnd.randrange(4)}'], g - Vector((0, 0, 0.01)), rnd.uniform(0, 6.28), s, leaf_color('broad', rnd))


def rocks(hx, count, taken=None, scale=1.0, water_ok=False, avoid=0.0):
    rnd = hx.rnd
    P = Ctx.style['pal']
    for g in scatter(hx, count, 0.09, taken=taken, water_ok=water_ok, avoid_center=avoid):
        add(Ctx.meshes[f'rock{rnd.randrange(3)}'], g - Vector((0, 0, 0.012)), rnd.uniform(0, 6.28), rnd.uniform(0.6, 1.4) * scale, jitter_col(P['rock'], rnd, 0.1))


def fields(hx, taken, avoid=0.0):
    rnd = hx.rnd
    P = Ctx.style['pal']
    ang = rnd.uniform(0, math.pi)
    for g in scatter(hx, rnd.randint(2, 4), 0.3, avoid_center=avoid, taken=taken, margin=0.2, river_clear=0.28):
        c = rnd.choice(P['field_cols'])
        sx, sy = rnd.uniform(0.26, 0.34), rnd.uniform(0.17, 0.23)
        add(Ctx.meshes[f'field{rnd.randrange(2)}'], g + Vector((0, 0, 0.002)), ang + rnd.uniform(-0.08, 0.08), rnd.uniform(0.9, 1.15), jitter_col(c, rnd, 0.06))


def mountain_obj(hx, lx, ly, h, r, segs=11, rings=6):
    rnd = hx.rnd
    bm = bmesh.new()
    rings_v = []
    for k in range(rings):
        t = k / rings
        rad = r * (1 - t) ** 0.85
        ring = []
        for i in range(segs):
            a = 2 * math.pi * i / segs + rnd.uniform(-0.12, 0.12) + k * 0.18
            j = 1 + rnd.uniform(-0.22, 0.22)
            zz = h * t + (rnd.uniform(-0.05, 0.05) * h if k else -0.04)
            ring.append(bm.verts.new((rad * j * math.cos(a), rad * j * math.sin(a), zz)))
        rings_v.append(ring)
    tip = bm.verts.new((rnd.uniform(-0.04, 0.04) * r, rnd.uniform(-0.04, 0.04) * r, h))
    for k in range(rings - 1):
        A, B = rings_v[k], rings_v[k + 1]
        for i in range(segs):
            jn = (i + 1) % segs
            bm.faces.new((A[i], A[jn], B[jn]))
            bm.faces.new((A[i], B[jn], B[i]))
    for i in range(segs):
        bm.faces.new((rings_v[-1][i], rings_v[-1][(i + 1) % segs], tip))
    bm.faces.new(list(reversed(rings_v[0])))
    me = to_mesh(bm, 'mountain', [Ctx.mats['mtn']])
    g = hx.ground(lx, ly)
    return add(me, g - Vector((0, 0, 0.02)), rnd.uniform(0, 6.28), name='mountain')


def decorate(hx):
    S = Ctx.style
    rnd = hx.rnd
    t = hx.terrain
    taken = []
    dens = S.get('density', 1.0)
    site_r = 0.36 if hx.site else 0.0
    if hx.enemies:
        site_r = max(site_r, 0.22)
    if t == 'plains':
        if S.get('fields', True):
            fields(hx, taken, avoid=site_r)
        plant_trees(hx, int(3 * dens), mix=0.3, taken=taken, avoid=site_r)
        for g in scatter(hx, int(5 * dens), 0.08, taken=taken, avoid_center=site_r):
            add(Ctx.meshes['bush'], g, rnd.uniform(0, 6), rnd.uniform(0.7, 1.2), leaf_color('broad', rnd))
    elif t == 'forest':
        plant_trees(hx, int(30 * dens), mix=S.get('conifer_mix', 0.6), taken=taken, avoid=site_r, dead=S.get('dead_mix', 0.0), min_dist=0.09)
    elif t == 'hills':
        rocks(hx, 5, taken, avoid=site_r)
        plant_trees(hx, int(3 * dens), mix=0.5, taken=taken, avoid=site_r)
        for g in scatter(hx, int(6 * dens), 0.08, taken=taken, avoid_center=site_r):
            add(Ctx.meshes['bush'], g, rnd.uniform(0, 6), rnd.uniform(0.6, 1.0), leaf_color('broad', rnd))
    elif t == 'mountain':
        peaks = [(0.0, 0.05, S.get('peak_h', 0.95), 0.52), (-0.33, -0.12, 0.55, 0.36), (0.32, -0.2, 0.48, 0.33), (0.25, 0.35, 0.42, 0.3)]
        for (x, y, h, r) in peaks[: 3 + (hx.seed % 2)]:
            if hx.river_dist(hx.center.x + x, hx.center.y + y) < r * 0.75:
                continue
            mountain_obj(hx, x, y, h * rnd.uniform(0.9, 1.1), r)
        rocks(hx, 6, taken, scale=1.2)
        plant_trees(hx, int(4 * dens), mix=0.9, taken=taken, avoid=0.55)
    elif t == 'marsh':
        for g in scatter(hx, int(16 * dens), 0.07, taken=taken, avoid_center=site_r, water_ok=True, river_clear=0.08):
            add(Ctx.meshes['reeds'], g - Vector((0, 0, 0.01)), rnd.uniform(0, 6), rnd.uniform(0.8, 1.3))
        plant_trees(hx, 3, mix=0.2, taken=taken, avoid=site_r, dead=0.6)
        for (mx, my, mr) in hx.pools:
            for j in range(3):
                a = rnd.uniform(0, 6.28)
                lp = Vector((mx + math.cos(a) * mr * 0.35, my + math.sin(a) * mr * 0.35))
                wz = hx.top + WATER + 0.002
                if hx.height(hx.center.x + lp.x, hx.center.y + lp.y) < WATER - 0.01:
                    add(Ctx.meshes['lily'], (hx.center.x + lp.x, hx.center.y + lp.y, wz), rnd.uniform(0, 6), rnd.uniform(0.7, 1.2))
    elif t == 'lake':
        rr = 1.0 - S['gap']
        for i in range(10):
            a = rnd.uniform(0, 6.28)
            d = rnd.uniform(0.55, 0.7)
            p = Vector((math.cos(a) * d, math.sin(a) * d))
            if edge_dist(p, rr) < 0.06:
                continue
            g = hx.ground(p.x, p.y)
            if i < 6:
                add(Ctx.meshes['reeds'], g - Vector((0, 0, 0.01)), rnd.uniform(0, 6), rnd.uniform(0.8, 1.2))
            else:
                add(Ctx.meshes[f'rock{rnd.randrange(3)}'], g - Vector((0, 0, 0.015)), rnd.uniform(0, 6), rnd.uniform(0.6, 1.1), jitter_col(S['pal']['rock'], rnd))
        plant_trees(hx, 3, mix=0.5, taken=taken)
    if hx.site:
        place_site(hx, taken)


def place_site(hx, taken):
    S = Ctx.style
    P = S['pal']
    rnd = hx.rnd
    me = Ctx.meshes
    site = hx.site
    c = hx.ground(0, 0)

    ss = S.get('site_scale', 1.45)

    def g(lx, ly):
        return hx.ground(lx * ss, ly * ss)

    def A(mesh, loc, rot=0.0, scale=1.0, color=None, tilt=None):
        sc = tuple(x * ss for x in scale) if isinstance(scale, tuple) else scale * ss
        return add(mesh, loc, rot, sc, color, tilt=tilt)

    def face(v):
        return math.atan2(-(v.x), v.y) if v.length else 0.0
    if site == 'village':
        n = 6
        for i in range(n):
            a = i / n * 6.283 + 0.4
            d = rnd.uniform(0.16, 0.26)
            lp = Vector((math.cos(a) * d, math.sin(a) * d))
            if hx.river_dist(hx.center.x + lp.x, hx.center.y + lp.y) < 0.15:
                lp *= 0.6
            A(me[f'house{rnd.randrange(3)}'], g(lp.x, lp.y) - Vector((0, 0, 0.01)), a + math.pi / 2 + rnd.uniform(-0.3, 0.3), rnd.uniform(0.95, 1.15), rnd.choice(P['roofs']))
        A(me['tower'], g(0.0, 0.0), 0.3, 0.55)
    elif site == 'town':
        ring = 0.4
        for i in range(8):
            if i == 6:
                continue
            a0, a1 = i / 8 * 6.283, (i + 1) / 8 * 6.283
            p0 = Vector((math.cos(a0) * ring, math.sin(a0) * ring))
            p1 = Vector((math.cos(a1) * ring, math.sin(a1) * ring))
            m = (p0 + p1) / 2
            seg = p1 - p0
            A(me['wall'], g(m.x, m.y) - Vector((0, 0, 0.015)), math.atan2(seg.y, seg.x), (seg.length, 1.0, 1.0))
            A(me['tower'], g(p0.x, p0.y) - Vector((0, 0, 0.01)), 0.0, 0.62)
        A(me['church'], g(0.05, 0.08), 0.2, 1.05)
        for i in range(11):
            a = rnd.uniform(0, 6.28)
            d = rnd.uniform(0.12, 0.31)
            lp = Vector((math.cos(a) * d, math.sin(a) * d))
            if (lp - Vector((0.05, 0.08))).length < 0.14:
                continue
            A(me[f'house{rnd.randrange(3)}'], g(lp.x, lp.y) - Vector((0, 0, 0.01)), a + math.pi / 2, rnd.uniform(0.9, 1.1), rnd.choice(P['roofs']))
    elif site == 'keep':
        A(me['keep'], c - Vector((0, 0, 0.02)), 0.15, 0.8)
        for s in ((-1, -1), (1, -1), (1, 1), (-1, 1)):
            lp = Vector((s[0] * 0.2, s[1] * 0.2))
            A(me['tower'], g(lp.x, lp.y) - Vector((0, 0, 0.02)), 0.0, 0.85)
        for (m, rot) in ((Vector((0, -0.2)), 0.0), (Vector((0, 0.2)), 0.0), (Vector((-0.2, 0)), math.pi / 2), (Vector((0.2, 0)), math.pi / 2)):
            A(me['wall'], g(m.x, m.y) - Vector((0, 0, 0.02)), rot, (0.4, 1.0, 1.0))
    elif site == 'monastery':
        A(me['church'], c - Vector((0, 0, 0.01)), 0.3, 1.3)
        for (m, rot, ln) in ((Vector((0, -0.24)), 0.3, 0.45), (Vector((-0.25, 0.02)), 0.3 + math.pi / 2, 0.4)):
            mm = Matrix.Rotation(0.3, 2) @ m
            A(me['wall'], g(mm.x, mm.y) - Vector((0, 0, 0.03)), rot, (ln, 0.8, 0.6))
        for i in range(4):
            a = rnd.uniform(0, 6.28)
            lp = Vector((math.cos(a) * 0.3, math.sin(a) * 0.3))
            A(me[f'house{rnd.randrange(3)}'], g(lp.x, lp.y) - Vector((0, 0, 0.01)), a, 0.9, P['roofs'][-1])
    elif site == 'cairn':
        for i in range(9):
            a = i / 9 * 6.283
            lp = Vector((math.cos(a) * 0.24, math.sin(a) * 0.24))
            A(me['menhir'], g(lp.x, lp.y) - Vector((0, 0, 0.02)), a, (1.0, 1.0, rnd.uniform(0.7, 1.3)), tilt=(rnd.uniform(-0.12, 0.12), rnd.uniform(-0.12, 0.12)))
        for i in range(6):
            A(me[f'rock{rnd.randrange(3)}'], c + Vector((rnd.uniform(-0.05, 0.05), rnd.uniform(-0.05, 0.05), 0.02 * (i // 3))), rnd.uniform(0, 6), 1.2, jitter_col(P['stone'], rnd))
    elif site == 'chapel':
        A(me['ruinwall'], c - Vector((0, 0, 0.01)), 0.3, 1.2)
        A(me['ruinwall'], g(0.12, 0.08) - Vector((0, 0, 0.02)), 0.3 + math.pi / 2, (0.8, 1.0, 0.7))
        for i in range(5):
            lp = Vector((-0.16 + i * 0.08, -0.13))
            A(me['column'], g(lp.x, lp.y) - Vector((0, 0, 0.01)), 0.0, (1.0, 1.0, rnd.uniform(0.3, 1.1)))
        rocks(hx, 4, taken, scale=0.6, water_ok=True)
    elif site == 'grotto':
        A(me['grotto'], c, 0.0, 1.0, jitter_col(P['rock'], rnd))
        L = bpy.data.lights.new('grotto_l', 'POINT')
        L.energy = S.get('grotto_light', 6.0)
        L.color = (0.3, 1.0, 0.85)
        L.shadow_soft_size = 0.05
        lo = bpy.data.objects.new('grotto_l', L)
        lo.location = c + Vector((0, -0.25, 0.08))
        link(lo)
    elif site == 'shrine':
        A(me['shrine'], c, 0.2, 1.3)
        flame_light(c + Vector((0, 0, 0.12)), 1.5)
    elif site == 'hermit':
        A(me['house0'], g(-0.08, 0.06), 0.4, 1.0, P['roofs'][-1])
        A(me['fire'], g(0.1, -0.08), 0.0, 1.0)
        flame_light(g(0.1, -0.08) + Vector((0, 0, 0.06)), S.get('fire_light', 3.0))
        A(me['tent'], g(0.08, 0.14), 0.3, 0.7)
    elif site == 'barge':
        A(me['boat'], c - Vector((0, 0, 0.02)), 0.5, 1.2, tilt=(0.12, 0.08))
        rocks(hx, 3, taken, scale=0.6)
    hx.site_anchor = c


def flame_light(loc, energy):
    L = bpy.data.lights.new('flame_l', 'POINT')
    L.energy = energy * Ctx.style.get('light_mult', 1.0)
    L.color = (1.0, 0.62, 0.3)
    L.shadow_soft_size = 0.03
    lo = bpy.data.objects.new('flame_l', L)
    lo.location = loc
    link(lo)


def place_tokens(hx, warlord=False):
    me = Ctx.meshes
    rnd = hx.rnd
    if warlord:
        g = hx.ground(-0.05, -0.1)
        add(me['warlord'], g, 0.0, Ctx.style.get('token_scale', 1.6) * 1.1)
        return g
    n = hx.enemies
    offs = [(0.0, -0.05)] if n == 1 else [(-0.17, -0.02), (0.17, -0.1)]
    pos = []
    for (x, y) in offs:
        g = hx.ground(x, y)
        add(me['enemy'], g, rnd.uniform(-0.3, 0.3), Ctx.style.get('token_scale', 1.6))
        pos.append(g)
    return pos


# --------------------------------------------------------------------------
# Region assembly

def build_region(slots=None, tokens=True, warlord=True, hero_at=(0, 0), hero_off=(0.2, -0.25), elevation=None,
                 rivers=None, hidden=(), sea=True, sea_z=None, table=False):
    S = Ctx.style
    rivers = RIVERS if rivers is None else rivers
    Ctx.rivers = [catmull(r) for r in rivers] if S.get('rivers', True) else []
    slots = slots or list(SLOTS.keys())
    flowers = {}
    for sid in slots:
        (cq, cr), tid = SLOTS[sid]
        for (dq, dr), terr, site, en in TILES[tid]:
            q, r = cq + dq, cr + dr
            hx = Hex(q, r, terr, site, en, sid)
            if elevation:
                hx.top = elevation.get(terr, 0.0)
            Ctx.hexes[(q, r)] = hx
            flowers.setdefault(sid, []).append(hx)
    for hx in Ctx.hexes.values():
        for poly in Ctx.rivers:
            for a, b in zip(poly, poly[1:]):
                if seg_dist(Vector((hx.center.x, hx.center.y)), a, b) < 1.3:
                    hx.segs.append((a, b))
    built = {}
    for sid, hexes in flowers.items():
        start = len(Ctx.created)
        for hx in hexes:
            tile_mesh(hx)
            decorate(hx)
        built[sid] = Ctx.created[start:]
    if tokens:
        for hx in Ctx.hexes.values():
            if hx.enemies and hx.slot not in hidden:
                hx.token_pos = place_tokens(hx)
        if warlord and (8, -1) in Ctx.hexes:
            Ctx.hexes[(8, -1)].warlord_pos = place_tokens(Ctx.hexes[(8, -1)], warlord=True)
        if hero_at in Ctx.hexes:
            hx = Ctx.hexes[hero_at]
            g = hx.ground(*hero_off)
            add(Ctx.meshes['hero'], g, 0.4, S.get('token_scale', 1.6))
            Ctx.hero = g
    T = S['thickness']
    if sea:
        bm = bmesh.new()
        bmesh.ops.create_grid(bm, x_segments=1, y_segments=1, size=60)
        add(to_mesh(bm, 'sea', [Ctx.mats['sea']]), (5, 0, sea_z if sea_z is not None else -T * 0.55), name='sea')
        bm = bmesh.new()
        bmesh.ops.create_grid(bm, x_segments=1, y_segments=1, size=60)
        add(to_mesh(bm, 'seabed', [Ctx.mats['seabed']]), (5, 0, -T - 0.6), name='seabed')
    if table:
        bm = bmesh.new()
        bmesh.ops.create_grid(bm, x_segments=1, y_segments=1, size=60)
        add(to_mesh(bm, 'table', [Ctx.mats['table']]), (5, 0, -T - 0.002 - (min((h.top for h in Ctx.hexes.values()), default=0))), name='table')
    return built


def face_down_flower(center_axial, lift=0.0):
    """An unexplored tile: seven face-down hexes with a gold rim and a wax seal."""
    S = Ctx.style
    rr = 1.0 - S['gap']
    T = S['thickness']
    objs = []
    for (dq, dr) in ((0, 0), (1, 0), (1, -1), (0, -1), (-1, 0), (-1, 1), (0, 1)):
        q, r = center_axial[0] + dq, center_axial[1] + dr
        c = hex_world(q, r)
        bm = bmesh.new()
        ret = bmesh.ops.create_circle(bm, cap_ends=True, segments=6, radius=rr)
        f = [g for g in bm.faces][0]
        ext = bmesh.ops.extrude_face_region(bm, geom=[f])
        for v in [g for g in ext['geom'] if isinstance(g, bmesh.types.BMVert)]:
            v.co.z -= T
        bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
        for fc in bm.faces:
            fc.material_index = 0 if fc.normal.z > 0.5 else 1
        ob = add(to_mesh(bm, 'back', [Ctx.mats['tile_back'], Ctx.mats['side']]), (c.x, c.y, lift), name='back')
        bv = ob.modifiers.new('bevel', 'BEVEL')
        bv.width = S['bevel']
        bv.segments = 2
        objs.append(ob)
        objs.append(hex_ring(None, c, 0.78, 0.84, Ctx.mats['gold'], z=lift + 0.004, flat=True))
    cc = hex_world(*center_axial)
    bm = bmesh.new()
    bm_cone(bm, 20, 0.16, 0.15, 0.03, 0.0, 0)
    bm_cone(bm, 20, 0.11, 0.105, 0.012, 0.03, 0)
    objs.append(add(to_mesh(bm, 'seal', [Ctx.mats['seal']], smooth=True, auto=50), (cc.x, cc.y, lift + 0.0), 0.0, 1.5))
    bm = bmesh.new()
    star = []
    for i in range(8):
        rad = 0.13 if i % 2 == 0 else 0.035
        a = i / 8 * 6.283 + math.pi / 2
        star.append(bm.verts.new((math.cos(a) * rad, math.sin(a) * rad, 0.0)))
    f = bm.faces.new(star)
    ext = bmesh.ops.extrude_face_region(bm, geom=[f])
    for v in [g for g in ext['geom'] if isinstance(g, bmesh.types.BMVert)]:
        v.co.z += 0.012
    objs.append(add(to_mesh(bm, 'star', [Ctx.mats['gold']]), (cc.x, cc.y, lift + 0.062), 0.0, 1.5))
    return objs


def hex_ring(hx, center, r_in, r_out, mat, z=0.02, flat=False, n=8):
    bm = bmesh.new()
    outer, inner = [], []
    for i in range(6):
        a0, a1 = math.radians(30 + 60 * i), math.radians(30 + 60 * (i + 1))
        for s in range(n):
            t = s / n
            co = Vector((math.cos(a0), math.sin(a0))).lerp(Vector((math.cos(a1), math.sin(a1))), t)
            for lst, rad in ((outer, r_out), (inner, r_in)):
                x, y = co.x * rad, co.y * rad
                zz = z if flat else hx.top + hx.height(center.x + x, center.y + y) + z
                lst.append(bm.verts.new((x, y, max(zz, (hx.top + WATER + z) if hx else zz))))
    m = len(outer)
    for i in range(m):
        j = (i + 1) % m
        bm.faces.new((inner[i], inner[j], outer[j], outer[i]))
    return add(to_mesh(bm, 'ring', [mat]), (center.x, center.y, 0.0), name='ring')


def path_dots(points, mat, spacing=0.09, r=0.02, lift=0.03):
    pts = catmull(points, 20)
    out = []
    acc = 0.0
    for a, b in zip(pts, pts[1:]):
        L = (b - a).length
        while acc < L:
            p = a.lerp(b, acc / L)
            hx = nearest_hex(p)
            z = hx.top + max(hx.height(p.x, p.y), WATER) + lift
            bm = bmesh.new()
            bm_uvsphere(bm, r, (0, 0, 0), 0, 8, 6)
            out.append(add(to_mesh(bm, 'dot', [mat], smooth=True), (p.x, p.y, z)))
            acc += spacing
        acc -= L
    return out


def nearest_hex(p):
    return min(Ctx.hexes.values(), key=lambda h: (h.center.x - p.x) ** 2 + (h.center.y - p.y) ** 2)


# --------------------------------------------------------------------------
# Camera, light, world, render

def camera(target, yaw, pitch, dist, lens=50.0, ortho=None, dof=None, shift=(0.0, 0.0)):
    cd = bpy.data.cameras.new('cam')
    cam = bpy.data.objects.new('cam', cd)
    bpy.context.scene.collection.objects.link(cam)
    bpy.context.scene.camera = cam
    yw, pt = math.radians(yaw), math.radians(pitch)
    tgt = Vector(target)
    off = Vector((math.sin(yw) * math.cos(pt), -math.cos(yw) * math.cos(pt), math.sin(pt))) * dist
    cam.location = tgt + off
    cam.rotation_euler = (tgt - cam.location).to_track_quat('-Z', 'Y').to_euler()
    cd.lens = lens
    cd.clip_end = 400
    cd.shift_x, cd.shift_y = shift
    if ortho:
        cd.type = 'ORTHO'
        cd.ortho_scale = ortho
    if dof:
        cd.dof.use_dof = True
        cd.dof.focus_distance = (tgt - cam.location).length + dof.get('offset', 0.0)
        cd.dof.aperture_fstop = dof['fstop']
    return cam


def sun(azimuth, elevation, energy, color=(1.0, 0.95, 0.85), angle=3.0):
    L = bpy.data.lights.new('sun', 'SUN')
    L.energy = energy
    L.color = color
    L.angle = math.radians(angle)
    ob = bpy.data.objects.new('sun', L)
    az, el = math.radians(azimuth), math.radians(elevation)
    d = Vector((math.cos(el) * math.sin(az), math.cos(el) * math.cos(az), math.sin(el)))
    ob.rotation_euler = (-d).to_track_quat('-Z', 'Y').to_euler()
    bpy.context.scene.collection.objects.link(ob)
    return ob


def world(ambient, strength, camera_bg=None, fog=None):
    w = bpy.data.worlds.new('world')
    bpy.context.scene.world = w
    w.use_nodes = True
    nt = w.node_tree
    bg = nt.nodes['Background']
    bg.inputs['Color'].default_value = ambient
    bg.inputs['Strength'].default_value = strength
    out = nt.nodes['World Output']
    if camera_bg is not None:
        bg2 = nt.nodes.new('ShaderNodeBackground')
        bg2.inputs['Strength'].default_value = 1.0
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
    if fog:
        v = nt.nodes.new('ShaderNodeVolumeScatter')
        v.inputs['Density'].default_value = fog[0]
        v.inputs['Color'].default_value = fog[1]
        v.inputs['Anisotropy'].default_value = fog[2] if len(fog) > 2 else 0.3
        nt.links.new(v.outputs['Volume'], out.inputs['Volume'])


def fog_box(center, size, density, color, aniso=0.2, clumps=None, fade_z=None):
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0), size, 0)
    m = bpy.data.materials.new('fog')
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.remove(nt.nodes['Principled BSDF'])
    v = nt.nodes.new('ShaderNodeVolumePrincipled')
    v.inputs['Density'].default_value = density
    v.inputs['Color'].default_value = color
    v.inputs['Anisotropy'].default_value = aniso
    if clumps:
        n = nt.nodes.new('ShaderNodeTexNoise')
        n.inputs['Scale'].default_value = clumps
        n.inputs['Detail'].default_value = 4.0
        geo = nt.nodes.new('ShaderNodeNewGeometry')
        nt.links.new(geo.outputs['Position'], n.inputs['Vector'])
        d = _maprange(nt, n.outputs['Fac'], 0.42, 0.72, 0.0, density)
        if fade_z:
            sep = nt.nodes.new('ShaderNodeSeparateXYZ')
            nt.links.new(geo.outputs['Position'], sep.inputs['Vector'])
            fz = _maprange(nt, sep.outputs['Z'], fade_z[0], fade_z[1], 1.0, 0.0)
            mul = nt.nodes.new('ShaderNodeMath')
            mul.operation = 'MULTIPLY'
            nt.links.new(d, mul.inputs[0])
            nt.links.new(fz, mul.inputs[1])
            d = mul.outputs['Value']
        nt.links.new(d, v.inputs['Density'])
    nt.links.new(v.outputs['Volume'], nt.nodes['Material Output'].inputs['Volume'])
    return add(to_mesh(bm, 'fog', [m]), center, name='fog')


def render(path, w=1920, h=1080, samples=128, look='AgX - Medium High Contrast', exposure=0.0, freestyle=None, preview=False):
    sc = bpy.context.scene
    sc.render.engine = 'CYCLES'
    prefs = bpy.context.preferences.addons['cycles'].preferences
    prefs.compute_device_type = 'METAL'
    prefs.get_devices()
    for d in prefs.devices:
        d.use = d.type == 'METAL'
    sc.cycles.device = 'GPU'
    sc.cycles.samples = 24 if preview else samples
    sc.cycles.use_adaptive_sampling = True
    sc.cycles.use_denoising = True
    sc.cycles.max_bounces = 8
    sc.cycles.transparent_max_bounces = 16
    sc.cycles.volume_bounces = 1
    sc.render.resolution_x = w
    sc.render.resolution_y = h
    sc.render.resolution_percentage = 50 if preview else 100
    sc.render.film_transparent = False
    sc.view_settings.view_transform = 'AgX'
    try:
        sc.view_settings.look = look
    except TypeError:
        pass
    sc.view_settings.exposure = exposure
    if freestyle:
        sc.render.use_freestyle = True
        sc.render.line_thickness_mode = 'ABSOLUTE'
        sc.render.line_thickness = freestyle.get('thickness', 1.2) * (0.5 if preview else 1.0)
        fs = sc.view_layers[0].freestyle_settings
        ls = fs.linesets[0] if fs.linesets else fs.linesets.new('lines')
        ls.select_by_visibility = True
        ls.select_silhouette = True
        ls.select_border = True
        ls.select_crease = freestyle.get('crease', False)
        if ls.linestyle is None:
            ls.linestyle = bpy.data.linestyles.new('ink')
        ls.linestyle.color = freestyle.get('color', (0.08, 0.06, 0.05))
        ls.linestyle.alpha = freestyle.get('alpha', 0.85)
    sc.render.image_settings.file_format = 'PNG'
    sc.render.filepath = path
    bpy.ops.render.render(write_still=True)


def project(co):
    sc = bpy.context.scene
    v = world_to_camera_view(sc, sc.camera, Vector(co))
    return (v.x * sc.render.resolution_x, (1 - v.y) * sc.render.resolution_y)


def dump_screen(path, extra=None):
    sc = bpy.context.scene
    data = {'hexes': {}, 'sites': {}}
    for (q, r), hx in Ctx.hexes.items():
        g = hx.ground(0, 0)
        data['hexes'][f'{q},{r}'] = {'xy': project(g), 'terrain': hx.terrain, 'site': hx.site}
        if (q, r) in SITE_NAMES:
            data['sites'][SITE_NAMES[(q, r)]] = project(g + Vector((0, 0, 0.45)))
    if getattr(Ctx, 'hero', None) is not None:
        data['hero'] = project(Ctx.hero + Vector((0, 0, 0.4)))
    if extra:
        data.update(extra)
    with open(path, 'w') as f:
        json.dump(data, f, indent=1)


# --------------------------------------------------------------------------
# Extra builders for individual concepts

def build_hex_at(center, terrain, site=None, enemies=0, river=None, key=None, label=None, decorate_it=True):
    q, r = key if key else (1000 + len(Ctx.hexes), 0)
    hx = Hex(q, r, terrain, site, enemies, 'kit')
    hx.center = Vector((center[0], center[1], 0.0))
    if river:
        poly = catmull([Vector((center[0] + x, center[1] + y, 0)) for (x, y) in river])
        Ctx.rivers.append(poly)
        hx.segs = list(zip(poly, poly[1:]))
    Ctx.hexes[(q, r)] = hx
    tile_mesh(hx)
    if decorate_it:
        decorate(hx)
    if label:
        Ctx.labels.append((label, hx.ground(0, -0.95) - Vector((0, 0, 0.0))))
    return hx


def transform_objects(objs, matrix):
    bpy.context.view_layer.update()
    for ob in objs:
        ob.matrix_world = matrix @ ob.matrix_world


def lanterns(energy=1.2, per_site=3, color=(1.0, 0.6, 0.28)):
    for hx in Ctx.hexes.values():
        if hx.site in ('village', 'town', 'keep', 'monastery'):
            n = per_site + (3 if hx.site == 'town' else 0)
            for i in range(n):
                a = i / n * 6.283 + 0.3
                d = 0.18 if hx.site != 'town' else 0.32
                g = hx.ground(math.cos(a) * d, math.sin(a) * d)
                L = bpy.data.lights.new('lantern', 'POINT')
                L.energy = energy
                L.color = color
                L.shadow_soft_size = 0.04
                lo = bpy.data.objects.new('lantern', L)
                lo.location = g + Vector((0, 0, 0.12))
                link(lo)


def m_alpha(name, col, alpha, emit=0.0):
    m, nt, p = _new(name)
    _in(p, 'Base Color', col)
    _in(p, 'Alpha', alpha)
    _in(p, 'Roughness', 0.9)
    if emit:
        _in(p, ('Emission Color', 'Emission'), col)
        _in(p, 'Emission Strength', emit)
    m.blend_method = 'BLEND'
    return m


def dump_labels(path, extra=None):
    data = {'labels': [(name, project(pos)) for name, pos in Ctx.labels]}
    if extra:
        data.update(extra)
    with open(path, 'w') as f:
        json.dump(data, f, indent=1)


def cloud_shadows(z=7.0, scale=0.18, cover=0.5):
    m, nt, p = _new('clouds')
    n = nt.nodes.new('ShaderNodeTexNoise')
    n.inputs['Scale'].default_value = scale
    n.inputs['Detail'].default_value = 3.0
    tc = nt.nodes.new('ShaderNodeTexCoord')
    nt.links.new(tc.outputs['Object'], n.inputs['Vector'])
    a = _maprange(nt, n.outputs['Fac'], cover, cover + 0.08, 0.0, 1.0)
    nt.links.new(a, p.inputs['Alpha'])
    m.blend_method = 'BLEND'
    m.shadow_method = 'HASHED'
    bm = bmesh.new()
    bmesh.ops.create_grid(bm, x_segments=1, y_segments=1, size=50)
    ob = add(to_mesh(bm, 'clouds', [m]), (5, 0, z), name='clouds')
    ob.visible_camera = False
    ob.visible_glossy = False
    ob.visible_transmission = False
    ob.visible_diffuse = False
    return ob
