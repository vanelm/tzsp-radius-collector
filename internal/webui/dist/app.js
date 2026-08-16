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
let wsWanted = false;
let wsReconnectTimer = null;
let recordingActive = false;
let recordingsPollTimer = null;
let catalogPollTimer = null;
let forwarderPollTimer = null;

/** @type {{id:string, msg:object, tr:HTMLTableRowElement}[]} */
let livePackets = [];
let selectedPacketId = null;
let nextPacketId = 1;
const LIVE_MAX = 200;

/** @type {object[]} */
let fwdConversations = [];
let selectedConvId = null;
let selectedConvFull = null;
let fwdInspectSide = 'request';
let fwdInspectedStatus = '';
let fwdInspectedResp = '';

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

function scheduleForwarderPoll() {
  const shouldPoll = isTabActive('forward');
  if (shouldPoll && !forwarderPollTimer) {
    forwarderPollTimer = setInterval(loadConversations, 1000);
  } else if (!shouldPoll && forwarderPollTimer) {
    clearInterval(forwarderPollTimer);
    forwarderPollTimer = null;
  }
}

function switchTab(name) {
  document.querySelectorAll('.nav-btn').forEach((b) => b.classList.toggle('active', b.dataset.tab === name));
  document.querySelectorAll('.tab').forEach((t) => t.classList.toggle('active', t.id === `tab-${name}`));
  if (name === 'catalog' || name === 'synthesize') loadCatalog();
  if (name === 'recordings') loadRecordings();
  if (name === 'forward') {
    loadForwarder();
    loadConversations();
  }
  scheduleRecordingsPoll();
  scheduleCatalogPoll();
  scheduleForwarderPoll();
  scheduleLiveWS();
}

document.querySelectorAll('.nav-btn').forEach((btn) => {
  btn.addEventListener('click', () => switchTab(btn.dataset.tab));
});

async function refreshStatus() {
  try {
    const s = await api('/api/v1/status');
    recordingActive = s.recorder.active;
    $('status-box').textContent =
      `Forward: ${s.forwarder.config.enabled ? 'ON' : 'off'} (${s.forwarder.forwarded_total} sent, ${s.forwarder.responses_total ?? 0} resp, ${s.forwarder.forward_errors} err)\n` +
      `Record: ${s.recorder.active ? 'REC ' + s.recorder.recording_id.slice(0, 8) : 'idle'}\n` +
      `Replay: ${s.replay.active ? s.replay.sent + '/' + s.replay.total : 'idle'}\n` +
      `Synth: ${s.synth.active ? s.synth.sent + ' sent' : 'idle'}`;
    $('fwd-stats').textContent =
      `Forwarded: ${s.forwarder.forwarded_total}, responses: ${s.forwarder.responses_total ?? 0}, pending: ${s.forwarder.pending ?? 0}, timeouts: ${s.forwarder.timed_out ?? 0}, errors: ${s.forwarder.forward_errors}`;
    const binds = [];
    if (s.forwarder.auth_bind) binds.push(`auth ${s.forwarder.auth_bind}`);
    if (s.forwarder.acct_bind) binds.push(`acct ${s.forwarder.acct_bind}`);
    $('fwd-bind').textContent = binds.length ? `Listening as ${binds.join(', ')}` : '';
    $('synth-status').textContent = s.synth.active ? `Running, sent ${s.synth.sent}` : 'Idle';
    scheduleRecordingsPoll();
  } catch (e) {
    $('status-box').textContent = 'Status unavailable';
  }
}

function liveFeedWanted() {
  return isTabActive('live') && !$('live-pause')?.checked;
}

function disconnectWS() {
  if (wsReconnectTimer) {
    clearTimeout(wsReconnectTimer);
    wsReconnectTimer = null;
  }
  if (ws) {
    ws.onclose = null;
    ws.close();
    ws = null;
  }
}

function scheduleLiveWS() {
  if (liveFeedWanted()) {
    wsWanted = true;
    if (!ws || ws.readyState === WebSocket.CLOSING || ws.readyState === WebSocket.CLOSED) {
      connectWS();
    }
    return;
  }
  wsWanted = false;
  disconnectWS();
}

