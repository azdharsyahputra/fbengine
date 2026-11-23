#!/bin/zsh
# eval.sh TAG [ENV=val ...] : Phase-2 harness. Builds ./match, plays Liverpool–Man City on
# SEEDS (default 19 6 7 3 11) with the given env, runs the live-mode cek metrics + the engine
# probe, prints one summary row (means over seeds) and keeps outputs in $OUT/TAG/.
S=${0:A:h}; OUT=${OUT:-/private/tmp/claude-501/-Users-csadeveloper-fm/1ae7a865-0c5a-4ce6-9f22-4e56e19db51c/scratchpad/p2}
C=/Users/csadeveloper/fm/fm-game/viewer/cek
T=$1; shift; mkdir -p $OUT/$T
(cd $S/.. && go build -o $S/match ./cmd/match) || exit 1
for s in ${=SEEDS:-19 6 7 3 11}; do (
  env FMENGINE_AUDIT=1 "$@" $S/match Liverpool "Man City" $s $OUT/$T/r_s$s.js > $OUT/$T/m_s$s.txt 2> $OUT/$T/a_s$s.txt
  { for c in chase pingpong mid swarm motion crit shape; do echo "######## $c"; node $C/$c.js $OUT/$T/r_s$s.js; done; echo "######## pressz"; node $S/pressz.js $OUT/$T/r_s$s.js; echo "######## press"; node $S/press.js $OUT/$T/r_s$s.js; echo "######## zone"; node $S/zone.js $OUT/$T/r_s$s.js; } > $OUT/$T/c_s$s.txt 2>&1
) & done; wait
node $S/summary.js $T $OUT/$T
