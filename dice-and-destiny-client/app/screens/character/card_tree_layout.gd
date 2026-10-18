extends RefCounted
## Card tree layout by XP. Every distinct XP value is its own row, higher XP
## higher up, with the base's row at y = 0. Every tree, in every screen, is
## laid out this way. Editors may drag a card sideways; the tree then keeps it
## there ("pinned": true and its x), and its row still follows its XP.
##
## Columns are a layered drawing: a path that skips rows gets a lane in each
## row it crosses, kept clear of that row's cards, and the path bends through
## its lanes, so no path runs over a card it doesn't join. Rows are ordered to
## reduce crossings, then each card moves as close to the cards it joins as the
## spacing allows.
const DIFF = preload("res://app/screens/character/card_tree_diff.gd")
const ROW := 320.0
const GAP := 330.0
## Spacing beside a path's placeholder, and between two placeholders.
const PATH_GAP := 170.0
const PATHS_GAP := 70.0

## Node ID -> world position (the medallion's centre).
static func positions(tree: Dictionary) -> Dictionary:
	return arrange(tree).at

## {"at": node ID -> position, "bends": edge ID -> the points (from the
## edge's `from` card to its `to` card) its path bends through}.
static func arrange(tree: Dictionary) -> Dictionary:
	var nodes: Array = tree.get("nodes", [])
	var out := {}
	var bends := {}
	if nodes.is_empty(): return {"at": out, "bends": bends}
	var xp := {}
	var values: Array = []
	for n in nodes:
		var value := DIFF.node_xp(n)
		xp[str(n.id)] = value
		if not values.has(value): values.append(value)
	values.sort()
	var root := str(tree.get("root", ""))
	var base_index: int = values.find(xp[root]) if xp.has(root) else 0
	var row := {}
	for id in xp: row[id] = values.find(xp[id]) - base_index
	var start := _branch_x(tree, xp, root)
	# Layered graph: cards plus one placeholder per row a path skips.
	var rows := {}
	var x := {}
	var links := {}
	for id in xp:
		if not rows.has(row[id]): rows[row[id]] = []
		rows[row[id]].append(id); x[id] = float(start[id]); links[id] = []
	var chain := {}
	var k := 0
	for e in tree.get("edges", []):
		var a := str(e.from); var b := str(e.to)
		if not xp.has(a) or not xp.has(b) or row[a] == row[b]: continue
		var flipped: bool = row[a] > row[b]
		if flipped:
			var t := a
			a = b; b = t
		var lanes: Array = []
		var prev := a
		for r in range(row[a] + 1, row[b]):
			var key := "~%d_%d" % [k, r]
			x[key] = lerpf(x[a], x[b], float(r - row[a]) / float(row[b] - row[a])); links[key] = []
			chain[key] = [a, b, float(r - row[a]) / float(row[b] - row[a])]
			if not rows.has(r): rows[r] = []
			rows[r].append(key)
			links[prev].append(key); links[key].append(prev); prev = key; lanes.append(key)
		links[prev].append(b); links[b].append(prev); k += 1
		if flipped: lanes.reverse()
		if not lanes.is_empty(): bends[str(e.id)] = lanes
	var order: Array = rows.keys(); order.sort()
	for r in order: rows[r].sort_custom(func(p, q): return x[p] < x[q] or (x[p] == x[q] and p < q))
	_order_rows(rows, order, links)
	_place_rows(rows, order, x, links, chain)
	var shift := -float(x[root]) if x.has(root) else 0.0
	var by_id := {}
	for n in nodes: by_id[str(n.id)] = n
	for id in xp:
		var n: Dictionary = by_id[id]
		var px: float = float(n.get("x", 0.0)) if bool(n.get("pinned", false)) else snappedf(x[id] + shift, 20.0)
		out[id] = Vector2(px, -float(row[id]) * ROW)
	for edge_id in bends:
		bends[edge_id] = bends[edge_id].map(func(key): return Vector2(x[key] + shift, -float(int(key.get_slice("_", 1))) * ROW))
	return {"at": out, "bends": bends}