function connectWS() {
  if (!wsWanted) return;
  disconnectWS();
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onopen = () => ws.send(JSON.stringify({ action: 'subscribe', filter: {} }));
  ws.onmessage = (ev) => {
    if (!liveFeedWanted()) return;
    const msg = JSON.parse(ev.data);
    if (msg.type === 'hello') return;
    addLiveRow(msg);
  };
  ws.onclose = () => {
    ws = null;
    if (wsWanted) wsReconnectTimer = setTimeout(connectWS, 2000);
  };
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

function packetSectionsHTML(msg) {
  const radius = msg.radius || {};
  const acct = msg.accounting || {};
  const snapPairs = [
    ['user', acct.user_name],
    ['MAC', acct.mac || acct.calling_station_id],
    ['NAS', acct.nas],
    ['NAS id', acct.nas_identifier],
    ['RF domain', acct.rf_domain],
    ['proxy', acct.has_proxy_state ? 'yes' : ''],
    ['vendor', acct.nas_vendor],
    ['status', acct.status_type],
    ['session', acct.acct_session_id],
    ['framed IP', acct.framed_ip],
    ['session time', acct.session_time_sec != null ? `${acct.session_time_sec}s` : ''],
    ['in/out octets', (acct.input_octets != null || acct.output_octets != null)
      ? `${acct.input_octets ?? '—'} / ${acct.output_octets ?? '—'}` : ''],
    ['terminate', acct.terminate_cause],
  ];
  return `
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
      <h4>Attributes (${Array.isArray(msg.decoded_attributes) ? msg.decoded_attributes.length : 0})</h4>
      <input type="search" class="attr-filter" placeholder="Filter by name or value…" />
      <table class="attr-table">
        <thead><tr><th>Name</th><th>Value</th></tr></thead>
        <tbody class="attr-rows"></tbody>
      </table>
    </div>
  `;
}

function bindPacketAttrs(root, msg) {
  const attrs = Array.isArray(msg.decoded_attributes) ? msg.decoded_attributes : [];
  const tbody = root.querySelector('.attr-rows');
  if (!tbody) return;
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
  const filter = root.querySelector('.attr-filter');
  if (filter) filter.oninput = (e) => renderAttrs(e.target.value);
}

function renderInspector(msg) {
  const body = $('inspector-body');
  body.innerHTML = packetSectionsHTML(msg);
  bindPacketAttrs(body, msg);
  body.dataset.json = JSON.stringify(msg, null, 2);
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
  if (isTabActive('forward')) {
    handleFwdKeys(e);
    return;
  }
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

function convStatusClass(c) {
  if (c.status === 'complete') {
    if (c.response_code === 3) return 'reject';
    if (c.response_code === 11) return 'challenge';
    if (c.response_code === 2 || c.response_code === 5) return 'accept';
    return 'complete';
  }
  return c.status || 'pending';
}

function rttLabel(c) {
  if (c.rtt_ms == null) return '';
  return `${Number(c.rtt_ms).toFixed(1)} ms`;
}

async function loadConversations() {
  try {
    const items = await api('/api/v1/forwarder/conversations');
    fwdConversations = Array.isArray(items) ? items : [];
  } catch {
    return;
  }
  const tbody = $('fwd-conv-rows');
  tbody.innerHTML = fwdConversations.map((c) => `
    <tr data-conv-id="${esc(c.id)}" class="${c.id === selectedConvId ? 'selected' : ''}">
      <td>${esc(c.started_at ? new Date(c.started_at).toLocaleTimeString() : '')}</td>
      <td>${esc(c.channel || '')}</td>
      <td>${esc(c.request_name || '')}</td>
      <td>${esc(c.response_name || '—')}</td>
      <td>${esc(c.user_name || '')}</td>
      <td>${esc(c.mac || '')}</td>
      <td>${esc(c.nas || c.original_nas || '')}</td>
      <td>${esc(rttLabel(c))}</td>
      <td><span class="status-pill ${esc(convStatusClass(c))}">${esc(c.status || '')}</span></td>
    </tr>
  `).join('') || `<tr><td colspan="9" class="muted">No conversations yet</td></tr>`;
  tbody.querySelectorAll('[data-conv-id]').forEach((tr) => {
    tr.onclick = () => selectConversation(tr.dataset.convId);
  });
  if (selectedConvId) {
    const still = fwdConversations.find((c) => c.id === selectedConvId);
    if (!still) {
      closeFwdInspector();
      return;
    }
    const respName = still.response_name || '';
    if (still.status !== fwdInspectedStatus || respName !== fwdInspectedResp) {
      fwdInspectedStatus = still.status;
      fwdInspectedResp = respName;
      openConversation(still.id);
    }
  }
}

function selectConversation(id) {
  const conv = fwdConversations.find((c) => c.id === id);
  if (!conv) return;
  selectedConvId = id;
  if (fwdInspectSide === 'response' && !conv.response_name) fwdInspectSide = 'request';
  fwdInspectedStatus = conv.status;
  fwdInspectedResp = conv.response_name || '';
  document.querySelectorAll('#fwd-conv-rows tr').forEach((tr) => {
    tr.classList.toggle('selected', tr.dataset.convId === id);
  });
  const panel = $('fwd-inspector');
  panel.classList.remove('hidden');
  panel.setAttribute('aria-hidden', 'false');
  $('fwd-layout').classList.add('inspector-open');
  openConversation(id);
}

async function openConversation(id) {
  try {
    selectedConvFull = await api(`/api/v1/forwarder/conversations/${id}`);
  } catch {
    selectedConvFull = fwdConversations.find((c) => c.id === id) || null;
  }
  if (selectedConvFull && selectedConvId === id) {
    if (fwdInspectSide === 'response' && !selectedConvFull.response) fwdInspectSide = 'request';
    renderFwdInspector(selectedConvFull);
  }
}

function closeFwdInspector() {
  selectedConvId = null;
  selectedConvFull = null;
  fwdInspectedStatus = '';
  fwdInspectedResp = '';
  document.querySelectorAll('#fwd-conv-rows tr').forEach((tr) => tr.classList.remove('selected'));
  const panel = $('fwd-inspector');
  panel.classList.add('hidden');
  panel.setAttribute('aria-hidden', 'true');
  $('fwd-layout')?.classList.remove('inspector-open');
  $('fwd-inspector-body').innerHTML = '';
}

function renderFwdInspector(conv) {
  const body = $('fwd-inspector-body');
  const pkt = fwdInspectSide === 'response' ? conv.response : conv.request;
  body.innerHTML = `
    <div class="inspector-section">
      <h4>Exchange</h4>
      <dl class="inspector-dl">${dlRows([
        ['status', conv.status],
        ['channel', conv.channel],
        ['target', conv.target],
        ['local', conv.local_addr],
        ['identifier', conv.identifier],
        ['rewritten', conv.rewritten ? 'yes (NAS-IP → this host)' : 'no'],
        ['original NAS', conv.original_nas],
        ['source', conv.source],
        ['RTT', rttLabel(conv)],
        ['error', conv.error],
      ])}</dl>
    </div>
    <div class="fwd-pkt-tabs">
      <button type="button" class="btn ${fwdInspectSide === 'request' ? 'active' : ''}" data-side="request">Request</button>
      <button type="button" class="btn ${fwdInspectSide === 'response' ? 'active' : ''}" data-side="response" ${conv.response ? '' : 'disabled'}>Response</button>
    </div>
    <div class="fwd-pkt-view">${pkt ? packetSectionsHTML(pkt) : '<p class="muted">No packet yet</p>'}</div>
  `;
  if (pkt) bindPacketAttrs(body.querySelector('.fwd-pkt-view'), pkt);
  body.querySelectorAll('[data-side]').forEach((btn) => {
    btn.onclick = () => {
      if (btn.disabled) return;
      fwdInspectSide = btn.dataset.side;
      renderFwdInspector(conv);
    };
  });
  body.dataset.json = JSON.stringify(conv, null, 2);
}

function handleFwdKeys(e) {
  if (e.key === 'Escape' && selectedConvId) {
    e.preventDefault();
    closeFwdInspector();
    return;
  }
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
  if (!fwdConversations.length) return;
  e.preventDefault();
  let idx = selectedConvId ? fwdConversations.findIndex((c) => c.id === selectedConvId) : -1;
  if (e.key === 'ArrowDown') {
    idx = idx < 0 ? 0 : Math.min(idx + 1, fwdConversations.length - 1);
  } else {
    idx = idx < 0 ? 0 : Math.max(idx - 1, 0);
  }
  selectConversation(fwdConversations[idx].id);
  document.querySelector(`#fwd-conv-rows tr[data-conv-id="${fwdConversations[idx].id}"]`)?.scrollIntoView({ block: 'nearest' });
}

$('fwd-conv-clear').onclick = async () => {
  await api('/api/v1/forwarder/conversations', { method: 'DELETE' });
  closeFwdInspector();
  loadConversations();
};

$('fwd-inspector-close').onclick = () => closeFwdInspector();

$('fwd-inspector-copy').onclick = async () => {
  const text = $('fwd-inspector-body').dataset.json || '';
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

/** @type {Record<string, object>} */
let nasById = {};
let editingNasId = null;

function resetNasForm() {
  editingNasId = null;
  $('nas-form').reset();
  $('nas-submit').textContent = 'Add NAS';
  $('nas-cancel').hidden = true;
}

function startNasEdit(id) {
  const n = nasById[id];
  if (!n) return;
  editingNasId = id;
  const form = $('nas-form');
  form.elements.name.value = n.name || '';
  form.elements.ip.value = n.ip || '';
  form.elements.secret.value = n.secret || '';
  form.elements.vendor.value = n.vendor || '';
  form.elements.identifier.value = n.identifier || '';
  $('nas-submit').textContent = 'Save NAS';
  $('nas-cancel').hidden = false;
  form.elements.secret.focus();
  loadCatalog();
}

async function loadCatalog() {
  const [nas, clients] = await Promise.all([api('/api/v1/nas'), api('/api/v1/clients')]);
  nasById = Object.fromEntries((nas || []).map((n) => [n.id, n]));
  $('nas-rows').innerHTML = (nas || []).map((n) => {
    const secret = n.secret
      ? `<td class="mono">${esc(n.secret)}</td>`
      : `<td class="muted">—</td>`;
    return `<tr class="${editingNasId === n.id ? 'editing' : ''}">
      <td>${esc(n.name)}</td>
      <td>${esc(n.ip)}</td>
      ${secret}
      <td>${esc(n.vendor)}</td>
      <td class="row-actions">
        <button type="button" class="btn" data-edit-nas="${esc(n.id)}">Edit</button>
        <button type="button" class="btn btn-danger" data-del-nas="${esc(n.id)}">Delete</button>
      </td>
    </tr>`;
  }).join('');
  $('client-rows').innerHTML = clients.map((c) => `<tr><td>${c.mac}</td><td>${c.username}</td><td><button type="button" class="btn btn-danger" data-del-client="${c.id}">Delete</button></td></tr>`).join('');
  $('nas-rows').querySelectorAll('[data-edit-nas]').forEach((b) => {
    b.onclick = () => startNasEdit(b.dataset.editNas);
  });
  $('nas-rows').querySelectorAll('[data-del-nas]').forEach((b) => {
    b.onclick = async () => {
      if (editingNasId === b.dataset.delNas) resetNasForm();
      await api(`/api/v1/nas/${b.dataset.delNas}`, { method: 'DELETE' });
      loadCatalog();
    };
  });
  $('client-rows').querySelectorAll('[data-del-client]').forEach((b) => {
    b.onclick = async () => { await api(`/api/v1/clients/${b.dataset.delClient}`, { method: 'DELETE' }); loadCatalog(); };
  });
  $('synth-nas').innerHTML = nas.map((n) => `<option value="${n.id}">${n.name} (${n.ip})</option>`).join('');
  $('synth-clients').innerHTML = clients.map((c) => `<option value="${c.id}">${c.mac}</option>`).join('');
}

$('nas-cancel').onclick = () => {
  resetNasForm();
  loadCatalog();
};

$('nas-form').onsubmit = async (e) => {
  e.preventDefault();
  const fd = Object.fromEntries(new FormData(e.target));
  if (editingNasId) {
    const prev = nasById[editingNasId] || {};
    await api(`/api/v1/nas/${editingNasId}`, {
      method: 'PUT',
      body: JSON.stringify({
        name: fd.name,
        ip: fd.ip,
        secret: fd.secret,
        vendor: fd.vendor,
        identifier: fd.identifier,
        auth_port: prev.auth_port || 1812,
        acct_port: prev.acct_port || 1813,
        notes: prev.notes || '',
      }),
    });
    resetNasForm();
  } else {
    await api('/api/v1/nas', { method: 'POST', body: JSON.stringify(fd) });
    e.target.reset();
  }
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

loadForwarder();
loadConversations();
refreshStatus();
setInterval(refreshStatus, 3000);
scheduleForwarderPoll();
$('live-pause').addEventListener('change', scheduleLiveWS);
scheduleLiveWS();
