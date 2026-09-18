const $=s=>document.querySelector(s);
const bytes=n=>{if(!Number.isFinite(n)||n<0)return '—';const u=['B','KB','MB','GB','TB','PB'];let i=0;while(n>=1024&&i<u.length-1){n/=1024;i++}return `${n.toFixed(i?2:0)} ${u[i]}`};
const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
async function refresh(){
  try{
    const [o,h]=await Promise.all([fetch('/api/overview').then(r=>r.json()),fetch('/api/history').then(r=>r.json())]);
    $('#total').textContent=bytes(o.total.Up+o.total.Down); $('#up').textContent=bytes(o.total.Up); $('#down').textContent=bytes(o.total.Down);
    $('#count').textContent=`${o.users.length} 位用户`; $('#updated').textContent=o.updated_at&&!o.updated_at.startsWith('0001')?`更新于 ${new Date(o.updated_at).toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'})}`:'暂无数据';
    const health=$('#health'); health.className='health '+(o.collector.connected?'ok':'bad'); health.querySelector('span').textContent=o.collector.connected?'已连接':'未连接'; health.title=o.collector.error||o.collector.grpc;
    const max=Math.max(1,...o.users.map(x=>x.total)); $('#users').innerHTML=o.users.length?o.users.map(x=>`<div class="user"><div class="user-name" title="${esc(x.name)}">${esc(x.name)}</div><div class="progress"><i style="width:${x.total/max*100}%"></i></div><div class="numbers"><b>${bytes(x.total)}</b><small>↑ ${bytes(x.up)} · ↓ ${bytes(x.down)}</small></div></div>`).join(''):'<div class="empty">等待第一笔流量…</div>';
    const days=h.slice(-14), totals=days.map(d=>Object.values(d.users||{}).reduce((a,x)=>a+x.Up+x.Down,0)), peak=Math.max(1,...totals);
    $('#chart').innerHTML=days.length?days.map((d,i)=>`<div class="bar-wrap" title="${d.date} · ${bytes(totals[i])}"><div class="bar" style="height:${Math.max(2,totals[i]/peak*135)}px"></div><label>${d.date.slice(5)}</label></div>`).join(''):'<div class="empty">暂无历史数据</div>';
  }catch(e){const h=$('#health');h.className='health bad';h.querySelector('span').textContent='面板异常';}
}
refresh();setInterval(refresh,5000);
