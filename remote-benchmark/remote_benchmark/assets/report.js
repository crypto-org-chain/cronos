const fieldTooltip=document.getElementById('fieldTooltip');
function showFieldTooltip(target) {
  fieldTooltip.textContent=target.dataset.tooltip; fieldTooltip.style.display='block';
  const targetRect=target.getBoundingClientRect(), tipRect=fieldTooltip.getBoundingClientRect(), gap=7;
  const left=Math.max(8,Math.min(targetRect.left+(targetRect.width-tipRect.width)/2,window.innerWidth-tipRect.width-8));
  const above=targetRect.top-tipRect.height-gap;
  fieldTooltip.style.left=left+'px';
  fieldTooltip.style.top=(above>=8?above:targetRect.bottom+gap)+'px';
}
function hideFieldTooltip() { fieldTooltip.style.display='none'; }
document.querySelectorAll('.field-help').forEach(target=>{
  target.addEventListener('mouseenter',()=>showFieldTooltip(target));
  target.addEventListener('mouseleave',hideFieldTooltip);
  target.addEventListener('focus',()=>showFieldTooltip(target));
  target.addEventListener('blur',hideFieldTooltip);
  target.addEventListener('click',()=>showFieldTooltip(target));
  target.addEventListener('keydown',event=>{ if(event.key==='Escape') { hideFieldTooltip(); target.blur(); } });
});
window.addEventListener('scroll',hideFieldTooltip,true);
window.addEventListener('resize',hideFieldTooltip);
function createBarChart(canvasId,tooltipId,valueKey,yLabel,color,tooltipText) {
  const canvas=document.getElementById(canvasId), wrap=canvas.parentElement;
  const tip=document.getElementById(tooltipId), ctx=canvas.getContext('2d');
  let bars=[];
  function draw() {
    const dpr=window.devicePixelRatio||1, rect=canvas.getBoundingClientRect();
    canvas.width=Math.max(1,Math.round(rect.width*dpr)); canvas.height=Math.max(1,Math.round(rect.height*dpr));
    ctx.setTransform(dpr,0,0,dpr,0,0); ctx.clearRect(0,0,rect.width,rect.height); bars=[];
    if(!data.length) { ctx.fillStyle='#66717a'; ctx.textAlign='center'; ctx.fillText('No block data recorded',rect.width/2,rect.height/2); return; }
    const max=Math.max(1,...data.map(d=>d[valueKey]));
    ctx.strokeStyle='#d8dee3'; ctx.fillStyle='#66717a'; ctx.font='12px system-ui'; ctx.lineWidth=1;
    const tickLabels=Array.from({length:5},(_,i)=>Math.round(max*i/4).toLocaleString());
    const maxTickWidth=Math.max(...tickLabels.map(label=>ctx.measureText(label).width));
    const pad={l:Math.max(76,Math.ceil(maxTickWidth)+36),r:20,t:20,b:48};
    const w=rect.width-pad.l-pad.r, h=rect.height-pad.t-pad.b;
    tickLabels.forEach((label,i)=>{ const y=pad.t+h-h*i/4; ctx.beginPath(); ctx.moveTo(pad.l,y); ctx.lineTo(pad.l+w,y); ctx.stroke(); ctx.textAlign='right'; ctx.fillText(label,pad.l-9,y+4); });
    const slot=w/data.length, bw=Math.max(1,Math.min(28,slot*.72));
    data.forEach((d,i)=>{ const bh=h*d[valueKey]/max, x=pad.l+slot*i+(slot-bw)/2, y=pad.t+h-bh;
      ctx.fillStyle=color; ctx.fillRect(x,y,bw,bh); bars.push({x,y,w:bw,h:bh,d}); });
    const ticks=Math.min(8,data.length); ctx.fillStyle='#66717a'; ctx.textAlign='center';
    for(let i=0;i<ticks;i++) { const idx=ticks===1?0:Math.round(i*(data.length-1)/(ticks-1)); ctx.fillText(data[idx].height,pad.l+slot*idx+slot/2,pad.t+h+20); }
    ctx.save(); ctx.translate(16,pad.t+h/2); ctx.rotate(-Math.PI/2); ctx.fillText(yLabel,0,0); ctx.restore();
    ctx.fillText('Block height',pad.l+w/2,rect.height-8);
  }
  canvas.addEventListener('mousemove',e=>{ const r=canvas.getBoundingClientRect(), x=e.clientX-r.left, y=e.clientY-r.top;
    const hit=bars.find(b=>x>=b.x&&x<=b.x+b.w&&y>=Math.min(b.y,b.y+b.h)&&y<=b.y+b.h);
    if(!hit) { tip.style.display='none'; return; }
    tip.textContent=tooltipText(hit.d); tip.style.display='block';
    tip.style.left=Math.min(x+12,wrap.clientWidth-tip.offsetWidth-8)+'px'; tip.style.top=Math.max(8,y-tip.offsetHeight-8)+'px';
  });
  canvas.addEventListener('mouseleave',()=>tip.style.display='none');
  new ResizeObserver(draw).observe(wrap); draw();
}
createBarChart('chart','tooltip','transactions','Transactions','#ff5a5f',d=>
  `Block ${d.height}: ${d.transactions.toLocaleString()} txs, ${d.tps.toLocaleString()} TPS`);
