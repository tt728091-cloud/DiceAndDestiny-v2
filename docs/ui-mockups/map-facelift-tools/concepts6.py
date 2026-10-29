"""Set 6: twelve ways to keep hexes behind mountains readable. Usage:
blender -b --factory-startup -P concepts6.py -- <01-12> <out.png> [preview]
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
# Set 6: dealing with mountains that hide hexes. One scenario, twelve fixes.
import json
from hexkit import smoothstep, lerp, hex_world, SQ3

W, H = 1920, 1080
JSON_ONLY = len(argv) > 2 and argv[2] == 'json'


def pass_x(y):
    return -3.0 + (y - 2.0) * 1.0


def pass_dist(x, y):
    p0 = Vector((-3.0, 2.0))
    u = Vector((1.0, 1.0)).normalized()
    d = Vector((x, y)) - p0
    return abs(d.x * u.y - d.y * u.x)


def terrain(authored):
    def tf(w, rnd):
        x, y = w.x, w.y
        if y < 1.7:
            v = noise.noise(Vector((x * 0.3, y * 0.3, 1.0)))
            return 'lake' if v > 0.42 else 'forest' if v > 0.1 else 'hills' if v > -0.08 else 'plains'
        if y < 8.6:
            if abs(x - pass_x(y)) < 1.35:
                return 'hills'
            dy = y - 2.0
            if dy < 0.9 or dy > 6.0:
                return 'mountain'
            return 'peaks'
        if y < 14.3:
            v = noise.noise(Vector((x * 0.28, y * 0.28, 4.0)))
            return 'forest' if v > 0.12 else 'lake' if v < -0.42 else 'plains'
        return 'peaks'
    return tf


def deco_peaks(hx):
    S = Ctx.style
    rnd = hx.rnd
    lean = S.get('lean', 0.0)
    peaks = [(0.0, 0.08, 1.75, 0.66), (-0.38, -0.15, 1.05, 0.44), (0.36, -0.22, 0.95, 0.42), (0.25, 0.42, 0.85, 0.38)]
    for (x, y, h, r) in peaks[: 3 + (hx.seed % 2)]:
        hs = 1.0
        if S.get('saddle'):
            hs = 0.32 + 0.68 * smoothstep(0.9, 3.4, pass_dist(hx.center.x + x, hx.center.y + y - lean))
        R.mountain_real(hx, x, y - lean, h * rnd.uniform(0.85, 1.12) * S.get('peak_scale', 1.0) * hs, r)
    K.rocks(hx, 6, [], scale=1.3)


B.CUSTOM['peaks'] = deco_peaks


def axial(x, y):
    import desert as D
    return D.axial_round(x, y)


def hex_dist(a, b):
    dq, dr = a[0] - b[0], a[1] - b[1]
    return (abs(dq) + abs(dr) + abs(dq + dr)) // 2


WALK = lambda t: t not in ('peaks', 'lake')

POS = dict(hero=(-2.8, 0.6), foes=(2.6, 9.4), warlord=(0.5, 12.6), keep=(-2.2, 11.2), shrine=(5.6, 11.6), village=(2.4, 0.2), hover=(0.3, 5.3))


def build_scene(authored=False):
    pal = nature()
    pal['roofs'] = [lin('#8a7448'), lin('#7a6440'), lin('#6e5a3a'), lin('#94804e')]
    g = dict(GRASS)
    opts = dict(grass=g, gap=0.0, thickness=0.6, bevel=0.0, peak_h=0.75 if authored else 1.25, window_glow=5.0, roof='thatch',
                overlays={'ao': (0.06, 0.5, (0.25, 0.25, 0.3, 1))}, ring_k=2.4, ring_col='#ffd27a', res=26,
                lean=0.32 if authored else 0.0, saddle=authored, peak_scale=0.9 if authored else 1.0)
    setup(pal, **opts)
    cells = B.gen_cells((-11.0, 11.0), (-3.5, 17.0), terrain(authored), 1)
    k = lambda key, allowed=None: B.nearest(cells, *POS[key], allowed)
    hero = k('hero', {'plains', 'hills', 'forest'})
    foes = k('foes', {'plains', 'forest'})
    war = k('warlord', {'plains', 'forest'})
    for key, site in (('keep', 'keep'), ('shrine', 'shrine'), ('village', 'village')):
        kk = k(key, {'plains', 'forest', 'hills'})
        cells[kk][0], cells[kk][1] = 'plains', site
    B.build_map(cells, [], hero, [foes], war, sea=True, sea_z=-0.33)
    R.apply_overlays()
    Ctx.keys = dict(hero=hero, foes=foes, warlord=war, hover=B.nearest(cells, *POS['hover'], {'hills'}))
    return cells


def tokens(kind):
    pref = {'foe': ('enemy', 'warlord'), 'hero': ('hero',)}[kind]
    return [o for o in bpy.context.scene.objects if o.type == 'MESH' and o.data and o.data.name.split('.')[0] in pref]


def mountains():
    return [o for o in bpy.context.scene.objects if o.name.startswith('mountain')]


def path_points():
    h = Ctx.hero
    return [Vector((h.x, h.y)), Vector((-2.4, 1.9)), Vector((-1.2, 3.3)), Vector((0.3, 5.0)), Vector((1.8, 6.8)), Vector((3.1, 8.4)), Vector((3.3, 9.4))]


def reach_rings():
    hk = Ctx.keys['hero']
    for key, hx in Ctx.hexes.items():
        if key != hk and hex_dist(key, hk) <= 2 and WALK(hx.terrain):
            K.hex_ring(hx, hx.center, 0.82, 0.88, Ctx.mats['ring_move'], z=0.02)


def lay_path():
    pts = [Vector((p.x, p.y, 0)) for p in path_points()]
    K.path_dots(pts, Ctx.mats['ring_path'], spacing=0.12, r=0.03, lift=0.04)


def ground(x, y):
    hx = K.nearest_hex(Vector((x, y)))
    return Vector((x, y, hx.top + hx.height(x, y)))


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


def visible_fraction(hx, n_ring=6):
    sc = bpy.context.scene
    dg = bpy.context.evaluated_depsgraph_get()
    o = sc.camera.matrix_world.translation.copy()
    pts = [hx.ground(0, 0)] + [hx.ground(math.cos(a) * 0.5, math.sin(a) * 0.5) for a in [math.radians(30 + 60 * i) for i in range(n_ring)]]
    vis = 0
    for p in pts:
        p = p + Vector((0, 0, 0.04))
        d = p - o
        L = d.length
        ok, loc, *_ = sc.ray_cast(dg, o, d.normalized(), distance=L - 0.02)
        if not ok:
            vis += 1
    return vis / len(pts)


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
    rim_k = K._maprange(nt, lw.outputs['Facing'], 0.45, 1.0, 0.0, 3.2)
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


def cutout(center_px, radius, target_dist):
    """Screen-space see-through circle: geometry nearer than the target, inside the circle, is cut away."""
    cx, cy = center_px[0] / W, 1.0 - center_px[1] / H
    for mt in {mt for ob in mountains() for mt in ob.data.materials}:
        nt = mt.node_tree
        out = nt.nodes['Material Output']
        src = out.inputs['Surface'].links[0].from_socket
        tc = nt.nodes.new('ShaderNodeTexCoord')
        sep = nt.nodes.new('ShaderNodeSeparateXYZ')
        nt.links.new(tc.outputs['Window'], sep.inputs['Vector'])
        dx = K._math if hasattr(K, '_math') else None
        import desert as D
        mx_ = D._math(nt, 'MULTIPLY', D._math(nt, 'SUBTRACT', sep.outputs['X'], cx), W / H)
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


def banner(base, color, emblem, top_z=2.75, name='banner', s=1.0):
    bm = bmesh.new()
    K.bm_cone(bm, 8, 0.012, 0.012, top_z - base.z, 0.0, 0)
    K.bm_ico(bm, 0.03, (0, 0, top_z - base.z + 0.02), 2, sub=2)
    z1 = top_z - base.z - 0.04
    vs = [bm.verts.new(c) for c in [(0.012, 0, z1), (0.46 * s, 0, z1), (0.46 * s, 0, z1 - 0.5 * s), (0.24 * s, 0, z1 - 0.38 * s), (0.012, 0, z1 - 0.5 * s)]]
    bm.faces.new(vs).material_index = 1
    ret = bmesh.ops.create_circle(bm, cap_ends=True, segments=24, radius=0.1 * s, matrix=Matrix.Translation((0.236 * s, -0.004, z1 - 0.19 * s)) @ Matrix.Rotation(math.pi / 2, 4, 'X'))
    for f in K.faces_of(ret['verts']):
        f.material_index = 3
    pole = K.m_flat('pole', lin('#2a2420'), 0.6)
    flag = K.m_flat('flag_' + name, color, 0.7, emit=color, emit_k=0.6)
    gold = K.m_flat('finial', lin('#c9a35a'), 0.3, metal=1.0)
    em = K.m_flat('emblem_' + name, emblem, 0.5, emit=emblem, emit_k=1.2)
    ob = K.add(K.to_mesh(bm, 'banner_' + name, [pole, flag, gold, em]), base, 0.0)
    return ob, Vector((base.x + 0.24, base.y, top_z))


def light_and_fog(fog=1.2):
    K.fog_box((0, 6, 0.0), (44, 34, 0.8), fog, lin('#e8dcea'), 0.0, clumps=0.7, fade_z=(-0.05, 0.3))
    K.fog_box((0, 8, 1.2), (60, 40, 4.0), 0.022, lin('#f0c8c0'), 0.5)
    R.sky_world(2, 260, strength=0.55, camera_bg=[lin('#f2c2a8'), lin('#6a7aa8')])
    K.sun(95, 9, 4.8, (1.0, 0.64, 0.46), 1.0)


CINE = ((-0.4, 4.4, 0.4), -10, 17, 12.5, 36)
PLAY = ((-0.3, 5.4, 0.3), -6, 55, 15.5, 36)
MID = ((-0.3, 5.0, 0.3), -8, 38, 13.5, 36)
ROT = ((0.6, 5.6, 0.4), -60, 24, 12.5, 36)
TACT = ((-1.0, 5.4, 0.0), 0, 80, 26.0, 36)


def cam(c):
    ob = K.camera(c[0], c[1], c[2], c[3], lens=c[4])
    sc = bpy.context.scene
    sc.render.resolution_x, sc.render.resolution_y, sc.render.resolution_percentage = W, H, 100
    bpy.context.view_layer.update()
    return ob


def dump(extra=None):
    data = {'hexes': [], 'res': [W, H]}
    for (q, r), hx in Ctx.hexes.items():
        c = hx.ground(0, 0)
        cp = K.project(c)
        if not (-200 < cp[0] < W + 200 and -200 < cp[1] < H + 200):
            continue
        corners = [K.project(hx.ground(math.cos(math.radians(30 + 60 * i)), math.sin(math.radians(30 + 60 * i)))) for i in range(6)]
        data['hexes'].append({'key': [q, r], 'terrain': hx.terrain, 'site': hx.site, 'c': cp, 'corners': corners})
    if extra:
        data.update(extra)
    with open(OUT.replace('.png', '.json'), 'w') as f:
        json.dump(data, f)


def render_beauty(samples=160):
    if JSON_ONLY:
        return
    K.render(OUT, preview=PREVIEW, look='AgX - Medium High Contrast', exposure=0.15, samples=samples)


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


# ---------------------------------------------------------------------------
# The twelve pictures

def s01():
    build_scene()
    reach_rings()
    lay_path()
    light_and_fog()
    cam(CINE)
    dump()
    render_beauty()


def s02():
    build_scene()
    reach_rings()
    lay_path()
    light_and_fog(0.9)
    cam(PLAY)
    dump()
    render_beauty()


def targets():
    pts = [Vector((p.x, p.y, 0)) for p in path_points()]
    pts = [ground(p.x, p.y) + Vector((0, 0, 0.05)) for p in pts]
    for key in ('foes', 'warlord', 'hover'):
        hx = Ctx.hexes[Ctx.keys[key]]
        pts.append(hx.ground(0, 0) + Vector((0, 0, 0.15)))
    extra = []
    for p in pts[:-1]:
        for t in (0.25, 0.5, 0.75):
            pass
    hk = Ctx.keys['hero']
    for key, hx in Ctx.hexes.items():
        if hex_dist(key, Ctx.keys['hover']) <= 1 and WALK(hx.terrain):
            extra.append(hx.ground(0, 0) + Vector((0, 0, 0.05)))
    return pts + extra


def s03():
    build_scene()
    reach_rings()
    lay_path()
    cam(CINE)
    occ = occluders(targets())
    ghost(occ, 0.34)
    light_and_fog()
    dump({'faded': len(occ)})
    render_beauty()


def s04():
    build_scene()
    reach_rings()
    lay_path()
    cam(CINE)
    occ = occluders(targets())
    for ob in occ:
        ob.scale.z *= 0.18
    light_and_fog()
    dump({'sunk': len(occ)})
    render_beauty()


def s05():
    build_scene()
    reach_rings()
    lay_path()
    light_and_fog()
    cam(CINE)
    dump()
    render_beauty()
    render_masks({'foe': tokens('foe')})


def s06():
    build_scene()
    reach_rings()
    lay_path()
    light_and_fog()
    cam(CINE)
    tops = {}
    specs = [('foes', lin('#8c2f2a'), lin('#1d1a1c'), 'Eel Hunters'), ('warlord', lin('#1d1a1c'), lin('#56f0c8'), 'The Drowned Regent'),
             ('hero', lin('#2f4f86'), lin('#e6c17c'), 'You')]
    for key, col, emb, label in specs:
        hx = Ctx.hexes[Ctx.keys[key]]
        base = hx.ground(0.32, 0.05)
        ob, top = banner(base, col, emb, top_z=2.75 if key != 'hero' else base.z + 0.7, name=key, s=1.0 if key != 'hero' else 0.5)
        tops[label] = K.project(top + Vector((0.0, 0.0, 0.08)))
    for site, col, emb, label in (('keep', lin('#c9a35a'), lin('#211e19'), 'Brinehold Keep'), ('shrine', lin('#6e5a3f'), lin('#efe3c6'), 'Wayside Shrine')):
        for hx in Ctx.hexes.values():
            if hx.site == site:
                base = hx.ground(0.42, -0.1)
                ob, top = banner(base, col, emb, top_z=2.55, name=site)
                tops[label] = K.project(top + Vector((0.0, 0.0, 0.08)))
    dump({'banners': tops})
    render_beauty()


def s07():
    build_scene()
    reach_rings()
    lay_path()
    cam(CINE)
    hx = Ctx.hexes[Ctx.keys['hover']]
    c = hx.ground(0, 0)
    cpx = K.project(c)
    tdist = (c - bpy.context.scene.camera.matrix_world.translation).length
    cutout(cpx, 0.2, tdist)
    K.hex_ring(hx, hx.center, 0.78, 0.9, K.m_emit('hover', lin('#fff1c9'), 6.0), z=0.03)
    light_and_fog()
    corners = [K.project(hx.ground(math.cos(math.radians(30 + 60 * i)), math.sin(math.radians(30 + 60 * i)))) for i in range(6)]
    dump({'cursor': cpx, 'radius_px': 0.2 * H, 'hover_corners': corners})
    render_beauty()


def s08():
    build_scene()
    cam(TACT)
    for o in bpy.context.scene.objects:
        if o.type != 'MESH':
            continue
        nm = o.data.name.split('.')[0]
        if o.name.startswith('mountain'):
            o.hide_render = True
        elif nm.startswith(('conifer', 'broad', 'dead', 'bush', 'rock', 'reeds')):
            o.scale.z *= 0.12
        elif o.name.startswith('tile_'):
            o.scale.z *= 0.2
    light_and_fog(0.0)
    pp = [K.project(ground(p.x, p.y)) for p in path_points()]
    pieces = {k: K.project(Ctx.hexes[Ctx.keys[k]].ground(0, 0)) for k in ('foes', 'warlord')}
    dump({'path': pp, 'hero': K.project(Ctx.hero), 'pieces': pieces})
    render_beauty(128)


def s09():
    build_scene(authored=True)
    reach_rings()
    lay_path()
    light_and_fog()
    cam(CINE)
    dump()
    render_beauty()


def s10():
    build_scene()
    reach_rings()
    lay_path()
    light_and_fog()
    cam(ROT)
    dump()
    render_beauty()


def s11():
    build_scene()
    light_and_fog()
    cam(CINE)
    bpy.context.view_layer.update()
    status = {}
    vols = [o for o in bpy.context.scene.objects if o.name.startswith('fog')]
    for o in vols:
        o.hide_viewport = True
    bpy.context.view_layer.update()
    for key, hx in Ctx.hexes.items():
        if WALK(hx.terrain):
            cp = K.project(hx.ground(0, 0))
            if 0 < cp[0] < W and 0 < cp[1] < H:
                status[f'{key[0]},{key[1]}'] = visible_fraction(hx)
    for o in vols:
        o.hide_viewport = False
    dump({'status': status})
    render_beauty()


def s12():
    build_scene()
    reach_rings()
    lay_path()
    cam(MID)
    occ = occluders(targets())
    ghost(occ, 0.34)
    tops = {}
    for key, col, emb, label in (('foes', lin('#8c2f2a'), lin('#1d1a1c'), 'Eel Hunters'), ('warlord', lin('#1d1a1c'), lin('#56f0c8'), 'The Drowned Regent')):
        hx = Ctx.hexes[Ctx.keys[key]]
        base = hx.ground(0.32, 0.05)
        ob, top = banner(base, col, emb, top_z=2.1, name=key)
        tops[label] = K.project(top + Vector((0.0, 0.0, 0.08)))
    hx = Ctx.hexes[Ctx.keys['hover']]
    K.hex_ring(hx, hx.center, 0.78, 0.9, K.m_emit('hover', lin('#fff1c9'), 6.0), z=0.03)
    light_and_fog()
    corners = [K.project(hx.ground(math.cos(math.radians(30 + 60 * i)), math.sin(math.radians(30 + 60 * i)))) for i in range(6)]
    dump({'banners': tops, 'hover_corners': corners, 'cursor': K.project(hx.ground(0, 0))})
    render_beauty()
    render_masks({'foe': tokens('foe')})


SCENES = {f'{i:02d}': globals()[f's{i:02d}'] for i in range(1, 13)}
K.reset()
SCENES[CID]()
