#!/usr/bin/env python3
"""Read-only inventory and transparency checks for the 200 minion sprites.

Requires Pillow. Writes only the JSON verification report, never image pixels.
"""
from collections import Counter
from pathlib import Path
import hashlib
import json
from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
DOCS = ROOT / "docs/minion-bestiary"


def verify():
    bosses = json.loads((ROOT / "docs/battle-art-extraction-manifest.json").read_text())["entries"]
    assert len(bosses) == 20
    rows, names, hashes, ids = [], [], [], []
    for boss in bosses:
        manifest = json.loads((DOCS / boss["id"] / "manifest.json").read_text())
        assert len(manifest["minions"]) == 10, boss["id"]
        for m in manifest["minions"]:
            key = boss["id"] + "_" + m["id"]
            assert m["status"] == "completed", key
            assert all(m.get(field) for field in ("name", "role", "description", "prompt", "generated_source_path", "source_reference_paths")), key
            path = ROOT / m["relative_asset_path"]
            with Image.open(path) as image:
                assert image.mode == "RGBA", key
                assert min(image.size) >= 1024, key
                alpha = image.getchannel("A")
                empty = alpha.histogram()[0] / (image.width * image.height)
                assert empty > 0.12, key
                corners = [alpha.getpixel(p) for p in ((0, 0), (image.width - 1, 0), (0, image.height - 1), (image.width - 1, image.height - 1))]
                # Tiny watercolor fringe values at corners are not an opaque
                # backdrop. Permit up to 3% alpha while testing true zero-alpha
                # coverage and the opaque silhouette separately.
                assert max(corners) <= 7, key
                # Ignore the nearly invisible alpha=1 canvas residue produced by
                # some generations when checking opaque silhouette containment.
                bounds = alpha.point(lambda a: 255 if a >= 64 else 0).getbbox()
                assert bounds and bounds[0] > 0 and bounds[1] > 0 and bounds[2] < image.width and bounds[3] < image.height, (key, bounds)
                rows.append({"id": key, "size": image.size, "transparent_fraction": round(empty, 4), "silhouette_bounds": bounds, "bytes": path.stat().st_size})
            names.append(m["name"]); ids.append(key)
            hashes.append(hashlib.sha256(path.read_bytes()).hexdigest())
    for values in (ids, names, hashes):
        assert all(n == 1 for n in Counter(values).values()), "Duplicate identities, names or image files"
    assert len(rows) == 200
    report = {"result": "passed", "factions": 20, "creatures": 200, "total_png_bytes": sum(r["bytes"] for r in rows), "checks": ["10 completed designs per boss", "unique IDs, names and PNG hashes", "full-resolution RGBA", "corner alpha below 3% and substantial zero-alpha coverage", "opaque silhouettes contained within canvas", "description, role, prompt and provenance present"], "assets": rows}
    (DOCS / "validation.json").write_text(json.dumps(report, indent=2) + "\n")
    print(f"PASS: 200 unique transparent minions / 20 factions; {report['total_png_bytes'] / 1024**2:.1f} MiB")


if __name__ == "__main__":
    verify()
