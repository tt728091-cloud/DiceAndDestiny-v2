"""Desert deep-dive for the set-4 concepts: dune geometry blended seamlessly
across hexes, detailed sand/salt/sandstone materials and desert props.
Layers on hexkit + real + biomes."""
import bpy, bmesh, math, random
from mathutils import Vector, Matrix, noise
import hexkit as K
import real as R
import biomes as B
from hexkit import Ctx, lin, smoothstep, lerp, edge_dist, faces_of, to_mesh, bm_cone, bm_box, bm_ico, bm_uvsphere, add, link, WATER, hex_world, catmull, seg_dist, SQ3

DESERT = ('dune', 'sandflat', 'canyon', 'rockland', 'saltpan', 'oasis', 'dcity')
B.ALIAS.update({'dune': 'plains', 'sandflat': 'plains', 'canyon': 'plains', 'rockland': 'hills', 'saltpan': 'plains', 'dcity': 'plains', 'oasis': 'lake'})
NB7 = [(0, 0), (1, 0), (1, -1), (0, -1), (-1, 0), (-1, 1), (0, 1)]


def V(x, y, z=0.0):
    return Vector((x, y, z))


# --------------------------------------------------------------------------
# Heights: one global field, blended between hexes so tiles meet seamlessly

def axial_round(x, y):
    rf = -y / 1.5
    qf = x / SQ3 - rf / 2.0
    cx, cz = qf, rf
    cy = -cx - cz
    rx, ry, rz = round(cx), round(cy), round(cz)
    dx, dy, dz = abs(rx - cx), abs(ry - cy), abs(rz - cz)
    if dx > dy and dx > dz:
        rx = -ry - rz
    elif dy <= dz:
        rz = -rx - ry
    return int(rx), int(rz)


def dune(x, y, amp=None):
    S = Ctx.style
    th = S.get('wind', -1.05)
    c, s = math.cos(th), math.sin(th)
    u, v = x * c + y * s, -x * s + y * c
    lam = S.get('dune_lambda', 2.4)
    ph = u / lam + 1.1 * noise.noise(V(v * 0.22, u * 0.08, 3.1)) + 0.35 * noise.noise(V(v * 0.75, u * 0.2, 7.7))
    k = math.floor(ph)
    t = ph - k
    a = 0.74
    prof = (t / a) ** 1.5 if t < a else (1.0 - (t - a) / (1.0 - a)) ** 1.15
    an = noise.noise(V(x * 0.16, y * 0.16, 11.0))
    seg = noise.noise(V(v * 0.55, k * 1.7, 5.0))
    A = (amp if amp is not None else S.get('dune_amp', 0.42)) * (0.4 + 0.6 * smoothstep(-0.5, 0.4, an)) * (0.45 + 0.55 * smoothstep(-0.45, 0.35, seg))
    lam2 = lam * 0.36
    ph2 = u / lam2 + 0.7 * noise.noise(V(v * 0.6, u * 0.3, 13.0))
    t2 = ph2 - math.floor(ph2)
    prof2 = (t2 / 0.72) ** 1.4 if t2 < 0.72 else (1.0 - (t2 - 0.72) / 0.28) ** 1.2
    th3 = th + 0.75
    u3 = x * math.cos(th3) + y * math.sin(th3)
    ph3 = u3 / 0.36 + 0.4 * noise.noise(V(x * 0.8, y * 0.8, 9.0))
    t3 = ph3 - math.floor(ph3)
    prof3 = (t3 / 0.7) ** 1.4 if t3 < 0.7 else (1.0 - (t3 - 0.7) / 0.3) ** 1.2
    sc = A / max(S.get('dune_amp', 0.42), 1e-6)
    return A * prof + sc * (0.07 * prof2 * (1.0 - prof) + 0.018 * prof3) + 0.06 * noise.noise(V(x * 0.3, y * 0.3, 2.0))


def terrain_h(hx, x, y):
    t = hx.terrain
    if t == 'dune':
        return 0.06 + dune(x, y)
    if t == 'sandflat':
        return 0.055 + dune(x, y, 0.07) + 0.012 * noise.fractal(V(x * 3.0, y * 3.0, 4.0), 0.6, 2.0, 4)
    if t == 'canyon':
        return 0.06 + dune(x, y, 0.12)
    if t == 'rockland':
        return 0.08 + 0.05 * noise.fractal(V(x * 2.2, y * 2.2, 4.0), 0.6, 2.0, 5)
    if t == 'saltpan':
        return 0.004 * noise.noise(V(x * 3.0, y * 3.0, 1.0))
    if t == 'oasis':
        d = math.hypot(x - hx.center.x, y - hx.center.y) + 0.08 * noise.noise(V(x * 2.0, y * 2.0, 6.0))
        return 0.06 - 0.3 * (1.0 - smoothstep(0.3, 0.6, d)) + smoothstep(0.55, 0.95, d) * dune(x, y, 0.12)
    if t == 'dcity':
        return 0.07 + 0.004 * noise.noise(V(x * 4.0, y * 4.0, 2.0))
    return 0.06


def desert_H(wx, wy):
    q, r = axial_round(wx, wy)
    acc = wsum = 0.0
    cache = {}
    for (dq, dr) in NB7:
        hx = Ctx.hexes.get((q + dq, r + dr))
        if hx is None or not getattr(hx, 'dz', False):
            continue
        d2 = (hx.center.x - wx) ** 2 + (hx.center.y - wy) ** 2
        w = math.exp(-d2 / 0.5)
        if w < 1e-4:
            continue
        key = (hx.q, hx.r) if hx.terrain == 'oasis' else hx.terrain
        if key not in cache:
            cache[key] = terrain_h(hx, wx, wy)
        acc += w * cache[key]
        wsum += w
    return acc / wsum if wsum else 0.06


_prev_height = K.Hex.height


def height(self, wx, wy):
    if not getattr(self, 'dz', False):
        return _prev_height(self, wx, wy)
    z = desert_H(wx, wy)
    if self.segs and Ctx.style.get('rivers', True):
        rd = self.river_dist(wx, wy) + 0.012 * noise.noise(V(wx * 5, wy * 5, 3.3))
        z = min(z, lerp(-0.085, 1.0, smoothstep(0.075, 0.26, rd)))
    return z


K.Hex.height = height


def ground(x, y):
    hx = K.nearest_hex(V(x, y))
    return V(x, y, hx.top + hx.height(x, y))


# --------------------------------------------------------------------------
# Materials

def _node(nt, kind, **inputs):
    n = nt.nodes.new(kind)
    for k, v in inputs.items():
        n.inputs[k].default_value = v
    return n


def _rotmap(nt, src, wind, scale=(1, 1, 1)):
    mp = nt.nodes.new('ShaderNodeMapping')
    mp.inputs['Rotation'].default_value = (0, 0, -wind)
    mp.inputs['Scale'].default_value = scale
    nt.links.new(src, mp.inputs['Vector'])
    return mp.outputs['Vector']


def _math(nt, op, a, b):
    m = nt.nodes.new('ShaderNodeMath')
    m.operation = op
    for i, x in enumerate((a, b)):
        if isinstance(x, (int, float)):
            m.inputs[i].default_value = x
        else:
            nt.links.new(x, m.inputs[i])
    return m.outputs['Value']


