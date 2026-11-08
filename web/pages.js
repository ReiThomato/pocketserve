'use strict';

/* ============================================================
   Páginas: início, compartilhamentos, atividade, acesso, ajustes
   ============================================================ */

function fileKind(name, isDir) {
  if (isDir) return 'folder';
  const ext = (name.split('.').pop() || '').toLowerCase();
  if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'svg', 'heic', 'heif', 'avif', 'ico'].includes(ext)) return 'image';
  if (['mp4', 'mkv', 'webm', 'mov', 'm4v', '3gp', 'avi', 'ts', 'flv', 'wmv'].includes(ext)) return 'video';
  if (['mp3', 'm4a', 'aac', 'wav', 'flac', 'ogg', 'opus', 'amr', 'mid', 'midi', 'wma'].includes(ext)) return 'audio';
  if (ext === 'pdf') return 'pdf';
  if (['zip', 'rar', '7z', 'tar', 'gz', 'bz2', 'xz', 'apk', 'aab', 'iso', 'zst'].includes(ext)) return 'archive';
  if (['txt', 'md', 'json', 'js', 'mjs', 'tsx', 'jsx', 'css', 'html', 'htm', 'xml', 'yaml', 'yml', 'ini', 'conf', 'cfg', 'log', 'csv', 'srt', 'vtt', 'go', 'py', 'java', 'kt', 'sh', 'bash', 'c', 'cpp', 'h', 'hpp', 'sql', 'rs', 'toml', 'env', 'properties'].includes(ext)) return 'text';
  if (['doc', 'docx', 'odt', 'rtf', 'epub'].includes(ext)) return 'doc';
  if (['xls', 'xlsx', 'ods'].includes(ext)) return 'sheet';
  if (['ppt', 'pptx', 'odp'].includes(ext)) return 'slides';
  return 'file';
}

function kindIcon(kind) {
  return ({ folder: 'folder', image: 'image', video: 'video', audio: 'audio', pdf: 'pdf', archive: 'archive', text: 'text', doc: 'doc', sheet: 'sheet', slides: 'slides', file: 'file' })[kind] || 'file';
}

function fileBadge(kind, name) {
  const fixed = { folder: 'DIR', image: 'IMG', video: 'VID', audio: 'AUD', pdf: 'PDF', archive: 'ZIP', text: 'TXT', doc: 'DOC', sheet: 'XLS', slides: 'PPT' };
  if (fixed[kind]) return fixed[kind];
  let ext = (name.split('.').pop() || 'FILE').toUpperCase();
  if (ext.length > 4 || ext === name.toUpperCase()) ext = ext.slice(0, 4);
  return ext || 'FILE';
}

const ACT_LABELS = {
  login: { label: 'Entrou no painel', icon: 'shield', ok: true },
  login_fail: { label: 'Tentativa de acesso falhou', icon: 'lock', bad: true },
  setup: { label: 'Administrador configurado', icon: 'shield', ok: true },
  logout: { label: 'Saiu do painel', icon: 'logout', ok: true },
  password: { label: 'Senha alterada', icon: 'key', ok: true },
  upload: { label: 'Enviou arquivo', icon: 'upload', ok: true },
  download: { label: 'Baixou arquivo', icon: 'download', ok: true },
  download_zip: { label: 'Baixou pasta (.zip)', icon: 'archive', ok: true },
  mkdir: { label: 'Criou pasta', icon: 'folderPlus', ok: true },
  rename: { label: 'Renomeou item', icon: 'edit', ok: true },
  delete: { label: 'Excluiu item', icon: 'trash', ok: true },
  move: { label: 'Moveu item', icon: 'move', ok: true },
  copy: { label: 'Copiou item', icon: 'copy', ok: true },
  save: { label: 'Editou arquivo', icon: 'edit', ok: true },
  unzip: { label: 'Extraiu arquivo ZIP', icon: 'archive', ok: true },
  share_create: { label: 'Criou link de compartilhamento', icon: 'share', ok: true },
  share_revoke: { label: 'Revogou link', icon: 'x', ok: true },
  share_download: { label: 'Download por link', icon: 'download', ok: true },
  share_unlock_fail: { label: 'Senha de link incorreta', icon: 'lock', bad: true },
  settings: { label: 'Alterou configurações', icon: 'settings', ok: true },
  restart: { label: 'Reiniciou o servidor', icon: 'refresh', ok: true }
};

/* ============================ dashboard ============================ */

function renderDashboard() {
  const page = $('#page-dashboard');
  if (!page.dataset.ready) {
    page.innerHTML = `
      <div class="page-head">
        <div><h1>Visão geral</h1><p class="muted" id="dash-greeting">Aqui está o estado do seu servidor.</p></div>
        <div class="head-actions">
          <button class="btn ghost" data-act="qr-access">${ico('qr')} Acesso rápido</button>
          <button class="btn primary" data-act="upload">${ico('upload')} Enviar arquivos</button>
        </div>
      </div>

      <section class="hero">
        <div class="hero-main">
          <span class="pill" id="hero-online"><span class="dot"></span> Servidor online</span>
          <h2 id="hero-name">PocketServe</h2>
          <p id="hero-sub">Carregando informações do aparelho…</p>
        </div>
        <div class="hero-side">
          <div class="hero-metric"><strong id="hero-free">—</strong><span>livres</span></div>
          <div class="hero-metric"><strong id="hero-batt">—</strong><span>bateria</span></div>
          <div class="hero-orb">${ico('cloud')}</div>
        </div>
      </section>

      <div class="stats-grid">
        <article class="card stat">
          <div class="stat-head"><span class="stat-ico">${ico('disk')}</span> Armazenamento</div>
          <div class="stat-value" id="st-storage">—</div>
          <div class="bar"><span id="bar-storage" style="width:0%"></span></div>
          <div class="stat-foot"><span id="st-storage-free">—</span><span id="st-storage-pct">—</span></div>
          <div class="vol-list" id="vol-list"></div>
          <div class="guard-note" id="st-reserve-note" hidden></div>
        </article>
        <article class="card stat">
          <div class="stat-head"><span class="stat-ico orange">${ico('battery')}</span> Bateria</div>
          <div class="battery-row">
            <div class="gauge" id="batt-gauge"><b><span id="batt-level">—</span><small>%</small></b></div>
            <div class="battery-meta"><strong id="batt-status">—</strong><span id="batt-temp">—</span><span id="batt-plug">—</span></div>
          </div>
          <div class="stat-foot"><span>Saúde do aparelho</span><span id="batt-note"></span></div>
        </article>
        <article class="card stat">
          <div class="stat-head"><span class="stat-ico violet">${ico('memory')}</span> Memória</div>
          <div class="stat-value" id="st-mem">—</div>
          <div class="bar pink"><span id="bar-mem" style="width:0%"></span></div>
          <div class="stat-foot"><span id="st-mem-free">—</span><span id="st-mem-pct">—</span></div>
        </article>
        <article class="card stat">
          <div class="stat-head"><span class="stat-ico pink">${ico('wifi')}</span> Rede</div>
          <div class="net-list" id="net-list"><span class="tiny">Detectando…</span></div>
          <div class="stat-foot"><span id="net-count">—</span><span class="chip" style="padding:2px 7px">porta <b id="net-port">—</b></span></div>
        </article>
      </div>

      <div class="dash-grid">
        <section class="card">
          <div class="card-title"><h3>Ações rápidas</h3></div>
          <div class="quick-grid">
            <button class="quick" data-act="upload"><span class="q-ico">${ico('upload')}</span><span><b>Enviar arquivos</b><span>Do computador ou celular</span></span></button>
            <button class="quick" data-act="new-folder"><span class="q-ico">${ico('folderPlus')}</span><span><b>Nova pasta</b><span>Organizar arquivos</span></span></button>
            <a class="quick" href="#/files"><span class="q-ico">${ico('folder')}</span><span><b>Explorar</b><span>Ver todos os arquivos</span></span></a>
            <a class="quick" href="#/shares"><span class="q-ico">${ico('share')}</span><span><b>Compartilhar</b><span>Links para enviar</span></span></a>
          </div>
          <div class="divider"></div>
          <div class="card-title" style="margin-bottom:8px"><h3>Sistema</h3><a href="#/settings">Ajustes</a></div>
          <div class="device-strip" id="device-strip"><span class="tiny">Carregando…</span></div>
          <div class="spark-meta"><span id="cpu-title">CPU</span><span id="cpu-label">—</span></div>
          <svg class="spark" id="spark-cpu" viewBox="0 0 100 30" preserveAspectRatio="none"></svg>
        </section>
        <section class="card">
          <div class="card-title"><h3>Atividade recente</h3><a href="#/activity">Ver tudo</a></div>
          <div class="mini-activity" id="dash-activity"><span class="tiny">Carregando…</span></div>
          <div class="divider"></div>
          <div class="card-title" style="margin-bottom:8px"><h3>Acesso</h3><a href="#/access">Todos os endereços</a></div>
          <div class="net-list" id="dash-access"><span class="tiny">—</span></div>
        </section>
      </div>`;
    page.dataset.ready = '1';
  }
  const g = $('#dash-greeting');
  if (g) g.textContent = `${greeting()}. Este é o estado do seu servidor.`;
  updateDashboard();
  loadRecentActivity();
  loadSystem(true);
}

