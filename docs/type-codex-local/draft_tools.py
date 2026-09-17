"""Small authoring helper; JSON dossiers are the editable design source."""
import json
from pathlib import Path
ROOT = Path(__file__).parent

def save(name, identity, symbols, loop, statuses, abilities, cards, pairs, example, counter, balance, hooks):
    rows = [line.split('|') for line in cards.strip().splitlines() if line.strip()]
    assert len(rows) == 24, (name, len(rows))
    assert all(len(row) == 4 for row in rows), name
    assert len(statuses) == 4 and len(abilities) == 6 and len(pairs) >= 3
    data = dict(id=name, name=name.title(), identity=identity, symbols=symbols, loop=loop,
                statuses=statuses, abilities=abilities, cards=rows, pairs=pairs,
                example=example, counter=counter, balance=balance, hooks=hooks,
                review_status='Draft — awaiting user review', version=1)
    (ROOT/'drafts'/f'{name}.json').write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')
