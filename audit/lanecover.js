// lanecover.js files... : held live samples (every 4th tick): for every teammate of the holder
// 5..30 m away (a passing option), the nearest outfield opponent to the lane holder->mate
// (interior 10%..90%). Share of options with a defender within 1/2 m: how much the defence
// cuts lanes, independent of the passer's choice. Also by option distance band.
const path=require('path');const all=[],near=[],far=[];
for(const file of process.argv.slice(2)){
 global.window={};const log=console.log;console.log=()=>{};require(path.resolve(file));console.log=log;
 const R=window.REPLAY,F=R.frames,S=47,T=R.ticks;
 const B=t=>[F[t*S]/10,F[t*S+1]/10,F[t*S+2]/10],P=(t,k)=>[F[t*S+3+2*k]/10,F[t*S+4+2*k]/10];
 const dead=require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js').deadMask(R);
 for(let t=0;t<T;t+=4){if(dead[t])continue;const[bx,by,bz]=B(t);if(bz>0.3||bx<0)continue;let h=-1,hd=0.8;for(let k=0;k<22;k++){const[x,y]=P(t,k);if(x<0)continue;const d=Math.hypot(x-bx,y-by);if(d<hd){hd=d;h=k}}if(h<0)continue;
  const ta=h<11?0:1,p0=P(t,h);
  for(let k=ta*11+1;k<ta*11+11;k++){if(k===h)continue;const p1=P(t,k);if(p1[0]<0)continue;const dx=p1[0]-p0[0],dy=p1[1]-p0[1],l2=dx*dx+dy*dy,len=Math.sqrt(l2);if(len<5||len>30)continue;
   let m=99;for(let o=(1-ta)*11+1;o<(1-ta)*11+11;o++){const r=P(t,o);if(r[0]<0)continue;const u=((r[0]-p0[0])*dx+(r[1]-p0[1])*dy)/l2;if(u<0.1||u>0.9)continue;m=Math.min(m,Math.hypot(r[0]-(p0[0]+dx*u),r[1]-(p0[1]+dy*u)));}
   all.push(m);(len<15?near:far).push(m);}
 }
}
const sh=(a,x)=>(100*a.filter(v=>v<x).length/a.length).toFixed(0)+'%';
console.log(`options ${all.length} | lane cut within 1 m ${sh(all,1)}, 2 m ${sh(all,2)} | short (5-15 m) 1 m ${sh(near,1)} 2 m ${sh(near,2)} | long (15-30 m) 1 m ${sh(far,1)} 2 m ${sh(far,2)}`);