function updateDashboard() {
  const sys = S.sys;
  if (!sys) return;
  const set = (id, text) => { const el = document.getElementById(id); if (el) el.textContent = text; };

  set('hero-name', S.name);
  set('hero-sub', `No ar há ${fmtUptime(sys.server_uptime)} · ${sys.cores} núcleos · ${sys.interfaces.filter(i => i.reachable).length} rede(s) disponível(is)`);
  set('hero-free', fmtBytes(sys.storage.free));
  set('hero-batt', sys.battery.available ? `${sys.battery.level}%` : '—');

  // storage
  const st = sys.storage || {};
  set('st-storage', fmtBytes(st.used || 0));
  set('st-storage-free', `${fmtBytes(st.free || 0)} livres de ${fmtBytes(st.total || 0)}`);
  set('st-storage-pct', `${Math.round(st.percent || 0)}% usado`);

  // Detalha cada armazenamento. O total acima é a soma de todos: antes o
  // cartão SD ficava de fora e o número parecia errado.
  const volBox = $('#vol-list');
  if (volBox) {
    const vols = sys.volumes || [];
    if (vols.length > 1) {
      volBox.innerHTML = vols.map(v => {
        const low = v.alert || v.low;
        return `
        <div class="vol-row${low ? ' warn' : ''}">
          <span class="vol-name">${esc(v.name)}${low ? ' ⚠' : ''}</span>
          <span class="vol-bar"><i style="width:${Math.min(100, v.percent || 0)}%"></i></span>
          <span class="vol-free">${fmtBytes(v.free)}</span>
        </div>`;
      }).join('');
    } else {
      volBox.innerHTML = '';
    }
  }
  // Aviso de reserva: se algum volume está abaixo do mínimo configurado,
  // uploads naquele volume vão ser recusados. Melhor dizer antes.
  const guardBox = $('#st-reserve-note');
  if (guardBox) {
    const short = (sys.volumes || []).filter(v => v.low || v.alert);
    if (short.length) {
      guardBox.hidden = false;
      guardBox.innerHTML = `${ico('info')} ${short.map(v => esc(v.name)).join(', ')} abaixo da reserva — uploads recusados.`;
    } else {
      guardBox.hidden = true;
      guardBox.innerHTML = '';
    }
  }
  const sb = $('#bar-storage');
  if (sb) { sb.style.width = `${Math.min(100, st.percent || 0)}%`; sb.parentElement.classList.toggle('danger', (st.percent || 0) > 90); }

  // memory
  set('st-mem', fmtBytes(sys.mem_used));
  set('st-mem-free', `${fmtBytes(sys.mem_free)} disponíveis de ${fmtBytes(sys.mem_total)}`);
  set('st-mem-pct', `${Math.round(sys.mem_percent || 0)}%`);
  const mb = $('#bar-mem');
  if (mb) { mb.style.width = `${Math.min(100, sys.mem_percent || 0)}%`; mb.parentElement.classList.toggle('danger', (sys.mem_percent || 0) > 92); }

  // battery
  const b = sys.battery || {};
  const level = b.available ? b.level : 0;
  const gauge = $('#batt-gauge');
  if (gauge) {
    const color = level <= 15 ? '#f08a80' : level <= 35 ? '#e8c27a' : '#74d6a5';
    gauge.style.background = `conic-gradient(${color} ${Math.max(0, Math.min(100, level))}%, rgba(255,255,255,.075) 0)`;
    gauge.style.setProperty('--v', level);
  }
  set('batt-level', b.available ? b.level : '—');
  const statusMap = { CHARGING: 'Carregando', FULL: 'Carga completa', DISCHARGING: 'Na bateria', NOT_CHARGING: 'Conectado', UNKNOWN: 'Bateria' };
  set('batt-status', b.available ? (statusMap[b.status] || b.status || 'Bateria') : 'Indisponível');
  set('batt-temp', b.available && b.temperature ? `${b.temperature.toFixed(1).replace('.', ',')} °C` : 'Temperatura —');
  const plugMap = { PLUGGED_AC: 'Carregador conectado', PLUGGED_USB: 'USB conectado', PLUGGED_WIRELESS: 'Carregamento sem fio' };
  set('batt-plug', b.available ? (plugMap[b.plugged] || 'Sem carregador') : 'Sem acesso à bateria');
  // Se o Android bloquear /sys, explica em vez de só mostrar vazio.
  set('batt-note', b.available ? (level >= 80 ? 'Boa carga' : level >= 30 ? 'Carga média' : 'Carga baixa') : 'O Android bloqueou a leitura');

  // network
  const port = sys.port;
  set('net-port', port);
  const nets = (sys.interfaces || []).filter(i => i.reachable);
  const list = $('#net-list');
  if (list) {
    list.innerHTML = nets.length ? nets.slice(0, 3).map(n => `
      <div class="net-row">
        <span class="dot ${n.tailscale ? 'ts' : ''}"></span>
        <span class="name">${esc(n.label)}</span>
        <code title="${esc(n.ipv4)}">${esc(n.ipv4)}</code>
        <button class="icon-btn" data-copy="http://${esc(n.ipv4)}:${port}" title="Copiar endereço">${ico('copy')}</button>
      </div>`).join('') : '<span class="tiny">Nenhuma rede detectada</span>';
  }
  set('net-count', `${nets.length} interface(s)`);

  // system strip — no Android o /proc/stat é bloqueado e mostramos a carga.
  const cpuOK = typeof sys.cpu_percent === 'number' && sys.cpu_percent >= 0;
  const loadTxt = Number(sys.load1 || 0).toFixed(2).replace('.', ',');
  const loadPct = Math.min(100, ((sys.load1 || 0) / Math.max(1, (sys.cores || 1) * 2)) * 100);
  const strip = $('#device-strip');
  if (strip) {
    strip.innerHTML = `
      ${cpuOK ? `<span class="kv">${ico('cpu')} <b>${Math.round(sys.cpu_percent)}%</b> CPU</span>
      <span class="sep"></span>
      <span class="kv">carga <b>${loadTxt}</b></span>` : `<span class="kv">${ico('cpu')} <b>${loadTxt}</b> carga</span>`}
      <span class="sep"></span>
      <span class="kv">${ico('memory')} <b>${sys.cores}</b> núcleos</span>
      <span class="sep"></span>
      <span class="kv">${ico('clock')} <b>${fmtUptime(sys.device_uptime)}</b> ligado</span>
      <span class="sep"></span>
      <span class="kv">${ico('server')} <b>v${esc(sys.version)}</b></span>`;
  }
  const cpuTitle = $('#cpu-title');
  if (cpuTitle) cpuTitle.textContent = cpuOK ? 'CPU' : 'Carga';
  set('cpu-label', cpuOK ? `${Math.round(sys.cpu_percent)}% · carga ${loadTxt}` : `carga ${loadTxt} · ${sys.cores} núcleos`);

  // history
  pushHist('cpu', cpuOK ? sys.cpu_percent : loadPct);
  pushHist('mem', sys.mem_percent);
  drawSpark('#spark-cpu', S.hist.cpu, '#9be7c4');

  // access summary
  const acc = $('#dash-access');
  if (acc) {
    const urls = (sys.urls || []).filter(Boolean);
    acc.innerHTML = urls.slice(0, 3).map(u => `
      <div class="net-row"><span class="dot ${u.includes(':100.') ? 'ts' : ''}"></span>
        <code style="flex:1">${esc(u)}</code>
        <button class="icon-btn" data-copy="${esc(u)}" title="Copiar">${ico('copy')}</button>
      </div>`).join('') || '<span class="tiny">—</span>';
  }
}

function pushHist(key, value) {
  const arr = S.hist[key];
  arr.push(Math.max(0, Math.min(100, Number(value) || 0)));
  if (arr.length > 48) arr.shift();
}

function drawSpark(sel, arr, color) {
  const el = $(sel);
  if (!el) return;
  if (arr.length < 2) { el.innerHTML = ''; return; }
  const max = 100;
  const step = 100 / (arr.length - 1);
  const pts = arr.map((v, i) => `${(i * step).toFixed(2)},${(30 - (v / max) * 28 - 1).toFixed(2)}`).join(' ');
  el.innerHTML = `<defs><linearGradient id="g-${sel.replace('#', '')}" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="${color}" stop-opacity=".28"/><stop offset="1" stop-color="${color}" stop-opacity="0"/></linearGradient></defs>
    <polygon points="0,30 ${pts} 100,30" fill="url(#g-${sel.replace('#', '')})" stroke="none"></polygon>
    <polyline points="${pts}" fill="none" stroke="${color}" stroke-width="1.6" vector-effect="non-scaling-stroke" stroke-linejoin="round" stroke-linecap="round"></polyline>`;
}

async function loadRecentActivity() {
  try {
    const data = await api('/api/activity?limit=6');
    const list = $('#dash-activity');
    if (!list) return;
    const items = data.activity || [];
    list.innerHTML = items.length ? items.map(a => {
      const meta = ACT_LABELS[a.action] || { label: a.action, icon: 'activity' };
      const ok = meta.bad ? 'bad' : (a.ok === false ? 'bad' : 'ok');
      return `<div class="mini-row">
        <span class="mini-ico ${ok}">${ico(meta.icon)}</span>
        <div class="mini-main"><b>${esc(meta.label)}</b><span>${esc(a.detail || a.ip || '')}</span></div>
        <span class="mini-time">${esc(fmtRelative(a.t))}</span>
      </div>`;
    }).join('') : '<span class="tiny">Nenhuma atividade ainda.</span>';
  } catch {}
}

/* ============================ shares ============================ */

