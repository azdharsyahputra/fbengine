// turnover.js files... : holder-segment transitions in live play (press.js holder test):
// same-team (pass completed/received) vs team change (turnover), and the gap (s) of a team change:
// short gap (<1 s, a tackle/duel at the ball) vs longer (a pass or loose ball won by the other side)
const path=require('path');let N=0,same=0,chg=0,chgShort=0,chgPass=0,liveMin=0;
for(const file of process.argv.slice(2)){
 global.window={};const log=console.log;console.log=()=>{};require(path.resolve(file));console.log=log;N++;
 const R=window.REPLAY,F=R.frames,S=47,T=R.ticks,tps=R.tps||4;
 const B=t=>[F[t*S]/10,F[t*S+1]/10,F[t*S+2]/10],P=(t,k)=>[F[t*S+3+2*k]/10,F[t*S+4+2*k]/10];
 const dead=require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js').deadMask(R);
 for(let t=0;t<T;t++)if(!dead[t])liveMin+=1/tps/60;
 const H=new Int8Array(T).fill(-1);for(let t=0;t<T;t++){if(dead[t])continue;const[bx,by,bz]=B(t);if(bz>0.3||bx<0)continue;let h=-1,hd=0.8;for(let k=0;k<22;k++){const[x,y]=P(t,k);if(x<0)continue;const d=Math.hypot(x-bx,y-by);if(d<hd){hd=d;h=k}}H[t]=h}
 const seg=[];for(let t=0;t<T;t++){const h=H[t];if(h<0)continue;const s=seg[seg.length-1];if(s&&s.h===h&&t-s.e<=2)s.e=t;else seg.push({h,s:t,e:t})}
 for(let i=1;i<seg.length;i++){const a=seg[i-1],b=seg[i];if(b.s-a.e>16)continue;let d=false;for(let t=a.e;t<=b.s;t++)if(dead[t]){d=true;break}if(d)continue;
  if((a.h<11)===(b.h<11)){if(a.h!==b.h)same++;}else{chg++;const g=(b.s-a.e)/tps;if(g<1)chgShort++;else chgPass++;}}
}
const pm=x=>(x/N).toFixed(0),p90=x=>(x/liveMin*90).toFixed(0);
console.log(`matches ${N} | live min/match ${(liveMin/N).toFixed(0)} | per match: receptions ${pm(same)} | turnovers ${pm(chg)} (gap<1s ${pm(chgShort)}, >=1s ${pm(chgPass)}) | turnover share ${(100*chg/(chg+same)).toFixed(1)}% | per 90 live min: receptions ${p90(same)} turnovers ${p90(chg)}`);
