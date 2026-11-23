#!/usr/bin/env node
// scorecard.js: one combined score for the team shape (PLAN-team-shape.md §5.1). Every shape
// metric for possession, defence and transition in every zone, the receptions by role, the
// speed shares and long carries are judged AT ONCE against the Metrica full-tracking range.
// The old cek metrics are guards (ranges from F0, Metrica and SkillCorner), in two groups:
//   shape guards (the Team Shape Controller's job, gate of F2/F3): chase3, liar5, dmAm,
//     own3in5, mid3in5, recvPress;
//   decision guards (the carrier's choices, M2; gate of F5): mundur, kept, bwFinal, bwMid, possMed.
// A change is accepted only if the total score improves and no guard breaks.
//
// Usage: node scorecard.js [--guard EVAL_DIR] [--brief] [--slots] replay.js [...]
//   --slots: judge the Team Shape Controller's slots (shadow mode) on the STRUCTURE metrics
//   (width, length, line gaps, back line depth). Spread/interaction metrics (pairs, nearest
//   mate, outer lanes, marking) depend on movement and opponents, so they are listed as info
//   only and judged on the players in F2; receptions, speeds and carries are behaviour.
//   EVAL_DIR: an audit/eval.sh output dir (summary.js reads its cek/probe files).
// Score: per metric err = |candidate - target| / tolerance (target = mean of the two Metrica
// games; tolerance = max(floor, spread between the games)). Pass = err <= 1. Total score =
// mean err over all metrics (lower is better), with the pass count.
'use strict';
const { execFileSync } = require('child_process');
const path = require('path');
const fs = require('fs');
const HERE = __dirname;
const VIEW = '/Users/csadeveloper/fm/fm-game/viewer';
const MET = [`${VIEW}/replay_metrica_g1.js`, `${VIEW}/replay_metrica_g2.js`];

let args = process.argv.slice(2), guardDir = null, brief = false, slots = false;
for (;;) {
  if (args[0] === '--slots') { slots = true; args = args.slice(1); continue; }
  if (args[0] === '--guard') { guardDir = args[1]; args = args.slice(2); continue; }
  if (args[0] === '--brief') { brief = true; args = args.slice(1); continue; }
  break;
}
if (!args.length) { console.error('usage: node scorecard.js [--guard EVAL_DIR] [--brief] replay.js ...'); process.exit(1); }

const suite = (files, sl) => JSON.parse(execFileSync('node', [path.join(HERE, 'shape_suite.js'), '--json', ...(sl ? ['--slots'] : []), ...files], { maxBuffer: 1 << 26 }).toString());

// long carries >= 25 m per match (holder <= 0.8 m, 0.75 s gap tolerance), live only
function carries(file) {
  global.window = {}; const log = console.log; console.log = () => {};
  delete require.cache[require.resolve(path.resolve(file))]; require(path.resolve(file));
  const R = window.REPLAY; const dead = require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js').deadMask(R); console.log = log;
  const F = R.frames, S = 47, T = R.ticks;
  const P = (t, k) => [F[t * S + 3 + 2 * k] / 10, F[t * S + 4 + 2 * k] / 10];
  let n = 0, cur = null;
  for (let t = 0; t < T; t++) {
    let h = -1;
    if (!dead[t]) { const bx = F[t * S] / 10, by = F[t * S + 1] / 10, bz = F[t * S + 2] / 10; let hd = 0.8;
      if (bz <= 0.3) for (let k = 0; k < 22; k++) { const [x, y] = P(t, k); if (x < 0) continue; const d = Math.hypot(x - bx, y - by); if (d < hd) { hd = d; h = k; } } }
    if (cur && (h === cur.h || (h < 0 && t - cur.last <= 3))) { if (h === cur.h) cur.last = t; continue; }
    if (cur) { const a = [F[cur.s * S] / 10, F[cur.s * S + 1] / 10], b = [F[cur.last * S] / 10, F[cur.last * S + 1] / 10]; if (Math.hypot(b[0] - a[0], b[1] - a[1]) >= 25) n++; }
    cur = h >= 0 ? { h, s: t, last: t } : null;
  }
  return n * (90 * 60 * (R.tps || 4)) / T; // per 90 min of recorded time
}

