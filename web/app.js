'use strict';

/* ============================================================
   PocketServe — núcleo da interface
   ============================================================ */

const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

const ICONS = {
  home: '<path d="M3 10.5 12 3.5l9 7V20a1 1 0 0 1-1 1h-5.2v-6.4H9.2V21H4a1 1 0 0 1-1-1Z"/>',
  folder: '<path d="M3.2 6.7A1.7 1.7 0 0 1 4.9 5h4l2 2.1h8.2a1.7 1.7 0 0 1 1.7 1.7v9.5a1.7 1.7 0 0 1-1.7 1.7H4.9a1.7 1.7 0 0 1-1.7-1.7Z"/>',
  bolt: '<path d="M13 2 4.5 13.5H11l-1 8.5 8.5-11.5H12z"/>',
  sd: '<rect x="6" y="3" width="12" height="18" rx="2"/><path d="M9.5 7h5v3h-5zM10 14h4M10 17h4"/>',
  server: '<rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.5h.01M7 16.5h.01"/>',
  share: '<circle cx="18" cy="5" r="2.6"/><circle cx="6" cy="12" r="2.6"/><circle cx="18" cy="19" r="2.6"/><path d="m8.4 10.8 7.2-4.2M8.4 13.2l7.2 4.2"/>',
  activity: '<path d="M3 12h4l2.5-6 4 12L16 12h5"/>',
  wifi: '<path d="M4.5 11.5a11 11 0 0 1 15 0M7.8 15a6 6 0 0 1 8.4 0M12 18.6h.01"/>',
  settings: '<path d="M4 6h9M18 6h2M4 12h3M10 12h10M4 18h8M17 18h3"/><circle cx="15" cy="6" r="2.1"/><circle cx="8" cy="12" r="2.1"/><circle cx="15" cy="18" r="2.1"/>',
  qr: '<rect x="3" y="3" width="7" height="7" rx="1.4"/><rect x="14" y="3" width="7" height="7" rx="1.4"/><rect x="3" y="14" width="7" height="7" rx="1.4"/><path d="M14 14h3v3h-3zM20 14h1M14 20h1M18 18h3v3h-3z"/>',
  upload: '<path d="M12 16V4M7 9l5-5 5 5M4 16v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3"/>',
  download: '<path d="M12 4v12M7 11l5 5 5-5M4 20h16"/>',
  logout: '<path d="M9 4H5a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h4M16 17l5-5-5-5M21 12H9"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  more: '<circle cx="12" cy="5" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="12" cy="19" r="1"/>',
  chevron: '<path d="m9 18 6-6-6-6"/>',
  grid: '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
  list: '<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>',
  x: '<path d="M6 6l12 12M18 6 6 18"/>',
  check: '<path d="m5 12 4.5 4.5L19 7"/>',
  link: '<path d="M10 13.5a4 4 0 0 0 5.7 0l2.6-2.6a4 4 0 0 0-5.7-5.7L11.4 6.4"/><path d="M14 10.5a4 4 0 0 0-5.7 0l-2.6 2.6a4 4 0 0 0 5.7 5.7l1.2-1.2"/>',
  refresh: '<path d="M20 11a8 8 0 1 0-2.3 5.7M20 4v7h-7"/>',
  trash: '<path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 11v6M14 11v6"/>',
  edit: '<path d="M12 20h9M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/>',
  move: '<path d="M3.5 6.8A1.8 1.8 0 0 1 5.3 5h4l2 2.2h7.4a1.8 1.8 0 0 1 1.8 1.8v8.2a1.8 1.8 0 0 1-1.8 1.8H5.3a1.8 1.8 0 0 1-1.8-1.8Z"/><path d="m9.5 14 4-4 4 4M13.5 10v7"/>',
  copy: '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>',
  file: '<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><path d="M14 3v6h6"/>',
  image: '<rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="8.5" cy="9.5" r="1.5"/><path d="m21 15-5-5L5 20"/>',
  video: '<rect x="3" y="6" width="13" height="12" rx="2"/><path d="m16 10 5-3v10l-5-3z"/>',
  audio: '<path d="M9 18V5l10-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="16" cy="16" r="3"/>',
  pdf: '<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><path d="M14 3v6h6M8 15h2.5a1.5 1.5 0 0 0 0-3H8v5M14 17v-5h1.5a1.5 1.5 0 0 1 0 5z"/>',
  archive: '<rect x="3" y="4" width="18" height="4" rx="1"/><path d="M5 8v11a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V8M10 12h4"/>',
  text: '<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><path d="M14 3v6h6M8 13h8M8 17h6"/>',
  doc: '<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><path d="M14 3v6h6M8 13h8M8 17h8"/>',
  sheet: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 10h18M9 4v16M15 4v16"/>',
  slides: '<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M12 16v4M8 21h8"/>',
  folderPlus: '<path d="M3.2 6.7A1.7 1.7 0 0 1 4.9 5h4l2 2.1h8.2a1.7 1.7 0 0 1 1.7 1.7v9.5a1.7 1.7 0 0 1-1.7 1.7H4.9a1.7 1.7 0 0 1-1.7-1.7Z"/><path d="M12 11v5M9.5 13.5h5"/>',
  server: '<rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.5h.01M7 16.5h.01M11 7.5h5M11 16.5h5"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  shield: '<path d="M12 3 5 6v6c0 4.4 3 7.5 7 9 4-1.5 7-4.6 7-9V6z"/><path d="m9 12 2 2 4-4"/>',
  key: '<circle cx="8" cy="15" r="4"/><path d="m11 12 9-9M17 6l3 3M14 9l3 3"/>',
  eye: '<path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z"/><circle cx="12" cy="12" r="3"/>',
  eyeOff: '<path d="m3 3 18 18M10.6 6.1A10.9 10.9 0 0 1 12 6c6 0 9.5 6 9.5 6a17 17 0 0 1-3 3.7M6.3 6.4A16.8 16.8 0 0 0 2.5 12S6 18 12 18a10.8 10.8 0 0 0 4.1-.8"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/>',
  battery: '<rect x="2.5" y="7" width="17" height="10" rx="2"/><path d="M22 10v4"/><rect x="5" y="9.5" width="9" height="5" rx="1" fill="currentColor" stroke="none"/>',
  cpu: '<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9.5" y="9.5" width="5" height="5" rx="1"/><path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4"/>',
  memory: '<rect x="4" y="7" width="16" height="10" rx="2"/><path d="M8 7V4M12 7V4M16 7V4M8 20v-3M12 20v-3M16 20v-3M4 11H1M4 15H1M23 11h-3M23 15h-3"/>',
  disk: '<ellipse cx="12" cy="6" rx="8" ry="3"/><path d="M4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/>',
  cloud: '<path d="M17.5 19a4.5 4.5 0 0 0 .6-9 6 6 0 0 0-11.5 1.4A4 4 0 0 0 6.5 19Z"/>',
  phone: '<rect x="6" y="2.5" width="12" height="19" rx="3"/><path d="M10.5 18.5h3"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>',
  external: '<path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/>',
  terminal: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7 9 3 3-3 3M13 15h4"/>',
  lock: '<rect x="4" y="10" width="16" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>',
  unlock: '<rect x="4" y="10" width="16" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 7.5-2"/>',
  zap: '<path d="M13 2 4 14h7l-1 8 9-12h-7z"/>',
  menu: '<path d="M4 6h16M4 12h16M4 18h16"/>',
  arrowUpRight: '<path d="M7 17 17 7M8 7h9v9"/>',
  sort: '<path d="M8 5v14M4 9l4-4 4 4M16 19V5M12 15l4 4 4-4"/>',
  filter: '<path d="M3 5h18l-7 8v6l-4-2v-4z"/>',
  folderMove: '<path d="M3.2 6.7A1.7 1.7 0 0 1 4.9 5h4l2 2.1h8.2a1.7 1.7 0 0 1 1.7 1.7v9.5a1.7 1.7 0 0 1-1.7 1.7H4.9a1.7 1.7 0 0 1-1.7-1.7Z"/><path d="M9 14h6M12 11l3 3-3 3"/>',
  chevronDown: '<path d="m6 9 6 6 6-6"/>',
  arrowUp: '<path d="M12 19V5M5 12l7-7 7 7"/>',
  arrowDown: '<path d="M12 5v14M19 12l-7 7-7-7"/>',
  moreVertical: '<circle cx="12" cy="5" r="1.4"/><circle cx="12" cy="12" r="1.4"/><circle cx="12" cy="19" r="1.4"/>',
  minimize: '<path d="M5 12h14"/>',
  maximize: '<path d="M8 3H5a2 2 0 0 0-2 2v3M16 3h3a2 2 0 0 1 2 2v3M8 21H5a2 2 0 0 1-2-2v-3M16 21h3a2 2 0 0 0 2-2v-3"/>'
};