def m_dune(name, pale, mid, dark, rust, wind, gravel=0.45, ripple=0.95, ripple_scale=30.0, cracks=None, green=None, streak=0.14):
    m, nt, p = K._new(name)
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    pos = geo.outputs['Position']
    n1 = _node(nt, 'ShaderNodeTexNoise', Scale=0.45, Detail=3.0)
    nt.links.new(pos, n1.inputs['Vector'])
    col = K._mix(nt, K._maprange(nt, n1.outputs['Fac'], 0.35, 0.65), pale, mid)
    n3 = _node(nt, 'ShaderNodeTexNoise', Scale=2.6, Detail=4.0)
    nt.links.new(pos, n3.inputs['Vector'])
    col = K._mix(nt, K._maprange(nt, n3.outputs['Fac'], 0.42, 0.62, 0.0, 0.25), col, rust)
    st = _node(nt, 'ShaderNodeTexNoise', Scale=2.2, Detail=5.0)
    nt.links.new(_rotmap(nt, pos, wind, (0.25, 5.0, 1.0)), st.inputs['Vector'])
    col = K._mix(nt, K._maprange(nt, st.outputs['Fac'], 0.5, 0.66, 0.0, streak), col, dark)
    col = K._mix(nt, K._maprange(nt, st.outputs['Fac'], 0.34, 0.5, streak * 0.8, 0.0), col, (pale[0] * 1.12, pale[1] * 1.1, pale[2] * 1.05, 1))
    sep = nt.nodes.new('ShaderNodeSeparateXYZ')
    nt.links.new(geo.outputs['Normal'], sep.inputs['Vector'])
    steep = K._maprange(nt, sep.outputs['Z'], 0.72, 0.93, 1.0, 0.0)
    col = K._mix(nt, _math(nt, 'MULTIPLY', steep, 0.5), col, rust)
    z = K._objz(nt)
    trough = K._maprange(nt, z, 0.03, 0.12, 1.0, 0.0)
    crest = K._maprange(nt, z, 0.22, 0.42, 0.0, 0.22)
    col = K._mix(nt, _math(nt, 'MULTIPLY', crest, _math(nt, 'SUBTRACT', 1.0, steep)), col, (pale[0] * 1.18, pale[1] * 1.15, pale[2] * 1.12, 1))
    col = K._mix(nt, _math(nt, 'MULTIPLY', trough, 0.35), col, dark)
    vo = _node(nt, 'ShaderNodeTexVoronoi', Scale=240.0)
    nt.links.new(pos, vo.inputs['Vector'])
    dots = K._maprange(nt, vo.outputs['Distance'], 0.05, 0.22, 1.0, 0.0)
    gmask = _math(nt, 'MULTIPLY', dots, _math(nt, 'MULTIPLY', trough, gravel))
    col = K._mix(nt, gmask, col, (dark[0] * 0.45, dark[1] * 0.4, dark[2] * 0.38, 1))
    w = nt.nodes.new('ShaderNodeTexWave')
    w.wave_profile = 'SAW'
    w.inputs['Scale'].default_value = ripple_scale
    w.inputs['Distortion'].default_value = 3.5
    w.inputs['Detail'].default_value = 2.0
    w.inputs['Detail Scale'].default_value = 1.5
    nt.links.new(_rotmap(nt, pos, wind), w.inputs['Vector'])
    rmask = _math(nt, 'MULTIPLY', _math(nt, 'SUBTRACT', 1.0, steep), _math(nt, 'SUBTRACT', 1.0, _math(nt, 'MULTIPLY', trough, 0.6)))
    col = K._mix(nt, _math(nt, 'MULTIPLY', K._maprange(nt, w.outputs['Fac'], 0.6, 1.0, 0.0, 0.1), rmask), col, (pale[0] * 1.2, pale[1] * 1.16, pale[2] * 1.1, 1))
    micro = _node(nt, 'ShaderNodeTexNoise', Scale=520.0, Detail=2.0)
    nt.links.new(pos, micro.inputs['Vector'])
    hgt = _math(nt, 'ADD', _math(nt, 'MULTIPLY', w.outputs['Fac'], rmask), _math(nt, 'MULTIPLY', micro.outputs['Fac'], 0.25))
    hgt = _math(nt, 'ADD', hgt, _math(nt, 'MULTIPLY', gmask, 0.6))
    if cracks:
        low = K._maprange(nt, z, -0.03, 0.0, 1.0, 0.0)
        cv = _node(nt, 'ShaderNodeTexVoronoi', Scale=26.0)
        cv.feature = 'DISTANCE_TO_EDGE'
        nt.links.new(pos, cv.inputs['Vector'])
        line = K._maprange(nt, cv.outputs['Distance'], 0.0, 0.05, 1.0, 0.0)
        mud = K._mix(nt, line, cracks, (cracks[0] * 0.35, cracks[1] * 0.32, cracks[2] * 0.3, 1))
        col = K._mix(nt, low, col, mud)
        hgt = _math(nt, 'ADD', hgt, _math(nt, 'MULTIPLY', _math(nt, 'MULTIPLY', line, low), -1.5))
    if green:
        gz = K._maprange(nt, z, -0.05, 0.03, 1.0, 0.0)
        gz2 = K._maprange(nt, z, -0.09, -0.05, 0.0, 1.0)
        gn = _node(nt, 'ShaderNodeTexNoise', Scale=14.0, Detail=4.0)
        nt.links.new(pos, gn.inputs['Vector'])
        gcol = K._mix(nt, K._maprange(nt, gn.outputs['Fac'], 0.4, 0.6), green, (green[0] * 0.6, green[1] * 0.7, green[2] * 0.5, 1))
        col = K._mix(nt, _math(nt, 'MULTIPLY', gz, gz2), col, gcol)
    nt.links.new(col, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.9)
    K._in(p, ('Specular IOR Level', 'Specular'), 0.3)
    K._in(p, 'Sheen Weight', 0.35)
    K._in(p, 'Sheen Roughness', 0.45)
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = ripple
    b.inputs['Distance'].default_value = 0.004
    nt.links.new(hgt, b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


def m_salt(name, base, stain, wet=0.0):
    m, nt, p = K._new(name)
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    pos = geo.outputs['Position']
    dn = _node(nt, 'ShaderNodeTexNoise', Scale=3.0, Detail=3.0)
    nt.links.new(pos, dn.inputs['Vector'])
    av = nt.nodes.new('ShaderNodeVectorMath')
    av.operation = 'MULTIPLY_ADD'
    nt.links.new(dn.outputs['Color'], av.inputs[0])
    av.inputs[1].default_value = (0.06, 0.06, 0.06)
    nt.links.new(pos, av.inputs[2])
    vo = _node(nt, 'ShaderNodeTexVoronoi', Scale=4.2)
    vo.feature = 'DISTANCE_TO_EDGE'
    nt.links.new(av.outputs['Vector'], vo.inputs['Vector'])
    ridge = K._maprange(nt, vo.outputs['Distance'], 0.0, 0.07, 1.0, 0.0)
    inner = K._maprange(nt, vo.outputs['Distance'], 0.05, 0.4, 0.0, 0.3)
    col = K._mix(nt, inner, base, (base[0] * 0.78, base[1] * 0.76, base[2] * 0.74, 1))
    sn = _node(nt, 'ShaderNodeTexNoise', Scale=1.4, Detail=5.0)
    nt.links.new(pos, sn.inputs['Vector'])
    col = K._mix(nt, K._maprange(nt, sn.outputs['Fac'], 0.55, 0.72, 0.0, 0.4), col, stain)
    col = K._mix(nt, _math(nt, 'MULTIPLY', ridge, 0.6), col, (1.0, 1.0, 0.98, 1))
    v2 = _node(nt, 'ShaderNodeTexVoronoi', Scale=34.0)
    v2.feature = 'DISTANCE_TO_EDGE'
    nt.links.new(pos, v2.inputs['Vector'])
    fine = K._maprange(nt, v2.outputs['Distance'], 0.0, 0.025, 0.35, 0.0)
    col = K._mix(nt, fine, col, (base[0] * 0.55, base[1] * 0.52, base[2] * 0.5, 1))
    micro = _node(nt, 'ShaderNodeTexNoise', Scale=300.0, Detail=2.0)
    nt.links.new(pos, micro.inputs['Vector'])
    hgt = _math(nt, 'ADD', ridge, _math(nt, 'MULTIPLY', micro.outputs['Fac'], 0.2))
    rough = 0.6
    if wet:
        wn = _node(nt, 'ShaderNodeTexNoise', Scale=0.7, Detail=3.0)
        nt.links.new(pos, wn.inputs['Vector'])
        wm = _math(nt, 'MULTIPLY', K._maprange(nt, wn.outputs['Fac'], 0.46, 0.52), wet)
        wm = _math(nt, 'MULTIPLY', wm, _math(nt, 'SUBTRACT', 1.0, ridge))
        col = K._mix(nt, _math(nt, 'MULTIPLY', wm, 0.45), col, (base[0] * 0.5, base[1] * 0.5, base[2] * 0.52, 1))
        nt.links.new(K._maprange(nt, wm, 0.0, 1.0, 0.6, 0.02), p.inputs['Roughness'])
        hgt = _math(nt, 'MULTIPLY', hgt, _math(nt, 'SUBTRACT', 1.0, wm))
    else:
        K._in(p, 'Roughness', rough)
    nt.links.new(col, p.inputs['Base Color'])
    K._in(p, ('Specular IOR Level', 'Specular'), 0.5)
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = 0.9
    b.inputs['Distance'].default_value = 0.01
    nt.links.new(hgt, b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


def m_redrock(name, cream, orange, red, varnish, sand):
    m, nt, p = K._new(name)
    geo = nt.nodes.new('ShaderNodeNewGeometry')
    pos = geo.outputs['Position']
    w = nt.nodes.new('ShaderNodeTexWave')
    w.bands_direction = 'Z'
    w.inputs['Scale'].default_value = 1.3
    w.inputs['Distortion'].default_value = 3.0
    w.inputs['Detail'].default_value = 6.0
    nt.links.new(pos, w.inputs['Vector'])
    ramp = nt.nodes.new('ShaderNodeValToRGB')
    els = ramp.color_ramp.elements
    els[0].position, els[0].color = 0.0, red
    els[1].position, els[1].color = 1.0, cream
    for pos_, c in ((0.35, orange), (0.55, red), (0.75, orange)):
        e = els.new(pos_)
        e.color = c
    nt.links.new(w.outputs['Fac'], ramp.inputs['Fac'])
    col = ramp.outputs['Color']
    vs = _node(nt, 'ShaderNodeTexNoise', Scale=3.0, Detail=6.0)
    mp = nt.nodes.new('ShaderNodeMapping')
    mp.inputs['Scale'].default_value = (7.0, 7.0, 0.35)
    nt.links.new(pos, mp.inputs['Vector'])
    nt.links.new(mp.outputs['Vector'], vs.inputs['Vector'])
    sep = nt.nodes.new('ShaderNodeSeparateXYZ')
    nt.links.new(geo.outputs['Normal'], sep.inputs['Vector'])
    vert = K._maprange(nt, sep.outputs['Z'], 0.35, 0.65, 1.0, 0.0)
    vm = _math(nt, 'MULTIPLY', K._maprange(nt, vs.outputs['Fac'], 0.6, 0.74, 0.0, 0.55), vert)
    col = K._mix(nt, vm, col, varnish)
    top = K._maprange(nt, sep.outputs['Z'], 0.75, 0.92, 0.0, 0.85)
    col = K._mix(nt, top, col, sand)
    cv = _node(nt, 'ShaderNodeTexVoronoi', Scale=16.0)
    cv.feature = 'DISTANCE_TO_EDGE'
    nt.links.new(pos, cv.inputs['Vector'])
    crack = K._maprange(nt, cv.outputs['Distance'], 0.0, 0.03, 0.55, 0.0)
    col = K._mix(nt, crack, col, (0.03, 0.02, 0.015, 1))
    nt.links.new(col, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.9)
    micro = _node(nt, 'ShaderNodeTexNoise', Scale=90.0, Detail=6.0)
    nt.links.new(pos, micro.inputs['Vector'])
    hgt = _math(nt, 'ADD', _math(nt, 'MULTIPLY', w.outputs['Fac'], 0.8), _math(nt, 'ADD', micro.outputs['Fac'], _math(nt, 'MULTIPLY', crack, -1.5)))
    b = nt.nodes.new('ShaderNodeBump')
    b.inputs['Strength'].default_value = 0.6
    b.inputs['Distance'].default_value = 0.008
    nt.links.new(hgt, b.inputs['Height'])
    nt.links.new(b.outputs['Normal'], p.inputs['Normal'])
    return m


def m_adobe(name, base):
    m, nt, p = K._new(name)
    oi = nt.nodes.new('ShaderNodeObjectInfo')
    tint = nt.nodes.new('ShaderNodeMixRGB')
    tint.blend_type = 'MULTIPLY'
    tint.inputs['Fac'].default_value = 1.0
    tint.inputs['Color1'].default_value = base
    nt.links.new(oi.outputs['Color'], tint.inputs['Color2'])
    n = K._noise(nt, 40.0, 5.0)
    col = K._mix(nt, K._maprange(nt, n.outputs['Fac'], 0.35, 0.7, 0.0, 0.3), tint.outputs['Color'], (base[0] * 0.55, base[1] * 0.5, base[2] * 0.45, 1))
    z = K._objz(nt)
    col = K._mix(nt, K._maprange(nt, z, 0.0, 0.02, 0.45, 0.0), col, (base[0] * 0.45, base[1] * 0.4, base[2] * 0.35, 1))
    nt.links.new(col, p.inputs['Base Color'])
    K._in(p, 'Roughness', 0.95)
    K._bump(nt, p, 110.0, 0.3, 0.003)
    return m


def desert_materials(pal):
    S = Ctx.style
    M = Ctx.mats
    wind = S.get('wind', -1.05)
    sand = pal['sand3']
    M['dune'] = m_dune('dune', sand[0], sand[1], sand[2], pal['rust'], wind, cracks=pal.get('mud_cracks') if S.get('dry_rivers') else None)
    M['sandflat'] = m_dune('sandflat', sand[0], sand[1], sand[2], pal['rust'], wind, gravel=0.9, ripple=0.35, cracks=pal.get('mud_cracks') if S.get('dry_rivers') else None)
    red = pal['redsand3']
    M['canyon'] = m_dune('canyon', red[0], red[1], red[2], pal['rust'], wind, gravel=0.7, cracks=pal.get('mud_cracks'))
    M['rockland'] = m_dune('rockland', red[1], red[2], pal['rust'], red[2], wind, gravel=1.0, ripple=0.3)
    M['saltpan'] = m_salt('saltpan', pal['salt'], pal['salt_stain'], S.get('wet', 0.0))
    M['oasis'] = m_dune('oasis', sand[0], sand[1], sand[2], pal['rust'], wind, green=pal['oasis_green'])
    M['dcity'] = m_dune('dcity', sand[1], sand[2], sand[2], pal['rust'], wind, gravel=0.3, ripple=0.2)
    rk = pal['rock3']
    M['sandstone'] = m_redrock('sandstone', rk[0], rk[1], rk[2], pal['varnish'], red[0])
    M['mesa'] = M['sandstone']
    M['side'] = K.m_strata('side_desert', pal['strata'], S['thickness'])
    M['adobe'] = m_adobe('adobe', pal['adobe'])
    M['dome'] = K.m_flat('dome', pal['dome'], 0.3, spec=0.6)
    M['dome_gold'] = K.m_flat('dome_gold', lin('#c89a48'), 0.3, metal=1.0)
    M['awning'] = K.m_flat('awning', (1, 1, 1, 1), 0.85, obj_color=True)
    M['camel'] = K.m_flat('camel', pal['camel'], 0.9)
    M['blanket'] = K.m_flat('blanket', lin('#8c2f2a'), 0.85)
    M['tentcloth'] = K.m_flat('tentcloth', pal['tent'], 0.95)
    M['rut'] = K.m_flat('rut', (sand[2][0] * 0.55, sand[2][1] * 0.5, sand[2][2] * 0.45, 1), 1.0)
    M['saltc'] = K.m_flat('saltc', lin('#f4f0e8'), 0.25, spec=0.6)
    M['bonemat2'] = K.m_flat('bonemat2', pal.get('bone', lin('#e2d6bc')), 0.55)
    M['socket'] = K.m_flat('socket', lin('#140e0a'), 0.9)
    M['acacia'] = K.m_flat('acacia', (1, 1, 1, 1), 0.8, obj_color=True)
    oasis_w = K.m_water('oasis_water', pal.get('oasis_water', lin('#2e8a86')), trans=0.6)
    oasis_w['no_overlay'] = True
    M['oasis_water'] = oasis_w


# --------------------------------------------------------------------------
# Meshes

def tube(bm, pts, radii, segs=8, mi=0, cap=True):
    rings, prev = [], None
    for i, p in enumerate(pts):
        a, b = pts[max(i - 1, 0)], pts[min(i + 1, len(pts) - 1)]
        t = (b - a).normalized()
        if prev is None:
            ref = V(0, 0, 1) if abs(t.z) < 0.9 else V(1, 0, 0)
            n = t.cross(ref).normalized()
        else:
            n = (prev - t * prev.dot(t)).normalized()
        bv = t.cross(n)
        prev = n
        rings.append([bm.verts.new(p + (n * math.cos(2 * math.pi * k / segs) + bv * math.sin(2 * math.pi * k / segs)) * radii[i]) for k in range(segs)])
    for i in range(len(rings) - 1):
        for k in range(segs):
            k2 = (k + 1) % segs
            bm.faces.new((rings[i][k], rings[i][k2], rings[i + 1][k2], rings[i + 1][k])).material_index = mi
    if cap:
        bm.faces.new(list(reversed(rings[0]))).material_index = mi
        bm.faces.new(rings[-1]).material_index = mi
    return rings


def half_dome(bm, r, c, mi, u=18, v=10):
    ret = bmesh.ops.create_uvsphere(bm, u_segments=u, v_segments=v, radius=r, matrix=Matrix.Translation(c))
    low = [x for x in ret['verts'] if x.co.z < c[2] - 1e-5]
    bmesh.ops.delete(bm, geom=low, context='VERTS')
    for f in faces_of([x for x in ret['verts'] if x.is_valid]):
        f.material_index = mi


def jitter(bm, amt, freq, off=0.0, zmin=-9.0):
    for v in bm.verts:
        if v.co.z < zmin:
            continue
        d = V(noise.noise(v.co * freq + V(off, 0, 0)), noise.noise(v.co * freq + V(0, off + 3, 0)), noise.noise(v.co * freq + V(0, 0, off + 7)))
        v.co += d * amt


def build_desert_meshes():
    M, me = Ctx.mats, Ctx.meshes
    rnd = random.Random(41)
    # adobe houses: [adobe, door, window, beam, dome]
    for k in range(5):
        bm = bmesh.new()
        w, d, h = rnd.uniform(0.06, 0.1), rnd.uniform(0.05, 0.07), rnd.uniform(0.04, 0.07)
        bm_box(bm, (0, 0, h / 2), (w, d, h), 0)
        t, ph = 0.004, 0.009
        for (cx, cy, sx, sy) in ((0, -d / 2 + t / 2, w, t), (0, d / 2 - t / 2, w, t), (-w / 2 + t / 2, 0, t, d), (w / 2 - t / 2, 0, t, d)):
            bm_box(bm, (cx, cy, h + ph / 2), (sx, sy, ph), 0)
        bm_box(bm, (w * 0.2, -d / 2 - 0.0012, 0.015), (0.014, 0.004, 0.03), 1)
        bm_box(bm, (-w * 0.25, -d / 2 - 0.0012, h * 0.68), (0.008, 0.004, 0.009), 2)
        bm_box(bm, (w / 2 + 0.0012, 0, h * 0.68), (0.004, 0.009, 0.009), 2)
        for i in range(4):
            bm_cone(bm, 5, 0.0018, 0.0018, 0.009, h - 0.007, 3, x=-w / 2 + (i + 0.5) * w / 4, y=-d / 2 + 0.001, tilt=(math.pi / 2, 0))
        if k in (1, 3):
            w2, d2, h2 = w * 0.5, d * 0.6, h * 0.55
            bm_box(bm, (w * 0.18, d * 0.12, h + h2 / 2), (w2, d2, h2), 0)
            bm_box(bm, (w * 0.18, d * 0.12 - d2 / 2 - 0.0012, h + h2 * 0.55), (0.008, 0.004, 0.009), 2)
        if k in (2, 4):
            r = min(w, d) * 0.38
            bm_cone(bm, 16, r, r, 0.01, h, 0, x=-w * 0.1)
            half_dome(bm, r, (-w * 0.1, 0, h + 0.01), 4)
        me[f'adobe{k}'] = to_mesh(bm, f'adobe{k}', [M['adobe'], M['door'], M['window'], M['beam'], M['dome']])
    # desert tower
    bm = bmesh.new()
    bm_cone(bm, 4, 0.06, 0.05, 0.3, 0.0, 0, rot=math.pi / 4)
    for i in range(4):
        for j in range(3):
            a = i * math.pi / 2
            o = (j - 1) * 0.022
            c, s_ = math.cos(a), math.sin(a)
            bm_box(bm, (c * 0.034 - s_ * o, s_ * 0.034 + c * o, 0.31), (0.012, 0.012, 0.022), 0)
    for z in (0.12, 0.2, 0.26):
        bm_box(bm, (0, -0.038, z), (0.006, 0.004, 0.018), 2)
    me['dtower'] = to_mesh(bm, 'dtower', [M['adobe'], M['door'], M['window']])
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.055), (1.0, 0.035, 0.11), 0)
    for j in range(9):
        bm_box(bm, (-0.45 + j * 0.1125, 0, 0.12), (0.05, 0.037, 0.022), 0)
    me['dwall'] = to_mesh(bm, 'dwall', [M['adobe']])
    # temple
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.05), (0.2, 0.2, 0.1), 0)
    bm_cone(bm, 20, 0.07, 0.07, 0.03, 0.1, 0)
    half_dome(bm, 0.07, (0, 0, 0.13), 4, 24, 14)
    bm_cone(bm, 6, 0.004, 0.0, 0.03, 0.2, 4)
    for (x, y) in ((-0.09, -0.09), (0.09, -0.09), (0.09, 0.09), (-0.09, 0.09)):
        bm_cone(bm, 10, 0.016, 0.014, 0.16, 0.0, 0, x=x, y=y)
        half_dome(bm, 0.016, (x, y, 0.16), 4, 10, 6)
    for i in range(5):
        bm_cone(bm, 10, 0.007, 0.007, 0.08, 0.0, 0, x=-0.06 + i * 0.03, y=-0.115)
    bm_box(bm, (0, -0.115, 0.085), (0.15, 0.03, 0.012), 0)
    bm_box(bm, (0, -0.1012, 0.03), (0.03, 0.004, 0.06), 1)
    me['temple'] = to_mesh(bm, 'temple', [M['adobe'], M['door'], M['window'], M['beam'], M['dome']])
    # obelisk, columns, drums, colossus
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.015), (0.07, 0.07, 0.03), 0)
    bm_cone(bm, 4, 0.024, 0.016, 0.32, 0.03, 0, rot=math.pi / 4)
    bm_cone(bm, 4, 0.016, 0.0, 0.03, 0.35, 1, rot=math.pi / 4)
    me['obelisk'] = to_mesh(bm, 'obelisk', [M['sandstone'], M['dome_gold']])
    bm = bmesh.new()
    bm_cone(bm, 4, 0.024, 0.016, 0.32, 0.0, 0, rot=math.pi / 4)
    bm_cone(bm, 4, 0.016, 0.0, 0.03, 0.32, 0, rot=math.pi / 4)
    me['obelisk_fallen'] = to_mesh(bm, 'obelisk_fallen', [M['sandstone']])
    bm = bmesh.new()
    bm_cone(bm, 14, 0.02, 0.018, 0.2, 0.0, 0)
    bm_box(bm, (0, 0, 0.205), (0.05, 0.05, 0.016), 0)
    me['dcolumn'] = to_mesh(bm, 'dcolumn', [M['sandstone']])
    bm = bmesh.new()
    bm_cone(bm, 14, 0.022, 0.022, 0.045, -0.0225, 0, tilt=(math.pi / 2, 0))
    me['drum'] = to_mesh(bm, 'drum', [M['sandstone']])
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.03), (0.17, 0.1, 0.06), 0)
    for s in (-1, 1):
        bm_box(bm, (s * 0.035, -0.012, 0.07), (0.035, 0.06, 0.02), 0)
        bm_cone(bm, 8, 0.02, 0.028, 0.2, 0.075, 0, x=s * 0.035, y=0.0)
        bm_ico(bm, 0.03, (s * 0.035, 0.0, 0.28), 0, sub=2, squash=(1.0, 1.0, 0.6), rnd=rnd, jit=0.01)
    jitter(bm, 0.004, 40.0, 2.0, zmin=0.07)
    me['colossus'] = to_mesh(bm, 'colossus', [M['sandstone']], smooth=False)
    # rock arch, hoodoos
    for k in range(2):
        bm = bmesh.new()
        R0 = 0.16 + 0.04 * k
        pts = [V(R0 * math.cos(th), 0, R0 * math.sin(th) * 1.15 - 0.04) for th in [math.pi * i / 16 for i in range(17)]]
        radii = [0.06 - 0.025 * math.sin(math.pi * i / 16) for i in range(17)]
        tube(bm, pts, radii, 12, 0)
        bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
        jitter(bm, 0.018, 9.0, k * 5.0)
        me[f'arch{k}'] = to_mesh(bm, f'arch{k}', [M['sandstone']], smooth=True)
    for k in range(3):
        bm = bmesh.new()
        h = rnd.uniform(0.18, 0.3)
        bm_cone(bm, 10, 0.055, 0.03, h * 0.55, -0.02, 0)
        bm_ico(bm, 0.032, (0, 0, h * 0.62), 0, sub=2, squash=(1.0, 1.0, 1.5))
        bm_ico(bm, 0.05, (0.005, 0, h * 0.86), 0, sub=2, squash=(1.0, 0.9, 0.45))
        jitter(bm, 0.012, 14.0, k * 3.0)
        me[f'hoodoo{k}'] = to_mesh(bm, f'hoodoo{k}', [M['sandstone']], smooth=True)
    # skeleton
    bm = bmesh.new()
    L, H0 = 0.5, 0.12
    spine = [V(-L / 2 + L * i / 20, 0, H0 + 0.025 * math.sin(math.pi * i / 20)) for i in range(21)]
    tube(bm, spine, [0.011] * 21, 8, 0)
    for i in range(0, 21, 2):
        bm_ico(bm, 0.016, spine[i], 0, sub=1)
    nrib = 9
    for j in range(nrib):
        x = -L / 2 + 0.1 + j * (L - 0.2) / (nrib - 1)
        Rr = 0.15 * (0.7 + 0.3 * math.sin(math.pi * j / (nrib - 1)))
        for side in (-1, 1):
            pts, rad = [], []
            for s in range(14):
                th = s / 13 * math.radians(150)
                pts.append(V(x + 0.03 * s / 13, side * Rr * math.sin(th) * 1.1, H0 - Rr + Rr * math.cos(th)))
                rad.append(0.009 - 0.004 * s / 13)
            tube(bm, pts, rad, 6, 0)
    sk = V(L / 2 + 0.07, 0, 0.07)
    bm_ico(bm, 0.055, sk, 0, sub=3, squash=(1.6, 1.0, 0.85))
    for s in (-1, 1):
        bm_ico(bm, 0.016, sk + V(0.03, s * 0.035, 0.02), 1, sub=2)
        horn = [sk + V(-0.02 - 0.05 * t, s * (0.05 + 0.09 * t), 0.04 + 0.08 * math.sin(t * 2.4)) for t in [i / 10 for i in range(11)]]
        tube(bm, horn, [0.016 - 0.014 * i / 10 for i in range(11)], 8, 0)
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    me['skeleton'] = to_mesh(bm, 'skeleton', [M['bonemat2'], M['socket']], smooth=True)
    # acacia, shrub
    for k in range(3):
        bm = bmesh.new()
        top = V(rnd.uniform(-0.01, 0.01), rnd.uniform(-0.01, 0.01), 0.075)
        tube(bm, [V(0, 0, -0.01), top * 0.5 + V(0.004, 0, 0), top], [0.009, 0.007, 0.006], 6, 0)
        for b_ in range(4):
            a = b_ / 4 * 6.283 + rnd.uniform(-0.3, 0.3)
            end = top + V(math.cos(a) * 0.065, math.sin(a) * 0.065, 0.045)
            tube(bm, [top, (top + end) / 2 + V(0, 0, 0.012), end], [0.005, 0.004, 0.003], 5, 0)
        for j in range(46):
            p = V(rnd.gauss(0, 0.05), rnd.gauss(0, 0.05), 0)
            if p.length > 0.095:
                p *= 0.095 / p.length
            p.z = 0.125 + rnd.uniform(-0.008, 0.01) - p.length * 0.12
            bm_ico(bm, rnd.uniform(0.012, 0.018), p, 1, sub=1, squash=(1.0, 1.0, 0.45))
        me[f'acacia{k}'] = to_mesh(bm, f'acacia{k}', [M['trunk'], M['acacia']])
    for k in range(3):
        bm = bmesh.new()
        for i in range(5):
            a = rnd.uniform(0, 6.28)
            R.branch(bm, V(0, 0, -0.005), V(math.cos(a) * 0.6, math.sin(a) * 0.6, 1).normalized(), 0.025, 0.0025, 2, rnd)
        for i in range(6):
            bm_ico(bm, 0.008, V(rnd.uniform(-0.025, 0.025), rnd.uniform(-0.025, 0.025), rnd.uniform(0.02, 0.04)), 1, sub=1)
        me[f'shrub{k}'] = to_mesh(bm, f'shrub{k}', [M['deadwood'], M['acacia']])
    # salt crystals
    for k in range(2):
        bm = bmesh.new()
        for i in range(rnd.randint(4, 7)):
            a = rnd.uniform(0, 6.28)
            rr = rnd.uniform(0, 0.03)
            bm_cone(bm, 4, rnd.uniform(0.008, 0.014), 0.0, rnd.uniform(0.03, 0.07), -0.01, 0, x=math.cos(a) * rr, y=math.sin(a) * rr,
                    tilt=(rnd.uniform(-0.4, 0.4), rnd.uniform(-0.4, 0.4)))
        me[f'saltc{k}'] = to_mesh(bm, f'saltc{k}', [M['saltc']])
    # camel, wagon, tent, well
    bm = bmesh.new()
    bm_ico(bm, 0.03, (0, 0, 0.058), 0, sub=2, squash=(1.5, 0.72, 0.78))
    bm_ico(bm, 0.019, (0.004, 0, 0.083), 0, sub=2, squash=(1.2, 0.85, 1.0))
    bm_box(bm, (0.004, 0, 0.074), (0.04, 0.05, 0.004), 1)
    tube(bm, [V(0.038, 0, 0.062), V(0.058, 0, 0.074), V(0.066, 0, 0.098), V(0.07, 0, 0.11)], [0.009, 0.008, 0.007, 0.007], 6, 0)
    bm_ico(bm, 0.011, (0.08, 0, 0.112), 0, sub=2, squash=(1.6, 0.8, 0.8))
    for (lx, ly) in ((0.03, 0.012), (0.03, -0.012), (-0.03, 0.012), (-0.03, -0.012)):
        bm_cone(bm, 6, 0.0045, 0.003, 0.05, -0.002, 0, x=lx, y=ly)
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    me['camel'] = to_mesh(bm, 'camel', [M['camel'], M['blanket']], smooth=True)
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.03), (0.09, 0.045, 0.012), 0)
    bm_cone(bm, 14, 0.024, 0.024, 0.08, 0.036, 1, x=-0.04, tilt=(0, math.pi / 2))
    for (x, y) in ((0.03, 0.03), (-0.03, 0.03), (0.03, -0.024), (-0.03, -0.024)):
        bm_cone(bm, 12, 0.014, 0.014, 0.006, 0.014, 0, x=x, y=y, tilt=(math.pi / 2, 0))
    me['wagon'] = to_mesh(bm, 'wagon', [M['deadwood'], M['canvas']], smooth=False)
    bm = bmesh.new()
    xs, ys = [-0.07, -0.035, 0.0, 0.035, 0.07], [-0.045, 0.0, 0.045]
    grid = {}
    for i, x in enumerate(xs):
        for j, y in enumerate(ys):
            z = 0.014 if j != 1 else (0.05 if i in (1, 3) else 0.04)
            if i in (0, 4):
                z *= 0.7
            grid[(i, j)] = bm.verts.new((x, y, z))
    for i in range(4):
        for j in range(2):
            bm.faces.new((grid[(i, j)], grid[(i + 1, j)], grid[(i + 1, j + 1)], grid[(i, j + 1)]))
    for i in (1, 3):
        bm_cone(bm, 4, 0.002, 0.002, 0.05, 0.0, 1, x=xs[i])
    me['dtent'] = to_mesh(bm, 'dtent', [M['tentcloth'], M['deadwood']])
    bm = bmesh.new()
    bm_cone(bm, 14, 0.03, 0.03, 0.025, 0.0, 0)
    bm_cone(bm, 14, 0.021, 0.021, 0.002, 0.024, 1)
    for s in (-1, 1):
        bm_box(bm, (s * 0.025, 0, 0.045), (0.004, 0.004, 0.05), 2)
    bm_box(bm, (0, 0, 0.07), (0.06, 0.004, 0.004), 2)
    me['well'] = to_mesh(bm, 'well', [M['sandstone'], M['socket'], M['deadwood']])
    bm = bmesh.new()
    bm_box(bm, (0, 0, 0.0), (1.0, 1.0, 0.004), 0)
    me['awning'] = to_mesh(bm, 'awning', [M['awning']])
    for k in range(3):
        me[f'rock{k}'].materials[0] = M['sandstone']


