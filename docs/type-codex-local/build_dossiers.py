#!/usr/bin/env python3
"""Render review-only type JSON as static HTML; preserve reviewed Curse/Venom panels."""
from pathlib import Path
import html, json, re, itertools, collections

ROOT = Path(__file__).resolve().parent
ORDER = 'flame tide storm frost stone verdant venom blood beast swarm construct grave arcane curse radiant fortune'.split()
COLORS = ['#dca36c','#79b8c5','#adabe0','#a7cadc','#beb18b','#a9c580','#adbc7e','#cf9599','#bda57b','#c3bb7b','#a5babb','#b9abc9','#b8a7df','#c18388','#ddd4a0','#dcca81']
esc = lambda value: html.escape(str(value), quote=True)

def details(title, body):
    return f'<details class="draft-details"><summary>{esc(title)}</summary>{body}</details>'

def ability_rows(recipe, effect):
    match = re.match(r'(\d+) / (\d+) / (\d+) damage(.*)', effect)
    if match and ' / ' in recipe:
        return ''.join(f'<tr><th>{esc(r)}</th><td>{esc(n+" damage"+match[4])}</td></tr>' for r,n in zip(recipe.split(' / '),match.groups()[:3]))
    return f'<tr><th>{esc(recipe)}</th><td>{esc(effect)}</td></tr>'

def probability(d, recipe):
    faces = [d['symbols'][0]]*3 + [d['symbols'][1]]*2 + [d['symbols'][2]]
    def qualifies(roll):
        if recipe == 'Any three equal numbers': return max(collections.Counter(roll).values()) >= 3
        if recipe == 'Any four-number straight': return any(set(range(n,n+4)) <= set(roll) for n in range(1,4))
        counts = collections.Counter(faces[n-1] for n in roll)
        return all(counts[symbol] >= int(count) for count,symbol in re.findall(r'(\d+) ([A-Za-z]+)',recipe))
    recipes = recipe.split(' / ')
    if len(recipes) > 1: return ' / '.join(probability(d,r) for r in recipes)
    return f'{sum(qualifies(r) for r in itertools.product(range(1,7),repeat=5))/7776:.2%}'

def render_status(d, i, label, scope, rule):
    note=d['status_examples'][label]
    return (f'<article class="venom-status-card" id="{d["id"]}-state-{i}">'
            f'<span class="section-label">{esc(scope)}</span><h4>{esc(label)}</h4>'
            f'<p class="status-purpose">{esc(note["job"])}</p><p>{esc(rule)}</p>'
            f'<p class="status-example"><strong>Example</strong> {esc(note["example"])}</p></article>')