function ico(name, cls = '') {
  const p = ICONS[name] || ICONS.file;
  return `<svg class="ico ${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${p}</svg>`;
}

function hydrateIcons(root = document) {
  $$('[data-icon]', root).forEach(el => { el.innerHTML = ico(el.dataset.icon); });
}

function logoMarkup() {
  return `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <path d="M20.5 44h24a9.5 9.5 0 0 0 1.2-18.9A13.5 13.5 0 0 0 19.4 26.5a8 8 0 0 0 1.1 17.5Z" fill="#071018" fill-opacity=".9"/>
    <path d="M32 39V25m-5 5 5-5 5 5" stroke="#d9fff9" stroke-width="3.2" stroke-linecap="round" stroke-linejoin="round"/>
  </svg>`;
}

/* ---------------- formatting ---------------- */

function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

// Usa base decimal (1 GB = 1000³), que é como o Android e os fabricantes
// anunciam a capacidade. Com base binária, 8 GB apareciam como "7,5 GB".
function fmtBytes(n) {
  if (n === null || n === undefined || isNaN(n)) return '—';
  if (n === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0, x = Number(n);
  while (x >= 1000 && i < units.length - 1) { x /= 1000; i++; }
  const digits = i === 0 ? 0 : (x >= 100 ? 0 : 1);
  return `${x.toFixed(digits).replace('.', ',')} ${units[i]}`;
}

function fmtDate(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  if (isNaN(d)) return '—';
  return d.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: '2-digit' });
}

function fmtDateTime(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  if (isNaN(d)) return '—';
  return d.toLocaleString('pt-BR', { day: '2-digit', month: '2-digit', year: '2-digit', hour: '2-digit', minute: '2-digit' });
}

function fmtRelative(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  const sec = (Date.now() - d.getTime()) / 1000;
  if (sec < 45) return 'agora';
  if (sec < 3600) return `${Math.floor(sec / 60)} min`;
  if (sec < 86400) return `${Math.floor(sec / 3600)} h`;
  if (sec < 604800) return `${Math.floor(sec / 86400)} d`;
  return fmtDate(iso);
}

