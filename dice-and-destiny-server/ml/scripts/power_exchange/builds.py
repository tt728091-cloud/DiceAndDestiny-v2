from race import *
REF = 'Standard · Standard D6'; die = BOARDS[REF]['die']
B = BOARDS[REF]
def at_avg(avg):
    """Board on the staircase whose whole average is closest to avg."""
    i = int(np.argmin([abs(a - avg) for a in B['averages']]))
    return np.array(B['odds'][i]), B['averages'][i]
def sc(ha, aa, hb, ab, P, w=2):
    A, _ = at_avg(aa); Bo, _ = at_avg(ab)
    return np.mean([score(A, Bo, P, P, ha + d, hb + d) for d in range(-w, w + 1)])
for d, H0 in [('holdfast', 15), ('holdfast', 25), ('none', 15), ('light', 15)]:
    P = prev(die, d); r, a0 = board_at(REF, 6.2); m0, _ = stats(defend(r, P))
    c = 10 * H0 / m0  # fair XP per +1 damage-through, from the base state
    print(f'\n## {d}, base health {H0}, base 6.2 (through {m0:.2f}): fair price {c:.0f} XP per +1 damage (= {H0/m0:.1f} blanks)')
    print('budget  dmg-only vs blanks-only   half/half vs dmg-only   half/half vs blanks-only')
    for X in [30, 60, 120, 240]:
        hA, aA = H0, 6.2 + X / c
        hB, aB = H0 + round(X / 10), 6.2
        hM, aM = H0 + round(X / 20), 6.2 + X / (2 * c)
        print(f'{X:5d}   {sc(hA, aA, hB, aB, P):.3f}                    {sc(hM, aM, hA, aA, P):.3f}                  {sc(hM, aM, hB, aB, P):.3f}    (dmg-only {aA:.1f}/{hA}, blanks {aB}/{hB}, half {aM:.1f}/{hM})')