async function renderSharesPage() {
  const page = $('#page-shares');
  page.innerHTML = `
    <div class="page-head">
      <div><h1>Compartilhamentos</h1><p class="muted">Links públicos para arquivos e pastas. Você pode revogar quando quiser.</p></div>
      <div class="head-actions"><a class="btn ghost" href="#/files">${ico('folder')} Ir para arquivos</a><button class="icon-btn" data-act="refresh" title="Atualizar">${ico('refresh')}</button></div>
    </div>
    <div id="shares-body"><div class="card"><span class="tiny">Carregando…</span></div></div>`;
  await loadShares();
}

async function loadShares() {
  try {
    const data = await api('/api/shares');
    S.shares = data.shares || [];
    renderSharesBody();
  } catch (e) {
    const body = $('#shares-body');
    if (body) body.innerHTML = `<div class="card"><p class="muted">${esc(e.message)}</p></div>`;
  }
}

function shareURL(s) { return `${location.origin}/s/${s.token}`; }

function renderSharesBody() {
  const body = $('#shares-body');
  if (!body) return;
  if (!S.shares.length) {
    body.innerHTML = `<div class="card" style="text-align:center;padding:54px 20px">
      <div style="width:54px;height:54px;border-radius:15px;background:var(--accent-soft);display:grid;place-items:center;margin:0 auto 14px;color:var(--accent)">${ico('share')}</div>
      <b style="display:block;font-size:15px;margin-bottom:5px">Nenhum link ativo</b>
      <span class="muted" style="font-size:12.5px">Abra um arquivo ou pasta em <a href="#/files" style="color:var(--accent)">Arquivos</a> e toque em Compartilhar.</span>
    </div>`;
    return;
  }
  body.innerHTML = `<div class="shares-grid">${S.shares.map(s => {
    const kind = fileKind(s.name, s.is_dir);
    const link = shareURL(s);
    const expired = s.expires_at && new Date(s.expires_at) < new Date();
    return `<article class="share-card">
      <div class="share-top">
        <span class="badge ${kind}">${fileBadge(kind, s.name)}</span>
        <div class="share-title"><b>${esc(s.name || s.path || 'Compartilhamento')}</b><span>${esc(s.path || '')}</span></div>
        ${expired ? '<span class="chip" style="color:#ffb4b4;border-color:rgba(248,113,113,.25)">expirado</span>' : ''}
      </div>
      <div class="share-link"><code>${esc(link)}</code><button class="icon-btn" data-copy="${esc(link)}" title="Copiar">${ico('copy')}</button></div>
      <div class="share-meta">
        <span>${ico('clock')} ${s.expires_at ? (expired ? 'Expirou ' : 'Expira ') + fmtDateTime(s.expires_at) : 'Sem validade'}</span>
        <span>${ico('download')} ${s.downloads || 0}</span>
        <span>${s.has_password ? ico('lock') + ' Protegido' : 'Aberto'}</span>
      </div>
      <div class="share-actions">
        <a class="btn small ghost" href="${esc(link)}" target="_blank" rel="noopener">${ico('external')} Abrir</a>
        <button class="btn small ghost" data-share-qr="${esc(s.token)}">${ico('qr')} QR</button>
        <button class="btn small danger" data-share-revoke="${esc(s.token)}">${ico('trash')} Revogar</button>
      </div>
    </article>`;
  }).join('')}</div>`;

  $$('[data-share-qr]', body).forEach(b => b.addEventListener('click', () => {
    const s = S.shares.find(x => x.token === b.dataset.shareQr);
    if (s) qrDialog('Link compartilhado', shareURL(s));
  }));
  $$('[data-share-revoke]', body).forEach(b => b.addEventListener('click', async () => {
    const s = S.shares.find(x => x.token === b.dataset.shareRevoke);
    if (!s) return;
    const ok = await confirmDialog('Revogar este link?', `O link de <b>${esc(s.name)}</b> deixará de funcionar imediatamente.`, { ok: 'Revogar', danger: true });
    if (!ok) return;
    try {
      await api(`/api/shares/${encodeURIComponent(s.token)}`, { method: 'DELETE' });
      toast('Link revogado.', 'ok');
      loadShares();
    } catch (e) { toast(e.message, 'error'); }
  }));
}

/* ============================ activity ============================ */

async function renderActivityPage() {
  const page = $('#page-activity');
  page.innerHTML = `
    <div class="page-head">
      <div><h1>Atividade</h1><p class="muted">Últimos acessos e operações no servidor.</p></div>
      <div class="head-actions"><button class="icon-btn" data-act="refresh" title="Atualizar">${ico('refresh')}</button></div>
    </div>
    <div class="filters" id="act-filters">
      <button class="filter active" data-filter="all">Tudo</button>
      <button class="filter" data-filter="access">Acessos</button>
      <button class="filter" data-filter="files">Arquivos</button>
      <button class="filter" data-filter="links">Links</button>
      <button class="filter" data-filter="system">Sistema</button>
    </div>
    <div class="activity-list" id="activity-list"><div class="act-row"><span class="act-ico">${ico('clock')}</span><div class="act-main"><b>Carregando…</b></div></div></div>`;
  $$('#act-filters .filter').forEach(b => b.addEventListener('click', () => {
    $$('#act-filters .filter').forEach(x => x.classList.toggle('active', x === b));
    S.activityFilter = b.dataset.filter;
    renderActivityRows();
  }));
  try {
    const data = await api('/api/activity?limit=250');
    S.activity = data.activity || [];
    renderActivityRows();
  } catch (e) {
    $('#activity-list').innerHTML = `<div class="act-row"><div class="act-main"><b>${esc(e.message)}</b></div></div>`;
  }
}

function activityGroup(action) {
  if (['login', 'login_fail', 'logout', 'password', 'setup'].includes(action)) return 'access';
  if (['upload', 'download', 'download_zip', 'mkdir', 'rename', 'delete', 'move', 'copy', 'save', 'unzip'].includes(action)) return 'files';
  if (action.startsWith('share_')) return 'links';
  return 'system';
}

function renderActivityRows() {
  const list = $('#activity-list');
  if (!list) return;
  const filter = S.activityFilter || 'all';
  const items = S.activity.filter(a => filter === 'all' || activityGroup(a.action) === filter);
  if (!items.length) {
    list.innerHTML = `<div class="act-row"><span class="act-ico">${ico('activity')}</span><div class="act-main"><b>Nada por aqui</b><span>Nenhuma atividade neste filtro.</span></div></div>`;
    return;
  }
  list.innerHTML = items.map(a => {
    const meta = ACT_LABELS[a.action] || { label: a.action, icon: 'activity' };
    const bad = meta.bad || a.ok === false;
    return `<div class="act-row">
      <span class="act-ico ${bad ? 'bad' : 'ok'}">${ico(meta.icon)}</span>
      <div class="act-main"><b>${esc(meta.label)}</b><span>${esc(a.detail || (bad ? 'Ação não concluída' : '—'))}</span></div>
      <div class="act-side"><span>${esc(fmtRelative(a.t))} · ${esc(fmtDateTime(a.t))}</span><code>${esc(a.ip || '')}</code></div>
    </div>`;
  }).join('');
}

/* ============================ access ============================ */

function renderAccessPage() {
  const page = $('#page-access');
  page.innerHTML = `
    <div class="page-head">
      <div><h1>Acesso</h1><p class="muted">Use estes endereços para abrir o servidor de outros aparelhos.</p></div>
      <div class="head-actions"><button class="icon-btn" data-act="refresh" title="Atualizar">${ico('refresh')}</button></div>
    </div>
    <div id="access-body"><div class="card"><span class="tiny">Detectando redes…</span></div></div>
    <div class="section-gap"></div>
    <div class="tip-card">
      <b>Como funciona em várias redes</b><br>
      Na mesma rede Wi-Fi, use o endereço <b>LAN</b>. De qualquer outra rede (dados móveis, outro Wi-Fi, fora de casa), use o endereço <b>Tailscale</b> — ele cria uma conexão privada criptografada entre seus aparelhos. O celular precisa estar com o app Tailscale conectado e o modo “sempre ligado” ativo.
    </div>`;
  S._accessSig = '';
  loadSystem(true);
}

function updateAccessLive() {
  if (!S.sys || S.page !== 'access') return;
  const sig = JSON.stringify({ urls: S.sys.urls, ts: S.sys.tailscale_name, port: S.sys.port });
  if (S._accessSig === sig) return;
  S._accessSig = sig;
  renderAccessBody();
}