function fmtUptime(sec) {
  sec = Math.max(0, Number(sec) || 0);
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d} d ${h} h`;
  if (h > 0) return `${h} h ${m} min`;
  return `${m} min`;
}

function encodePath(path) {
  return String(path || '').split('/').filter(Boolean).map(encodeURIComponent).join('/');
}

function greeting() {
  const h = new Date().getHours();
  if (h < 5) return 'Boa madrugada';
  if (h < 12) return 'Bom dia';
  if (h < 18) return 'Boa tarde';
  return 'Boa noite';
}

/* ---------------- state ---------------- */

const S = {
  state: null,
  name: 'PocketServe',
  page: 'dashboard',
  sys: null,
  sysBusy: false,
  hist: { cpu: [], mem: [] },
  files: {
    path: '', parent: '', entries: [], loadedPath: null,
    loading: false, loadError: '', writable: true,
    roots: [], rootLabel: 'Armazenamento', trashDays: null, primaryFree: 0,
    selection: new Set(), lastChecked: null,
    showHidden: localStorage.getItem('ps_hidden') === '1',
    view: localStorage.getItem('ps_view') || 'list',
    sort: localStorage.getItem('ps_sort') || 'name',
    dir: localStorage.getItem('ps_sort_dir') || 'asc',
    filter: localStorage.getItem('ps_filter') || 'all',
    query: '', searching: false, results: null, remoteSearch: false, searchSeq: 0
  },
  shares: [],
  activity: [],
  settings: null,
  uploads: [],
  uploadSeq: 0,
  uploadsCollapsed: false,
  bound: false,
  deferredPrompt: null
};

/* ---------------- api ---------------- */

class ApiError extends Error {
  constructor(message, status, code) { super(message); this.status = status; this.code = code; }
}

async function api(path, opts = {}) {
  const headers = Object.assign({ 'X-PS': '1' }, opts.headers || {});
  let body = opts.body;
  if (opts.json !== undefined) {
    headers['Content-Type'] = 'application/json';
    body = JSON.stringify(opts.json);
  }
  let res;
  try {
    res = await fetch(path, { method: opts.method || 'GET', headers, body, credentials: 'same-origin' });
  } catch (e) {
    throw new ApiError('Não foi possível conectar ao servidor.', 0, 'network');
  }
  const ct = res.headers.get('content-type') || '';
  let data = null;
  try { data = ct.includes('application/json') ? await res.json() : await res.text(); } catch { data = null; }
  if (!res.ok) {
    if (res.status === 401 && !opts.noAuthRedirect) {
      showLogin('Sua sessão expirou. Entre novamente.');
    }
    const msg = (data && data.error) || `Falha na requisição (${res.status})`;
    throw new ApiError(msg, res.status, data && data.code);
  }
  return data;
}

/* ---------------- toast / copy ---------------- */

function toast(message, type = 'ok', title = '') {
  const root = $('#toasts');
  if (!root) return;
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  const icon = type === 'error' ? 'x' : type === 'info' ? 'info' : 'check';
  el.innerHTML = `<span class="t-ico">${ico(icon)}</span><div><b>${esc(title || (type === 'error' ? 'Algo deu errado' : type === 'info' ? 'Aviso' : 'Tudo certo'))}</b><span>${esc(message)}</span></div>`;
  root.appendChild(el);
  setTimeout(() => { el.classList.add('leaving'); setTimeout(() => el.remove(), 250); }, type === 'error' ? 5200 : 3400);
}

async function copyText(text) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      toast('Copiado para a área de transferência.', 'ok', 'Copiado');
      return true;
    }
  } catch {}
  try {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.cssText = 'position:fixed;left:-9999px;top:0;opacity:0';
    document.body.appendChild(ta);
    ta.focus(); ta.select();
    const ok = document.execCommand('copy');
    ta.remove();
    if (ok) { toast('Copiado para a área de transferência.', 'ok', 'Copiado'); return true; }
  } catch {}
  toast('Não foi possível copiar automaticamente.', 'error');
  return false;
}

/* ---------------- modal ---------------- */

let modalOnClose = null;
let modalEscHandler = null;

function openModal(opts) {
  closeModal();
  const root = $('#modal-root');
  root.hidden = false;
  const cls = `modal${opts.wide ? ' wide' : ''}${opts.preview ? ' preview' : ''}`;
  root.innerHTML = `<div class="modal-backdrop">
    <div class="${cls}" role="dialog" aria-modal="true">
      <div class="modal-head">
        <div min-width="0"><h3>${opts.title || ''}${opts.sub ? `<span class="sub">${opts.sub}</span>` : ''}</h3></div>
        <button class="icon-btn modal-close" data-close aria-label="Fechar">${ico('x')}</button>
      </div>
      ${opts.body !== undefined ? `<div class="modal-bod${opts.flush ? ' flush' : ''}">${opts.body}</div>` : ''}
      ${opts.footer !== undefined ? `<div class="modal-foot">${opts.footer}</div>` : ''}
    </div>
  </div>`;
  const backdrop = root.querySelector('.modal-backdrop');
  backdrop.addEventListener('mousedown', e => { if (e.target === backdrop) closeModal(); });
  root.querySelectorAll('[data-close]').forEach(b => b.addEventListener('click', closeModal));
  modalEscHandler = e => { if (e.key === 'Escape') closeModal(); };
  document.addEventListener('keydown', modalEscHandler);
  modalOnClose = opts.onClose || null;
  hydrateIcons(root);
  return root.querySelector('.modal');
}

function closeModal() {
  const root = $('#modal-root');
  if (!root || root.hidden) return;
  root.hidden = true;
  root.innerHTML = '';
  if (modalEscHandler) { document.removeEventListener('keydown', modalEscHandler); modalEscHandler = null; }
  const cb = modalOnClose; modalOnClose = null;
  if (cb) cb(false);
}

function confirmDialog(title, message, opts = {}) {
  return new Promise(resolve => {
    let settled = false;
    const done = v => { if (!settled) { settled = true; resolve(v); } };
    openModal({
      title: esc(title),
      body: `<p>${message}</p>`,
      footer: `<button class="btn ghost" data-close>Cancelar</button><button class="btn ${opts.danger ? 'danger' : 'primary'}" id="modal-confirm">${esc(opts.ok || 'Confirmar')}</button>`,
      onClose: () => done(false)
    });
    $('#modal-confirm').addEventListener('click', () => { done(true); closeModal(); });
  });
}

function promptDialog(title, label, value = '', opts = {}) {
  return new Promise(resolve => {
    let settled = false;
    const done = v => { if (!settled) { settled = true; resolve(v); } };
    openModal({
      title: esc(title),
      body: `<div class="field"><label>${esc(label)}</label><input class="input" id="modal-input" value="${esc(value)}" ${opts.placeholder ? `placeholder="${esc(opts.placeholder)}"` : ''} autocomplete="off"></div>${opts.hint ? `<p class="hint">${opts.hint}</p>` : ''}`,
      footer: `<button class="btn ghost" data-close>Cancelar</button><button class="btn primary" id="modal-confirm">${esc(opts.ok || 'Salvar')}</button>`,
      onClose: () => done(null)
    });
    const input = $('#modal-input');
    input.focus();
    if (opts.select !== false) input.select();
    const submit = () => { const v = input.value.trim(); if (!v) { input.focus(); return; } done(v); closeModal(); };
    $('#modal-confirm').addEventListener('click', submit);
    input.addEventListener('keydown', e => { if (e.key === 'Enter') { e.preventDefault(); submit(); } });
  });
}

function qrDialog(title, url) {
  const qr = `/api/qr?text=${encodeURIComponent(url)}`;
  openModal({
    title: esc(title),
    sub: 'Aponte a câmera do celular para o código',
    body: `<div class="qr-large"><img src="${qr}" alt="QR code"></div>
      <div class="copy-line"><input class="input" value="${esc(url)}" readonly id="qr-url"><button class="btn ghost" id="qr-copy">${ico('copy')} Copiar</button></div>`,
    footer: `<a class="btn ghost" href="${esc(url)}" target="_blank" rel="noopener">${ico('external')} Abrir</a><button class="btn primary" data-close>Fechar</button>`
  });
  $('#qr-copy').addEventListener('click', () => copyText(url));
  $('#qr-url').addEventListener('click', e => e.target.select());
}

/* ---------------- context menu ---------------- */

let ctxEscHandler = null;

function closeCtx() {
  const root = $('#ctx-root');
  if (root) root.innerHTML = '';
  document.removeEventListener('click', ctxOutside, true);
  if (ctxEscHandler) {
    document.removeEventListener('keydown', ctxEscHandler);
    ctxEscHandler = null;
  }
}

function ctxOutside(e) {
  if (!e.target.closest('.ctx')) closeCtx();
}

function openCtx(anchor, items, opts = {}) {
  closeCtx();
  const root = $('#ctx-root');
  if (!root) return;

  const sheet = opts.sheet !== false && window.matchMedia('(max-width: 560px)').matches;
  if (sheet) {
    const back = document.createElement('div');
    back.className = 'ctx-backdrop';
    back.addEventListener('click', closeCtx);
    root.appendChild(back);
  }

  const el = document.createElement('div');
  el.className = `ctx${sheet ? ' sheet' : ''}`;
  const head = opts.title
    ? `<div class="ctx-head"><b>${esc(opts.title)}</b><button type="button" data-ctx-close aria-label="Fechar">${ico('x')}</button></div>`
    : '';
  el.innerHTML = `${head}${items.map((it, i) => it.sep
    ? '<div class="sep"></div>'
    : `<button type="button" class="${it.danger ? 'danger ' : ''}${it.checked ? 'checked' : ''}" data-i="${i}" ${it.disabled ? 'disabled' : ''}>${ico(it.icon || 'file')}<span class="ctx-label">${esc(it.label)}</span>${it.meta ? `<span class="ctx-meta">${esc(it.meta)}</span>` : ''}${it.checked ? `<span class="ctx-meta">${ico('check')}</span>` : ''}</button>`).join('')}`;
  root.appendChild(el);

  if (!sheet) {
    const r = anchor.getBoundingClientRect();
    const w = el.offsetWidth || 230;
    const h = el.offsetHeight || items.length * 38;
    let left = r.right - w;
    if (left < 8) left = 8;
    if (left + w > window.innerWidth - 8) left = window.innerWidth - w - 8;
    let top = r.bottom + 6;
    if (top + h > window.innerHeight - 8) top = r.top - h - 6;
    el.style.left = `${Math.max(8, left)}px`;
    el.style.top = `${Math.max(8, top)}px`;
  }

  el.querySelectorAll('button').forEach(btn => {
    if (btn.hasAttribute('data-ctx-close')) {
      btn.addEventListener('click', closeCtx);
      return;
    }
    btn.addEventListener('click', () => {
      const it = items[Number(btn.dataset.i)];
      closeCtx();
      if (it && it.action) it.action();
    });
  });

  if (!sheet) setTimeout(() => document.addEventListener('click', ctxOutside, true), 0);
  ctxEscHandler = e => { if (e.key === 'Escape') closeCtx(); };
  document.addEventListener('keydown', ctxEscHandler);
}

/* ---------------- auth ---------------- */

function showLogin(message = '') {
  $('#boot').hidden = true;
  $('#shell').hidden = true;
  const auth = $('#auth');
  auth.hidden = false;
  $('#auth-card').innerHTML = `
    <div class="auth-logo logo" data-logo>${logoMarkup()}</div>
    <span class="auth-badge"><span class="dot live"></span> ${esc(S.name || 'PocketServe')}</span>
    <h1 class="auth-title">Bem-vindo de volta</h1>
    <p class="auth-sub">Entre com a senha do administrador para acessar seus arquivos.</p>
    ${message ? `<div class="auth-error err" id="auth-error">${esc(message)}</div>` : ''}
    <form class="auth-form" id="login-form" autocomplete="on">
      <div class="field">
        <label for="login-pw">Senha</label>
        <div class="pw-wrap">
          <input class="input" id="login-pw" type="password" autocomplete="current-password" autocapitalize="none" autocorrect="off" spellcheck="false" inputmode="text" required autofocus placeholder="Sua senha">
          <button type="button" class="pw-toggle" data-eye="login-pw" aria-label="Mostrar senha">${ico('eye')}</button>
        </div>
      </div>
      <div class="field" id="code-field" hidden>
        <label for="login-code">Código de verificação</label>
        <input class="input" id="login-code" inputmode="numeric" autocomplete="one-time-code" autocapitalize="characters" autocorrect="off" spellcheck="false" maxlength="9" placeholder="000000">
        <span class="hint">Do seu autenticador, ou um código de recuperação.</span>
      </div>
      <label class="trust-row" id="trust-row" hidden>
        <input type="checkbox" id="login-remember" checked>
        <span>Lembrar deste aparelho por 30 dias</span>
      </label>
      <button class="btn primary" type="submit" id="login-btn" style="width:100%;padding:12px">Entrar</button>
    </form>
    <div class="auth-foot">Servidor pessoal · ${esc(location.host || 'local')}</div>`;
  bindEyeToggles($('#auth-card'));
  $('#login-form').addEventListener('submit', async e => {
    e.preventDefault();
    const btn = $('#login-btn');
    const pw = $('#login-pw').value;
    const codeEl = $('#login-code');
    const code = codeEl ? codeEl.value.replace(/\s+/g, '').trim() : '';
    const rememberEl = $('#login-remember');
    const remember = rememberEl ? rememberEl.checked : false;
    btn.disabled = true; btn.textContent = 'Entrando…';
    try {
      await api('/api/login', { method: 'POST', json: { password: pw, code, remember }, noAuthRedirect: true });
      await enterApp();
    } catch (err) {
      btn.disabled = false; btn.textContent = 'Entrar';
      // 2FA ativo: revela o campo de código e move o foco para ele.
      if (err.code === 'totp_required' || err.code === 'bad_code') {
        const f = $('#code-field');
        const tr = $('#trust-row');
        if (f) f.hidden = false;
        if (tr) tr.hidden = false;
        if (codeEl) { codeEl.focus(); codeEl.select(); }
      } else {
        $('#login-pw').select();
      }
      const box = $('#auth-error');
      if (box) box.textContent = err.message;
      else {
        const d = document.createElement('div'); d.className = 'auth-error err'; d.id = 'auth-error'; d.textContent = err.message;
        $('#login-form').before(d);
      }
    }
  });
}

function showSetup() {
  $('#boot').hidden = true;
  $('#shell').hidden = true;
  const auth = $('#auth');
  auth.hidden = false;
  $('#auth-card').innerHTML = `
    <div class="auth-logo logo" data-logo>${logoMarkup()}</div>
    <span class="auth-badge"><span class="dot live"></span> Primeiro acesso</span>
    <h1 class="auth-title">Vamos configurar</h1>
    <p class="auth-sub">Crie a senha do administrador. Ela protege o acesso ao servidor e aos seus arquivos.</p>
    <form class="auth-form" id="setup-form">
      <div class="field">
        <label for="setup-name">Nome do servidor</label>
        <input class="input" id="setup-name" maxlength="40" value="${esc(S.name || 'PocketServe')}" placeholder="PocketServe">
      </div>
      <div class="field">
        <label for="setup-pw">Senha</label>
        <div class="pw-wrap">
          <input class="input" id="setup-pw" type="password" minlength="8" autocomplete="new-password" required placeholder="Mínimo de 8 caracteres">
          <button type="button" class="pw-toggle" data-eye="setup-pw" aria-label="Mostrar senha">${ico('eye')}</button>
        </div>
      </div>
      <div class="field">
        <label for="setup-pw2">Confirmar senha</label>
        <input class="input" id="setup-pw2" type="password" minlength="8" autocomplete="new-password" required placeholder="Repita a senha">
      </div>
      <button class="btn primary" type="submit" id="setup-btn" style="width:100%;padding:12px">Criar servidor</button>
    </form>
    <div class="auth-foot">Você poderá trocar a senha depois, em Configurações.</div>`;
  bindEyeToggles($('#auth-card'));
  $('#setup-form').addEventListener('submit', async e => {
    e.preventDefault();
    const name = $('#setup-name').value.trim() || 'PocketServe';
    const pw = $('#setup-pw').value;
    const pw2 = $('#setup-pw2').value;
    const errBox = () => {
      let b = $('#auth-error');
      if (!b) { b = document.createElement('div'); b.className = 'auth-error err'; b.id = 'auth-error'; $('#setup-form').before(b); }
      return b;
    };
    if (pw.length < 8) { errBox().textContent = 'A senha precisa ter pelo menos 8 caracteres.'; return; }
    if (pw !== pw2) { errBox().textContent = 'As senhas não conferem.'; return; }
    const btn = $('#setup-btn');
    btn.disabled = true; btn.textContent = 'Criando…';
    try {
      await api('/api/setup', { method: 'POST', json: { password: pw, server_name: name }, noAuthRedirect: true });
      S.name = name;
      await enterApp();
    } catch (err) {
      btn.disabled = false; btn.textContent = 'Criar servidor';
      errBox().textContent = err.message;
    }
  });
}

function bindEyeToggles(root) {
  $$('[data-eye]', root).forEach(btn => {
    btn.addEventListener('click', () => {
      const input = document.getElementById(btn.dataset.eye);
      if (!input) return;
      const show = input.type === 'password';
      input.type = show ? 'text' : 'password';
      btn.innerHTML = ico(show ? 'eyeOff' : 'eye');
    });
  });
}

async function enterApp() {
  $('#auth').hidden = true;
  $('#boot').hidden = true;
  $('#shell').hidden = false;
  $$('[data-logo]').forEach(el => el.innerHTML = logoMarkup());
  hydrateIcons();
  try {
    const st = await api('/api/state', { noAuthRedirect: true });
    S.state = st;
    S.name = st.name || S.name || 'PocketServe';
  } catch {}
  $('#brand-name').textContent = S.name;
  const topbarName = $('#topbar-name');
  if (topbarName) topbarName.textContent = S.name;
  document.title = `${S.name} · Servidor pessoal`;
  if (!location.hash) history.replaceState(null, '', '#/dashboard');
  bindGlobals();
  renderRoute();
  startPolling();
  loadSystem(true);
}

/* ---------------- router / shell ---------------- */

const TITLES = { dashboard: 'Visão geral', files: 'Arquivos', shares: 'Compartilhamentos', activity: 'Atividade', trash: 'Lixeira', duplicates: 'Duplicados', access: 'Acesso', settings: 'Configurações' };

function parseHash() {
  const h = location.hash.replace(/^#\/?/, '');
  const i = h.indexOf('/');
  return i < 0 ? { page: h || 'dashboard', arg: '' } : { page: h.slice(0, i), arg: h.slice(i + 1) };
}

function navigate(page, arg) {
  const hash = arg ? `#/${page}/${encodePath(arg)}` : `#/${page}`;
  if (location.hash === hash) renderRoute();
  else location.hash = hash;
}