# --------------------------------------------------------------------------
# Placement

def ok(x, y, minz=WATER + 0.02):
    hx = K.nearest_hex(V(x, y))
    return hx.height(x, y) > minz


def place(mesh, x, y, rot=0.0, scale=1.0, color=None, sink=0.0, tilt=None):
    g = ground(x, y)
    return add(mesh, g - V(0, 0, sink), rot, scale, color, tilt=tilt)


def scatter_xy(hx, n, min_dist, avoid=0.0, margin=0.08, seed=0, minz=WATER + 0.02, taken=None):
    rnd = random.Random(hx.seed + seed)
    pts, taken = [], taken if taken is not None else []
    tries = 0
    while len(pts) < n and tries < n * 40:
        tries += 1
        p = V(rnd.uniform(-1, 1), rnd.uniform(-1, 1))
        if edge_dist(p, 1.0 - Ctx.style['gap']) < margin or p.length < avoid:
            continue
        x, y = hx.center.x + p.x, hx.center.y + p.y
        if hx.river_dist(x, y) < 0.18 or not ok(x, y, minz):
            continue
        if any((p - q).length < min_dist for q in pts + taken):
            continue
        pts.append(p)
    taken.extend(pts)
    return [(hx.center.x + p.x, hx.center.y + p.y) for p in pts]