createBarChart('gasChart','gasTooltip','gas_consumed','Gas consumed','#087e8b',d=>
  `Block ${d.height}: ${d.gas_consumed.toLocaleString()} gas consumed`);
function createSecondChart(canvasId,tooltipId,valueKey,yLabel,color,rolling=false) {
  const canvas=document.getElementById(canvasId), wrap=canvas.parentElement;
  const tip=document.getElementById(tooltipId), ctx=canvas.getContext('2d');
  let points=[];
  function draw() {
    const dpr=window.devicePixelRatio||1, rect=canvas.getBoundingClientRect();
    canvas.width=Math.max(1,Math.round(rect.width*dpr)); canvas.height=Math.max(1,Math.round(rect.height*dpr));
    ctx.setTransform(dpr,0,0,dpr,0,0); ctx.clearRect(0,0,rect.width,rect.height); points=[];
    if(!secondData.length) { ctx.fillStyle='#66717a'; ctx.textAlign='center'; ctx.fillText('No timestamped transaction data recorded',rect.width/2,rect.height/2); return; }
    const values=secondData.flatMap(d=>rolling?[d[valueKey],d.rolling_tps_5s]:[d[valueKey]]);
    const max=Math.max(1,...values), tickLabels=Array.from({length:5},(_,i)=>Math.round(max*i/4).toLocaleString());
    ctx.font='12px system-ui'; const maxTickWidth=Math.max(...tickLabels.map(label=>ctx.measureText(label).width));
    const pad={l:Math.max(76,Math.ceil(maxTickWidth)+36),r:20,t:20,b:48}, w=rect.width-pad.l-pad.r, h=rect.height-pad.t-pad.b;
    ctx.strokeStyle='#d8dee3'; ctx.fillStyle='#66717a'; ctx.lineWidth=1;
    tickLabels.forEach((label,i)=>{ const y=pad.t+h-h*i/4; ctx.beginPath(); ctx.moveTo(pad.l,y); ctx.lineTo(pad.l+w,y); ctx.stroke(); ctx.textAlign='right'; ctx.fillText(label,pad.l-9,y+4); });
    const slot=w/secondData.length, bw=Math.max(1,Math.min(32,slot*.72));
    secondData.forEach((d,i)=>{ const bh=h*d[valueKey]/max, x=pad.l+slot*i+(slot-bw)/2, y=pad.t+h-bh;
      ctx.fillStyle=color; ctx.fillRect(x,y,bw,bh); points.push({x:pad.l+slot*i+slot/2,y,d}); });
    if(rolling) { ctx.beginPath(); ctx.strokeStyle='#182026'; ctx.lineWidth=2;
      secondData.forEach((d,i)=>{ const x=pad.l+slot*i+slot/2, y=pad.t+h-h*d.rolling_tps_5s/max; i?ctx.lineTo(x,y):ctx.moveTo(x,y); }); ctx.stroke();
      ctx.fillStyle='#182026'; ctx.fillRect(pad.l+8,pad.t+4,18,2); ctx.fillText('5-second moving average',pad.l+32,pad.t+9);
    }
    const ticks=Math.min(8,secondData.length); ctx.fillStyle='#66717a'; ctx.textAlign='center';
    for(let i=0;i<ticks;i++) { const idx=ticks===1?0:Math.round(i*(secondData.length-1)/(ticks-1)); ctx.fillText(secondData[idx].elapsed_second+'s',pad.l+slot*idx+slot/2,pad.t+h+20); }
    ctx.save(); ctx.translate(16,pad.t+h/2); ctx.rotate(-Math.PI/2); ctx.fillText(yLabel,0,0); ctx.restore();
    ctx.fillText('Elapsed time from first committed transaction',pad.l+w/2,rect.height-8);
  }
  canvas.addEventListener('mousemove',e=>{ const r=canvas.getBoundingClientRect(), x=e.clientX-r.left;
    const hit=points.reduce((best,p)=>!best||Math.abs(p.x-x)<Math.abs(best.x-x)?p:best,null); if(!hit) return;
    const d=hit.d, when=new Date(d.timestamp).toLocaleTimeString();
    tip.textContent=valueKey==='transactions'
      ? `${d.elapsed_second}s (${when}): ${d.transactions.toLocaleString()} TPS; 5s avg ${d.rolling_tps_5s.toLocaleString(undefined,{maximumFractionDigits:1})} TPS`
      : `${d.elapsed_second}s (${when}): ${d.gas_consumed.toLocaleString()} gas`;
    tip.style.display='block'; tip.style.left=Math.min(x+12,wrap.clientWidth-tip.offsetWidth-8)+'px'; tip.style.top='8px';
  });
  canvas.addEventListener('mouseleave',()=>tip.style.display='none');
  new ResizeObserver(draw).observe(wrap); draw();
}
createSecondChart('secondChart','secondTooltip','transactions','Transactions / second','#ff5a5f',true);
createSecondChart('secondGasChart','secondGasTooltip','gas_consumed','Gas / second','#087e8b');
