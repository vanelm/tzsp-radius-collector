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
let recordingActive = false;
let recordingsPollTimer = null;
let catalogPollTimer = null;

/** @type {{id:string, msg:object, tr:HTMLTableRowElement}[]} */
let livePackets = [];
let selectedPacketId = null;
let nextPacketId = 1;
const LIVE_MAX = 200;

function $(id) { return document.getElementById(id); }

function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

function isTabActive(name) {
  return $(`tab-${name}`)?.classList.contains('active');
}

function scheduleRecordingsPoll() {
  const shouldPoll = isTabActive('recordings') || recordingActive;
  if (shouldPoll && !recordingsPollTimer) {
    recordingsPollTimer = setInterval(loadRecordings, 1000);
  } else if (!shouldPoll && recordingsPollTimer) {
    clearInterval(recordingsPollTimer);
    recordingsPollTimer = null;
  }
}

function scheduleCatalogPoll() {
  const shouldPoll = isTabActive('catalog') || isTabActive('synthesize');
  if (shouldPoll && !catalogPollTimer) {
    catalogPollTimer = setInterval(loadCatalog, 2000);
  } else if (!shouldPoll && catalogPollTimer) {
    clearInterval(catalogPollTimer);
    catalogPollTimer = null;
  }
}

function switchTab(name) {
  document.querySelectorAll('.nav-btn').forEach((b) => b.classList.toggle('active', b.dataset.tab === name));
  document.querySelectorAll('.tab').forEach((t) => t.classList.toggle('active', t.id === `tab-${name}`));
  if (name === 'catalog' || name === 'synthesize') loadCatalog();
  if (name === 'recordings') loadRecordings();
  if (name === 'forward') loadForwarder();
  scheduleRecordingsPoll();
  scheduleCatalogPoll();
}

document.querySelectorAll('.nav-btn').forEach((btn) => {
  btn.addEventListener('click', () => switchTab(btn.dataset.tab));
});

async function refreshStatus() {
  try {
    const s = await api('/api/v1/status');
    recordingActive = s.recorder.active;
    $('status-box').textContent =
      `Forward: ${s.forwarder.config.enabled ? 'ON' : 'off'} (${s.forwarder.forwarded_total} sent, ${s.forwarder.forward_errors} err)\n` +
      `Record: ${s.recorder.active ? 'REC ' + s.recorder.recording_id.slice(0, 8) : 'idle'}\n` +
      `Replay: ${s.replay.active ? s.replay.sent + '/' + s.replay.total : 'idle'}\n` +
      `Synth: ${s.synth.active ? s.synth.sent + ' sent' : 'idle'}`;
    $('fwd-stats').textContent = `Forwarded: ${s.forwarder.forwarded_total}, errors: ${s.forwarder.forward_errors}`;
    $('synth-status').textContent = s.synth.active ? `Running, sent ${s.synth.sent}` : 'Idle';
    scheduleRecordingsPoll();
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
    if (msg.type === 'hello') return;
    addLiveRow(msg);
  };
  ws.onclose = () => setTimeout(connectWS, 2000);
}

function addLiveRow(msg) {
  const tbody = $('live-rows');
  const id = String(nextPacketId++);
  const tr = document.createElement('tr');
  tr.dataset.packetId = id;
  const acct = msg.accounting || {};
  tr.innerHTML =
    `<td>${esc(new Date(msg.timestamp).toLocaleTimeString())}</td>` +
    `<td>${esc(msg.source)}</td>` +
    `<td>${esc(msg.radius?.code_name)}</td>` +
    `<td>${esc(acct.user_name || '')}</td>` +
    `<td>${esc(acct.mac || '')}</td>` +
    `<td>${esc(acct.nas || '')}</td>`;
  tr.addEventListener('click', () => selectPacket(id));
  tbody.prepend(tr);

  livePackets.unshift({ id, msg, tr });
  while (livePackets.length > LIVE_MAX) {
    const old = livePackets.pop();
    if (old.tr.parentNode) old.tr.parentNode.removeChild(old.tr);
    if (selectedPacketId === old.id) {
      selectedPacketId = null;
      closeInspector();
    }
  }
}