def shrubs(hx, n, seed=1):
    rnd = random.Random(hx.seed + seed)
    for (x, y) in scatter_xy(hx, n, 0.1, seed=seed):
        place(Ctx.meshes[f'shrub{rnd.randrange(3)}'], x, y, rnd.uniform(0, 6), rnd.uniform(1.0, 1.6), K.jitter_col(Ctx.style['pal']['acacia'][0], rnd))


def stones(hx, n, scale=(0.4, 1.0), seed=2, mat_col=None):
    rnd = random.Random(hx.seed + seed)
    col = mat_col or Ctx.style['pal']['rock']
    for (x, y) in scatter_xy(hx, n, 0.05, seed=seed, margin=0.04):
        place(Ctx.meshes[f'rock{rnd.randrange(3)}'], x, y, rnd.uniform(0, 6), rnd.uniform(*scale), K.jitter_col(col, rnd, 0.12), sink=0.01)


def deco_dune(hx):
    rnd = hx.rnd
    if rnd.random() < 0.5:
        shrubs(hx, 2)
    if rnd.random() < 0.15:
        x, y = hx.center.x + rnd.uniform(-0.4, 0.4), hx.center.y + rnd.uniform(-0.4, 0.4)
        place(Ctx.meshes['skull'], x, y, rnd.uniform(0, 6), 1.8, sink=0.004)
    site(hx)


