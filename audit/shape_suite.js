#!/usr/bin/env node
// shape_suite.js: team-shape metrics (PLAN-team-shape.md §3, S1-S10 + speeds) for any
// schema-2 replay (engine, Metrica full tracking, SkillCorner broadcast).
//
// Usage: node shape_suite.js [--json] replay.js [replay2.js ...]
//   Several files: per-file values are averaged (each metric is a per-file median or mean).
//
// Definitions (live samples only, via cek/_live.js):
// - holder: nearest player within 0.8 m of a ball at z <= 0.3 m;
//   possession: the last holder's team, reset at every dead sample.
// - phase: settled = the same team in possession for >= 4 s; transition = < 4 s.
// - zone: ball depth in the measured team's own frame: own < 35 m, mid < 70 m, final.
//   For the defending team "own" means the ball is in its own third.
// - Outfielders only (GK excluded); a sample needs all 10 outfielders of the team
//   (frames with players off camera are skipped, so broadcast data counts few frames).
//
// Metrics per team-sample (att = side in possession, def = the other):
// S1 width   y p10-p90 (m)           S2 length  x p10-p90 (m)
// S3 pairs   teammate pairs < 4 m;   nn  nearest-teammate distance (median of the 10)
// S4 lines   gaps between role-group mean depths: back(FB+CB)-DM, DM-AM, AM-ST
// S5 lanes   (att) occupied of 5 width lanes; % with both outer lanes occupied
// S6 backU   back line mean depth from own goal; backRel = back line minus ball depth
// S7 mark    (def) defenders within 3 m of an attacker; attacker->nearest defender median
// S8 shift   (def) team centroid y offset / ball y offset (ball > 10 m off centre)
// S9 support (att) teammates within 15 m of the holder: ahead / level(±3 m) / behind
// S10 recv   receptions (same-team holder change < 5 s) by role, per zone of the receiver
// SPD        outfielder speed shares: <2, 2-4, 4-5.5, 5.5-7, >7 m/s (live samples)
'use strict';
const path = require('path');
const { deadMask } = require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js');

const args = process.argv.slice(2);
const asJson = args[0] === '--json';
const files = asJson ? args.slice(1) : args;
if (!files.length) { console.error('usage: node shape_suite.js [--json] replay.js ...'); process.exit(1); }

const q = (a, p) => { if (!a.length) return NaN; const s = Float64Array.from(a).sort(); return s[Math.min(s.length - 1, Math.floor(s.length * p))]; };
const mean = a => a.length ? a.reduce((x, y) => x + y, 0) / a.length : NaN;

