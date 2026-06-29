const api = (path, opts = {}) => fetch(path, {
  headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
  ...opts,
}).then(async (r) => {
  if (!r.ok) {
    const err = await r.json().catch(() => ({}));
    throw new Error(err.error || r.statusText);
  }
  if (r.status === 204) return null;
  return r.json();
});

let selectedRecordingId = null;
let ws = null;

function $(id) { return document.getElementById(id); }

function switchTab(name) {
  document.querySelectorAll('.nav-btn').forEach((b) => b.classList.toggle('active', b.dataset.tab === name));
  document.querySelectorAll('.tab').forEach((t) => t.classList.toggle('active', t.id === `tab-${name}`));
  if (name === 'catalog' || name === 'synthesize') loadCatalog();
  if (name === 'recordings') loadRecordings();
  if (name === 'forward') loadForwarder();
}

document.querySelectorAll('.nav-btn').forEach((btn) => {
  btn.addEventListener('click', () => switchTab(btn.dataset.tab));
});

async function refreshStatus() {
  try {
    const s = await api('/api/v1/status');
    $('status-box').textContent =
      `Forward: ${s.forwarder.config.enabled ? 'ON' : 'off'} (${s.forwarder.forwarded_total} sent, ${s.forwarder.forward_errors} err)\n` +
      `Record: ${s.recorder.active ? 'REC ' + s.recorder.recording_id.slice(0, 8) : 'idle'}\n` +
      `Replay: ${s.replay.active ? s.replay.sent + '/' + s.replay.total : 'idle'}\n` +
      `Synth: ${s.synth.active ? s.synth.sent + ' sent' : 'idle'}`;
    $('fwd-stats').textContent = `Forwarded: ${s.forwarder.forwarded_total}, errors: ${s.forwarder.forward_errors}`;
    $('synth-status').textContent = s.synth.active ? `Running, sent ${s.synth.sent}` : 'Idle';
  } catch (e) {
    $('status-box').textContent = 'Status unavailable';
  }
}

function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onopen = () => ws.send(JSON.stringify({ action: 'subscribe', filter: {} }));
  ws.onmessage = (ev) => {
    if ($('live-pause').checked) return;
    const msg = JSON.parse(ev.data);
    addLiveRow(msg);
  };
  ws.onclose = () => setTimeout(connectWS, 2000);
}

function addLiveRow(msg) {
  const tbody = $('live-rows');
  const tr = document.createElement('tr');
  const acct = msg.accounting || {};
  tr.innerHTML = `<td>${new Date(msg.timestamp).toLocaleTimeString()}</td><td>${msg.source}</td><td>${msg.radius.code_name}</td><td>${acct.user_name || ''}</td><td>${acct.mac || ''}</td><td>${acct.nas || ''}</td>`;
  tbody.prepend(tr);
  while (tbody.children.length > 200) tbody.removeChild(tbody.lastChild);
}

$('live-clear').onclick = () => { $('live-rows').innerHTML = ''; };

async function loadForwarder() {
  const cfg = await api('/api/v1/forwarder');
  $('fwd-enabled').checked = cfg.enabled;
  $('fwd-auth').value = cfg.auth_target || '';
  $('fwd-acct').value = cfg.acct_target || '';
}

$('forward-form').onsubmit = async (e) => {
  e.preventDefault();
  await api('/api/v1/forwarder', {
    method: 'PUT',
    body: JSON.stringify({
      enabled: $('fwd-enabled').checked,
      auth_target: $('fwd-auth').value,
      acct_target: $('fwd-acct').value,
    }),
  });
  refreshStatus();
};

async function loadRecordings() {
  const items = await api('/api/v1/recordings');
  $('rec-rows').innerHTML = items.map((r) => `
    <tr>
      <td>${r.name}</td><td>${r.packet_count}</td><td>${new Date(r.started_at).toLocaleString()}</td>
      <td>
        <button data-replay="${r.id}">Replay</button>
        <button data-del="${r.id}" class="danger">Delete</button>
      </td>
    </tr>`).join('');
  $('rec-rows').querySelectorAll('[data-replay]').forEach((b) => {
    b.onclick = () => replayRecording(b.dataset.replay);
  });
  $('rec-rows').querySelectorAll('[data-del]').forEach((b) => {
    b.onclick = async () => { await api(`/api/v1/recordings/${b.dataset.del}`, { method: 'DELETE' }); loadRecordings(); };
  });
}