def deco_sandflat(hx):
    rnd = hx.rnd
    stones(hx, 14, (0.25, 0.7))
    shrubs(hx, 5)
    if rnd.random() < 0.45:
        for (x, y) in scatter_xy(hx, 1, 0.2, seed=5):
            place(Ctx.meshes[f'acacia{rnd.randrange(3)}'], x, y, rnd.uniform(0, 6), rnd.uniform(1.2, 1.6), K.jitter_col(Ctx.style['pal']['acacia'][1], rnd))
    site(hx)


def deco_canyon(hx):
    rnd = hx.rnd
    if not hx.site and rnd.random() < 0.38:
        a = rnd.uniform(0, 6.28)
        d = rnd.uniform(0.0, 0.25)
        x, y = hx.center.x + math.cos(a) * d, hx.center.y + math.sin(a) * d
        if hx.river_dist(x, y) > 0.55:
            wide = rnd.random() < 0.6
            B.mesa_obj(hx, x - hx.center.x, y - hx.center.y, rnd.uniform(0.5, 0.72) if wide else rnd.uniform(0.26, 0.36),
                       (rnd.uniform(0.32, 0.55) if wide else rnd.uniform(0.6, 0.95)) * Ctx.style.get('mesa_scale', 1.0), rnd)
    elif rnd.random() < 0.35:
        for (x, y) in scatter_xy(hx, 1, 0.25, avoid=0.2, seed=7):
            place(Ctx.meshes[f'hoodoo{rnd.randrange(3)}'], x, y, rnd.uniform(0, 6), rnd.uniform(1.4, 2.2), sink=0.02)
    if rnd.random() < 0.12:
        for (x, y) in scatter_xy(hx, 1, 0.3, avoid=0.35, seed=8):
            place(Ctx.meshes[f'arch{rnd.randrange(2)}'], x, y, rnd.uniform(0, 6), rnd.uniform(1.6, 2.2), sink=0.01)
    stones(hx, 7, (0.4, 1.1), mat_col=Ctx.style['pal']['rock3'][1])
    shrubs(hx, 3)
    site(hx)