function measure(file) {
  global.window = {};
  const log = console.log; console.log = () => {};
  delete require.cache[require.resolve(path.resolve(file))];
  require(path.resolve(file));
  const R = window.REPLAY, F = R.frames, S = 47, T = R.ticks, tps = R.tps || 4;
  const dead = deadMask(R);
  console.log = log;
  const P = (t, k) => [F[t * S + 3 + 2 * k] / 10, F[t * S + 4 + 2 * k] / 10];
  const B = t => [F[t * S] / 10, F[t * S + 1] / 10, F[t * S + 2] / 10];
  const role = k => (k < 11 ? R.home : R.away).players[k % 11].pos;
  const out = {};
  const bucket = k => (out[k] = out[k] || { w: [], l: [], pairs: [], nn: [], bd: [], da: [], as: [], lanes: [], outer: [], backU: [], backRel: [], mark3: [], markD: [], shift: [], sAhead: [], sLevel: [], sBehind: [], n: 0 });
  const recv = {}, spd = [0, 0, 0, 0, 0];
  let team = -1, since = 0, lastHolder = -1, lastHolderT = -1;
  let used = 0;
  for (let t = 0; t < T; t++) {
    if (dead[t]) { team = -1; lastHolder = -1; continue; }
    const [bx, by, bz] = B(t);
    let h = -1, hd = 0.8;
    if (bz <= 0.3) for (let k = 0; k < 22; k++) { const [x, y] = P(t, k); if (x < 0) continue; const d = Math.hypot(x - bx, y - by); if (d < hd) { hd = d; h = k; } }
    if (h >= 0) {
      const tm = h < 11 ? 0 : 1;
      if (tm !== team) { team = tm; since = t; }
      if (lastHolder >= 0 && h !== lastHolder && (lastHolder < 11) === (h < 11) && t - lastHolderT < 5 * tps) {
        const left = P(t, (h < 11 ? 0 : 11))[0] < 52.5; const u = left ? P(t, h)[0] : 105 - P(t, h)[0];
        const z = u < 35 ? 'own' : u < 70 ? 'mid' : 'final';
        const r = recv[z] = recv[z] || {}; r[role(h)] = (r[role(h)] || 0) + 1;
      }
      lastHolder = h; lastHolderT = t;
    }
    // speeds (every live sample, outfielders)
    if (t > 0 && !dead[t - 1]) for (let k = 0; k < 22; k++) {
      if (k % 11 === 0) continue;
      const a = P(t, k), b = P(t - 1, k); if (a[0] < 0 || b[0] < 0) continue;
      const v = Math.hypot(a[0] - b[0], a[1] - b[1]) * tps;
      if (v > 12) continue; // tracking glitch
      spd[v < 2 ? 0 : v < 4 ? 1 : v < 5.5 ? 2 : v < 7 ? 3 : 4]++;
    }
    if (team < 0 || t % 2) continue; // shape metrics at 2 Hz
    const settled = (t - since) >= 4 * tps;
    const pts = [[], []];
    for (const tm of [0, 1]) for (let k = tm * 11 + 1; k < tm * 11 + 11; k++) { const p = P(t, k); if (p[0] >= 0) pts[tm].push({ k, x: p[0], y: p[1], r: role(k) }); }
    if (pts[0].length < 10 || pts[1].length < 10) continue;
    used++;
    for (const tm of [0, 1]) {
      const me = pts[tm], opp = pts[1 - tm];
      const left = P(t, tm * 11)[0] < 52.5; // keeper's half = own half
      const u = x => left ? x : 105 - x;
      const ub = u(bx);
      const zone = ub < 35 ? 'own' : ub < 70 ? 'mid' : 'final';
      const side = tm === team ? 'att' : 'def';
      const b = bucket(`${side}-${settled ? 'settled' : 'trans'}-${zone}`); b.n++;
      const xs = me.map(p => p.x), ys = me.map(p => p.y);
      b.w.push(q(ys, 0.9) - q(ys, 0.1)); b.l.push(q(xs, 0.9) - q(xs, 0.1));
      let pairs = 0; const nn = me.map((p, i) => { let m = 1e9; me.forEach((o, j) => { if (i !== j) { const d = Math.hypot(p.x - o.x, p.y - o.y); m = Math.min(m, d); if (j > i && d < 4) pairs++; } }); return m; });
      b.pairs.push(pairs); b.nn.push(q(nn, 0.5));
      const grp = g => { const a = me.filter(p => g.includes(p.r)).map(p => u(p.x)); return a.length ? mean(a) : NaN; };
      const back = grp(['FB', 'CB']), dm = grp(['DM']), am = grp(['AM']), st = grp(['ST']);
      b.bd.push(dm - back); b.da.push(am - dm); b.as.push(st - am);
      b.backU.push(back); b.backRel.push(back - ub);
      if (side === 'att') {
        const lanes = new Set(me.map(p => Math.min(4, Math.max(0, Math.floor(p.y / 13.6)))));
        b.lanes.push(lanes.size); b.outer.push(lanes.has(0) && lanes.has(4) ? 1 : 0);
        if (h >= 0 && (h < 11 ? 0 : 1) === tm) {
          const hp = P(t, h); let a = 0, l = 0, be = 0;
          for (const p of me) { if (p.k === h) continue; if (Math.hypot(p.x - hp[0], p.y - hp[1]) > 15) continue; const dx = u(p.x) - u(hp[0]); if (dx > 3) a++; else if (dx < -3) be++; else l++; }
          b.sAhead.push(a); b.sLevel.push(l); b.sBehind.push(be);
        }
      } else {
        let m3 = 0; for (const p of me) if (opp.some(o => Math.hypot(o.x - p.x, o.y - p.y) < 3)) m3++;
        b.mark3.push(m3);
        b.markD.push(q(opp.map(o => Math.min(...me.map(p => Math.hypot(o.x - p.x, o.y - p.y)))), 0.5));
        if (Math.abs(by - 34) > 10) b.shift.push((mean(ys) - 34) / (by - 34));
      }
    }
  }
  const res = { file: path.basename(file), frames: used, groups: {}, recv: {}, speed: {} };
  for (const [k, b] of Object.entries(out)) {
    res.groups[k] = {
      n: b.n, width: q(b.w, 0.5), length: q(b.l, 0.5), pairs: mean(b.pairs), nn: q(b.nn, 0.5),
      backDM: q(b.bd.filter(isFinite), 0.5), dmAM: q(b.da.filter(isFinite), 0.5), amST: q(b.as.filter(isFinite), 0.5),
      backU: q(b.backU, 0.5), backRel: q(b.backRel, 0.5),
      lanes: mean(b.lanes), outer: 100 * mean(b.outer), mark3: mean(b.mark3), markD: q(b.markD, 0.5), shift: q(b.shift, 0.5),
      sAhead: mean(b.sAhead), sLevel: mean(b.sLevel), sBehind: mean(b.sBehind),
    };
  }
  for (const [z, r] of Object.entries(recv)) { const n = Object.values(r).reduce((a, c) => a + c, 0); res.recv[z] = { n }; for (const [k, v] of Object.entries(r)) res.recv[z][k] = 100 * v / n; }
  const ns = spd.reduce((a, c) => a + c, 0) || 1;
  ['<2', '2-4', '4-5.5', '5.5-7', '>7'].forEach((k, i) => res.speed[k] = 100 * spd[i] / ns);
  return res;
}