function renderRoute() {
  const { page, arg } = parseHash();
  const known = Object.keys(TITLES);
  S.page = known.includes(page) ? page : 'dashboard';
  $$('.page').forEach(p => p.classList.toggle('active', p.id === `page-${S.page}`));
  $$('[data-page]').forEach(a => a.classList.toggle('active', a.dataset.page === S.page));
  const sub = $('#topbar-sub');
  if (sub) sub.textContent = TITLES[S.page];
  if (S.page === 'dashboard') renderDashboard();
  else if (S.page === 'files') {
    let raw = '';
    if (arg) {
      try { raw = decodeURIComponent(arg); } catch { raw = arg; }
    }
    S.files.path = raw;
    renderFilesPage();
  } else if (S.page === 'shares') renderSharesPage();
  else if (S.page === 'activity') renderActivityPage();
  else if (S.page === 'trash') renderTrashPage();
  else if (S.page === 'duplicates') renderDuplicatesPage();
  else if (S.page === 'access') renderAccessPage();
  else if (S.page === 'settings') renderSettingsPage();
  window.scrollTo(0, 0);
}

let pollTimer = null;
function startPolling() {
  if (pollTimer) return;
  pollTimer = setInterval(() => {
    if (document.hidden) return;
    if (S.page === 'dashboard' || S.page === 'access') loadSystem();
  }, 4000);
}

