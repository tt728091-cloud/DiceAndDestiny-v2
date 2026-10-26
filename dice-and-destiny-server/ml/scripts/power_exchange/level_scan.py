from chars import *
budgets = [100, 150, 250, 400, 600, 900, 1100]
four = ['none', 'light', 'holdfast', 'heavy']
six = ['none', 'light', 'light2', 'holdfast', 'heavy', 'bulwark']
def line(tag, rows):
    cells = []
    for r in rows:
        vs = [x['value'] for x in r['chars'].values()]; pv = list(r['pairs'].values())
        lo = min(r['chars'], key=lambda p: r['chars'][p]['value']); hi = max(r['chars'], key=lambda p: r['chars'][p]['value'])
        cells.append(f"{min(vs):.2f}({lo[:4]})-{max(vs):.2f}({hi[:4]})/{min(pv):.2f}")
    print(f'{tag:40s} | ' + '  '.join(cells), flush=True)
if __name__ == "__main__":
    print("budgets", budgets)
    line('flat 40 (reference)', [solve(four, b, cost_fn=level_price(40, 0, 40)) for b in budgets])
    for c0, sl, cm in [(25, 5, 40), (25, 3, 40), (30, 4, 40), (20, 5, 40), (25, 5, 45), (25, 2, 40)]:
        f = level_price(c0, sl, cm)
        line(f.name, [solve(four, b, cost_fn=f) for b in budgets])