def render(d):
    name, key = d['name'], d['id']
    number = ORDER.index(key)+1
    syms = d['symbols']; faces = [syms[0]]*3+[syms[1]]*2+[syms[2]]
    dice = ''.join(f'<span class="die-face">{i}<small>{esc(s)}</small></span>' for i,s in enumerate(faces,1))
    tabs=[('overview','Overview'),('dice','Dice & statuses'),('engine','Rules & examples'),('abilities','Six abilities'),('cards','24 cards'),('counterplay','Synergies & review')]
    out=[f'<section class="section type-panel draft-panel" id="{key}-panel" data-draft-version="{d["version"]}" style="--type-accent:{COLORS[number-1]}" hidden>',
         f'<div class="type-heading"><div><span class="section-label">Type {number:02d} · Review draft {d["version"]} · Not implemented</span><h2>{esc(name)}</h2><p class="lead">{esc(d["identity"])}</p></div><div class="curse-specimen" aria-label="{name} die face map">{dice}</div></div>',
         '<nav class="subtabs" role="tablist" aria-label="'+name+' design sections">'+''.join(f'<button data-subtab="{a}" role="tab">{esc(b)}</button>' for a,b in tabs)+'</nav>',
         '<div class="subpanel" data-subpanel="overview">',
         '<dl class="venom-ledger"><div><dt>Combat dice</dt><dd>5</dd><small>3 / 2 / 1 face map</small></div><div><dt>Abilities</dt><dd>6</dd><small>4 offense · 2 defense</small></div><div><dt>Card pool</dt><dd>24</dd><small>6 / 6 / 9 / 3</small></div><div><dt>Signature statuses</dt><dd>4</dd><small>A purpose and example for each</small></div></dl>',
         '<div class="doctrine-callout"><strong>The core loop</strong><p>'+esc(d['loop'])+'</p></div>',
         '<div class="comparison-grid">'+''.join('<article><h4>'+esc(a)+'</h4><p>'+esc(b)+'</p></article>' for a,b in d['combos'])+'</div>',
         '<p class="draft-note">Start with the four status definitions, then review the abilities and card pool. '+str(len(d['pairs']))+' concrete partner interactions are explained under Synergies &amp; review. A type pool does not determine a creature’s final deck or health.</p>',
         '</div><div class="subpanel" data-subpanel="dice" hidden>',
         f'<h3>Five {name} D6</h3><p>Each die has numbers 1–6 with the symbols shown below. A mixed creature keeps five total combat dice; matching numbers can connect different types.</p>',
         '<div class="draft-table-wrap"><table class="venom-table"><thead><tr><th>Faces</th><th>Symbol</th><th>Chance per die</th><th>Role</th></tr></thead><tbody>'+''.join(f'<tr><td>{f}</td><td>{esc(s)}</td><td>{p}</td><td>{role}</td></tr>' for f,s,p,role in [('1–3',syms[0],'1/2','Reliable attack tier'),('4–5',syms[1],'1/3','Setup / defense'),('6',syms[2],'1/6','Rare payoff')])+'</tbody></table></div>',
         '<div class="venom-status-grid">'+''.join(render_status(d, i, label, scope, rule) for i,(label,scope,rule) in enumerate(d['statuses'],1))+'</div>',
         '<p class="draft-note">A creature with this type can earn its resources even when it currently has zero stacks. A cap is the maximum you can hold. Card-specific marks are explained on their cards. See Rules &amp; examples for timing and mixed-type details.</p>',
         '</div><div class="subpanel" data-subpanel="engine" hidden><h3>How these rules work</h3>',
         '<div class="doctrine-callout"><strong>Type-specific limits &amp; balance</strong><p>'+esc(d['balance'])+'</p></div>',
         '<h4>Worked interaction</h4><p>'+esc(d['example'])+'</p>',
         details('Developer reference: existing game hooks', '<p>For implementation after design review.</p><ul>'+''.join('<li>'+esc(x)+'</li>' for x in d['hooks'])+'</ul><p><a href="../battle-hooks-local/index.html">Battle Hooks Field Guide</a> · <a href="../game-design/turn-structure.md">Turn structure</a></p>'),
         details('Shared play rules: timing, dice, costs and recovery', '<p>'+esc(SHARED['scope'])+'</p>'+''.join('<h4>'+esc(a)+'</h4><p>'+esc(b)+'</p>' for a,b in SHARED['rules'])),
         '</div><div class="subpanel" data-subpanel="abilities" hidden><h3>Six abilities</h3><p>Choose one Offensive ability per round and one Defensive ability when defending. Pay optional ability costs when choosing the ability. Resources it gives you afterward cannot pay those costs.</p><div class="ability-grid">']
    for i,(label,kind,recipe,effect) in enumerate(d['abilities'],1):
        chance = '' if i > 4 else '<p class="draft-probability">Initial unmodified five-die roll: '+probability(d,recipe)+'. Rerolls and mixed loadouts change these odds.</p>'
        out.append(f'<article class="ability-card {"defensive" if i>4 else ""}" data-draft-ability="{key}-a{i:02d}"><div><span>{key.upper()} A{i:02d}</span><b>{esc(kind)}</b></div><h4>{esc(label)}</h4><table><tbody>{ability_rows(recipe,effect)}</tbody></table>{chance}</article>')
    out+=['</div></div><div class="subpanel" data-subpanel="cards" hidden><h3>Twenty-four cards</h3><p>Listed Energy is paid in addition to all costs in the rules text. Every card is a distinct pool option; deck copies and progression are a later creature-design decision.</p>',
          f'<label class="draft-search" for="{key}-card-search">Find a card, status or timing<input id="{key}-card-search" data-card-search="{key}" type="search" placeholder="Search this type’s 24 cards"></label><p class="draft-result-count" id="{key}-card-count" aria-live="polite">24 cards</p><div class="card-groups">']
    starts=[(0,6,'Build your setup'),(6,12,'Convert & develop'),(12,21,'Tactics & responses'),(21,24,'Keystones')]
    for gi,(start,end,title) in enumerate(starts,1):
        out.append(f'<section data-card-group><header><span>{gi:02d}</span><div><h4>{title} · {end-start} cards</h4><p>Use with the type’s statuses and shared play rules.</p></div></header><div class="card-list">')
        for i,(label,cost,timing,effect) in enumerate(d['cards'][start:end],start+1):
            out.append(f'<article data-draft-card="{key}-c{i:02d}"><b>{i:02d} · {esc(label)}</b><small>{esc(timing)} · {esc(cost)} E</small><p>{esc(effect)}</p></article>')
        out.append('</div></section>')
    out+=['</div></div><div class="subpanel" data-subpanel="counterplay" hidden><h3>Partners with a concrete exchange</h3><div class="comparison-grid">']
    for partner,rule in d['pairs']:
        out.append(f'<article data-pair="{key}:{partner}"><span class="method-tag">{name} + {partner.title()}</span><h4><a href="#{partner}/counterplay">Review {partner.title()} →</a></h4><p>{esc(rule)}</p></article>')
    out+=['</div><div class="doctrine-callout"><strong>Counterplay</strong><p>'+esc(d['counter'])+'</p></div>',
          '<h4>Review questions</h4><ul><li>Does the setup remain useful without drawing a specific partner card?</li><li>Are the resource costs worth the tempo compared with simply attacking?</li><li>Can the opponent disrupt at least one part of the loop?</li><li>Which cards should enter a specific creature’s starting deck, and which belong later?</li></ul>',
          f'<div class="draft-review"><h4>Your review notes</h4><label><input type="checkbox" data-review-check="{key}"> Mark this draft reviewed</label><label for="{key}-review-notes">Changes to discuss<textarea id="{key}-review-notes" data-review-notes="{key}" rows="5" placeholder="Balance changes, favorite combos, unclear rules…"></textarea></label><p>Saved only in this browser when local storage is available. Marking reviewed does not implement or approve a character automatically. Export notes to keep a file copy.</p><button type="button" data-export-reviews>Export all review notes</button><span data-review-save="{key}" role="status"></span></div>',
          '</div></section>']
    return '\n'.join(out)

