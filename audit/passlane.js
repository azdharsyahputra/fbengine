// passlane.js files... : every same-team holder change in live play (a completed pass) and every
// team change after >=1 s (a pass won by the other side): at the passer's last held tick, the
// nearest outfield opponent to the segment passer->receiver (interior 10%..90%), and the
// opponent nearest the receiver. Reports shares with a defender within 1/2/3 m of the lane.
const path=require('path');const L=[];const R2=[];let lost=0,all=0;
for(const file of process.argv.slice(2)){
 global.window={};const log=console.log;console.log=()=>{};require(path.resolve(file));console.log=log;
 const R=window.REPLAY,F=R.frames,S=47,T=R.ticks,tps=R.tps||4;
 const B=t=>[F[t*S]/10,F[t*S+1]/10,F[t*S+2]/10],P=(t,k)=>[F[t*S+3+2*k]/10,F[t*S+4+2*k]/10];
 const dead=require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js').deadMask(R);
 const H=new Int8Array(T).fill(-1);for(let t=0;t<T;t++){if(dead[t])continue;const[bx,by,bz]=B(t);if(bz>0.3||bx<0)continue;let h=-1,hd=0.8;for(let k=0;k<22;k++){const[x,y]=P(t,k);if(x<0)continue;const d=Math.hypot(x-bx,y-by);if(d<hd){hd=d;h=k}}H[t]=h}
 const seg=[];for(let t=0;t<T;t++){const h=H[t];if(h<0)continue;const s=seg[seg.length-1];if(s&&s.h===h&&t-s.e<=2)s.e=t;else seg.push({h,s:t,e:t})}
 for(let i=1;i<seg.length;i++){const a=seg[i-1],b=seg[i];if(b.s-a.e>16||b.s-a.e<2)continue;let d=false;for(let t=a.e;t<=b.s;t++)if(dead[t]){d=true;break}if(d)continue;
  const ta=a.h<11?0:1,tb=b.h<11?0:1;if(ta===tb&&a.h===b.h)continue;
  const p0=P(a.e,a.h),p1=ta===tb?P(b.s,b.h):B(b.s);const dx=p1[0]-p0[0],dy=p1[1]-p0[1],l2=dx*dx+dy*dy;const len=Math.sqrt(l2);if(len<5)continue;
  let m=99;for(let o=(1-ta)*11+1;o<(1-ta)*11+11;o++){const r=P(a.e,o);if(r[0]<0)continue;let u=((r[0]-p0[0])*dx+(r[1]-p0[1])*dy)/l2;if(u<0.1||u>0.9)continue;m=Math.min(m,Math.hypot(r[0]-(p0[0]+dx*u),r[1]-(p0[1]+dy*u)));}
  all++;if(ta!==tb)lost++;L.push(m);
 }
}
const sh=x=>(100*L.filter(v=>v<x).length/L.length).toFixed(0)+'%';
console.log(`passes(>=5 m) ${all} | lost ${(100*lost/all).toFixed(1)}% | defender within 1/2/3/5 m of the lane at the strike: ${sh(1)} / ${sh(2)} / ${sh(3)} / ${sh(5)}`);