const all = files.map(measure);
// average across files
const avg = { files: all.map(r => r.file), frames: all.map(r => r.frames), groups: {}, recv: {}, speed: {} };
const keys = new Set(all.flatMap(r => Object.keys(r.groups)));
for (const k of keys) {
  const g = {}; const rs = all.map(r => r.groups[k]).filter(Boolean);
  for (const m of Object.keys(rs[0])) g[m] = mean(rs.map(r => r[m]).filter(v => Number.isFinite(v)));
  avg.groups[k] = g;
}
for (const z of ['own', 'mid', 'final']) {
  const rs = all.map(r => r.recv[z]).filter(Boolean); if (!rs.length) continue;
  const g = {}; for (const role of ['CB', 'FB', 'DM', 'AM', 'ST']) g[role] = mean(rs.map(r => r[role] || 0)); g.n = mean(rs.map(r => r.n)); avg.recv[z] = g;
}
for (const k of ['<2', '2-4', '4-5.5', '5.5-7', '>7']) avg.speed[k] = mean(all.map(r => r.speed[k]));

if (asJson) { console.log(JSON.stringify(avg)); process.exit(0); }
const f1 = v => Number.isFinite(v) ? v.toFixed(1) : '-';
const f0 = v => Number.isFinite(v) ? v.toFixed(0) : '-';
console.log(`files: ${avg.files.join(', ')} | shape frames: ${avg.frames.join(', ')}`);
const order = ['att-settled', 'def-settled', 'att-trans', 'def-trans'];
console.log('group               n   width length pairs  nn  bk-DM DM-AM AM-ST backU bkRel lanes outer% mark3 markD shift  sup a/l/b');
for (const o of order) for (const z of ['own', 'mid', 'final']) {
  const g = avg.groups[`${o}-${z}`]; if (!g) continue;
  const att = o.startsWith('att');
  console.log(`${(o + '-' + z).padEnd(18)} ${f0(g.n).padStart(4)} ${f1(g.width).padStart(6)} ${f1(g.length).padStart(6)} ${g.pairs.toFixed(2).padStart(5)} ${f1(g.nn).padStart(5)} ${f1(g.backDM).padStart(5)} ${f1(g.dmAM).padStart(5)} ${f1(g.amST).padStart(5)} ${f1(g.backU).padStart(5)} ${f1(g.backRel).padStart(5)} ` +
    (att ? `${f1(g.lanes).padStart(5)} ${f0(g.outer).padStart(5)}  ${'-'.padStart(5)} ${'-'.padStart(5)} ${'-'.padStart(5)}  ${f1(g.sAhead)}/${f1(g.sLevel)}/${f1(g.sBehind)}`
      : `${'-'.padStart(5)} ${'-'.padStart(5)}  ${f1(g.mark3).padStart(5)} ${f1(g.markD).padStart(5)} ${f1(g.shift).padStart(5)}`));
}
console.log('receptions by role (%): ' + Object.entries(avg.recv).map(([z, g]) => `${z}: ` + ['CB', 'FB', 'DM', 'AM', 'ST'].map(r => `${r} ${f0(g[r])}`).join(' ')).join(' | '));
console.log('speed shares (%): ' + Object.entries(avg.speed).map(([k, v]) => `${k} ${f1(v)}`).join(' | '));