## Barycentre sweeps, up and down, keeping the order with the fewest crossings.
static func _order_rows(rows: Dictionary, order: Array, links: Dictionary) -> void:
	var best := _copy(rows); var fewest := _crossings(rows, order, links)
	for sweep in 12:
		var seq: Array = order.duplicate()
		if sweep % 2 == 1: seq.reverse()
		for i in range(1, seq.size()):
			var r: int = seq[i]; var ref: int = seq[i - 1]
			var at := {}
			for j in rows[ref].size(): at[rows[ref][j]] = j
			var bary := {}
			for j in rows[r].size():
				var key = rows[r][j]
				var total := 0.0; var count := 0
				for other in links[key]:
					if at.has(other): total += at[other]; count += 1
				bary[key] = total / count if count > 0 else float(j) * float(rows[ref].size()) / maxf(1.0, rows[r].size())
			var index := {}
			for j in rows[r].size(): index[rows[r][j]] = j
			rows[r].sort_custom(func(p, q): return bary[p] < bary[q] or (bary[p] == bary[q] and index[p] < index[q]))
		var c := _crossings(rows, order, links)
		if c < fewest: fewest = c; best = _copy(rows)
	for r in best: rows[r] = best[r]

static func _crossings(rows: Dictionary, order: Array, links: Dictionary) -> int:
	var total := 0
	for i in range(order.size() - 1):
		var lower: Array = rows[order[i]]; var upper: Array = rows[order[i + 1]]
		var at := {}
		for j in upper.size(): at[upper[j]] = j
		var pairs: Array = []
		for j in lower.size():
			for other in links[lower[j]]:
				if at.has(other): pairs.append(Vector2i(j, at[other]))
		for p in pairs.size():
			for q in range(p + 1, pairs.size()):
				var a: Vector2i = pairs[p]; var b: Vector2i = pairs[q]
				if (a.x - b.x) * (a.y - b.y) < 0: total += 1
	return total

static func _copy(rows: Dictionary) -> Dictionary:
	var out := {}
	for r in rows: out[r] = rows[r].duplicate()
	return out

## Each card moves toward the mean x of what it joins, keeping its row's order
## and spacing. A placeholder holds to the straight line between its path's two
## cards, so cards are pushed off the path as it is drawn. Repeated until the
## drawing settles.
static func _place_rows(rows: Dictionary, order: Array, x: Dictionary, links: Dictionary, chain: Dictionary) -> void:
	for r in order: _pack(rows[r], x, x, chain)
	for sweep in 60:
		var seq: Array = order.duplicate()
		if sweep % 2 == 1: seq.reverse()
		for r in seq:
			var wanted := {}
			for key in rows[r]:
				if chain.has(key):
					var c: Array = chain[key]; wanted[key] = lerpf(x[c[0]], x[c[1]], c[2]); continue
				var total := 0.0; var count := 0
				for other in links[key]: total += x[other]; count += 1
				wanted[key] = total / count if count > 0 else x[key]
			_pack(rows[r], wanted, x, chain)

## Closest positions to `wanted` that keep the order and the gaps (weighted
## pool adjacent violators; placeholders weigh most), written to x.
static func _pack(keys: Array, wanted: Dictionary, x: Dictionary, chain: Dictionary) -> void:
	var offset: Array = [0.0]
	for i in range(1, keys.size()): offset.append(offset[i - 1] + _gap(keys[i - 1], keys[i]))
	var blocks: Array = []
	for i in keys.size():
		var w := 20.0 if chain.has(keys[i]) else 1.0
		blocks.append({"sum": (float(wanted[keys[i]]) - offset[i]) * w, "w": w, "n": 1})
		while blocks.size() > 1 and blocks[-1].sum / blocks[-1].w < blocks[-2].sum / blocks[-2].w:
			var top: Dictionary = blocks.pop_back()
			blocks[-1].sum += top.sum; blocks[-1].w += top.w; blocks[-1].n += top.n
	var i := 0
	for b in blocks:
		var y: float = b.sum / b.w
		for j in b.n: x[keys[i]] = y + offset[i]; i += 1