$('rec-start').onclick = async () => {
  await api('/api/v1/recordings/start', { method: 'POST', body: JSON.stringify({ name: $('rec-name').value || 'recording' }) });
  refreshStatus();
  loadRecordings();
};

$('rec-stop').onclick = async () => {
  await api('/api/v1/recordings/stop', { method: 'POST', body: '{}' });
  refreshStatus();
  loadRecordings();
};

async function replayRecording(id) {
  await api(`/api/v1/recordings/${id}/replay`, {
    method: 'POST',
    body: JSON.stringify({
      rate_rps: parseFloat($('replay-rate').value) || 10,
      preserve_timing: $('replay-timing').checked,
      loop: $('replay-loop').checked,
      mirror_to_stream: true,
    }),
  });
  selectedRecordingId = id;
  refreshStatus();
}

$('replay-stop').onclick = async () => { await api('/api/v1/replay/stop', { method: 'POST', body: '{}' }); refreshStatus(); };

async function loadCatalog() {
  const [nas, clients] = await Promise.all([api('/api/v1/nas'), api('/api/v1/clients')]);
  $('nas-rows').innerHTML = nas.map((n) => `<tr><td>${n.name}</td><td>${n.ip}</td><td>${n.vendor}</td><td><button data-del-nas="${n.id}" class="danger">Delete</button></td></tr>`).join('');
  $('client-rows').innerHTML = clients.map((c) => `<tr><td>${c.mac}</td><td>${c.username}</td><td><button data-del-client="${c.id}" class="danger">Delete</button></td></tr>`).join('');
  $('nas-rows').querySelectorAll('[data-del-nas]').forEach((b) => {
    b.onclick = async () => { await api(`/api/v1/nas/${b.dataset.delNas}`, { method: 'DELETE' }); loadCatalog(); };
  });
  $('client-rows').querySelectorAll('[data-del-client]').forEach((b) => {
    b.onclick = async () => { await api(`/api/v1/clients/${b.dataset.delClient}`, { method: 'DELETE' }); loadCatalog(); };
  });
  $('synth-nas').innerHTML = nas.map((n) => `<option value="${n.id}">${n.name} (${n.ip})</option>`).join('');
  $('synth-clients').innerHTML = clients.map((c) => `<option value="${c.id}">${c.mac}</option>`).join('');
}

$('nas-form').onsubmit = async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  await api('/api/v1/nas', { method: 'POST', body: JSON.stringify(Object.fromEntries(fd)) });
  e.target.reset();
  loadCatalog();
};

$('client-form').onsubmit = async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  await api('/api/v1/clients', { method: 'POST', body: JSON.stringify(Object.fromEntries(fd)) });
  e.target.reset();
  loadCatalog();
};

$('synth-form').onsubmit = async (e) => {
  e.preventDefault();
  const clientIds = Array.from($('synth-clients').selectedOptions).map((o) => o.value);
  await api('/api/v1/scenarios/run', {
    method: 'POST',
    body: JSON.stringify({
      nas_id: $('synth-nas').value,
      client_ids: clientIds,
      pattern: $('synth-pattern').value,
      auth_rate_rps: parseFloat($('synth-rate').value) || 2,
      sessions: parseInt($('synth-sessions').value, 10) || 1,
      interim_interval_sec: parseInt($('synth-interim').value, 10) || 60,
      session_duration_sec: parseInt($('synth-duration').value, 10) || 180,
      mirror_to_stream: true,
    }),
  });
  refreshStatus();
};

$('synth-stop').onclick = async () => { await api('/api/v1/synth/stop', { method: 'POST', body: '{}' }); refreshStatus(); };

connectWS();
loadForwarder();
refreshStatus();
setInterval(refreshStatus, 3000);