function findPacketIndex(id) {
  return livePackets.findIndex((p) => p.id === id);
}

function selectPacket(id) {
  const idx = findPacketIndex(id);
  if (idx < 0) return;
  selectedPacketId = id;
  livePackets.forEach((p) => p.tr.classList.toggle('selected', p.id === id));
  openInspector(livePackets[idx].msg);
}

function openInspector(msg) {
  const panel = $('packet-inspector');
  const layout = document.querySelector('.live-layout');
  panel.classList.remove('hidden');
  panel.setAttribute('aria-hidden', 'false');
  layout.classList.add('inspector-open');
  renderInspector(msg);
}

function closeInspector() {
  selectedPacketId = null;
  livePackets.forEach((p) => p.tr.classList.remove('selected'));
  const panel = $('packet-inspector');
  panel.classList.add('hidden');
  panel.setAttribute('aria-hidden', 'true');
  document.querySelector('.live-layout')?.classList.remove('inspector-open');
  $('inspector-body').innerHTML = '';
}

function dlRows(pairs) {
  return pairs
    .filter(([, v]) => v !== undefined && v !== null && v !== '')
    .map(([k, v]) => `<dt>${esc(k)}</dt><dd>${esc(v)}</dd>`)
    .join('');
}

function renderInspector(msg) {
  const radius = msg.radius || {};
  const acct = msg.accounting || {};
  const attrs = Array.isArray(msg.decoded_attributes) ? msg.decoded_attributes : [];

  const snapPairs = [
    ['user', acct.user_name],
    ['MAC', acct.mac || acct.calling_station_id],
    ['NAS', acct.nas],
    ['NAS id', acct.nas_identifier],
    ['vendor', acct.nas_vendor],
    ['status', acct.status_type],
    ['session', acct.acct_session_id],
    ['framed IP', acct.framed_ip],
    ['session time', acct.session_time_sec != null ? `${acct.session_time_sec}s` : ''],
    ['in/out octets', (acct.input_octets != null || acct.output_octets != null)
      ? `${acct.input_octets ?? '—'} / ${acct.output_octets ?? '—'}` : ''],
    ['terminate', acct.terminate_cause],
  ];

  $('inspector-body').innerHTML = `
    <div class="inspector-section">
      <h4>Capture</h4>
      <dl class="inspector-dl">${dlRows([
        ['time', msg.timestamp ? new Date(msg.timestamp).toLocaleString() : ''],
        ['source', msg.source],
        ['remote', msg.remote_addr],
        ['iface', msg.capture_interface],
        ['src', msg.src_addr],
        ['dst', msg.dst_addr],
      ])}</dl>
    </div>
    <div class="inspector-section">
      <h4>RADIUS</h4>
      <dl class="inspector-dl">${dlRows([
        ['code', radius.code_name != null ? `${radius.code_name} (${radius.code})` : radius.code],
        ['identifier', radius.identifier],
        ['length', radius.length],
        ['authenticator', radius.authenticator],
      ])}</dl>
    </div>
    <div class="inspector-section">
      <h4>Snapshot</h4>
      <dl class="inspector-dl">${dlRows(snapPairs) || '<dt></dt><dd class="muted">—</dd>'}</dl>
    </div>
    <div class="inspector-section">
      <h4>Attributes (${attrs.length})</h4>
      <input type="search" class="attr-filter" id="attr-filter" placeholder="Filter by name or value…" />
      <table class="attr-table">
        <thead><tr><th>Name</th><th>Value</th></tr></thead>
        <tbody id="attr-rows"></tbody>
      </table>
    </div>
  `;

  const tbody = $('attr-rows');
  const renderAttrs = (q) => {
    const needle = (q || '').trim().toLowerCase();
    tbody.innerHTML = attrs
      .filter((a) => {
        if (!needle) return true;
        const hay = `${a.name || ''} ${a.value || ''} ${a.raw || ''} ${a.vendor_name || ''}`.toLowerCase();
        return hay.includes(needle);
      })
      .map((a) => {
        const label = a.is_vsa
          ? `${a.name || 'VSA'}${a.vendor_name ? ` (${a.vendor_name})` : ''}`
          : (a.name || `attr-${a.attr_id}`);
        const val = a.enum_name ? `${a.value} [${a.enum_name}]` : (a.value ?? a.raw ?? '');
        return `<tr class="${a.is_vsa ? 'vsa' : ''}"><td>${esc(label)}</td><td class="attr-value">${esc(val)}</td></tr>`;
      })
      .join('') || `<tr><td colspan="2" class="muted">No attributes</td></tr>`;
  };
  renderAttrs('');
  $('attr-filter').oninput = (e) => renderAttrs(e.target.value);

  $('inspector-body').dataset.json = JSON.stringify(msg, null, 2);
}