static func _gap(a: String, b: String) -> float:
	var pa := a.begins_with("~"); var pb := b.begins_with("~")
	if pa and pb: return PATHS_GAP
	if pa or pb: return PATH_GAP
	return GAP

## Starting columns: each card's steps from the base form layers (up for
## upgrades, down for cheaper cards), and each layer's cards sit under the
## cards they come from, GAP apart.
static func _branch_x(tree: Dictionary, xp: Dictionary, root: String) -> Dictionary:
	var indegree := {}
	for id in xp: indegree[id] = 0
	for e in tree.get("edges", []):
		if indegree.has(str(e.to)): indegree[str(e.to)] += 1
	var queue: Array = [root] if xp.has(root) else []
	for id in xp:
		if id != root and indegree[id] == 0: queue.append(id)
	var order: Array = []
	while not queue.is_empty():
		var id: String = queue.pop_front(); order.append(id)
		for e in tree.get("edges", []):
			if str(e.from) != id or not indegree.has(str(e.to)): continue
			indegree[str(e.to)] -= 1
			if indegree[str(e.to)] == 0: queue.append(str(e.to))
	for id in xp:
		if not order.has(id): order.append(id)
	var layer := {}
	for id in order:
		if not layer.has(id): layer[id] = 0
		for e in tree.get("edges", []):
			if str(e.from) != id or not xp.has(str(e.to)): continue
			var l: int = layer[id] + (1 if xp[str(e.to)] >= xp[id] else -1)
			if not layer.has(str(e.to)) or absi(l) > absi(layer[str(e.to)]): layer[str(e.to)] = l
	var layers := {}
	for id in order:
		if not layers.has(layer[id]): layers[layer[id]] = []
		layers[layer[id]].append(id)
	var above: Array = layers.keys().filter(func(l): return l > 0); above.sort()
	var below: Array = layers.keys().filter(func(l): return l < 0); below.sort(); below.reverse()
	var x := {}
	for l in ([0] if layers.has(0) else []) + above + below:
		var ids: Array = layers[l]
		var desired := {}
		for id in ids: desired[id] = 0.0 if id == root else _desired_x(tree, id, x)
		ids.sort_custom(func(a, b): return desired[a] < desired[b] or (desired[a] == desired[b] and a < b))
		var placed := _spread(ids, desired)
		var shift := 0.0
		if ids.has(root): shift = -placed[ids.find(root)]
		else:
			for i in ids.size(): shift += desired[ids[i]] - placed[i]
			shift /= ids.size()
		for i in ids.size(): x[ids[i]] = placed[i] + shift
	return x

## Positions for sorted IDs: each at its wanted x, but at least GAP right of the last.
static func _spread(ids: Array, wanted: Dictionary) -> Array:
	var placed: Array = []
	for i in ids.size():
		var px: float = wanted[ids[i]]
		if i > 0: px = maxf(px, placed[i - 1] + GAP)
		placed.append(px)
	return placed

## The mean x of the already placed cards this one comes from, else of any
## placed card it connects to, else 0.
static func _desired_x(tree: Dictionary, id: String, x: Dictionary) -> float:
	for parents_only in [true, false]:
		var total := 0.0; var count := 0
		for e in tree.get("edges", []):
			var other := ""
			if str(e.to) == id: other = str(e.from)
			elif str(e.from) == id and not parents_only: other = str(e.to)
			if other != "" and x.has(other): total += float(x[other]); count += 1
		if count > 0: return total / count
	return 0.0
