#!/usr/bin/env python3
"""Static content/route audit. Does not launch a browser or run game code."""
from collections import Counter, defaultdict
from html.parser import HTMLParser
from pathlib import Path
import hashlib, json, re, subprocess, tempfile
from build_dossiers import ORDER, DATA, probability
ROOT=Path(__file__).resolve().parent

class Tree(HTMLParser):
    VOID={'area','base','br','col','embed','hr','img','input','link','meta','param','source','track','wbr'}
    def __init__(self):
        super().__init__(); self.ids=[];self.attrs=[];self.stack=[];self.errors=[];self.scripts=[];self.current_script=None
    def handle_starttag(self, tag, attrs):
        attrs=dict(attrs);self.attrs.append((tag,attrs))
        if 'id' in attrs:self.ids.append(attrs['id'])
        if tag=='script' and 'src' not in attrs:self.current_script=''
        if tag not in self.VOID:self.stack.append(tag)
    def handle_endtag(self,tag):
        if tag=='script' and self.current_script is not None:self.scripts.append(self.current_script);self.current_script=None
        if not self.stack or self.stack[-1]!=tag:self.errors.append(f'Unexpected closing {tag}, stack {self.stack[-4:]}')
        else:self.stack.pop()
    def handle_data(self,d):
        if self.current_script is not None:self.current_script+=d

def run():
    text=(ROOT/'index.html').read_text();tree=Tree();tree.feed(text)
    assert not tree.errors,tree.errors[:8]
    assert not tree.stack,tree.stack
    assert not [i for i,n in Counter(tree.ids).items() if n>1], 'Duplicate HTML IDs'
    assert set(DATA)==set(ORDER)-{'curse','venom'}
    assert {a['id'] for _,a in tree.attrs if 'type-panel' in a.get('class','').split()}=={k+'-panel' for k in ORDER}
    timing={'Action','Offensive reaction','Defensive reaction','Damage reaction','Application reaction','Roll reaction'}
    pairs=set();neighbors=defaultdict(set)
    odds={}
    for key,d in DATA.items():
        assert d['id']==key and len(d['symbols'])==3 and len(set(d['symbols']))==3
        assert len(d['abilities'])==6 and len(d['statuses'])==4 and len(d['cards'])==24
        assert len({c[0] for c in d['cards']})==24
        assert len({a[0] for a in d['abilities']})==6
        assert sum(a[1].startswith('Offense') for a in d['abilities'])==4
        assert sum(a[1].startswith('Defense') for a in d['abilities'])==2
        assert all(c[1].isdigit() and c[2] in timing and bool(c[3].strip()) for c in d['cards'])
        assert all(len(s)==3 and ('cap' in s[1].lower() or 'max' in s[1].lower()) for s in d['statuses'])
        assert len(d['pairs'])>=3 and len(d['combos'])>=2
        assert d['version'] == 2, (key, 'outdated review version')
        assert set(d['status_examples']) == {row[0] for row in d['statuses']}
        assert all(v.get('job') and v.get('example') for v in d['status_examples'].values())
        # These terms obscured player rules in v1; technical hook names are excluded.
        player_fields = ['identity','loop','statuses','status_examples','abilities','cards','pairs','combos','example','counter','balance']
        player_text = json.dumps({field:d[field] for field in player_fields}, ensure_ascii=False)
        jargon = re.search(r'\b(batch(?:es)?|commit(?:s|ted)?|accepted|acceptance|revalidat(?:e|ion)|proposals?|overlays?|controller|ancestry|sources?|queue(?:d)?)\b', player_text, re.I)
        assert jargon is None, (key, 'engine wording in player rules', jargon.group(0) if jargon else '')

        for partner,explanation in d['pairs']:
            assert partner in ORDER and partner!=key and len(explanation)>70
            pairs.add(tuple(sorted((key,partner))));neighbors[key].add(partner);neighbors[partner].add(key)
        assert sum(a.get('data-draft-card','').startswith(key+'-') for _,a in tree.attrs)==24
        assert sum(a.get('data-draft-ability','').startswith(key+'-') for _,a in tree.attrs)==6
        panel=text.split(f'id="{key}-panel"',1)[1].split('</section>\n<section class="section type-panel',1)[0]
        for sub in ['overview','dice','engine','abilities','cards','counterplay']:
            assert f'data-subpanel="{sub}"' in panel
        for ability in d['abilities'][:4]:
            for recipe in ability[2].split(' / '):
                if recipe.startswith('Any '):assert recipe in ['Any three equal numbers','Any four-number straight']
                else:
                    terms=re.findall(r'(\d+) ([A-Za-z]+)',recipe)
                    assert terms and all(symbol in d['symbols'] for _,symbol in terms),(key,recipe)
                    assert sum(int(n) for n,_ in terms)<=5,(key,recipe)
            odds[key+': '+ability[0]]=probability(d,ability[2])
        assert not re.search(r'\b(TODO|TBD|placeholder|lorem ipsum)\b',json.dumps(d),re.I)
    assert text.count('class="status-purpose"') == 56
    assert text.count('class="status-example"') == 56
    assert text.count('Review draft 2') == 14
    pairs.add(('curse','venom')); neighbors['curse'].add('venom'); neighbors['venom'].add('curse')
    assert all(len(neighbors[k])>=3 for k in ORDER),dict(neighbors)
    baselines=json.loads((ROOT/'review/baseline-hashes.json').read_text())
    for name,expected in baselines.items():
        start=text.index('      <section class="section type-panel '+name+'-panel"');end=text.index('\n      </section>',start)+len('\n      </section>')
        assert hashlib.sha256(text[start:end].encode()).hexdigest()==expected, name+' was changed'
    for tag,attrs in tree.attrs:
        for key in ['href','src']:
            value=attrs.get(key,'')
            if not value:continue
            if value.startswith('#'):
                route=value[1:].split('/')[0]
                assert route in ORDER or route in tree.ids,(tag,value)
            elif not re.match(r'^[a-z]+:',value):
                target=(ROOT/value.split('#')[0]).resolve()
                assert target.exists(),str(target)
    for script in tree.scripts:
        with tempfile.NamedTemporaryFile('w',suffix='.js') as f:
            f.write(script);f.flush();subprocess.run(['node','--check',f.name],check=True,capture_output=True)
    subprocess.run(['node','--check',str(ROOT/'review-tools.js')],check=True,capture_output=True)
    results={'dossiers':14,'abilities':84,'cards':336,'signature_states':56,'status_examples':56,'review_version':2,'player_wording_audit':'Passed: engine terms excluded from new player-facing dossiers','distinct_pairings':len(pairs),'partner_counts':{k:len(neighbors[k]) for k in ORDER},'baseline_panels':'Curse and Venom match their current reviewed panel hashes','initial_five_die_qualification':odds,'browser_screenshots':'Blocked by Browser URL policy; no fresh capture attempted by alternate means'}
    (ROOT/'review/validation-results.json').write_text(json.dumps(results,indent=2)+'\n')
    print(json.dumps({k:v for k,v in results.items() if k!='initial_five_die_qualification'},indent=2))
if __name__=='__main__':run()