SHARED=json.loads((ROOT/'shared-rules.json').read_text())
DATA={p.stem:json.loads(p.read_text()) for p in (ROOT/'drafts').glob('*.json')}

def build():
    page=(ROOT/'index.html').read_text()
    rendered='\n'.join(render(DATA[key]) for key in ORDER if key in DATA)
    start='<!-- GENERATED DOSSIERS START -->'; end='<!-- GENERATED DOSSIERS END -->'
    block=start+'\n'+rendered+'\n'+end
    if start in page:
        page=re.sub(re.escape(start)+r'.*?'+re.escape(end),lambda _:block,page,flags=re.S)
    else:
        marker='      <section class="section type-panel curse-panel"'
        page=page.replace(marker,block+'\n\n'+marker,1)
    pairs = {}
    for key in ORDER:
        if key not in DATA: continue
        for partner, rule in DATA[key]['pairs']:
            pairs.setdefault(tuple(sorted((key,partner))), []).append((key,rule))
    pairs[('curse','venom')] = [('venom', 'Reviewed design pairing: natural and Provoked toxin rolls give a cursed die more chances to hit its marked face. The proposed mixed creature must roll its marked combat dice for toxins; playable Venom currently uses separate effect dice.')]
    pair_rows = ''.join('<tr><th><a href="#'+a+'/counterplay">'+a.title()+'</a> + <a href="#'+b+'/counterplay">'+b.title()+'</a></th><td>'+esc(rows[0][1])+'</td></tr>' for (a,b),rows in sorted(pairs.items()))
    page = re.sub(r'Cross-type interaction index · \d+ distinct pairings', 'Cross-type interaction index · '+str(len(pairs))+' distinct pairings', page)
    page = re.sub(r'(<aside class="review-overview".*?<tbody>).*?(</tbody>)', lambda m:m[1]+pair_rows+m[2], page, count=1, flags=re.S)
    (ROOT/'index.html').write_text(page)

if __name__=='__main__':
    build()
    print(f'Rendered {len(DATA)} dossiers / {sum(len(d["abilities"]) for d in DATA.values())} abilities / {sum(len(d["cards"]) for d in DATA.values())} cards')