def deco_rockland(hx):
    rnd = hx.rnd
    stones(hx, 10, (0.5, 1.4), mat_col=Ctx.style['pal']['rock3'][2])
    if rnd.random() < 0.4:
        for (x, y) in scatter_xy(hx, 1, 0.3, seed=9, avoid=0.3 if hx.site else 0.0):
            place(Ctx.meshes[f'hoodoo{rnd.randrange(3)}'], x, y, rnd.uniform(0, 6), rnd.uniform(1.0, 1.6), sink=0.02)
    shrubs(hx, 3)
    site(hx)


def deco_saltpan(hx):
    rnd = hx.rnd
    for (x, y) in scatter_xy(hx, 3, 0.2, seed=11, minz=-1):
        place(Ctx.meshes[f'saltc{rnd.randrange(2)}'], x, y, rnd.uniform(0, 6), rnd.uniform(1.0, 1.8))
    site(hx)


def deco_oasis(hx):
    rnd = hx.rnd
    pal = Ctx.style['pal']
    for i in range(16):
        a = i / 16 * 6.283 + rnd.uniform(-0.15, 0.15)
        d = rnd.uniform(0.6, 0.78)
        x, y = hx.center.x + math.cos(a) * d, hx.center.y + math.sin(a) * d
        if not ok(x, y):
            continue
        place(Ctx.meshes[f'palm{rnd.randrange(3)}'], x, y, rnd.uniform(0, 6), rnd.uniform(1.2, 1.8), K.jitter_col(rnd.choice(pal['palm']), rnd), sink=0.01)
    for i in range(10):
        a = rnd.uniform(0, 6.28)
        x, y = hx.center.x + math.cos(a) * 0.52, hx.center.y + math.sin(a) * 0.52
        place(Ctx.meshes['reeds'], x, y, rnd.uniform(0, 6), 1.3, sink=0.01)
    for i in range(6):
        a = rnd.uniform(0, 6.28)
        x, y = hx.center.x + math.cos(a) * 0.68, hx.center.y + math.sin(a) * 0.68
        if ok(x, y):
            place(Ctx.meshes['bush'], x, y, rnd.uniform(0, 6), rnd.uniform(1.2, 1.7), K.jitter_col(pal['oasis_green'], rnd))
    site(hx)