function renderAccessBody() {
  const body = $('#access-body');
  if (!body || !S.sys) return;
  const sys = S.sys;
  const reachable = (sys.interfaces || []).filter(i => i.reachable);
  const cards = [];

  // Endereço curto: funciona em qualquer aparelho com Tailscale conectado
  // graças ao domínio de busca configurado pelo próprio Tailscale.
  if (sys.short_host) {
    const shortUrl = `http://${sys.short_host}:${sys.port}`;
    cards.push(`<article class="access-card tall">
      <div class="access-head"><span class="stat-ico mint">${ico('bolt')}</span>
        <div style="min-width:0"><b style="font-size:13px">Endereço curto</b><div class="tiny">O mais fácil de lembrar · requer Tailscale</div></div>
        <span class="chip ok" style="margin-left:auto">recomendado</span>
      </div>
      <div class="access-url big">${esc(shortUrl)}</div>
      <div class="access-actions">
        <button class="btn small ghost" data-copy="${esc(shortUrl)}">${ico('copy')} Copiar</button>
        <button class="btn small ghost" data-access-qr="${esc(shortUrl)}">${ico('qr')} QR code</button>
        <a class="btn small ghost" href="${esc(shortUrl)}" target="_blank" rel="noopener">${ico('external')} Abrir</a>
      </div>
    </article>`);
  }
  for (const n of reachable) {
    const url = `http://${n.ipv4}:${sys.port}`;
    const isTs = n.tailscale;
    cards.push(`<article class="access-card">
      <div class="access-head"><span class="stat-ico ${isTs ? 'violet' : ''}">${ico(isTs ? 'shield' : 'wifi')}</span>
        <div style="min-width:0"><b style="font-size:13px">${esc(n.label)}</b><div class="tiny">${isTs ? 'Funciona de qualquer rede' : 'Mesma rede Wi-Fi'}</div></div>
        <span class="chip" style="margin-left:auto">${isTs ? 'Tailscale' : 'LAN'}</span>
      </div>
      <div class="access-url">${esc(url)}</div>
      <div class="access-actions">
        <button class="btn small ghost" data-copy="${esc(url)}">${ico('copy')} Copiar</button>
        <button class="btn small ghost" data-access-qr="${esc(url)}">${ico('qr')} QR code</button>
        <a class="btn small ghost" href="${esc(url)}" target="_blank" rel="noopener">${ico('external')} Abrir</a>
      </div>
    </article>`);
  }
  if (sys.tailscale_name) {
    const url = `http://${sys.tailscale_name}:${sys.port}`;
    cards.push(`<article class="access-card">
      <div class="access-head"><span class="stat-ico violet">${ico('cloud')}</span>
        <div><b style="font-size:13px">Nome Tailscale</b><div class="tiny">MagicDNS · qualquer rede</div></div>
        <span class="chip" style="margin-left:auto">MagicDNS</span>
      </div>
      <div class="access-url">${esc(url)}</div>
      <div class="access-actions">
        <button class="btn small ghost" data-copy="${esc(url)}">${ico('copy')} Copiar</button>
        <button class="btn small ghost" data-access-qr="${esc(url)}">${ico('qr')} QR code</button>
        <a class="btn small ghost" href="${esc(url)}" target="_blank" rel="noopener">${ico('external')} Abrir</a>
      </div>
    </article>`);
  }
  const tail = (sys.interfaces || []).find(i => i.tailscale);
  cards.push(`<article class="access-card">
    <div class="access-head"><span class="stat-ico">${ico('phone')}</span>
      <div><b style="font-size:13px">Status do servidor</b><div class="tiny">${esc(sys.hostname && sys.hostname !== 'localhost' ? sys.hostname : 'Celular Android')}</div></div>
      <span class="chip" style="margin-left:auto">${tail ? 'Tailscale ativo' : 'Só LAN'}</span>
    </div>
    <div class="kv-list">
      <div class="kv"><span>Endereço de acesso</span><b>http://${esc(location.host || ('127.0.0.1:' + sys.port))}</b></div>
      <div class="kv"><span>Porta</span><b>${sys.port}</b></div>
      <div class="kv"><span>No ar há</span><b>${fmtUptime(sys.server_uptime)}</b></div>
      <div class="kv"><span>Versão</span><b>${esc(sys.version)}</b></div>
    </div>
    ${S.deferredPrompt ? `<button class="btn small primary" id="install-pwa" style="align-self:flex-start">${ico('download')} Instalar atalho</button>` : ''}
  </article>`);
  body.innerHTML = `<div class="access-grid">${cards.join('')}</div>`;
  $$('[data-access-qr]', body).forEach(b => b.addEventListener('click', () => qrDialog('Acesso ao servidor', b.dataset.accessQr)));
  const install = $('#install-pwa');
  if (install) install.addEventListener('click', async () => {
    if (!S.deferredPrompt) return;
    S.deferredPrompt.prompt();
    await S.deferredPrompt.userChoice;
    S.deferredPrompt = null;
    install.remove();
  });
}

/* ============================ settings ============================ */

async function renderSettingsPage() {
  const page = $('#page-settings');
  page.innerHTML = `
    <div class="page-head"><div><h1>Configurações</h1><p class="muted">Servidor, segurança e manutenção.</p></div></div>
    <div id="settings-body"><div class="card"><span class="tiny">Carregando…</span></div></div>`;
  try {
    S.settings = await api('/api/settings');
    renderSettingsBody();
  } catch (e) {
    $('#settings-body').innerHTML = `<div class="card"><p class="muted">${esc(e.message)}</p></div>`;
  }
}

function renderSettingsBody() {
  const s = S.settings;
  if (!s) return;
  $('#settings-body').innerHTML = `
    <div class="settings-grid">
      <div class="settings-col">
        <section class="card">
          <div class="card-title"><h3>Servidor</h3><span class="chip">v${esc(s.version)}</span></div>
          <form id="settings-form">
            <div class="form-grid">
              <div class="field"><label for="set-name">Nome do servidor</label><input class="input" id="set-name" maxlength="40" value="${esc(s.server_name)}"></div>
              <div class="field"><label for="set-port">Porta HTTP</label><input class="input" id="set-port" type="number" min="1024" max="65535" value="${s.port}"></div>
            </div>
            <div class="field"><label for="set-root">Pasta raiz</label><input class="input" id="set-root" value="${esc(s.root)}"><span class="hint">Pasta do celular que o servidor pode acessar. Padrão: <code>/sdcard</code>.</span></div>
            <div class="form-grid">
              <div class="field"><label for="set-session">Sessão (dias)</label><input class="input" id="set-session" type="number" min="1" max="365" value="${s.session_days}"></div>
              <div class="field"><label for="set-upload">Limite de upload (MB)</label><input class="input" id="set-upload" type="number" min="0" value="${s.max_upload_mb}"><span class="hint">0 = sem limite.</span></div>
            </div>
            <div class="form-grid">
              <div class="field"><label for="set-reserve">Reserva de disco (MB)</label><input class="input" id="set-reserve" type="number" min="0" value="${s.min_free_mb != null ? s.min_free_mb : 2048}"><span class="hint">Espaço que nunca é ocupado por upload. 0 desliga.</span></div>
              <div class="field"><label for="set-trash">Lixeira (dias)</label><input class="input" id="set-trash" type="number" min="0" max="3650" value="${s.trash_days}"><span class="hint">0 desliga a lixeira (exclusão definitiva).</span></div>
            </div>
            <div class="form-grid">
              <div class="field"><label for="set-backup">Backup automático</label>
                <label class="switch-row"><input type="checkbox" id="set-backup" ${s.backup_daily ? 'checked' : ''}><span>Um snapshot por dia</span></label>
                <span class="hint">Roda enquanto o servidor estiver ligado.</span>
              </div>
            </div>
            <div class="field"><label for="set-ts">Nome MagicDNS do Tailscale</label><input class="input" id="set-ts" value="${esc(s.tailscale_name || '')}" placeholder="celular.tailXXXX.ts.net"><span class="hint">Usado para mostrar o endereço de acesso remoto.</span></div>
            <div id="restart-banner" class="tip-card" hidden style="margin-bottom:14px">As alterações de porta ou pasta raiz exigem reiniciar o servidor. <button class="btn small primary" id="restart-inline" type="button" style="margin-left:8px">Reiniciar agora</button></div>
            <div class="form-actions"><button class="btn primary" type="submit" id="settings-save">Salvar alterações</button></div>
          </form>
        </section>

        <section class="card" id="card-roots">
          <div class="card-title"><h3>Armazenamento</h3></div>
          <div id="roots-body"><span class="tiny">Carregando…</span></div>
        </section>

        <section class="card">
          <div class="card-title"><h3>Segurança</h3></div>
          <form id="password-form">
            <div class="field"><label for="pw-current">Senha atual</label><input class="input" id="pw-current" type="password" autocomplete="current-password"></div>
            <div class="form-grid">
              <div class="field"><label for="pw-new">Nova senha</label><input class="input" id="pw-new" type="password" minlength="8" autocomplete="new-password"></div>
              <div class="field"><label for="pw-new2">Confirmar</label><input class="input" id="pw-new2" type="password" minlength="8" autocomplete="new-password"></div>
            </div>
            <div class="form-actions"><button class="btn primary" type="submit">Trocar senha</button></div>
          </form>
        </section>
      </div>

      <div class="settings-col">
        <section class="card" id="card-totp">
          <div class="card-title"><h3>Verificação em duas etapas</h3><span class="chip" id="totp-chip">…</span></div>
          <div id="totp-body"><span class="tiny">Carregando…</span></div>
        </section>

        <section class="card" id="card-tokens">
          <div class="card-title"><h3>Tokens de API</h3><span class="chip" id="tokens-chip">…</span></div>
          <div id="tokens-body"><span class="tiny">Carregando…</span></div>
        </section>

        <section class="card" id="card-backups">
          <div class="card-title"><h3>Backups</h3><span class="chip" id="backup-chip">…</span></div>
          <div id="backup-body"><span class="tiny">Carregando…</span></div>
        </section>

        <section class="card">
          <div class="card-title"><h3>Manutenção</h3></div>
          <div class="kv-list">
            <div class="kv"><span>Configuração</span><b>${esc(s.home)}/config.json</b></div>
            <div class="kv"><span>Log do servidor</span><b>${esc(s.home)}/server.log</b></div>
            <div class="kv"><span>Versão</span><b>${esc(s.version)}</b></div>
          </div>
          <div class="divider"></div>
          <div class="access-actions">
            <button class="btn small ghost" id="btn-log">${ico('terminal')} Ver log</button>
            <button class="btn small ghost" id="btn-restart">${ico('refresh')} Reiniciar servidor</button>
          </div>
        </section>

        <section class="card danger-card">
          <div class="card-title"><h3>Zona de risco</h3></div>
          <p style="font-size:12px;color:var(--muted);line-height:1.6;margin:0 0 12px">Sair encerra esta sessão no navegador. A senha continua valendo.</p>
          <button class="btn small danger" data-act="logout">${ico('logout')} Sair do painel</button>
        </section>

        <section class="card">
          <div class="card-title"><h3>Sobre</h3></div>
          <p style="font-size:12px;color:var(--muted);line-height:1.7;margin:0">
            <b style="color:var(--text)">${esc(S.name)}</b> roda direto no celular, com Termux.<br>
            Seus arquivos ficam no aparelho; nada passa por servidores externos. O acesso remoto usa o Tailscale, uma rede privada criptografada.
          </p>
        </section>
      </div>
    </div>`;

  $('#settings-form').addEventListener('submit', saveSettings);
  $('#password-form').addEventListener('submit', savePassword);
  $('#btn-log').addEventListener('click', showLog);
  $('#btn-restart').addEventListener('click', restartServer);
  const inline = $('#restart-inline');
  if (inline) inline.addEventListener('click', restartServer);
  loadTOTP();
  loadBackups();
  loadTokens();
  renderRootsCard();
}