async function loadSystem(force = false) {
  if (S.sysBusy) return;
  S.sysBusy = true;
  try {
    const sys = await api('/api/system');
    S.sys = sys;
    if (S.page === 'dashboard') updateDashboard();
    if (S.page === 'access') updateAccessLive();
    updateSidebar(sys);
  } catch (e) {
    if (force && S.page === 'dashboard') {
      const el = $('#hero-online');
      if (el) el.innerHTML = '<span class="dot" style="background:#f87171"></span> Sem conexão';
    }
  } finally {
    S.sysBusy = false;
  }
}

function updateSidebar(sys) {
  const u = $('#side-uptime');
  if (u) u.textContent = `No ar há ${fmtUptime(sys.server_uptime)}`;
}

/* ---------------- global bindings ---------------- */

function bindGlobals() {
  if (S.bound) return;
  S.bound = true;
  window.addEventListener('hashchange', renderRoute);

  document.addEventListener('click', e => {
    const actEl = e.target.closest('[data-act]');
    if (actEl) {
      const act = actEl.dataset.act;
      if (act === 'upload') { e.preventDefault(); $('#file-input').click(); }
      else if (act === 'upload-folder') { e.preventDefault(); $('#folder-input').click(); }
      else if (act === 'upload-menu') { e.preventDefault(); openUploadMenu(actEl); }
      else if (act === 'new-folder') { e.preventDefault(); createFolderDialog(S.page === 'files' ? S.files.path : ''); }
      else if (act === 'files-up') { e.preventDefault(); goToParentFolder(); }
      else if (act === 'files-sort') { e.preventDefault(); openSortMenu(actEl); }
      else if (act === 'toggle-hidden') { e.preventDefault(); toggleHiddenFiles(); }
      else if (act === 'qr-access') { e.preventDefault(); showAccessQR(); }
      else if (act === 'logout') { e.preventDefault(); doLogout(); }
      else if (act === 'refresh') { e.preventDefault(); refreshCurrent(); }
      return;
    }
    const copyEl = e.target.closest('[data-copy]');
    if (copyEl) { e.preventDefault(); copyText(copyEl.dataset.copy); }
  });

  const fileInput = $('#file-input');
  fileInput.addEventListener('change', e => {
    const files = Array.from(e.target.files || []);
    e.target.value = '';
    if (!files.length) return;
    const dest = S.page === 'files' ? S.files.path : '';
    const spread = consumeSpreadFlag();
    uploadFiles(files, dest, false, { spread });
  });
  const folderInput = $('#folder-input');
  folderInput.addEventListener('change', e => {
    const files = Array.from(e.target.files || []);
    e.target.value = '';
    if (!files.length) return;
    const dest = S.page === 'files' ? S.files.path : '';
    const spread = consumeSpreadFlag();
    // Pasta mantém a estrutura dentro de cada volume, então espalhar por
    // arquivo quebraria subpastas. O spread de pasta envia a árvore inteira
    // para o volume com mais espaço.
    if (spread) {
      const plan = pickVolumeForTree();
      uploadFiles(files, plan, true);
      return;
    }
    uploadFiles(files, dest, true);
  });

  window.addEventListener('beforeunload', e => {
    if (S.uploads.some(i => i.status === 'uploading' || i.status === 'waiting')) {
      e.preventDefault();
      e.returnValue = '';
    }
  });

  window.addEventListener('beforeinstallprompt', e => {
    e.preventDefault();
    S.deferredPrompt = e;
  });
}

