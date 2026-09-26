const $ = selector => document.querySelector(selector);

const bytes = number => {
  if (!Number.isFinite(number) || number < 0) return '—';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let index = 0;
  while (number >= 1024 && index < units.length - 1) {
    number /= 1024;
    index++;
  }
  return `${number.toFixed(index ? 2 : 0)} ${units[index]}`;
};

const esc = value => String(value).replace(/[&<>"']/g, character => ({
  '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
})[character]);

const formatDate = date => new Intl.DateTimeFormat('zh-CN', {
  month: 'long', day: 'numeric', weekday: 'short', timeZone: 'UTC'
}).format(new Date(`${date}T00:00:00Z`));

let overview = null;
let history = [];
let selectedDate = '';
let rankingMode = 'total';

function dayTotal(day) {
  return Object.values(day?.users || {}).reduce((sum, traffic) => ({
    Up: sum.Up + traffic.Up,
    Down: sum.Down + traffic.Down
  }), { Up: 0, Down: 0 });
}

function selectDay(date) {
  selectedDate = date;
  rankingMode = 'day';
  renderChart();
  renderRanking();
}

function renderChart() {
  const days = history.slice(-14);
  if (!days.length) {
    $('#chart').innerHTML = '<div class="empty">暂无历史数据</div>';
    $('#day-detail').innerHTML = '<span>选择一天查看详情</span>';
    return;
  }

  if (!days.some(day => day.date === selectedDate)) selectedDate = days.at(-1).date;
  const totals = days.map(day => {
    const traffic = dayTotal(day);
    return traffic.Up + traffic.Down;
  });
  const peak = Math.max(1, ...totals);

  $('#chart').innerHTML = days.map((day, index) => {
    const selected = day.date === selectedDate;
    const label = `${formatDate(day.date)}，流量 ${bytes(totals[index])}`;
    return `<button class="bar-wrap${selected ? ' selected' : ''}" type="button" data-date="${day.date}" aria-label="${label}" aria-pressed="${selected}">
      <span class="bar" style="height:${Math.max(2, totals[index] / peak * 135)}px"></span>
      <span class="bar-label">${day.date.slice(5)}</span>
    </button>`;
  }).join('');

  document.querySelectorAll('.bar-wrap').forEach(button => {
    button.addEventListener('click', () => selectDay(button.dataset.date));
  });

  const selected = days.find(day => day.date === selectedDate);
  const traffic = dayTotal(selected);
  $('#day-detail').innerHTML = `<div><strong>${formatDate(selected.date)}</strong><span>所选日期</span></div>
    <b>${bytes(traffic.Up + traffic.Down)}</b>
    <small>↑ ${bytes(traffic.Up)} · ↓ ${bytes(traffic.Down)}</small>`;
}

function rankingUsers() {
  if (rankingMode === 'total') return overview?.users || [];
  const day = history.find(item => item.date === selectedDate);
  return Object.entries(day?.users || {}).map(([name, traffic]) => ({
    name,
    up: traffic.Up,
    down: traffic.Down,
    total: traffic.Up + traffic.Down
  })).sort((left, right) => right.total - left.total);
}

function renderRanking() {
  const users = rankingUsers();
  const isDaily = rankingMode === 'day';
  $('#rank-total').classList.toggle('active', !isDaily);
  $('#rank-day').classList.toggle('active', isDaily);
  $('#rank-day').disabled = !history.length;
  $('#ranking-period').textContent = isDaily && selectedDate ? formatDate(selectedDate) : '全部时间';
  $('#count').textContent = `${users.length} 位用户`;

  if (!users.length) {
    $('#users').innerHTML = `<div class="empty">${isDaily ? '这一天暂无用户流量' : '等待第一笔流量…'}</div>`;
    return;
  }

  const maximum = Math.max(1, ...users.map(user => user.total));
  $('#users').innerHTML = users.map(user => `<div class="user">
    <div class="user-name" title="${esc(user.name)}">${esc(user.name)}</div>
    <div class="progress"><i style="width:${user.total / maximum * 100}%"></i></div>
    <div class="numbers"><b>${bytes(user.total)}</b><small>↑ ${bytes(user.up)} · ↓ ${bytes(user.down)}</small></div>
  </div>`).join('');
}

async function refresh() {
  try {
    [overview, history] = await Promise.all([
      fetch('/api/overview').then(response => response.json()),
      fetch('/api/history').then(response => response.json())
    ]);
    if (!Array.isArray(history)) history = [];

    $('#total').textContent = bytes(overview.total.Up + overview.total.Down);
    $('#up').textContent = bytes(overview.total.Up);
    $('#down').textContent = bytes(overview.total.Down);
    $('#updated').textContent = overview.updated_at && !overview.updated_at.startsWith('0001')
      ? `更新于 ${new Date(overview.updated_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}`
      : '暂无数据';

    const health = $('#health');
    health.className = `health ${overview.collector.connected ? 'ok' : 'bad'}`;
    health.querySelector('span').textContent = overview.collector.connected ? '已连接' : '未连接';
    health.title = overview.collector.error || overview.collector.grpc;

    renderChart();
    renderRanking();
  } catch (error) {
    const health = $('#health');
    health.className = 'health bad';
    health.querySelector('span').textContent = '面板异常';
  }
}

$('#rank-total').addEventListener('click', () => {
  rankingMode = 'total';
  renderRanking();
});

$('#rank-day').addEventListener('click', () => {
  rankingMode = 'day';
  renderRanking();
});

refresh();
setInterval(refresh, 5000);
