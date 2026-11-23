#!/usr/bin/env node
// fit_formation.js: fits the Team Shape Controller's formation model from full-tracking replays
// (Metrica). Per phase (att = side in possession, def) and per slot, the slot position in the
// team frame (u from the own goal line along the attack, w from the left touchline) is a
// least-squares quadratic in the ball position (ub, wb):
//   u = c0 + c1*x + c2*y + c3*x^2 + c4*y^2 + c5*x*y + c6*x^3 + c7*x*y^2
//   with x = (ub-52.5)/52.5, y = (wb-34)/34
// Settled samples only (same team in possession >= 4 s), 2 Hz. Roles are assigned per frame by
// EM + Hungarian matching to the template (role swaps do not average the shape narrow).
// Writes engine/team_shape_table.go (generated) and prints the fit error per slot.
//
// Usage: node fit_formation.js out.go replay.js [...]
'use strict';
const path = require('path'), fs = require('fs');
const { deadMask } = require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js');
const [out, ...files] = process.argv.slice(2);
const SLOT = ['GK', 'FB-L', 'CB-L', 'CB-R', 'FB-R', 'DM-L', 'DM-R', 'AM-L', 'AM-C', 'AM-R', 'ST'];
const feat = (ub, wb) => { const x = (ub - 52.5) / 52.5, y = (wb - 34) / 34; return [1, x, y, x * x, y * y, x * y, x * x * x, x * y * y]; };
const NF = 8;
// phases: settled (same team in possession >= 4 s) and transition (< 4 s after a change)
const PHASES = ['att', 'def', 'att-trans', 'def-trans'];
const samples = Object.fromEntries(PHASES.map(p => [p, []]));
for (const file of files) {
  global.window = {}; const log = console.log; console.log = () => {};
  require(path.resolve(file)); const R = window.REPLAY; const dead = deadMask(R); console.log = log;
  const F = R.frames, S = 47, T = R.ticks, tps = R.tps || 4;
  const P = (t, k) => [F[t * S + 3 + 2 * k] / 10, F[t * S + 4 + 2 * k] / 10];
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
    if (team < 0 || t % 2) continue;
    const trans = t - since < 4 * tps;
    for (const tm of [0, 1]) {
      const rot = P(t, tm * 11)[0] >= 52.5, fl = flip[tm][half(t)];
      const U = x => rot ? 105 - x : x, W = y => { const w = rot ? 68 - y : y; return fl ? 68 - w : w; };
      const ub = U(bx), wb = W(by), f = feat(ub, wb);
      const phase = (tm === team ? 'att' : 'def') + (trans ? '-trans' : '');
      const row = [];
      for (let k = 0; k < 11; k++) { const p = P(t, tm * 11 + k); row.push(p[0] < 0 ? null : [U(p[0]), W(p[1])]); }
      if (row.every(Boolean)) samples[phase].push({ ub, wb, f, row, role: row.map((_, k) => k) });
    }
  }
}
function solve(A, b) { // Gaussian elimination with partial pivoting (+ tiny ridge)
  const n = b.length, M = A.map((r, i) => [...r, b[i]]); for (let i = 0; i < n; i++) M[i][i] += 1e-6;
  for (let c = 0; c < n; c++) {
    let p = c; for (let r = c + 1; r < n; r++) if (Math.abs(M[r][c]) > Math.abs(M[p][c])) p = r;
    [M[c], M[p]] = [M[p], M[c]];
    for (let r = 0; r < n; r++) if (r !== c) { const k = M[r][c] / M[c][c]; for (let j = c; j <= n; j++) M[r][j] -= k * M[c][j]; }
  }
  return M.map((r, i) => r[n] / r[i]);
}
// Hungarian (Munkres) for a small square cost matrix: returns col assigned to each row
function hungarian(C) {
  const n = C.length, u = new Float64Array(n + 1), v = new Float64Array(n + 1), p = new Int32Array(n + 1), way = new Int32Array(n + 1);
  for (let i = 1; i <= n; i++) {
    p[0] = i; let j0 = 0; const minv = new Float64Array(n + 1).fill(Infinity), used = new Uint8Array(n + 1);
    do {
      used[j0] = 1; const i0 = p[j0]; let delta = Infinity, j1 = 0;
      for (let j = 1; j <= n; j++) if (!used[j]) { const cur = C[i0 - 1][j - 1] - u[i0] - v[j]; if (cur < minv[j]) { minv[j] = cur; way[j] = j0; } if (minv[j] < delta) { delta = minv[j]; j1 = j; } }
      for (let j = 0; j <= n; j++) if (used[j]) { u[p[j]] += delta; v[j] -= delta; } else minv[j] -= delta;
      j0 = j1;
    } while (p[j0] !== 0);
    do { const j1 = way[j0]; p[j0] = p[j1]; j0 = j1; } while (j0);
  }
  const ans = new Int32Array(n); for (let j = 1; j <= n; j++) ans[p[j] - 1] = j - 1; return ans;
}
// EM role assignment (Bialkowski et al. 2014): fit the template per slot from the current
// role labels, then re-assign the 10 outfielders of each frame to the template slots by
// minimum total squared distance; the keeper keeps slot 0.
function fitPhase(list) {
  let c;
  for (let it = 0; it < 8; it++) {
    const acc = Array.from({ length: 11 }, () => [0, 1].map(() => ({ A: Array.from({ length: NF }, () => new Float64Array(NF)), b: new Float64Array(NF) })));
    for (const sm of list) for (let k = 0; k < 11; k++) {
      const slot = sm.role[k], uw = sm.row[k];
      for (const ax of [0, 1]) { const a = acc[slot][ax]; for (let i = 0; i < NF; i++) { a.b[i] += sm.f[i] * uw[ax]; for (let j = 0; j < NF; j++) a.A[i][j] += sm.f[i] * sm.f[j]; } }
    }
    c = acc.map(ax => ax.map(a => solve(a.A.map(r => Array.from(r)), Array.from(a.b))));
    let changed = 0;
    for (const sm of list) {
      const tpl = c.map(cc => cc.map(cf => cf.reduce((s2, v, i) => s2 + v * sm.f[i], 0)));
      const C = []; for (let k = 1; k < 11; k++) { const r = []; for (let sl = 1; sl < 11; sl++) r.push((sm.row[k][0] - tpl[sl][0]) ** 2 + (sm.row[k][1] - tpl[sl][1]) ** 2); C.push(r); }
      const as = hungarian(C);
      for (let k = 1; k < 11; k++) { const nr = as[k - 1] + 1; if (nr !== sm.role[k]) changed++; sm.role[k] = nr; }
    }
    console.log(`  EM iteration ${it}: ${changed} role changes`);
    if (!changed) break;
  }
  return c;
}
const coef = Object.fromEntries(PHASES.map(p => [p, fitPhase(samples[p])]));
// fit error (median distance slot model -> actual) per phase and slot
const ev = (c, ub, wb) => { const f = feat(ub, wb); return c.map(cc => cc.reduce((s, v, i) => s + v * f[i], 0)); };
for (const phase of PHASES) {
  const errs = SLOT.map(() => []);
  for (const s of samples[phase]) for (let k = 0; k < 11; k++) { const [u, w] = ev(coef[phase][s.role[k]], s.ub, s.wb); errs[s.role[k]].push(Math.hypot(u - s.row[k][0], w - s.row[k][1])); }
  const med = a => { a.sort((x, y) => x - y); return a[a.length >> 1]; };
  console.log(`${phase} (n ${samples[phase].length}) median |model - actual| m: ` + SLOT.map((s, k) => `${s} ${med(errs[k]).toFixed(1)}`).join(' '));
}
// spread correction: an averaged template is narrower and shorter than the real shape; the
// factor per phase and axis is the real median (p10-p90 of the 10 outfielders) over the
// template's, both on the same frames. The engine scales the outfield slots about their centre.
const spread = {};
for (const phase of PHASES) {
  const q = (a, pp) => { const z = [...a].sort((x, y) => x - y); return z[Math.floor(z.length * pp)]; };
  const rw = [], tw = [], rl = [], tl = [];
  for (const sm of samples[phase]) {
    const act = sm.row.slice(1), tpl = [];
    for (let k = 1; k < 11; k++) tpl.push(ev(coef[phase][k], sm.ub, sm.wb));
    for (const [arr, W, L] of [[act, rw, rl], [tpl, tw, tl]]) { const us = arr.map(p => p[0]), ws = arr.map(p => p[1]); W.push(q(ws, 0.9) - q(ws, 0.1)); L.push(q(us, 0.9) - q(us, 0.1)); }
  }
  const md = a => q(a, 0.5);
  spread[phase] = [md(rl) / md(tl), md(rw) / md(tw)];
  console.log(`spread ${phase}: length x${spread[phase][0].toFixed(3)} width x${spread[phase][1].toFixed(3)}`);
}
const fmt = v => v.toFixed(4);
let go = `// Code generated by audit/fit_formation.js; DO NOT EDIT.
// Source: ${files.map(f => path.basename(f)).join(', ')} (Metrica Sports sample data, full
// tracking; settled phases, 2 Hz; per-frame EM role assignment). Least-squares quadratic per
// slot and phase in the ball position, team frame (see team_shape.go).

package engine

// teamShapeModel[phase][slot][axis][term]: phase 0 = in possession, 1 = defending,
// 2 = in possession < 4 s after winning the ball, 3 = defending < 4 s after losing it; slot in
// the replay order (GK, FB-L, CB-L, CB-R, FB-R, DM-L, DM-R, AM-L, AM-C, AM-R, ST); axis 0 = u
// (metres from the own goal line), 1 = w (metres from the left touchline); terms
// 1, x, y, x², y², xy, x³, xy² with x = (ub-52.5)/52.5, y = (wb-34)/34.
var teamShapeModel = [4][11][2][8]float64{
`;
for (const phase of PHASES) {
  go += '\t{\n';
  for (let k = 0; k < 11; k++) go += `\t\t{{${coef[phase][k][0].map(fmt).join(', ')}}, {${coef[phase][k][1].map(fmt).join(', ')}}}, // ${SLOT[k]}\n`;
  go += '\t},\n';
}
go += '}\n';
go += `
// teamShapeSpread[phase] = {u, w}: the outfield slots are scaled about their centre by these
// factors (an averaged template is narrower and shorter than the real shape).
var teamShapeSpread = [4][2]float64{
${PHASES.map(p => `\t{${spread[p].map(v => v.toFixed(4)).join(', ')}}, // ${p}`).join('\n')}
}
`;
fs.writeFileSync(out, go);
console.log(`wrote ${out}`);