/* ---------------- armazenamento (múltiplas pastas) ---------------- */

function fmtSpace(bytes) {
  if (!bytes) return '—';
  const u = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0, n = bytes;
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
  return `${n.toFixed(n >= 10 || i === 0 ? 0 : 1)} ${u[i]}`;
}

function renderRootsCard() {
  const box = $('#roots-body');
  if (!box) return;
  const list = (S.settings && S.settings.roots) || [];
  const detected = ((S.settings && S.settings.volumes) || []).filter(v => !v.configured);
  const currentPaths = new Set(list.map(r => r.path));

  const detectedHtml = detected.length ? `
    <div class="divider"></div>
    <div class="tiny" style="margin:0 0 6px">Encontrados no aparelho</div>
    ${detected.filter(v => !currentPaths.has(v.path)).map(v => `
      <div class="kv">
        <span>${esc(v.label)}${v.removable ? ' · removível' : ''}</span>
        <b class="mono">${esc(v.path)} · ${fmtSpace(v.free)} livres</b>
      </div>
      <button class="btn small ghost" style="margin:6px 0 10px" data-root-add-detected="${esc(v.path)}" data-root-add-label="${esc(aliasFromLabel(v.label))}">
        Usar como “${esc(aliasFromLabel(v.label))}”
      </button>`).join('')}` : '';

  box.innerHTML = list.map(r => `
    <div class="kv">
      <span>${r.alias ? esc(r.name) : 'principal'}</span>
      <b>${esc(r.path)}${r.total ? ` · ${fmtSpace(r.free)} livres` : ''}</b>
    </div>`).join('') + `
    <div class="field" style="margin-top:14px">
      <label for="root-add-path">Adicionar outra pasta</label>
      <input class="input mono" id="root-add-path" placeholder="/storage/XXXX-XXXX/PocketServe">
      <span class="hint">Caminho completo no celular. Aparece como uma aba em Arquivos.</span>
    </div>
    <div class="form-grid">
      <div class="field"><label for="root-add-name">Apelido (curto)</label>
        <input class="input" id="root-add-name" maxlength="16" placeholder="cartao"></div>
      <div class="field" style="justify-content:flex-end;display:flex">
        <button class="btn small primary" id="root-add" type="button" style="width:100%">Adicionar</button>
      </div>
    </div>
    ${detectedHtml}
    ${list.filter(r => r.alias).length ? `<div class="divider"></div>
      <div class="access-actions">
        ${list.filter(r => r.alias).map(r => `<button class="btn small ghost" data-root-remove="${esc(r.alias)}">Remover ${esc(r.name)}</button>`).join('')}
      </div>` : ''}`;

  $('#root-add').addEventListener('click', addRoot);
  $$('[data-root-remove]', box).forEach(b => b.addEventListener('click', () => removeRoot(b.dataset.rootRemove)));
  $$('[data-root-add-detected]', box).forEach(b => b.addEventListener('click', () => {
    addRootWith(b.dataset.rootAddDetected, b.dataset.rootAddLabel);
  }));
}

// aliasFromLabel transforma "Cartão XXXX-XXXX" num apelido curto e válido.
function aliasFromLabel(label) {
  return String(label || '')
    .toLowerCase()
    .normalize('NFD').replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 16) || 'cartao';
}

async function addRoot() {
  const path = $('#root-add-path').value.trim();
  const name = $('#root-add-name').value.trim();
  if (!path) return toast('Informe o caminho completo.', 'error');
  await addRootWith(path, name);
}

async function addRootWith(path, name) {
  const alias = (name || path.split('/').filter(Boolean).pop() || 'pasta').toLowerCase().replace(/[^a-z0-9_-]/g, '');
  const list = (S.settings.roots || []).filter(r => r.alias).map(r => ({ name: r.alias, path: r.path }));
  list.push({ name: alias, path });
  await saveRoots(list);
}

async function removeRoot(alias) {
  const ok = await confirmDialog(`Remover a pasta "${alias}"?`,
    'O conteúdo no disco continua intacto; apenas deixa de aparecer no painel.', { ok: 'Remover' });
  if (!ok) return;
  const list = (S.settings.roots || []).filter(r => r.alias && r.alias !== alias).map(r => ({ name: r.alias, path: r.path }));
  await saveRoots(list);
}

async function saveRoots(list) {
  try {
    await saveSettingsBody(Object.assign({}, currentSettingsBody(), { roots: list }));
    S.settings = await api('/api/settings');
    renderRootsCard();
    toast('Armazenamento atualizado.', 'ok');
  } catch (e) { toast(e.message, 'error'); }
}

function currentSettingsBody() {
  return {
    server_name: $('#set-name').value.trim(),
    port: Number($('#set-port').value),
    root: $('#set-root').value.trim(),
    session_days: Number($('#set-session').value),
    max_upload_mb: Number($('#set-upload').value),
    min_free_mb: Number($('#set-reserve').value),
    tailscale_name: $('#set-ts').value.trim(),
    trash_days: Number($('#set-trash').value),
    backup_daily: $('#set-backup').checked
  };
}

async function saveSettingsBody(body) {
  return api('/api/settings', { method: 'POST', json: body });
}

/* ---------------- tokens de API ---------------- */

const TOKEN_SCOPE_LABELS = {
  'files:read': 'Ler arquivos',
  'files:write': 'Enviar e editar',
  'files:delete': 'Excluir arquivos',
  'shares:manage': 'Gerenciar links',
  'admin': 'Administrador (tudo)'
};

let tokensCache = [];

async function loadTokens() {
  const body = $('#tokens-body');
  const chip = $('#tokens-chip');
  if (!body) return;
  try {
    const d = await api('/api/tokens');
    tokensCache = Array.isArray(d.tokens) ? d.tokens : [];
    renderTokens(chip, body);
  } catch (e) {
    body.innerHTML = `<span class="tiny">${esc(e.message)}</span>`;
  }
}

function renderTokens(chip, body) {
  const active = tokensCache.filter(t => t.active).length;
  if (chip) {
    chip.textContent = active ? `${active} ativo(s)` : 'nenhum';
    chip.className = 'chip ' + (active ? 'ok' : '');
  }

  const rows = tokensCache.map(t => {
    const scopes = (t.scopes || []).map(s => TOKEN_SCOPE_LABELS[s] || s).join(' · ');
    const exp = t.expires_at ? formatTokenDate(t.expires_at) : 'nunca expira';
    const used = t.last_used ? `último uso ${formatTokenDate(t.last_used)}` : 'nunca usado';
    const state = t.revoked_at ? 'revogado' : (t.active ? 'ativo' : 'expirado');
    const cls = t.active ? 'ok' : 'warn';
    return `<div class="token-row">
      <div class="token-head">
        <b>${esc(t.name)}</b>
        <span class="chip ${cls}">${state}</span>
      </div>
      <div class="tiny mono">${esc(t.prefix)}…</div>
      <div class="tiny">${esc(scopes)}</div>
      <div class="tiny">${esc(exp)} · ${esc(used)}</div>
      <div class="token-actions">
        <button class="btn small ghost" data-token-copy="${esc(t.id)}">${ico('copy')} Copiar prefixo</button>
        ${t.active ? `<button class="btn small ghost" data-token-revoke="${esc(t.id)}">${ico('lock')} Revogar</button>` : ''}
        <button class="btn small danger" data-token-delete="${esc(t.id)}">${ico('trash')} Apagar</button>
      </div>
    </div>`;
  }).join('');

  body.innerHTML = `
    ${tokensCache.length ? rows : `<p class="tiny" style="margin:0 0 12px">Nenhum token ainda. Tokens servem para scripts e agentes acessarem o servidor sem usar a senha do administrador.</p>`}
    <div class="divider"></div>
    <button class="btn small primary" id="token-new">${ico('plus')} Criar token</button>`;

  const nb = $('#token-new');
  if (nb) nb.addEventListener('click', openTokenDialog);
  $$('[data-token-revoke]', body).forEach(b => b.addEventListener('click', () => revokeToken(b.dataset.tokenRevoke)));
  $$('[data-token-delete]', body).forEach(b => b.addEventListener('click', () => deleteToken(b.dataset.tokenDelete)));
  $$('[data-token-copy]', body).forEach(b => b.addEventListener('click', () => {
    const t = tokensCache.find(x => x.id === b.dataset.tokenCopy);
    if (t) copyText(t.prefix + '…');
  }));
}

