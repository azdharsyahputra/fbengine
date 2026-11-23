// node zone.js replay.js (live samples via cek/_live.js) — real-data comparable decision metric
// backward/forward share by zone (u0) and pass length, same pass definition as mid.js
global.window={};require(process.argv[2]);const R=window.REPLAY,F=R.frames,S=47,T=R.ticks;
const B=t=>[F[t*S]/10,F[t*S+1]/10,F[t*S+2]/10],P=(t,k)=>[F[t*S+3+2*k]/10,F[t*S+4+2*k]/10];
const dead=require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js').deadMask(R);
const H=new Int8Array(T).fill(-1);for(let t=0;t<T;t++){if(dead[t])continue;const[bx,by,bz]=B(t);if(bz>0.3||bx<0)continue;let h=-1,hd=0.8;for(let k=0;k<22;k++){const[x,y]=P(t,k);if(x<0)continue;const d=Math.hypot(x-bx,y-by);if(d<hd){hd=d;h=k}}H[t]=h}
const seg=[];for(let t=0;t<T;t++){const h=H[t];if(h<0)continue;const s=seg[seg.length-1];if(s&&s.h===h&&t-s.e<=2)s.e=t;else seg.push({h,s:t,e:t})}
const L=(t,team)=>P(t,team*11)[0]<52.5;const U=(t,team,x)=>L(t,team)?x:105-x;
const z={};const add=(k,v)=>{z[k]=z[k]||{n:0,f:0,b:0,short:0,shortb:0};z[k].n++;if(v.d>3)z[k].f++;if(v.d<-3)z[k].b++;if(v.len<8){z[k].short++;if(v.d<-3)z[k].shortb++}};
for(let i=1;i<seg.length;i++){const a=seg[i-1],b=seg[i];if(b.s-a.e>16)continue;if((a.h<11)!==(b.h<11))continue;const team=a.h<11?0:1;const p0=B(a.e),p1=B(b.s);
 const u0=U(a.e,team,p0[0]),u1=U(b.s,team,p1[0]);const v={d:u1-u0,len:Math.hypot(p1[0]-p0[0],p1[1]-p0[1])};
 add(u0<35?'1/3 sendiri':u0<70?'1/3 tengah':'1/3 akhir',v);add('semua',v)}
console.log(Object.entries(z).map(([k,v])=>`${k}: n${v.n} maju ${(100*v.f/v.n).toFixed(0)}% mundur ${(100*v.b/v.n).toFixed(0)}% | pendek<8m ${(100*v.short/v.n).toFixed(0)}% (mundur ${(100*v.shortb/Math.max(1,v.short)).toFixed(0)}%)`).join('\n'));
