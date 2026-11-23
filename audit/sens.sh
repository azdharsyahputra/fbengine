#!/bin/zsh
# sens.sh < sens_cfg.txt: each line "label|ENV assignments"; builds ./match, runs 3 seeds with FMENGINE_AUDIT=1, prints one TSV row per config (audit-engine-go.md §5)
S=${0:A:h}; (cd $S/.. && go build -o $S/match ./cmd/match) || exit 1; C=/Users/csadeveloper/fm/fm-game/viewer/cek; mkdir -p $S/sens
while IFS='|' read -r label envs; do
  [[ -z $label ]] && continue
  for s in 19 6 7; do ( env FMENGINE_AUDIT=1 ${=envs} $S/match Liverpool "Man City" $s $S/sens/r_${label}_s$s.js > /dev/null 2> $S/sens/a_${label}_s$s.txt; node $C/mid.js $S/sens/r_${label}_s$s.js > $S/sens/m_${label}_s$s.txt 2>&1 ) & done; wait
  SENSDIR=$S python3 - $label <<'PY'
import re,sys,glob,statistics as st
L=sys.argv[1];import os; d=os.environ['SENSDIR']+'/sens/'
def g(f,rx):
    try: t=open(f).read()
    except: return []
    return [float(x) for x in re.findall(rx,t)]
vals={}
for s in (19,6,7):
    a=d+f'a_{L}_s{s}.txt'; m=d+f'm_{L}_s{s}.txt'
    for k,rx,f in [('mundur',r'mundur (\d+)% \| median',m),('dmMaju',r'lanjut umpan maju (\d+)%',m),('held',r'ball held (\d+)%',a),('loose',r'truly loose ([\d.]+)%',a),
                   ('loose5',r'truly loose: runners-at-ball median \d+ p90 \d+ ≥5 ([\d.]+)%',a),('flight5',r'pass in flight: runners-at-ball median \d+ p90 \d+ ≥5 ([\d.]+)%',a),
                   ('press2',r'defenders <5 m median \d+ ≥2 (\d+)%',a),('boxDef',r'defenders in box ([\d.]+)',a),('boxFree',r'free attackers \(<3 m none\) (\d+)%',a)]:
        v=g(f,rx); 
        if v: vals.setdefault(k,[]).append(st.mean(v))
print(L+'\t'+'\t'.join(f'{k}={st.mean(v):.1f}' for k,v in vals.items()))
PY
done