$('live-clear').onclick = () => {
  livePackets = [];
  selectedPacketId = null;
  $('live-rows').innerHTML = '';
  closeInspector();
};

$('inspector-close').onclick = () => closeInspector();

$('inspector-copy').onclick = async () => {
  const text = $('inspector-body').dataset.json || '';
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = document.createElement('textarea');
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
  }
};

document.addEventListener('keydown', (e) => {
  if (!isTabActive('live')) return;
  if (e.key === 'Escape' && selectedPacketId) {
    e.preventDefault();
    closeInspector();
    return;
  }
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
  if (!livePackets.length) return;
  e.preventDefault();
  let idx = selectedPacketId ? findPacketIndex(selectedPacketId) : -1;
  if (e.key === 'ArrowDown') {
    idx = idx < 0 ? 0 : Math.min(idx + 1, livePackets.length - 1);
  } else {
    idx = idx < 0 ? 0 : Math.max(idx - 1, 0);
  }
  selectPacket(livePackets[idx].id);
  livePackets[idx].tr.scrollIntoView({ block: 'nearest' });
});

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
        <button type="button" class="btn" data-replay="${r.id}">Replay</button>
        <button type="button" class="btn btn-danger" data-del="${r.id}">Delete</button>
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
  $('nas-rows').innerHTML = nas.map((n) => `<tr><td>${n.name}</td><td>${n.ip}</td><td>${n.vendor}</td><td><button type="button" class="btn btn-danger" data-del-nas="${n.id}">Delete</button></td></tr>`).join('');
  $('client-rows').innerHTML = clients.map((c) => `<tr><td>${c.mac}</td><td>${c.username}</td><td><button type="button" class="btn btn-danger" data-del-client="${c.id}">Delete</button></td></tr>`).join('');
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

$('catalog-export').onclick = async () => {
  const r = await fetch('/api/v1/catalog/export');
  if (!r.ok) {
    const err = await r.json().catch(() => ({}));
    alert(err.error || 'Export failed');
    return;
  }
  const blob = await r.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  a.href = url;
  a.download = `radius-catalog-${stamp}.json`;
  a.click();
  URL.revokeObjectURL(url);
};

$('catalog-import-btn').onclick = () => $('catalog-import-file').click();

$('catalog-import-file').onchange = async (ev) => {
  const file = ev.target.files?.[0];
  ev.target.value = '';
  if (!file) return;
  const status = $('catalog-import-status');
  status.textContent = 'Importing...';
  status.className = 'muted';
  try {
    const text = await file.text();
    const payload = JSON.parse(text);
    if (!payload.mode) {
      payload.mode = $('catalog-import-mode').value;
    }
    const result = await api('/api/v1/catalog/import', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    status.textContent =
      `Imported: NAS +${result.nas_created}/~${result.nas_updated}, clients +${result.clients_created}/~${result.clients_updated}`;
    status.className = 'ok';
    loadCatalog();
  } catch (e) {
    status.textContent = e.message || 'Import failed';
    status.className = 'danger';
  }
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