function goToParentFolder() {
  if (!S.files.path) return;
  const parent = S.files.path.split('/').filter(Boolean).slice(0, -1).join('/');
  navigate('files', parent);
}

function toggleHiddenFiles() {
  S.files.showHidden = !S.files.showHidden;
  persistFilesPrefs();
  updateHiddenButton();
  toast(S.files.showHidden ? 'Arquivos ocultos visíveis.' : 'Arquivos ocultos escondidos.', 'ok', 'Visualização');
  if (S.page === 'files') loadFiles(S.files.path, { keepSelection: true });
}

function setFileSort(key) {
  if (S.files.sort === key) S.files.dir = S.files.dir === 'desc' ? 'asc' : 'desc';
  else {
    S.files.sort = key;
    S.files.dir = key === 'name' ? 'asc' : 'desc';
  }
  persistFilesPrefs();
  updateFilesChrome();
  renderFilesBody();
}

function openSortMenu(anchor) {
  const sorts = [
    { id: 'name', label: 'Nome', icon: 'text' },
    { id: 'date', label: 'Modificado', icon: 'clock' },
    { id: 'size', label: 'Tamanho', icon: 'disk' },
    { id: 'type', label: 'Tipo', icon: 'filter' }
  ];
  const items = sorts.map(s => ({
    label: s.label,
    icon: s.icon,
    checked: S.files.sort === s.id,
    meta: S.files.sort === s.id ? (S.files.dir === 'desc' ? 'decrescente' : 'crescente') : '',
    action: () => setFileSort(s.id)
  }));
  items.push({ sep: true });
  items.push({
    label: S.files.dir === 'desc' ? 'Inverter para crescente' : 'Inverter para decrescente',
    icon: S.files.dir === 'desc' ? 'arrowUp' : 'arrowDown',
    action: () => {
      S.files.dir = S.files.dir === 'desc' ? 'asc' : 'desc';
      persistFilesPrefs();
      updateFilesChrome();
      renderFilesBody();
    }
  });
  openCtx(anchor, items, { title: 'Ordenar por' });
}

function openUploadMenu(anchor) {
  if (S.page === 'files' && !S.files.writable) {
    toast('Esta pasta é somente leitura.', 'error', 'Upload');
    return;
  }
  const destLabel = S.page === 'files' && S.files.path ? baseName(S.files.path) : baseRootLabel();
  const roots = (S.files && S.files.roots) || [];
  const items = [
    { label: 'Escolher arquivos', icon: 'upload', meta: 'para esta pasta', action: () => $('#file-input').click() },
    { label: 'Enviar uma pasta', icon: 'folderPlus', meta: 'com subpastas', action: () => $('#folder-input').click() }
  ];
  // Com dois ou mais volumes, oferece espalhar os arquivos por espaço livre.
  // Sem isto o usuário teria de enviar duas vezes, trocando de aba.
  if (roots.length && !(S.page === 'files' && S.files.path)) {
    items.push({ sep: true });
    items.push({
      label: 'Enviar para os dois',
      icon: 'sd',
      meta: 'divide por espaço livre',
      action: () => withSpreadUpload('#file-input')
    });
    items.push({
      label: 'Enviar pasta para os dois',
      icon: 'sd',
      meta: 'mantém a estrutura',
      action: () => withSpreadUpload('#folder-input')
    });
  }
  items.push({ sep: true });
  items.push({
    label: 'Nova pasta aqui',
    icon: 'folder',
    action: () => createFolderDialog(S.page === 'files' ? S.files.path : '')
  });
  openCtx(anchor, items, { title: `Enviar para ${destLabel}` });
}

// Marca o próximo envio como "espalhar entre volumes". O input nativo abre em
// seguida e o handler de change consome a flag.
let spreadUploadNext = false;

function withSpreadUpload(inputSel) {
  spreadUploadNext = true;
  const el = $(inputSel);
  if (el) el.click();
}

function consumeSpreadFlag() {
  const spread = spreadUploadNext;
  spreadUploadNext = false;
  return spread;
}

// pickVolumeForTree escolhe o volume com mais espaço para receber uma árvore
// de pastas inteira. Não dá para dividir uma pasta por arquivo sem quebrar a
// estrutura de subpastas.
function pickVolumeForTree() {
  const roots = (S.files && S.files.roots) || [];
  const vols = [
    { path: '', free: Number((S.files && S.files.primaryFree) || 0) },
    ...roots.map(r => ({ path: r.name, free: Number(r.free || 0) }))
  ].filter(v => v.free > 0);
  if (!vols.length) return '';
  vols.sort((a, b) => b.free - a.free);
  return vols[0].path;
}

async function doLogout() {
  const ok = await confirmDialog('Sair do painel?', 'Você precisará da senha para entrar novamente.', { ok: 'Sair', danger: true });
  if (!ok) return;
  try { await api('/api/logout', { method: 'POST' }); } catch {}
  S.state = null;
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
  closeModal();
  showLogin();
}

function refreshCurrent() {
  if (S.page === 'files') loadFiles(S.files.path);
  else if (S.page === 'shares') renderSharesPage();
  else if (S.page === 'activity') renderActivityPage();
  else if (S.page === 'duplicates') renderDuplicatesPage();
  else if (S.page === 'trash') renderTrashPage();
  else if (S.page === 'settings') renderSettingsPage();
  else if (S.page === 'access') loadSystem(true);
  else loadSystem(true);
}

function showAccessQR() {
  let url = location.origin;
  if (S.sys && S.sys.urls && S.sys.urls.length) {
    const local = S.sys.urls.find(u => u.includes('192.168.') || u.includes('10.') || u.includes('172.'));
    url = local || S.sys.urls[0];
  }
  qrDialog('Acesso rápido', url);
}

/* ---------------- uploads ---------------- */

function uploadFiles(files, dest, fromFolder = false, opts = {}) {
  if (S.page === 'files' && !S.files.writable) {
    toast('Esta pasta é somente leitura.', 'error', 'Upload');
    return;
  }
  if (!files || !files.length) return;

  // Destino explícito: o usuário escolhe o volume. Sem isso, o envio usa a
  // pasta aberta (comportamento previsível e sem surpresa).
  const plan = planDestinations(files, dest, fromFolder, opts.spread);
  if (plan.split) {
    toast(`${files.length} arquivo(s) divididos entre ${plan.volumes} volume(s) por espaço livre.`,
      'info', 'Enviando para os dois');
  }
  // Uma nova leva começa limpa quando a anterior já terminou.
  if (!S.uploads.some(i => i.status === 'uploading' || i.status === 'waiting')) S.uploads = [];
  const items = files.map((f, i) => {
    const rel = String(fromFolder ? (f._rcRel || f.webkitRelativePath || f.name) : (f._rcRel || f.name)).replace(/\\/g, '/');
    return {
      id: ++S.uploadSeq,
      file: f,
      name: f.name,
      rel,
      dest: plan.dests[i] || dest || '',
      size: f.size || 0,
      pct: 0,
      status: 'waiting',
      error: '',
      xhr: null
    };
  });
  S.uploads.push(...items);
  S.uploadsCollapsed = false;
  renderUploadPanel();
  updateUploadPanel();
  runUploads(items);
}

