#!/usr/bin/env node
// formation_table.js: per-slot median positions in the team's attacking frame (u along the
// attack from the own goal line, w across), per phase (att/def, settled = same team >= 4 s in
// possession) and ball cell (3 depth zones x 3 width zones), absolute and relative to the ball.
// The ground truth for the Team Shape Controller's formation table (PLAN-team-shape.md §4.1).
//
// Usage: node formation_table.js [--json out.json] replay.js [...]
// Slot order is the replay's: GK, FB(L), CB, CB, FB(R), DM, DM, AM(L), AM(C), AM(R), ST.
// Team frame: a team whose keeper stands in the right half is rotated 180° (u = 105 - x,
// w = 68 - y), so "left" stays left of the attack.
'use strict';
const path = require('path');
const { deadMask } = require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js');
let args = process.argv.slice(2), jsonOut = null;
if (args[0] === '--json') { jsonOut = args[1]; args = args.slice(2); }
const SLOT = ['GK', 'FB-L', 'CB-L', 'CB-R', 'FB-R', 'DM-L', 'DM-R', 'AM-L', 'AM-C', 'AM-R', 'ST'];
const acc = {}; // key phase|cell|slot -> {u:[],w:[],du:[],dw:[]}
for (const file of args) {
  global.window = {}; const log = console.log; console.log = () => {};
  require(path.resolve(file)); const R = window.REPLAY; const dead = deadMask(R); console.log = log;
  const F = R.frames, S = 47, T = R.ticks, tps = R.tps || 4;
  const P = (t, k) => [F[t * S + 3 + 2 * k] / 10, F[t * S + 4 + 2 * k] / 10];
  // per (team, half): flip w so slot 1 (FB-L) is on the low-w side, whatever the source's
  // mirroring convention (the engine mirrors x only; real data is a 180° rotation).
  const half = t => (t >= (R.events.find(e => e.k === 'ht') || { t: T }).t ? 1 : 0);
  const flip = [[false, false], [false, false]];
  for (const tm of [0, 1]) for (const hf of [0, 1]) {
    let a = 0, b = 0, n = 0;
    for (let t = 0; t < T; t += 8) {
      if (dead[t] || half(t) !== hf) continue;
      const rot = P(t, tm * 11)[0] >= 52.5, W = y => rot ? 68 - y : y;
      const l = P(t, tm * 11 + 1), r = P(t, tm * 11 + 4); if (l[0] < 0 || r[0] < 0) continue;
      a += W(l[1]); b += W(r[1]); n++;
    }
    flip[tm][hf] = n > 0 && a > b;
  }
  let team = -1, since = 0;
  for (let t = 0; t < T; t++) {
    if (dead[t]) { team = -1; continue; }
    const bx = F[t * S] / 10, by = F[t * S + 1] / 10, bz = F[t * S + 2] / 10;
    let h = -1, hd = 0.8;
    if (bz <= 0.3) for (let k = 0; k < 22; k++) { const [x, y] = P(t, k); if (x < 0) continue; const d = Math.hypot(x - bx, y - by); if (d < hd) { hd = d; h = k; } }
    if (h >= 0) { const tm = h < 11 ? 0 : 1; if (tm !== team) { team = tm; since = t; } }
    if (team < 0 || t % 2 || t - since < 4 * tps) continue;
    for (const tm of [0, 1]) {
      const rot = P(t, tm * 11)[0] >= 52.5;
      const fl = flip[tm][half(t)];
      const U = x => rot ? 105 - x : x, W = y => { const w = rot ? 68 - y : y; return fl ? 68 - w : w; };
      const ub = U(bx), wb = W(by);
      const cell = (ub < 35 ? 'own' : ub < 70 ? 'mid' : 'final') + '-' + (wb < 22.7 ? 'w0' : wb < 45.3 ? 'w1' : 'w2');
      const phase = tm === team ? 'att' : 'def';
      for (let k = 0; k < 11; k++) {
        const p = P(t, tm * 11 + k); if (p[0] < 0) continue;
        const key = `${phase}|${cell}|${k}`;
        const a = acc[key] = acc[key] || { u: [], w: [], du: [], dw: [] };
        const u = U(p[0]), w = W(p[1]);
        a.u.push(u); a.w.push(w); a.du.push(u - ub); a.dw.push(w - wb);
      }
    }
  }
}
const med = a => { const s = Float64Array.from(a).sort(); return s[s.length >> 1]; };
const table = {};
for (const [key, a] of Object.entries(acc)) {
  const [phase, cell, k] = key.split('|');
  ((table[phase] = table[phase] || {})[cell] = table[phase][cell] || {})[SLOT[k]] =
    { u: +med(a.u).toFixed(1), w: +med(a.w).toFixed(1), du: +med(a.du).toFixed(1), dw: +med(a.dw).toFixed(1), n: a.u.length };
}
if (jsonOut) require('fs').writeFileSync(jsonOut, JSON.stringify({ sources: args.map(f => path.basename(f)), slots: SLOT, table }, null, 1));
for (const phase of ['att', 'def']) for (const cell of ['own-w1', 'mid-w1', 'mid-w0', 'final-w1']) {
  const c = (table[phase] || {})[cell]; if (!c) continue;
  console.log(`${phase} ${cell} (n ${c.GK ? c.GK.n : '?'}): ` + SLOT.slice(1).map(s => c[s] ? `${s} ${c[s].u}/${c[s].w}` : `${s} -`).join(' | '));
}
