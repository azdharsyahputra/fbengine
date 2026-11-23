// recvzone.js files... (audit/recvzone.js) : press.js receptions split by the receiver's zone (own/mid/final third of
// his team's attack), share of receptions and % pressed (<4 m) per zone; also keeper index check
const path=require('path');
const Z=[0,35,70,200],acc=[0,1,2].map(()=>({n:0,p:0,k:0}));let gkNot0=0;
for(const file of process.argv.slice(2)){
 global.window={};const log=console.log;console.log=()=>{};require(path.resolve(file));console.log=log;
 const R=window.REPLAY,F=R.frames,S=47,T=R.ticks;
 const B=t=>[F[t*S]/10,F[t*S+1]/10,F[t*S+2]/10],P=(t,k)=>[F[t*S+3+2*k]/10,F[t*S+4+2*k]/10];
 const dead=require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js').deadMask(R);
 if(R.home.players[0].pos!=='GK'||R.away.players[0].pos!=='GK')gkNot0++;
 const H=new Int8Array(T).fill(-1);for(let t=0;t<T;t++){const[bx,by,bz]=B(t);if(bz>0.3||bx<0)continue;let h=-1,hd=0.8;for(let k=0;k<22;k++){const[x,y]=P(t,k);if(x<0)continue;const d=Math.hypot(x-bx,y-by);if(d<hd){hd=d;h=k}}H[t]=h}
 const seg=[];for(let t=0;t<T;t++){const h=H[t];if(h<0)continue;const s=seg[seg.length-1];if(s&&s.h===h&&t-s.e<=2)s.e=t;else seg.push({h,s:t,e:t})}
 for(let i=1;i+1<seg.length;i++){const a=seg[i-1],b=seg[i],c=seg[i+1];if(b.s-a.e>16||dead[b.s])continue;if((a.h<11)!==(b.h<11))continue;const team=b.h<11?0:1;
  const q=P(b.s,b.h);let m=99;for(let o=(1-team)*11+1;o<(1-team)*11+11;o++){const r=P(b.s,o);if(r[0]<0)continue;m=Math.min(m,Math.hypot(r[0]-q[0],r[1]-q[1]))}
  // own goal = side of own keeper (index team*11)
  const gk=P(b.s,team*11);const depth=gk[0]<R.pl/2?q[0]:R.pl-q[0];
  const z=depth<35?0:depth<70?1:2;acc[z].n++;if(m<4)acc[z].p++;if((c.h<11?0:1)===team)acc[z].k++;}
}
const N=acc.reduce((s,a)=>s+a.n,0);
console.log(['own','mid','final'].map((nm,i)=>`${nm}: share ${(100*acc[i].n/N).toFixed(0)}% pressed ${(100*acc[i].p/acc[i].n).toFixed(0)}% kept ${(100*acc[i].k/acc[i].n).toFixed(0)}%`).join(' | ')+` | all pressed ${(100*acc.reduce((s,a)=>s+a.p,0)/N).toFixed(0)}% | files with GK not at index 0: ${gkNot0}`);