// planDestinations decide para onde cada arquivo vai.
//
// Antes, todo upload usava exatamente a pasta aberta, e o envio para o interno
// nem funcionava por causa do bug de apelido. Agora:
//   - padrão: tudo vai para a pasta aberta (previsível);
//   - spread=true (opção explícita no menu): os arquivos são distribuídos entre
//     os volumes por espaço livre, do maior arquivo para o menor, para usar os
//     dois discos de forma simultânea sem encher um só.
function planDestinations(files, dest, fromFolder, spread) {
  const roots = (S.files && S.files.roots) || [];
  const single = { dests: files.map(() => dest || ''), split: false, volumes: 1 };
  if (!spread || fromFolder || roots.length < 1) return single;

  const vols = [
    { path: '', free: Number((S.files && S.files.primaryFree) || 0) },
    ...roots.map(r => ({ path: r.name, free: Number(r.free || 0) }))
  ].filter(v => v.free > 0);
  if (vols.length < 2) return single;

  const order = files.map((f, i) => ({ i, size: Number(f.size) || 0 }))
    .sort((a, b) => b.size - a.size);
  const remaining = vols.map(v => v.free);
  const dests = files.map(() => '');
  for (const item of order) {
    let best = 0;
    for (let k = 1; k < vols.length; k++) {
      if (remaining[k] > remaining[best]) best = k;
    }
    // Junta ao destino escolhido: raiz + subpasta digitada, quando houver.
    dests[item.i] = [vols[best].path, dest].filter(Boolean).join('/');
    remaining[best] -= item.size;
  }
  const used = new Set(dests);
  return { dests, split: used.size > 1, volumes: used.size };
}

async function runUploads(items) {
  let cursor = 0;
  const workers = Array.from({ length: Math.min(2, items.length) }, () => (async () => {
    while (cursor < items.length) {
      const item = items[cursor++];
      await uploadOne(item);
    }
  })());
  await Promise.all(workers);

  const ok = items.filter(i => i.status === 'done').length;
  const fail = items.filter(i => i.status === 'error').length;
  const dests = new Set(items.map(i => i.dest || ''));
  if (S.page === 'files' && dests.has(S.files.path)) loadFiles(S.files.path, { keepSelection: true });

  if (fail === 0) toast(`${ok} arquivo(s) enviado(s).`, 'ok', 'Upload concluído');
  else if (ok === 0) toast(`${fail} arquivo(s) falharam.`, 'error', 'Upload');
  else toast(`${ok} enviado(s), ${fail} com erro.`, 'info', 'Upload parcial');

  updateUploadPanel();
  if (fail === 0) scheduleUploadAutoHide();
}

function uploadOne(item) {
  return new Promise(resolve => {
    let dir = item.dest || '';
    const rel = String(item.rel || item.name || '');
    if (rel.includes('/')) {
      const sub = rel.split('/').slice(0, -1).join('/');
      dir = [item.dest, sub].filter(Boolean).join('/');
    }
    const url = `/api/upload?path=${encodeURIComponent(dir)}`;
    const xhr = new XMLHttpRequest();
    item.xhr = xhr;
    item.status = 'uploading';
    item.error = '';
    updateUploadRow(item);
    updateUploadPanel();

    xhr.open('POST', url);
    xhr.setRequestHeader('X-PS', '1');
    xhr.upload.onprogress = e => {
      if (e.lengthComputable) {
        item.pct = Math.round((e.loaded / e.total) * 100);
        updateUploadRow(item);
        updateUploadPanel();
      }
    };
    xhr.onload = () => {
      let data = {};
      try { data = JSON.parse(xhr.responseText); } catch {}
      if (xhr.status >= 200 && xhr.status < 300) {
        item.status = 'done';
        item.pct = 100;
      } else {
        item.status = 'error';
        item.error = (data && data.error) || `HTTP ${xhr.status}`;
        if (xhr.status === 401) showLogin('Sua sessão expirou. Entre novamente.');
      }
      item.xhr = null;
      updateUploadRow(item);
      updateUploadPanel();
      resolve();
    };
    xhr.onerror = () => {
      item.status = 'error';
      item.error = 'Falha de rede';
      item.xhr = null;
      updateUploadRow(item);
      updateUploadPanel();
      resolve();
    };
    xhr.onabort = () => {
      item.status = 'canceled';
      item.error = 'Cancelado';
      item.xhr = null;
      updateUploadRow(item);
      updateUploadPanel();
      resolve();
    };
    const fd = new FormData();
    fd.append('file', item.file, item.file.name);
    xhr.send(fd);
  });
}

function uploadOverall() {
  const total = S.uploads.reduce((a, i) => a + (i.size || 0), 0);
  if (!total) return 0;
  const loaded = S.uploads.reduce((a, i) => {
    const pct = i.status === 'done' ? 100 : (i.pct || 0);
    return a + (i.size || 0) * pct / 100;
  }, 0);
  return Math.max(0, Math.min(100, Math.round(loaded / total * 100)));
}

