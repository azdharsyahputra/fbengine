// node pressz.js replay.js : nearest opponent to the carrier by the carrier's zone (live samples, cek holder test) — real-data comparable pressing
// nearest opponent to the carrier (holder test as cek), live samples only, by the carrier's zone
global.window={};require(process.argv[2]);const R=window.REPLAY,F=R.frames,S=47,T=R.ticks;
const dead=require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js').deadMask(R);
const B=t=>[F[t*S]/10,F[t*S+1]/10,F[t*S+2]/10],P=(t,k)=>[F[t*S+3+2*k]/10,F[t*S+4+2*k]/10];
const L=(t,team)=>P(t,team*11)[0]<52.5;const U=(t,team,x)=>L(t,team)?x:105-x;
const z={};for(let t=0;t<T;t++){if(dead[t])continue;const[bx,by,bz]=B(t);if(bz>0.3)continue;let h=-1,hd=0.8;for(let k=0;k<22;k++){const[x,y]=P(t,k);if(x<0)continue;const d=Math.hypot(x-bx,y-by);if(d<hd){hd=d;h=k}}
 if(h<0||h%11===0)continue;const team=h<11?0:1;const u=U(t,team,bx);const zone=u<35?'sendiri':u<70?'tengah':'akhir';const[cx,cy]=P(t,h);let n=99,c5=0;for(let o=(1-team)*11+1;o<(1-team)*11+11;o++){const[x,y]=P(t,o);if(x<0)continue;const d=Math.hypot(x-cx,y-cy);n=Math.min(n,d);if(d<5)c5++}
 (z[zone]=z[zone]||[]).push([n,c5])}
const out=[];for(const k of ['sendiri','tengah','akhir']){const v=z[k]||[];const d=v.map(x=>x[0]).sort((a,b)=>a-b);out.push(`${k}: n${v.length} bek terdekat median ${d[d.length>>1]?.toFixed(1)} m p25 ${d[d.length>>2]?.toFixed(1)} | ≥1 dalam 5 m ${(100*v.filter(x=>x[1]>=1).length/v.length).toFixed(0)}%`)}
console.log(out.join(' || '));
