// speed per role group x phase (att/def x settled/trans), live, 4 Hz, speed over 0.5 s: p50, p80, p90, share >4 m/s
// usage: node speedtable.js [--json] replay.js ...
let args=process.argv.slice(2),json=false;if(args[0]==='--json'){json=true;args=args.slice(1)}
const acc={};
for(const f of args){global.window={};const log=console.log;console.log=()=>{};delete require.cache[require.resolve(f)];require(f);const R=window.REPLAY;const dead=require('/Users/csadeveloper/fm/fm-game/viewer/cek/_live.js').deadMask(R);console.log=log;
 const F=R.frames,S=47,T=R.ticks,TPS=R.tps||4;const P=(t,k)=>[F[t*S+3+2*k]/10,F[t*S+4+2*k]/10];const role=k=>(k<11?R.home:R.away).players[k%11].pos;
 let team=-1,since=0;
 for(let t=2;t<T;t++){if(dead[t]||dead[t-2]){team=-1;continue}const bx=F[t*S]/10,by=F[t*S+1]/10,bz=F[t*S+2]/10;let h=-1,hd=.8;if(bz<=.3)for(let k=0;k<22;k++){const[x,y]=P(t,k);if(x<0)continue;const d=Math.hypot(x-bx,y-by);if(d<hd){hd=d;h=k}}
  if(h>=0){const tm=h<11?0:1;if(tm!==team){team=tm;since=t}}if(team<0)continue;const settled=t-since>=4*TPS;
  for(let k=0;k<22;k++){if(k%11===0||k===h)continue;const a=P(t,k),b=P(t-2,k);if(a[0]<0||b[0]<0)continue;const v=Math.hypot(a[0]-b[0],a[1]-b[1])*TPS/2;if(v>12)continue;
   const key=role(k)+' '+((k<11?0:1)===team?'att':'def')+'-'+(settled?'set':'tr');(acc[key]=acc[key]||[]).push(v)}}}
const q=(a,p)=>a[Math.floor(a.length*p)];const out={};
for(const [k,a] of Object.entries(acc)){a.sort((x,y)=>x-y);out[k]={p50:q(a,.5),p80:q(a,.8),p90:q(a,.9),fast:100*a.filter(v=>v>4).length/a.length,n:a.length}}
if(json){console.log(JSON.stringify(out));process.exit(0)}
for(const r of ['CB','FB','DM','AM','ST'])console.log(r.padEnd(3)+' '+['att-set','att-tr','def-set','def-tr'].map(ph=>{const o=out[r+' '+ph];return o?`${ph} ${o.p50.toFixed(1)}/${o.p80.toFixed(1)}/${o.p90.toFixed(1)} >4:${o.fast.toFixed(0)}%`:ph+' -'}).join(' | '));