function renderUploadPanel() {
  let el = $('#uploads');
  if (!el) {
    el = document.createElement('div');
    el.id = 'uploads';
    document.body.appendChild(el);
  }
  el.className = `uploads${S.uploadsCollapsed ? ' collapsed' : ''}`;

  const active = S.uploads.filter(i => i.status === 'uploading' || i.status === 'waiting').length;
  const failed = S.uploads.filter(i => i.status === 'error' || i.status === 'canceled').length;
  const done = S.uploads.filter(i => i.status === 'done').length;
  const dest = S.uploads[0] ? S.uploads[0].dest : (S.page === 'files' ? S.files.path : '');
  const destLabel = dest ? baseName(dest) : baseRootLabel();
  const totalSize = S.uploads.reduce((a, i) => a + (i.size || 0), 0);

  el.innerHTML = `
    <div class="uploads-head">
      <div class="up-title">
        <span class="dot"></span>
        <span id="up-count">${active ? `Enviando ${active}` : 'Envios'}</span>
        <span class="up-dest" id="up-dest" title="Destino: ${esc(dest || baseRootLabel())}">${esc(destLabel)}</span>
      </div>
      <div class="up-head-actions">
        <button class="up-min" id="up-min" aria-label="${S.uploadsCollapsed ? 'Expandir' : 'Minimizar'}" title="${S.uploadsCollapsed ? 'Expandir' : 'Minimizar'}">${ico(S.uploadsCollapsed ? 'maximize' : 'minimize')}</button>
        <button class="up-x" id="up-close" aria-label="Fechar" title="Fechar">${ico('x')}</button>
      </div>
    </div>
    <div class="up-overall"><span id="up-overall-bar" style="width:${uploadOverall()}%"></span></div>
    <div class="up-list" id="up-list">
      ${S.uploads.map(item => {
        const kind = fileKind(item.name, false);
        const action = item.status === 'uploading' || item.status === 'waiting'
          ? `<button class="up-act" data-up-act="${item.id}" title="Cancelar" aria-label="Cancelar envio">${ico('x')}</button>`
          : item.status === 'error' || item.status === 'canceled'
            ? `<button class="up-act" data-up-act="${item.id}" title="Tentar novamente" aria-label="Tentar novamente">${ico('refresh')}</button>`
            : `<button class="up-act" hidden></button>`;
        return `<div class="up-row" id="uprow-${item.id}">
          <span class="file-ico ${kind}">${ico(kindIcon(kind))}</span>
          <div class="up-main">
            <div class="up-top">
              <span class="up-name" title="${esc(item.rel || item.name)}">${esc(item.name)}</span>
              <span class="up-status${item.status === 'error' ? ' error' : item.status === 'done' ? ' done' : ''}" id="upstat-${item.id}">${esc(uploadLabel(item))}</span>
            </div>
            <div class="up-bar"><span id="upbar-${item.id}" style="width:${item.pct}%"></span></div>
          </div>
          ${action}
        </div>`;
      }).join('')}
    </div>
    <div class="uploads-foot">
      <span id="up-foot">${done} de ${S.uploads.length} concluído(s) · ${fmtBytes(totalSize)}</span>
      <div class="foot-actions">
        ${failed ? `<button class="btn ghost" id="up-retry-all">${ico('refresh')} Tentar falhas</button>` : ''}
        ${done || failed ? `<button class="btn ghost" id="up-clear">Limpar</button>` : ''}
      </div>
    </div>`;

  $('#up-min').addEventListener('click', () => {
    S.uploadsCollapsed = !S.uploadsCollapsed;
    renderUploadPanel();
  });
  $('#up-close').addEventListener('click', closeUploadPanel);

  const list = $('#up-list');
  if (list) {
    list.addEventListener('click', e => {
      const btn = e.target.closest('[data-up-act]');
      if (!btn) return;
      const item = S.uploads.find(x => x.id === Number(btn.dataset.upAct));
      if (!item) return;
      if (item.status === 'uploading' || item.status === 'waiting') {
        if (item.xhr) item.xhr.abort();
      } else if (item.status === 'error' || item.status === 'canceled') {
        retryUpload(item);
      }
    });
  }

  const retryAll = $('#up-retry-all');
  if (retryAll) retryAll.addEventListener('click', () => {
    const items = S.uploads.filter(i => i.status === 'error' || i.status === 'canceled');
    if (!items.length) return;
    items.forEach(i => { i.status = 'waiting'; i.pct = 0; i.error = ''; updateUploadRow(i); });
    renderUploadPanel();
    runUploads(items);
  });

  const clear = $('#up-clear');
  if (clear) clear.addEventListener('click', () => {
    S.uploads = S.uploads.filter(i => i.status === 'uploading' || i.status === 'waiting');
    if (!S.uploads.length) {
      const node = $('#uploads');
      if (node) node.remove();
      return;
    }
    renderUploadPanel();
  });
}

function retryUpload(item) {
  item.status = 'waiting';
  item.pct = 0;
  item.error = '';
  updateUploadRow(item);
  updateUploadPanel();
  runUploads([item]);
}

async function closeUploadPanel() {
  const active = S.uploads.filter(i => i.status === 'uploading' || i.status === 'waiting');
  if (active.length) {
    const ok = await confirmDialog('Interromper os envios?', `${active.length} arquivo(s) ainda estão sendo enviados. Fechar o painel cancela o restante.`, { ok: 'Interromper', danger: true });
    if (!ok) return;
    active.forEach(i => { if (i.xhr) i.xhr.abort(); });
  }
  S.uploads = [];
  S.uploadsCollapsed = false;
  const el = $('#uploads');
  if (el) el.remove();
}

function scheduleUploadAutoHide() {
  setTimeout(() => {
    if (S.uploads.some(i => i.status === 'uploading' || i.status === 'waiting')) return;
    if (S.uploads.some(i => i.status === 'error' || i.status === 'canceled')) return;
    const el = $('#uploads');
    if (el) el.remove();
  }, 6000);
}

function uploadLabel(item) {
  if (item.status === 'done') return 'Concluído';
  if (item.status === 'error') return item.error || 'Erro';
  if (item.status === 'canceled') return 'Cancelado';
  if (item.status === 'uploading') return `${item.pct}%`;
  return 'Aguardando';
}

function updateUploadRow(item) {
  const bar = document.getElementById(`upbar-${item.id}`);
  const stat = document.getElementById(`upstat-${item.id}`);
  const row = document.getElementById(`uprow-${item.id}`);
  if (!bar || !stat || !row) return;
  bar.style.width = `${item.pct}%`;
  stat.textContent = uploadLabel(item);
  stat.classList.toggle('error', item.status === 'error' || item.status === 'canceled');
  stat.classList.toggle('done', item.status === 'done');
  const btn = row.querySelector('[data-up-act]');
  if (btn) {
    const active = item.status === 'uploading' || item.status === 'waiting';
    const retry = item.status === 'error' || item.status === 'canceled';
    btn.hidden = !active && !retry;
    btn.innerHTML = active ? ico('x') : retry ? ico('refresh') : '';
    btn.title = active ? 'Cancelar' : 'Tentar novamente';
    btn.setAttribute('aria-label', btn.title);
  }
}

function updateUploadPanel() {
  const active = S.uploads.filter(i => i.status === 'uploading' || i.status === 'waiting').length;
  const done = S.uploads.filter(i => i.status === 'done').length;
  const failed = S.uploads.filter(i => i.status === 'error' || i.status === 'canceled').length;
  const count = $('#up-count');
  if (count) count.textContent = active ? `Enviando ${active}` : 'Envios';
  const bar = $('#up-overall-bar');
  if (bar) bar.style.width = `${uploadOverall()}%`;
  const foot = $('#up-foot');
  if (foot) {
    if (active) foot.textContent = `${done} de ${S.uploads.length} concluído(s)`;
    else if (failed) foot.textContent = `${done} concluído(s), ${failed} com falha`;
    else foot.textContent = `${done} concluído(s)`;
  }
}

/* ---------------- boot ---------------- */

async function boot() {
  $$('[data-logo]').forEach(el => el.innerHTML = logoMarkup());
  hydrateIcons();
  try {
    const st = await api('/api/state', { noAuthRedirect: true });
    S.state = st;
    S.name = st.name || 'PocketServe';
    $('#boot').hidden = true;
    if (st.needs_setup) showSetup();
    else if (!st.authenticated) showLogin();
    else await enterApp();
  } catch (e) {
    $('#boot').innerHTML = `<div class="boot-logo">${logoMarkup()}</div><div class="boot-text">PocketServe</div><p class="muted" style="max-width:340px;text-align:center">${esc(e.message)}</p><button class="btn primary" onclick="location.reload()">Tentar novamente</button>`;
  }
}

document.addEventListener('DOMContentLoaded', boot);