function formatTokenDate(v) {
  const d = new Date(v);
  if (isNaN(d)) return '—';
  return d.toLocaleDateString('pt-BR') + ' ' + d.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' });
}

function openTokenDialog() {
  const scopes = Object.keys(TOKEN_SCOPE_LABELS);
  openModal({
    title: 'Criar token de API',
    sub: 'A chave aparece uma única vez',
    body: `<div class="field">
        <label for="tok-name">Nome</label>
        <input class="input" id="tok-name" maxlength="60" placeholder="Ex.: rclone do PC, script de backup">
        <span class="hint">Serve para você lembrar para que este token existe.</span>
      </div>
      <div class="field">
        <label>Permissões</label>
        ${scopes.map(s => `<label class="switch-row"><input type="checkbox" data-scope="${esc(s)}" ${s === 'files:read' ? 'checked' : ''}><span>${esc(TOKEN_SCOPE_LABELS[s])}</span></label>`).join('')}
      </div>
      <div class="field">
        <label for="tok-exp">Validade</label>
        <select class="select" id="tok-exp">
          <option value="30">30 dias</option>
          <option value="90">90 dias</option>
          <option value="365">1 ano</option>
          <option value="0">Nunca expira</option>
        </select>
        <span class="hint">Prefira um prazo. Revogar é imediato, mas um token esquecido não expira sozinho.</span>
      </div>`,
    footer: `<button class="btn ghost" data-close>Cancelar</button>
      <button class="btn primary" id="tok-create">${ico('plus')} Criar token</button>`
  });

  $('#tok-create').addEventListener('click', async () => {
    const name = $('#tok-name').value.trim();
    const chosen = $$('[data-scope]').filter(c => c.checked).map(c => c.dataset.scope);
    if (!name) return toast('Dê um nome ao token.', 'error');
    if (!chosen.length) return toast('Escolha pelo menos uma permissão.', 'error');
    const btn = $('#tok-create');
    btn.disabled = true;
    btn.textContent = 'Criando…';
    try {
      const d = await api('/api/tokens', {
        method: 'POST',
        json: { name, scopes: chosen, expires_days: Number($('#tok-exp').value) || 0 }
      });
      closeModal();
      showTokenOnce(d.token, name);
      await loadTokens();
    } catch (e) {
      btn.disabled = false;
      btn.innerHTML = `${ico('plus')} Criar token`;
      toast(e.message, 'error', 'Não foi possível criar');
    }
  });
}

// A chave é exibida uma única vez — o servidor guarda apenas o hash.
function showTokenOnce(token, name) {
  openModal({
    title: 'Token criado',
    sub: name,
    body: `<p class="tiny" style="margin:0 0 10px">Copie agora. Por segurança, esta chave não será mostrada de novo — o servidor guarda apenas um hash dela.</p>
      <div class="copy-line"><input class="input mono" id="tok-value" value="${esc(token)}" readonly><button class="btn ghost" id="tok-copy">${ico('copy')} Copiar</button></div>
      <p class="hint" style="margin-top:10px">Use assim:</p>
      <pre class="preview-text" style="max-height:none;padding:10px;border-radius:8px">curl -H "Authorization: Bearer ${esc(token)}" \\
  http://127.0.0.1:8080/api/files?path=</pre>`,
    footer: `<button class="btn primary" id="tok-done">${ico('check')} Já copiei</button>`
  });
  const field = $('#tok-value');
  const copy = async () => {
    const ok = await copyText(token);
    if (ok) field.select();
  };
  $('#tok-copy').addEventListener('click', copy);
  field.addEventListener('click', () => field.select());
  $('#tok-done').addEventListener('click', closeModal);
  copy();
}

async function revokeToken(id) {
  const ok = await confirmDialog('Revogar este token?',
    'Ele deixa de funcionar imediatamente. O histórico de uso continua visível.', { ok: 'Revogar', danger: true });
  if (!ok) return;
  try {
    await api(`/api/tokens/${encodeURIComponent(id)}/revoke`, { method: 'POST', json: {} });
    toast('Token revogado.', 'ok');
    await loadTokens();
  } catch (e) {
    toast(e.message, 'error', 'Não foi possível revogar');
  }
}

async function deleteToken(id) {
  const ok = await confirmDialog('Apagar este token?',
    'O registro desaparece da lista. A chave já não funciona se estava revogada.', { ok: 'Apagar', danger: true });
  if (!ok) return;
  try {
    await api(`/api/tokens/${encodeURIComponent(id)}`, { method: 'DELETE' });
    toast('Token apagado.', 'ok');
    await loadTokens();
  } catch (e) {
    toast(e.message, 'error', 'Não foi possível apagar');
  }
}

/* ---------------- verificação em duas etapas ---------------- */

async function loadTOTP() {
  const body = $('#totp-body');
  const chip = $('#totp-chip');
  if (!body) return;
  try {
    const t = await api('/api/totp');
    chip.textContent = t.enabled ? 'ativa' : 'desligada';
    chip.className = 'chip ' + (t.enabled ? 'ok' : 'warn');
    if (t.enabled) {
      body.innerHTML = `
        <p class="muted" style="font-size:12px;line-height:1.6;margin:0 0 12px">
          Ao entrar, além da senha será pedido o código de 6 dígitos do seu autenticador.
          Você tem <b>${t.recovery_codes}</b> código(s) de recuperação restante(s).
        </p>
        <div class="kv-list" id="trusted-list"><div class="kv"><span>Aparelhos confiáveis</span><b>—</b></div></div>
        <div class="access-actions" style="margin-top:12px">
          <button class="btn small ghost" id="trust-revoke">Esquecer aparelhos</button>
          <button class="btn small ghost" id="totp-off">Desativar</button>
        </div>`;
      $('#totp-off').addEventListener('click', totpDisableDialog);
      $('#trust-revoke').addEventListener('click', revokeTrustedDevices);
      loadTrustedDevices();
    } else {
      body.innerHTML = `
        <p class="muted" style="font-size:12px;line-height:1.6;margin:0 0 12px">
          Proteja o acesso com um código temporário do celular, além da senha.
          Funciona com Google Authenticator, Aegis, Bitwarden e similares.
        </p>
        <div class="access-actions"><button class="btn small primary" id="totp-on">Ativar</button></div>`;
      $('#totp-on').addEventListener('click', totpSetupDialog);
    }
  } catch (e) {
    body.innerHTML = `<p class="muted">${esc(e.message)}</p>`;
  }
}

async function totpSetupDialog() {
  let data;
  try { data = await api('/api/totp/setup', { method: 'POST' }); }
  catch (e) { return toast(e.message, 'error'); }
  const secretFmt = data.secret.replace(/(.{4})/g, '$1 ').trim();
  openModal({
    title: 'Ativar verificação em duas etapas',
    body: `
      <p class="muted" style="font-size:13px;line-height:1.6;margin:0 0 14px">
        Escaneie o código no seu autenticador. Se preferir, digite a chave manualmente.
      </p>
      <div style="text-align:center;margin-bottom:14px">
        <img src="/api/totp/qr?secret=${encodeURIComponent(data.secret)}" alt="QR code" width="200" height="200" style="border-radius:12px;background:#fff;padding:8px">
      </div>
      <div class="field"><label>Chave manual</label>
        <input class="input mono" readonly value="${esc(secretFmt)}" onclick="this.select()">
      </div>
      <div class="field"><label for="totp-code">Código de 6 dígitos</label>
        <input class="input" id="totp-code" inputmode="numeric" maxlength="6" autocomplete="one-time-code" placeholder="000000">
      </div>
      <p class="hint" style="margin-top:10px">O relógio do celular precisa estar correto.</p>`,
    footer: `<button class="btn ghost" data-close>Cancelar</button>
             <button class="btn primary" id="totp-confirm">Ativar</button>`
  });
  $('#totp-code').focus();
  $('#totp-confirm').addEventListener('click', async () => {
    const code = $('#totp-code').value.trim();
    try {
      const res = await api('/api/totp/enable', { method: 'POST', json: { code } });
      showRecoveryCodes(res.recovery_codes || []);
      loadTOTP();
    } catch (e) { toast(e.message, 'error'); }
  });
}

function showRecoveryCodes(codes) {
  openModal({
    title: 'Guarde seus códigos de recuperação',
    body: `
      <p class="muted" style="font-size:13px;line-height:1.6;margin:0 0 14px">
        Cada código funciona <b>uma única vez</b> e serve para entrar se você perder o celular.
        Guarde fora do aparelho — sem eles, o acesso pode ficar impossível.
      </p>
      <div class="recovery-grid">${codes.map(c => `<code>${esc(c)}</code>`).join('')}</div>`,
    footer: `<button class="btn ghost" id="copy-recovery">Copiar</button>
             <button class="btn primary" data-close>Salvei os códigos</button>`
  });
  $('#copy-recovery').addEventListener('click', async () => {
    try { await navigator.clipboard.writeText(codes.join('\n')); toast('Códigos copiados.', 'ok'); }
    catch { toast('Copie manualmente.', 'info'); }
  });
}