def deco_dcity(hx):
    rnd = hx.rnd
    me = Ctx.meshes
    pal = Ctx.style['pal']
    core = hx.site == 'citycore'
    taken = []
    if core:
        ring = 0.74
        n = 10
        for i in range(n):
            if i == 7:
                continue
            a0, a1 = i / n * 6.283, (i + 1) / n * 6.283
            p0 = V(math.cos(a0) * ring, math.sin(a0) * ring)
            p1 = V(math.cos(a1) * ring, math.sin(a1) * ring)
            m = (p0 + p1) / 2
            seg = p1 - p0
            place(me['dwall'], hx.center.x + m.x, hx.center.y + m.y, math.atan2(seg.y, seg.x), (seg.length, 1.3, 1.6), (1, 1, 1, 1), sink=0.02)
            place(me['dtower'], hx.center.x + p0.x, hx.center.y + p0.y, 0.0, 1.25, (1, 1, 1, 1), sink=0.02)
        place(me['temple'], hx.center.x + 0.05, hx.center.y + 0.12, 0.0, 1.9, (1, 1, 1, 1))
        taken.append(V(0.05, 0.12))
        n_h, rmax = 30, 0.64
    else:
        n_h, rmax = 22, 0.78
    for i in range(n_h * 3):
        if n_h <= 0:
            break
        a = rnd.uniform(0, 6.28)
        d = rnd.uniform(0.12 if not core else 0.25, rmax)
        p = V(math.cos(a) * d, math.sin(a) * d)
        if edge_dist(p, 0.97) < 0.05 or any((p - q).length < 0.1 for q in taken):
            continue
        if core and (p - V(0.05, 0.12)).length < 0.27:
            continue
        taken.append(p)
        n_h -= 1
        k = rnd.randrange(5)
        place(me[f'adobe{k}'], hx.center.x + p.x, hx.center.y + p.y, rnd.choice((0, 1.5708, 3.1416, 4.712)) + rnd.uniform(-0.1, 0.1), rnd.uniform(1.2, 1.5),
              K.jitter_col(rnd.choice(pal['adobe_tints']), rnd, 0.05), sink=0.004)
        if rnd.random() < 0.25:
            place(me['awning'], hx.center.x + p.x + 0.06, hx.center.y + p.y - 0.05, rnd.uniform(0, 3), (0.06, 0.04, 1.0), rnd.choice(pal['awnings']), sink=-0.05)
    for i in range(5 if not core else 3):
        a = rnd.uniform(0, 6.28)
        p = V(math.cos(a) * rnd.uniform(0.2, 0.6), math.sin(a) * rnd.uniform(0.2, 0.6))
        if all((p - q).length > 0.08 for q in taken):
            place(me[f'palm{rnd.randrange(3)}'], hx.center.x + p.x, hx.center.y + p.y, rnd.uniform(0, 6), rnd.uniform(1.3, 1.7), K.jitter_col(rnd.choice(pal['palm']), rnd))


