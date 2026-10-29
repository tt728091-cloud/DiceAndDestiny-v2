import itertools
from eight import *
keys = [(7, 'none'), (12, 'none'), (7, 'holdfast'), (12, 'holdfast')]
print('flat 40: every XP pool where all four builds exist; worst matchup and average score of each build (smoothed +-2 cards)')
best = None
for B in range(380, 1001, 20):
    bs = {k: build(k[0], k[1], B)[0] for k in keys}
    s = {(r, c): score(bs[r], bs[c]) for r, c in itertools.permutations(keys, 2)}
    worst = min(s.values())
    avg = {r: np.mean([s[(r, c)] for c in keys if c != r]) for r in keys}
    if B % 60 == 20 or B == 380:
        print(f'  +{B}: worst {worst:.2f} | ' + '  '.join(f'{NAMES[r]} {v:.2f}' for r, v in avg.items()))
    if best is None or worst > best[0]: best = (worst, B)
print(f'most even pool: +{best[1]} (worst matchup {best[0]:.2f})')
print('\nEach single swap at flat 40: the XP pool where the two builds are even (smoothed)')
for a, b in [((7, 'none'), (12, 'none')), ((7, 'holdfast'), (12, 'holdfast')), ((7, 'none'), (7, 'holdfast')), ((12, 'none'), (12, 'holdfast'))]:
    prev = None; found = None
    for B in range(150, 1500, 5):
        A, Bb = build(a[0], a[1], B)[0], build(b[0], b[1], B)[0]
        if A[2] < 5 or Bb[2] < 5: continue
        s = score(A, Bb)
        if prev is not None and (s - 0.5) * (prev[1] - 0.5) <= 0:
            found = (B, A, Bb); break
        prev = (B, s)
    if found:
        B, A, Bb = found
        print(f'  {NAMES[a]} vs {NAMES[b]}: even at +{B} XP ({A[2]} cards vs {Bb[2]} cards); below it {NAMES[a] if prev[1] < 0.5 else NAMES[b]} is behind... ')
    else:
        print(f'  {NAMES[a]} vs {NAMES[b]}: never even in +150..1500 (first score {prev[1] if prev else None})')