// metric list: [label, getter, floor tolerance]
const groups = [];
for (const side of ['att', 'def']) for (const ph of ['settled', 'trans']) for (const z of ['own', 'mid', 'final']) groups.push(`${side}-${ph}-${z}`);
const metrics = [];
for (const g of groups) {
  const att = g.startsWith('att');
  metrics.push([`${g} width`, s => s.groups[g]?.width, 3]);
  metrics.push([`${g} length`, s => s.groups[g]?.length, 3]);
  metrics.push([`${g} pairs<4m`, s => s.groups[g]?.pairs, 0.25]);
  metrics.push([`${g} nearest mate`, s => s.groups[g]?.nn, 1.5]);
  metrics.push([`${g} back-DM`, s => s.groups[g]?.backDM, 2.5]);
  metrics.push([`${g} DM-AM`, s => s.groups[g]?.dmAM, 2.5]);
  metrics.push([`${g} AM-ST`, s => s.groups[g]?.amST, 2.5]);
  metrics.push([`${g} back line depth`, s => s.groups[g]?.backU, 3]);
  if (att) metrics.push([`${g} outer lanes %`, s => s.groups[g]?.outer, 10]);
  else {
    metrics.push([`${g} defenders <3m of attacker`, s => s.groups[g]?.mark3, 0.35]);
    metrics.push([`${g} attacker->defender m`, s => s.groups[g]?.markD, 1]);
  }
}
if (!slots) {
  for (const z of ['own', 'mid', 'final']) for (const r of ['CB', 'FB', 'DM', 'AM', 'ST']) metrics.push([`receptions ${z} ${r} %`, s => s.recv[z]?.[r], 6]);
  for (const k of ['<2', '2-4', '4-5.5', '5.5-7', '>7']) metrics.push([`speed ${k} m/s %`, s => s.speed[k], k === '>7' ? 1 : 4]);
  metrics.push(['carries >=25 m per 90', s => s.carries, 3]);
}

const cacheFile = '/Users/csadeveloper/fm/refdata/derived/scorecard_metrica.json';
let met;
try { met = JSON.parse(fs.readFileSync(cacheFile)); } catch (_) {
  met = MET.map(f => { const s = suite([f]); s.carries = carries(f); return s; });
  fs.mkdirSync(path.dirname(cacheFile), { recursive: true }); fs.writeFileSync(cacheFile, JSON.stringify(met));
}
const cand = suite(args, slots); if (!slots) cand.carries = args.map(carries).reduce((a, b) => a + b, 0) / args.length;

const infoOnly = l => slots && /pairs<4m|nearest mate|outer lanes|defenders <3m|attacker->defender/.test(l);
let total = 0, n = 0, pass = 0; const rows = [];
for (const [label, get, floor] of metrics) {
  const a = get(met[0]), b = get(met[1]), c = get(cand);
  if (![a, b, c].every(Number.isFinite)) continue;
  const target = (a + b) / 2, tol = Math.max(floor, Math.abs(a - b) / 2 + floor / 2);
  const err = Math.abs(c - target) / tol;
  const info = infoOnly(label);
  if (!info) { total += Math.min(err, 4); n++; if (err <= 1) pass++; }
  rows.push({ label, target, tol, c, err, info });
}
const score = total / n;

// guards (old cek metrics; ranges = Metrica .. SkillCorner from F0, widened a little)
const GUARD = { chase3: [5, 10], liar5: [6, 10.5], mundur: [22, 35], dmAm: [7, 10.5], own3in5: [36, 50], mid3in5: [42, 54],
  recvPress: [24, 37], kept: [74, 86], bwFinal: [27, 48], bwMid: [22, 34], possMed: [4.5, 7.5] };
const DECISION = new Set(['mundur', 'kept', 'bwFinal', 'bwMid', 'possMed']);
const guards = [];
if (guardDir) {
  const line = execFileSync('node', [path.join(HERE, 'summary.js'), 'cand', guardDir]).toString();
  for (const [k, [lo, hi]] of Object.entries(GUARD)) {
    const m = line.match(new RegExp(`\\b${k} ([0-9.]+)`)); if (!m) continue;
    const v = +m[1]; guards.push({ k, v, lo, hi, ok: v >= lo && v <= hi, decision: DECISION.has(k) });
  }
}

const f = v => Math.abs(v) >= 10 ? v.toFixed(0) : v.toFixed(1);
if (!brief) {
  console.log('metric                                   target   tol  cand   err');
  for (const r of rows.sort((x, y) => (x.info - y.info) || (y.err - x.err))) console.log(`${r.info ? 'info' : r.err <= 1 ? 'ok  ' : 'XX  '} ${r.label.padEnd(36)} ${f(r.target).padStart(6)} ${f(r.tol).padStart(5)} ${f(r.c).padStart(5)} ${r.err.toFixed(2).padStart(5)}`);
}
console.log(`SCORE ${score.toFixed(3)} (lower is better) | pass ${pass}/${n} metrics` +
  (guards.length ? ['shape', 'decision'].map(kind => {
    const gs = guards.filter(g => g.decision === (kind === 'decision'));
    return ` | ${kind} guards ${gs.filter(g => g.ok).length}/${gs.length} ok` + gs.filter(g => !g.ok).map(g => ` [${g.k} ${g.v} not in ${g.lo}-${g.hi}]`).join('');
  }).join('') : ''));