def site(hx):
    s = hx.site
    if not s:
        return
    me = Ctx.meshes
    rnd = random.Random(hx.seed + 99)
    cx, cy = hx.center.x, hx.center.y
    if s == 'ruins':
        place(me['colossus'], cx, cy + 0.05, 0.3, 1.8, sink=0.03)
        place(me['obelisk_fallen'], cx + 0.32, cy - 0.22, 0, 1.4, sink=0.012, tilt=(math.pi / 2 - 0.05, 0.0))
        for i in range(5):
            h = rnd.uniform(0.35, 1.1)
            place(me['dcolumn'], cx - 0.42 + i * 0.1, cy + 0.36, 0, (1.3, 1.3, h * 1.3), sink=0.02)
        for i in range(6):
            place(me['drum'], cx + rnd.uniform(-0.45, 0.2), cy + rnd.uniform(-0.45, -0.1), rnd.uniform(0, 6), 1.3, sink=0.01)
        place(me['ruinwall'], cx - 0.35, cy - 0.15, 0.4, 1.4, sink=0.03)
    elif s == 'obelisk':
        place(me['obelisk'], cx, cy, 0.3, 1.7)
        for i in range(4):
            a = i / 4 * 6.283 + 0.4
            place(me['drum'], cx + math.cos(a) * 0.25, cy + math.sin(a) * 0.25, a, 1.3, sink=0.01)
    elif s == 'skeleton':
        place(me['skeleton'], cx, cy, rnd.uniform(0, 6.28), 1.7, sink=0.04)
    elif s == 'camp':
        for i in range(3):
            a = i / 3 * 6.283 + 0.5
            place(me['dtent'], cx + math.cos(a) * 0.3, cy + math.sin(a) * 0.3, a + 1.57, 1.7)
        for i in range(5):
            a = rnd.uniform(0, 6.28)
            place(me['camel'], cx + math.cos(a) * 0.5, cy + math.sin(a) * 0.5, rnd.uniform(0, 6), 1.0)
        place(me['fire'], cx, cy - 0.05, 0, 1.8)
        R.point(ground(cx, cy - 0.05) + V(0, 0, 0.12), 12.0 * Ctx.style.get('light_mult', 1.0), (1.0, 0.55, 0.25), 0.05)
    elif s == 'fort':
        place(me['dtower'], cx, cy, 0.0, 1.6, (1, 1, 1, 1))
        for (dx, dy, rot) in ((0, -0.25, 0), (0, 0.25, 0), (-0.25, 0, 1.5708), (0.25, 0, 1.5708)):
            place(me['dwall'], cx + dx, cy + dy, rot, (0.5, 1.2, 1.2), (1, 1, 1, 1), sink=0.02)
        for (dx, dy) in ((-0.25, -0.25), (0.25, -0.25), (0.25, 0.25), (-0.25, 0.25)):
            place(me['dtower'], cx + dx, cy + dy, 0, 0.8, (1, 1, 1, 1), sink=0.02)
    elif s == 'temple':
        place(me['temple'], cx, cy, 0.2, 2.0, (1, 1, 1, 1))
        place(me['well'], cx + 0.32, cy - 0.25, 0, 1.6)
    elif s == 'well':
        place(me['well'], cx, cy, 0, 1.8)


CUSTOM = {'dune': deco_dune, 'sandflat': deco_sandflat, 'canyon': deco_canyon, 'rockland': deco_rockland, 'saltpan': deco_saltpan,
          'oasis': deco_oasis, 'dcity': deco_dcity}
B.CUSTOM.update(CUSTOM)


def caravan(path, camels=6, wagons=2, spacing=0.13, ruts=True, offset=0.0):
    pts = [p.xy for p in catmull([V(x, y) for (x, y) in path], 30)]
    acc, samples = 0.0, [(pts[0], (pts[1] - pts[0]).normalized())]
    for a, b in zip(pts, pts[1:]):
        L = (b - a).length
        acc += L
        samples.append((b, (b - a).normalized() if L > 1e-6 else samples[-1][1]))
    me = Ctx.meshes
    if ruts:
        for off in (-0.018, 0.018):
            bm = bmesh.new()
            left, right = [], []
            for (p, t) in samples[::2]:
                n = Vector((-t.y, t.x))
                for lst, o in ((left, off - 0.004), (right, off + 0.004)):
                    q = p + n * o
                    g = ground(q.x, q.y)
                    lst.append(bm.verts.new((q.x, q.y, g.z + 0.003)))
            for i in range(len(left) - 1):
                bm.faces.new((left[i], right[i], right[i + 1], left[i + 1]))
            add(to_mesh(bm, 'rut', [Ctx.mats['rut']]), (0, 0, 0))
    order = ['wagon'] * wagons + ['camel'] * camels
    random.Random(3).shuffle(order)
    d, k = offset, 0
    total, i = 0.0, 0
    for idx in range(len(samples) - 1):
        a, b = samples[idx][0], samples[idx + 1][0]
        seg = (b - a).length
        while k < len(order) and total + seg >= d:
            t = (d - total) / seg if seg > 0 else 0
            p = a.lerp(b, t)
            tan = samples[idx][1]
            rot = math.atan2(tan.y, tan.x)
            place(me[order[k]], p.x, p.y, rot, 1.0 if order[k] == 'camel' else 1.4)
            d += spacing
            k += 1
        total += seg


def footprints(path, spacing=0.035):
    pts = [p.xy for p in catmull([V(x, y) for (x, y) in path], 40)]
    bm = bmesh.new()
    d, total, side = 0.0, 0.0, 1
    for a, b in zip(pts, pts[1:]):
        seg = (b - a).length
        while total + seg >= d and seg > 0:
            t = (d - total) / seg
            p = a.lerp(b, t)
            tan = (b - a).normalized()
            n = Vector((-tan.y, tan.x))
            q = p + n * 0.008 * side
            g = ground(q.x, q.y)
            ret = bmesh.ops.create_circle(bm, cap_ends=True, segments=8, radius=0.006,
                                          matrix=Matrix.Translation((q.x, q.y, g.z + 0.0015)) @ Matrix.Rotation(math.atan2(tan.y, tan.x), 4, 'Z') @ Matrix.Diagonal((1.6, 1.0, 1.0, 1.0)))
            side = -side
            d += spacing
        total += seg
    add(to_mesh(bm, 'steps', [Ctx.mats['rut']]), (0, 0, 0))


def crest_point(x, y, r=0.5, n=17):
    best, bz = (x, y), -9
    for i in range(n):
        for j in range(n):
            px, py = x + (i / (n - 1) - 0.5) * 2 * r, y + (j / (n - 1) - 0.5) * 2 * r
            z = ground(px, py).z
            if z > bz:
                best, bz = (px, py), z
    return best


def build_desert(cells, rivers=(), hero=None, enemies=(), warlord=None, floor_z=None, floor_mat=None):
    S = Ctx.style
    Ctx.rivers = [catmull([V(x, y) for (x, y) in r]) for r in rivers] if S.get('rivers', True) else []
    for (q, r), (t, st, en) in cells.items():
        hx = B.make_hex(q, r, t, st, en)
        hx.dz = True
        Ctx.hexes[(q, r)] = hx
    for hx in Ctx.hexes.values():
        for poly in Ctx.rivers:
            for a, b in zip(poly, poly[1:]):
                if seg_dist(Vector((hx.center.x, hx.center.y)), a, b) < 1.3:
                    hx.segs.append((a, b))
    for hx in Ctx.hexes.values():
        K.tile_mesh(hx)
        B.decorate(hx)
    ts = S.get('token_scale', 1.5)
    if hero:
        hx, (x, y) = hero
        g = ground(x, y)
        add(Ctx.meshes['hero'], g - V(0, 0, 0.01), 0.4, ts)
        Ctx.hero = g
    for (x, y) in enemies:
        place(Ctx.meshes['enemy'], x, y, random.uniform(-0.3, 0.3), ts, sink=0.01)
    if warlord:
        place(Ctx.meshes['warlord'], warlord[0], warlord[1], 0.0, ts * 1.1, sink=0.01)
    if floor_mat is not None:
        R.plane((0, 5, floor_z), 90, floor_mat)