function totpDisableDialog() {
  openModal({
    title: 'Desativar duas etapas',
    body: `
      <p class="muted" style="font-size:13px;line-height:1.6;margin:0 0 14px">
        Confirme com a senha e um código atual do autenticador.
      </p>
      <div class="field"><label for="off-pw">Senha</label><input class="input" id="off-pw" type="password" autocomplete="current-password"></div>
      <div class="field"><label for="off-code">Código</label><input class="input" id="off-code" inputmode="numeric" maxlength="6" autocomplete="one-time-code"></div>`,
    footer: `<button class="btn ghost" data-close>Cancelar</button>
             <button class="btn danger" id="off-confirm">Desativar</button>`
  });
  $('#off-confirm').addEventListener('click', async () => {
    try {
      await api('/api/totp/disable', { method: 'POST', json: { password: $('#off-pw').value, code: $('#off-code').value.trim() } });
      closeModal();
      toast('Verificação em duas etapas desativada.', 'ok');
      loadTOTP();
    } catch (e) { toast(e.message, 'error'); }
  });
}

/* ---------------- backups ---------------- */

function fmtWhen(iso) {
  try { return new Date(iso).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' }); }
  catch { return iso; }
}

async function loadBackups() {
  const body = $('#backup-body');
  const chip = $('#backup-chip');
  if (!body) return;
  try {
    const d = await api('/api/backups');
    const s = d.stats || {};
    chip.textContent = s.snapshots ? `${s.snapshots}/${s.retention || 14}` : 'nenhum';
    chip.className = 'chip ' + (s.snapshots ? 'ok' : 'warn');
    const rows = (d.snapshots || []).slice(0, 5).map(sn => `
      <div class="kv"><span>${esc(fmtWhen(sn.at))}</span><b>${sn.files} arq · ${fmtBytes(sn.size)}</b></div>`).join('');
    body.innerHTML = `
      <div class="kv-list">
        <div class="kv"><span>Último backup</span><b>${s.last_at ? esc(fmtWhen(s.last_at)) : 'nunca'}</b></div>
        <div class="kv"><span>Espaço usado</span><b>${s.packs_size ? fmtBytes(s.packs_size) : '—'}</b></div>
        <div class="kv"><span>Deduplicação</span><b>${s.packs || 0} bloco(s) para ${s.snapshots || 0} snapshot(s)</b></div>
      </div>
      ${rows ? `<div class="divider"></div><div class="kv-list">${rows}</div>` : ''}
      <div class="divider"></div>
      <div class="access-actions">
        <button class="btn small primary" id="backup-run">Fazer backup agora</button>
      </div>
      <p class="hint" style="margin-top:10px">O snapshot fica no próprio servidor. Copie a pasta <code>${esc(s.dir || '')}</code> para fora do celular de tempos em tempos.</p>`;
    $('#backup-run').addEventListener('click', runBackupNow);
  } catch (e) {
    body.innerHTML = `<p class="muted">${esc(e.message)}</p>`;
  }
}

async function runBackupNow() {
  const btn = $('#backup-run');
  if (btn) { btn.disabled = true; btn.textContent = 'Fazendo backup…'; }
  try {
    const r = await api('/api/backups/run', { method: 'POST', json: { note: 'manual' } });
    toast(`Backup concluído: ${r.files} arquivo(s), ${r.added} novo(s).`, 'ok');
    loadBackups();
  } catch (e) {
    toast(e.message, 'error');
    if (btn) { btn.disabled = false; btn.textContent = 'Fazer backup agora'; }
  }
}

async function saveSettings(e) {
  e.preventDefault();
  const btn = $('#settings-save');
  btn.disabled = true;
  const body = {
    server_name: $('#set-name').value.trim(),
    port: Number($('#set-port').value),
    root: $('#set-root').value.trim(),
    session_days: Number($('#set-session').value),
    max_upload_mb: Number($('#set-upload').value),
    min_free_mb: Number($('#set-reserve').value),
    tailscale_name: $('#set-ts').value.trim(),
    trash_days: Number($('#set-trash').value),
    backup_daily: $('#set-backup').checked
  };
  const oldPort = S.settings ? S.settings.port : null;
  try {
    const res = await api('/api/settings', { method: 'POST', json: body });
    S.name = body.server_name || S.name;
    $('#brand-name').textContent = S.name;
    const topbarName = $('#topbar-name');
    if (topbarName) topbarName.textContent = S.name;
    document.title = `${S.name} · Servidor pessoal`;
    toast('Configurações salvas.', 'ok');
    if (res.restart_required) {
      const newPort = Number(body.port) !== oldPort ? Number(body.port) : 0;
      S.pendingPort = newPort;
      const banner = $('#restart-banner');
      if (banner) {
        banner.hidden = false;
        if (newPort) {
          banner.innerHTML = `A porta mudou para <b>${newPort}</b>. Reinicie o servidor; o painel abrirá no novo endereço. <button class="btn small primary" id="restart-inline" type="button" style="margin-left:8px">Reiniciar agora</button>`;
          $('#restart-inline').addEventListener('click', restartServer);
        }
      }
      toast('Reinicie o servidor para aplicar porta/raiz.', 'info');
    }
  } catch (err) {
    toast(err.message, 'error');
  } finally {
    btn.disabled = false;
  }
}

async function savePassword(e) {
  e.preventDefault();
  const current = $('#pw-current').value;
  const pw = $('#pw-new').value;
  const pw2 = $('#pw-new2').value;
  if (pw !== pw2) return toast('As senhas novas não conferem.', 'error');
  if (pw.length < 8) return toast('A nova senha precisa ter pelo menos 8 caracteres.', 'error');
  try {
    await api('/api/password', { method: 'POST', json: { current, new: pw } });
    $('#pw-current').value = $('#pw-new').value = $('#pw-new2').value = '';
    toast('Senha alterada. O navegador continua conectado.', 'ok');
  } catch (err) {
    toast(err.message, 'error');
  }
}

async function showLog() {
  try {
    const data = await api('/api/log');
    openModal({
      title: 'Log do servidor',
      sub: 'Últimas linhas de server.log',
      wide: true,
      body: `<pre class="preview-text" style="max-height:60vh;overflow:auto">${esc(data.log || 'Sem registros.')}</pre>`,
      footer: `<button class="btn ghost" data-close>Fechar</button>`
    });
  } catch (e) { toast(e.message, 'error'); }
}

async function restartServer() {
  const ok = await confirmDialog('Reiniciar o servidor?', 'A conexão cai por alguns segundos e volta sozinha.', { ok: 'Reiniciar', danger: true });
  if (!ok) return;
  closeModal();
  const newPort = S.pendingPort || 0;
  try { await api('/api/restart', { method: 'POST' }); } catch {}
  const boot = $('#boot');
  boot.hidden = false;
  if (newPort) {
    boot.innerHTML = `<div class="boot-logo">${logoMarkup()}</div><div class="boot-text">Reiniciando…</div><div class="spinner"></div><p class="muted" style="font-size:12px;max-width:300px;text-align:center">O painel vai abrir na porta ${newPort} em alguns segundos.</p>`;
    $('#shell').hidden = true;
    setTimeout(() => {
      location.href = `${location.protocol}//${location.hostname}:${newPort}/`;
    }, 3500);
    return;
  }
  boot.innerHTML = `<div class="boot-logo">${logoMarkup()}</div><div class="boot-text">Reiniciando…</div><div class="spinner"></div><p class="muted" style="font-size:12px">Aguardando o servidor voltar</p>`;
  $('#shell').hidden = true;
  let attempts = 0;
  const timer = setInterval(async () => {
    attempts++;
    try {
      const st = await api('/api/state', { noAuthRedirect: true });
      if (st) {
        clearInterval(timer);
        boot.hidden = true;
        location.reload();
      }
    } catch {
      if (attempts > 45) {
        clearInterval(timer);
        boot.innerHTML = `<div class="boot-logo">${logoMarkup()}</div><div class="boot-text">Não voltou ainda</div><p class="muted" style="font-size:12px;max-width:300px;text-align:center">Verifique o app Termux no celular. O log fica em <code>~/.pocketserve/server.log</code>.</p><button class="btn primary" onclick="location.reload()">Tentar novamente</button>`;
      }
    }
  }, 1000);
}

/* ---------------- lixeira ---------------- */

async function renderTrashPage() {
  const page = $('#page-trash');
  page.innerHTML = `
    <div class="page-head">
      <div><h1>Lixeira</h1><p class="muted">Itens excluídos ficam aqui antes de sumir de vez.</p></div>
      <div class="page-actions"><button class="btn small danger" id="trash-empty" disabled>Esvaziar lixeira</button></div>
    </div>
    <div id="trash-body"><div class="card"><span class="tiny">Carregando…</span></div></div>`;
  try {
    const d = await api('/api/trash');
    S.trash = d;
    renderTrashBody();
  } catch (e) {
    $('#trash-body').innerHTML = `<div class="card"><p class="muted">${esc(e.message)}</p></div>`;
  }
}

function renderTrashBody() {
  const d = S.trash;
  const body = $('#trash-body');
  if (!d || !body) return;
  const items = d.items || [];
  const emptyBtn = $('#trash-empty');
  if (emptyBtn) emptyBtn.disabled = items.length === 0;

  if (!items.length) {
    body.innerHTML = `
      <div class="card empty-state">
        <div class="empty-ico">${ico('trash')}</div>
        <h3>Lixeira vazia</h3>
        <p class="muted">Nada foi excluído recentemente.</p>
        ${d.days ? `<p class="tiny">Itens são removidos automaticamente após ${d.days} dias.</p>` : `<p class="tiny">A remoção automática está desligada.</p>`}
      </div>`;
    return;
  }

  body.innerHTML = `
    <div class="card">
      <div class="trash-list">
        ${items.map(it => `
          <div class="trash-row" data-id="${esc(it.id)}">
            <span class="file-ico ${it.is_dir ? 'folder' : 'file'}">${ico(it.is_dir ? 'folder' : 'file')}</span>
            <span class="trash-meta">
              <b>${esc(it.name)}</b>
              <span>${esc(it.orig_path)} · ${fmtBytes(it.size)} · ${esc(fmtWhen(it.deleted_at))}</span>
            </span>
            <span class="trash-actions">
              <button class="btn small ghost" data-restore="${esc(it.id)}">Restaurar</button>
              <button class="btn small danger" data-purge="${esc(it.id)}">Apagar</button>
            </span>
          </div>`).join('')}
      </div>
      <p class="hint" style="margin:14px 0 0">${items.length} item(ns)${d.days ? ` · removidos automaticamente após ${d.days} dias` : ''}.</p>
    </div>`;

  $$('[data-restore]', body).forEach(b => b.addEventListener('click', () => restoreTrashItem(b.dataset.restore, b)));
  $$('[data-purge]', body).forEach(b => b.addEventListener('click', () => purgeTrashItem(b.dataset.purge)));
  const empty = $('#trash-empty');
  if (empty) empty.onclick = emptyTrash;
}

async function restoreTrashItem(id, btn) {
  if (btn) { btn.disabled = true; btn.textContent = 'Restaurando…'; }
  try {
    const r = await api('/api/trash/restore', { method: 'POST', json: { id } });
    toast(`Restaurado em ${r.path}`, 'ok');
    renderTrashPage();
  } catch (e) {
    toast(e.message, 'error');
    if (btn) { btn.disabled = false; btn.textContent = 'Restaurar'; }
  }
}

async function purgeTrashItem(id) {
  const ok = await confirmDialog('Apagar definitivamente?', 'Esta ação não pode ser desfeita.', { ok: 'Apagar', danger: true });
  if (!ok) return;
  try {
    await api('/api/trash/delete', { method: 'POST', json: { id } });
    toast('Item apagado.', 'ok');
    renderTrashPage();
  } catch (e) { toast(e.message, 'error'); }
}

async function emptyTrash() {
  const ok = await confirmDialog('Esvaziar a lixeira?', 'Todos os itens serão apagados definitivamente.', { ok: 'Esvaziar', danger: true });
  if (!ok) return;
  try {
    const r = await api('/api/trash/empty', { method: 'POST', json: {} });
    toast(`${r.deleted} item(ns) apagado(s).`, 'ok');
    renderTrashPage();
  } catch (e) { toast(e.message, 'error'); }
}

/* ---------------- aparelhos confiáveis ---------------- */

async function loadTrustedDevices() {
  const box = $('#trusted-list');
  if (!box) return;
  try {
    const d = await api('/api/trusted');
    const list = d.devices || [];
    if (!list.length) {
      box.innerHTML = '<div class="kv"><span>Aparelhos confiáveis</span><b>nenhum</b></div>';
      return;
    }
    box.innerHTML = list.map(x => `
      <div class="kv"><span>${esc(x.label || 'navegador')}</span>
      <b>${esc(fmtWhen(x.added_at))} · até ${esc(fmtWhen(x.expires))}</b></div>`).join('');
  } catch (e) {
    box.innerHTML = `<div class="kv"><span>Aparelhos confiáveis</span><b>${esc(e.message)}</b></div>`;
  }
}

async function revokeTrustedDevices() {
  const ok = await confirmDialog('Esquecer todos os aparelhos?',
    'Todos precisarão digitar o código de verificação no próximo acesso. Este navegador também.', { ok: 'Esquecer', danger: true });
  if (!ok) return;
  try {
    await api('/api/trusted/revoke', { method: 'POST', json: {} });
    toast('Aparelhos esquecidos.', 'ok');
    loadTrustedDevices();
  } catch (e) { toast(e.message, 'error'); }
}

/* ---------------- duplicados ---------------- */

let dupReport = null;
let dupSelected = new Set();

async function renderDuplicatesPage() {
  const page = $('#page-duplicates');
  if (!page) return;
  page.innerHTML = `
    <div class="page-head">
      <div>
        <h1>Duplicados</h1>
        <p class="muted">Arquivos idênticos, comparados por SHA-256. Vêm do último backup.</p>
      </div>
      <div class="page-actions">
        <button class="btn small ghost" id="dup-refresh">${ico('refresh')} Atualizar</button>
      </div>
    </div>
    <div id="dup-body"><div class="card"><span class="tiny">Analisando…</span></div></div>`;

  $('#dup-refresh').addEventListener('click', () => loadDuplicates(true));
  loadDuplicates(false);
}

async function loadDuplicates(force) {
  const body = $('#dup-body');
  if (!body) return;
  if (force) body.innerHTML = `<div class="card"><span class="tiny">Analisando…</span></div>`;
  try {
    const d = await api('/api/duplicates');
    dupReport = d;
    dupSelected = new Set();
    renderDuplicatesBody();
  } catch (e) {
    body.innerHTML = `<div class="card"><p class="muted">${esc(e.message)}</p></div>`;
  }
}

function renderDuplicatesBody() {
  const body = $('#dup-body');
  if (!body || !dupReport) return;
  const rep = dupReport;

  if (rep.note) {
    body.innerHTML = `<div class="card"><p class="muted" style="margin:0">${esc(rep.note)}</p></div>`;
    return;
  }

  if (!rep.groups || !rep.groups.length) {
    body.innerHTML = `<div class="card">
      <div class="file-empty" style="padding:26px 10px">
        <div class="empty-ico">${ico('check')}</div>
        <b>Nenhum duplicado</b>
        <span>Nenhum arquivo repetido no snapshot de ${esc(rep.at || '—')}.</span>
      </div></div>`;
    return;
  }

  const summary = `<div class="card dup-summary">
    <div class="dup-stats">
      <div><b>${esc(fmtBytes(rep.total_waste))}</b><span>recuperáveis</span></div>
      <div><b>${rep.group_count}</b><span>grupo(s)</span></div>
      <div><b>${rep.file_count}</b><span>arquivo(s)</span></div>
    </div>
    <p class="tiny" style="margin:8px 0 0">Snapshot de ${esc(rep.at || '—')}. O arquivo sugerido como "manter" nunca é apagado, e cada grupo preserva ao menos uma cópia.</p>
  </div>`;

  const groups = rep.groups.map((g, gi) => {
    const files = g.files.map(f => {
      const isKeep = f.path === g.keep;
      const sel = dupSelected.has(f.path);
      return `<div class="dup-file${sel ? ' selected' : ''}${isKeep ? ' keep' : ''}" data-dup-path="${esc(f.path)}">
        <span class="dup-check">${isKeep
          ? `<span class="chip ok">manter</span>`
          : `<input type="checkbox" data-dup-select="${esc(f.path)}" ${sel ? 'checked' : ''} aria-label="Selecionar ${esc(f.path)}">`}</span>
        <span class="dup-path mono">${esc(f.path)}</span>
        <span class="dup-size">${fmtBytes(f.size)}</span>
      </div>`;
    }).join('');
    return `<div class="card dup-group">
      <div class="dup-head">
        <b>${g.count} cópias · ${esc(fmtBytes(g.size))} cada</b>
        <span class="chip">${esc(fmtBytes(g.wasted))} recuperáveis</span>
      </div>
      <div class="dup-files">${files}</div>
    </div>`;
  }).join('');

  body.innerHTML = summary + groups + `
    <div class="dup-actions">
      <span class="tiny">${dupSelected.size} selecionado(s)</span>
      <button class="btn small danger" id="dup-clean" ${dupSelected.size ? '' : 'disabled'}>
        ${ico('trash')} Mover selecionados para a lixeira
      </button>
    </div>`;

  $$('[data-dup-select]', body).forEach(cb => {
    cb.addEventListener('change', () => {
      if (cb.checked) dupSelected.add(cb.dataset.dupSelect);
      else dupSelected.delete(cb.dataset.dupSelect);
      renderDuplicatesBody();
    });
  });
  const btn = $('#dup-clean');
  if (btn) btn.addEventListener('click', cleanupDuplicates);
}

async function cleanupDuplicates() {
  const paths = [...dupSelected];
  if (!paths.length) return;
  const ok = await confirmDialog(
    'Mover para a lixeira?',
    `${paths.length} cópia(s) vão para a lixeira e podem ser restauradas. ` +
    `A cópia marcada como "manter" nunca é removida.`,
    { ok: 'Mover', danger: true });
  if (!ok) return;
  try {
    const r = await api('/api/duplicates/cleanup', { method: 'POST', json: { paths } });
    toast(`${r.trashed} item(ns) na lixeira.`, 'ok', 'Espaço liberado');
    if (r.skipped) toast(`${r.skipped} ignorado(s) por segurança.`, 'info');
    await loadDuplicates(true);
  } catch (e) {
    toast(e.message, 'error', 'Não foi possível limpar');
  }
}
