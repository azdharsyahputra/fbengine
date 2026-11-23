// node summary.js TAG DIR : mean over seeds of the live-mode cek metrics + probe lines
const fs = require('fs'), path = require('path');
const [tag, dir] = process.argv.slice(2);
const files = fs.readdirSync(dir).filter(f => /^c_s\d+\.txt$/.test(f));
const avg = a => a.length ? a.reduce((x, y) => x + y, 0) / a.length : NaN;
const M = {}; const put = (k, v) => { if (!isNaN(v)) (M[k] = M[k] || []).push(v); };
let legacy = false;
for (const f of files) {
  const t = fs.readFileSync(path.join(dir, f), 'utf8');
  const a = fs.readFileSync(path.join(dir, f.replace('c_', 'a_')), 'utf8');
  const m = fs.readFileSync(path.join(dir, f.replace('c_', 'm_')), 'utf8');
  if (/LEGACY/.test(t)) legacy = true;
  const all = re => [...t.matchAll(re)].map(x => +x[1]), one = (s, re) => +((s.match(re) || [])[1]);
  put('pingpong', one(t, /ping-pong \(A→B→A dalam 6 dtk, B <3 dtk\): (\d+)/));
  put('possMed', one(t, /penguasaan bola hidup: \d+ \| median ([\d.]+)/));
  put('chase3', one(t, /≥3: (\d+)%/));
  put('liar5', one(t, /bola liar: total.*≥5: (\d+)%/));
  put('mundur', avg(all(/mundur (\d+)% \| median/g)));
  put('dmMaju', avg(all(/lanjut umpan maju (\d+)%/g)));
  put('dmAm', avg([...t.matchAll(/serang .*jarak bek-DM -?\d+ DM-AM (-?\d+)/g)].map(x => +x[1])));
  put('press5med', one(t, /lawan dalam 5 m dari pembawa: median (\d+)/));
  put('lockstep', one(t, /lockstep .*?: ([\d.]+)/));
  put('diam', one(t, /sambil diam <0.5 m\/s: (\d+)%/));
  put('own3near', one(t, /sendiri: n\d+ bek terdekat median ([\d.]+) m/));
  put('own3in5', one(t, /sendiri: n\d+ bek terdekat median [\d.]+ m p25 [\d.]+ \| ≥1 dalam 5 m (\d+)%/));
  put('mid3in5', one(t, /tengah: n\d+ bek terdekat median [\d.]+ m p25 [\d.]+ \| ≥1 dalam 5 m (\d+)%/));
  put('buEng', one(a, /M1 engager assigned (\d+)%/));
  put('pLoose5', one(a, /truly loose: runners-at-ball median \d+ p90 \d+ ≥5 ([\d.]+)%/));
  put('pPress2', one(a, /defenders <5 m median \d+ ≥2 (\d+)%/));
  put('pBoxDef', one(a, /defenders in box ([\d.]+)/));
  put('pBoxFree', one(a, /front of box[^:]*: defenders in box [\d.]+ attackers in box [\d.]+ free attackers \(no defender <=3 m\) (\d+)%/));
  const g = m.match(/(\d+)-(\d+)/); if (g) put('goals', +g[1] + +g[2]);
  const fo = m.match(/foul (\d+) vs (\d+)/); if (fo) put('fouls', (+fo[1] + +fo[2]) / 2);
}
const fmt = v => (Math.abs(v) < 2 ? v.toFixed(2) : Math.abs(v) < 20 ? v.toFixed(1) : v.toFixed(0));
console.log(`${tag} [${files.length} seed${legacy ? ', LEGACY!' : ''}] ` + Object.entries(M).map(([k, v]) => `${k} ${fmt(avg(v))}`).join(' | '));
