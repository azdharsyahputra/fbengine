// node press.js replay.js (live samples via cek/_live.js) — real-data comparable decision metric
global.window={};require(process.argv[2]);const R=window.REPLAY,F=R.frames,S=47,T=R.ticks;
const B=t=>[F[t*S]/10,F[t*S+1]/10,F[t*S+2]/10],P=(t,k)=>[F[t*S+3+2*k]/10,F[t*S+4+2*k]/10];
const dead=require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js').deadMask(R);
const H=new Int8Array(T).fill(-1);for(let t=0;t<T;t++){const[bx,by,bz]=B(t);if(bz>0.3||bx<0)continue;let h=-1,hd=0.8;for(let k=0;k<22;k++){const[x,y]=P(t,k);if(x<0)continue;const d=Math.hypot(x-bx,y-by);if(d<hd){hd=d;h=k}}H[t]=h}
const seg=[];for(let t=0;t<T;t++){const h=H[t];if(h<0)continue;const s=seg[seg.length-1];if(s&&s.h===h&&t-s.e<=2)s.e=t;else seg.push({h,s:t,e:t})}
let n=0,pr=0,kept=0,tot=0;for(let i=1;i+1<seg.length;i++){const a=seg[i-1],b=seg[i],c=seg[i+1];if(b.s-a.e>16||dead[b.s])continue;if((a.h<11)!==(b.h<11))continue;const team=b.h<11?0:1;
 const q=P(b.s,b.h);let m=99;for(let o=(1-team)*11+1;o<(1-team)*11+11;o++){const r=P(b.s,o);if(r[0]<0)continue;m=Math.min(m,Math.hypot(r[0]-q[0],r[1]-q[1]))}n++;if(m<4)pr++;tot++;if(((c.h<11?0:1)===team))kept++}
console.log(`terima ditekan <4m ${(100*pr/n).toFixed(0)}% | lanjut tim sendiri ${(100*kept/tot).toFixed(0)}%`);
