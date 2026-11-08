'use strict';

/* ============================================================
   Gerenciador de arquivos
   ============================================================ */

let filesPreviewSeq = 0;

const FILE_SORTS = [
  { id: 'name', label: 'Nome' },
  { id: 'date', label: 'Modificado' },
  { id: 'size', label: 'Tamanho' },
  { id: 'type', label: 'Tipo' }
];

const FILE_FILTERS = [
  { id: 'all', label: 'Tudo' },
  { id: 'folder', label: 'Pastas' },
  { id: 'image', label: 'Imagens' },
  { id: 'video', label: 'Vídeos' },
  { id: 'audio', label: 'Áudio' },
  { id: 'doc', label: 'Documentos' },
  { id: 'archive', label: 'Compactados' }
];

function renderFilesPage() {
  const page = $('#page-files');
  if (!page) return;

  if (!page.dataset.ready) {
    page.innerHTML = `
      <div class="files-head">
        <div>
          <h1>Arquivos</h1>
          <p id="files-sub">Carregando…</p>
        </div>
        <div class="files-actions">
          <button class="btn ghost" data-act="new-folder">${ico('folderPlus')} <span>Nova pasta</span></button>
          <div class="split-btn">
            <button class="btn primary" data-act="upload">${ico('upload')} <span>Enviar</span></button>
            <button class="btn primary split-toggle" data-act="upload-menu" title="Mais opções de envio" aria-label="Mais opções de envio">${ico('chevronDown')}</button>
          </div>
        </div>
      </div>

      <div class="file-toolbar" id="file-toolbar">
        <div class="root-tabs" id="root-tabs" hidden></div>
        <div class="path-row">
          <button class="icon-btn" id="files-up" data-act="files-up" title="Subir um nível" aria-label="Subir um nível">${ico('arrowUp')}</button>
          <div class="crumbs" id="file-crumbs"></div>
          <button class="icon-btn" data-act="refresh" title="Atualizar" aria-label="Atualizar">${ico('refresh')}</button>
        </div>
        <div class="toolrow">
          <div class="tool-search" id="file-search-wrap">
            ${ico('search')}
            <input class="input" id="file-search" placeholder="Buscar nesta pasta e subpastas…" autocomplete="off" spellcheck="false" aria-label="Buscar arquivos">
            <button class="search-clear" id="file-search-clear" type="button" aria-label="Limpar busca" hidden>${ico('x')}</button>
          </div>
          <div class="file-tools">
            <button class="sort-btn" id="files-sort" data-act="files-sort" title="Ordenar arquivos">
              ${ico('sort')} <span id="files-sort-label">Nome</span> ${ico('chevronDown')}
            </button>
            <button class="icon-btn" id="files-hidden" data-act="toggle-hidden" title="Mostrar arquivos ocultos" aria-label="Mostrar arquivos ocultos">${ico('eyeOff')}</button>
            <div class="view-toggle" role="group" aria-label="Modo de visualização">
              <button class="icon-btn" id="files-view-list" title="Ver em lista" aria-label="Ver em lista">${ico('list')}</button>
              <button class="icon-btn" id="files-view-grid" title="Ver em grade" aria-label="Ver em grade">${ico('grid')}</button>
            </div>
          </div>
        </div>
        <div class="filter-chips" id="file-filters" aria-label="Filtros de arquivos"></div>
      </div>

      <div id="file-selection"></div>
      <div id="files-body"></div>`;

    bindFilesPageOnce();
    page.dataset.ready = '1';
  }

  const search = $('#file-search');
  if (search && document.activeElement !== search) search.value = S.files.query || '';
  updateFilesChrome();
  loadFiles(S.files.path);
}

let filesDragBound = false;

function bindFilesPageOnce() {
  $('#files-view-list').addEventListener('click', () => setFilesView('list'));
  $('#files-view-grid').addEventListener('click', () => setFilesView('grid'));

  const search = $('#file-search');
  let timer = null;
  const runSearch = () => {
    const q = search.value.trim();
    S.files.query = q;
    S.files.results = null;
    S.files.remoteSearch = false;
    S.files.searching = q.length >= 2;
    S.files.searchSeq++;
    const seq = S.files.searchSeq;
    updateSearchState();
    renderFilesBody();
    clearTimeout(timer);
    if (q.length >= 2) {
      timer = setTimeout(() => runFileSearch(q, seq), 450);
    }
  };
  search.addEventListener('input', runSearch);
  search.addEventListener('keydown', e => {
    if (e.key === 'Escape') {
      search.value = '';
      runSearch();
      search.blur();
    }
  });

  $('#file-search-clear').addEventListener('click', () => {
    search.value = '';
    runSearch();
    search.focus();
  });

  $('#file-filters').addEventListener('click', e => {
    const btn = e.target.closest('[data-filter]');
    if (!btn) return;
    S.files.filter = btn.dataset.filter || 'all';
    S.files.selection.clear();
    S.files.lastChecked = null;
    renderFilters();
    renderFilesBody();
    renderSelectionBar();
  });

  const body = $('#files-body');
  body.addEventListener('change', handleFilesChange);
  body.addEventListener('keydown', e => {
    const row = e.target.closest('[data-path]');
    if (!row || !['Enter', ' '].includes(e.key)) return;
    if (e.target.closest('input,button')) return;
    e.preventDefault();
    const entry = findFileEntry(row.dataset.path);
    if (entry) openEntry(entry);
  });

  if (!filesDragBound) {
    filesDragBound = true;
    bindFilesDragDrop();
  }
}

function bindFilesDragDrop() {
  const overlay = $('#drop-overlay');
  let depth = 0;
  const hasFiles = e => {
    const t = e.dataTransfer && e.dataTransfer.types;
    return !!t && Array.prototype.indexOf.call(t, 'Files') >= 0;
  };
  const hide = () => {
    depth = 0;
    if (overlay) overlay.hidden = true;
  };
  const show = () => {
    if (!overlay) return;
    const dest = $('#drop-dest');
    if (dest) dest.textContent = S.files.path ? baseName(S.files.path) : baseRootLabel();
    overlay.hidden = false;
  };

  document.addEventListener('dragenter', e => {
    if (S.page !== 'files' || !hasFiles(e)) return;
    e.preventDefault();
    depth++;
    show();
  });
  document.addEventListener('dragover', e => {
    if (S.page !== 'files' || !hasFiles(e)) return;
    e.preventDefault();
    show();
  });
  document.addEventListener('dragleave', e => {
    if (S.page !== 'files' || !hasFiles(e)) return;
    depth = Math.max(0, depth - 1);
    if (depth === 0) hide();
  });
  document.addEventListener('drop', async e => {
    if (S.page !== 'files') return;
    e.preventDefault();
    hide();
    if (!S.files.writable) {
      toast('Esta pasta é somente leitura.', 'error', 'Upload');
      return;
    }
    const dt = e.dataTransfer;
    if (!dt) return;
    const entries = Array.from(dt.items || []).map(i => i.webkitGetAsEntry && i.webkitGetAsEntry()).filter(Boolean);
    let files = Array.from(dt.files || []);
    if (entries.length) {
      try {
        const collected = [];
        for (const entry of entries) await walkDropEntry(entry, '', collected);
        if (collected.length) files = collected;
      } catch {}
    }
    if (!files.length) return;
    uploadFiles(files, S.files.path, true);
  });
  window.addEventListener('blur', hide);
}

async function walkDropEntry(entry, prefix, out) {
  if (entry.isFile) {
    const file = await new Promise((resolve, reject) => entry.file(resolve, reject));
    file._rcRel = prefix + file.name;
    out.push(file);
    return;
  }
  if (!entry.isDirectory) return;
  const reader = entry.createReader();
  let batch;
  do {
    batch = await new Promise((resolve, reject) => reader.readEntries(resolve, reject));
    for (const child of batch) await walkDropEntry(child, prefix + entry.name + '/', out);
  } while (batch.length);
}

/* ---------------- listing ---------------- */

function joinPath(a, b) {
  return [a, b].filter(Boolean).join('/');
}

async function loadFiles(path, opts = {}) {
  const requested = path || '';
  S.files.loading = true;
  S.files.loadError = '';
  S.files.searchSeq++;
  if (!opts.silent) renderFilesLoading(requested);

  try {
    const qs = new URLSearchParams();
    if (requested) qs.set('path', requested);
    if (S.files.showHidden) qs.set('hidden', '1');
    const data = await api(`/api/files?${qs.toString()}`);
    const changed = data.path !== S.files.loadedPath;

    S.files.path = data.path || '';
    S.files.parent = data.parent || '';
    S.files.roots = Array.isArray(data.roots) ? data.roots : (S.files.roots || []);
    S.files.rootLabel = data.root_label || S.files.rootLabel || 'Armazenamento';
    if (typeof data.primary_free === 'number') S.files.primaryFree = data.primary_free;
    if (typeof data.trash_days === 'number') S.files.trashDays = data.trash_days;
    S.files.writable = data.writable !== false;
    S.files.entries = Array.isArray(data.entries) ? data.entries : [];
    S.files.loadedPath = S.files.path;
    S.files.loading = false;

    if (changed) {
      S.files.selection.clear();
      S.files.lastChecked = null;
      S.files.query = '';
      S.files.results = null;
      S.files.remoteSearch = false;
      S.files.searching = false;
      const input = $('#file-search');
      if (input && input.value) input.value = '';
    } else if (!opts.keepSelection) {
      S.files.selection.clear();
      S.files.lastChecked = null;
    } else {
      const visible = new Set(S.files.entries.map(e => e.path));
      S.files.selection = new Set([...S.files.selection].filter(p => visible.has(p)));
    }

    const canonical = S.files.path ? `#/files/${encodePath(S.files.path)}` : '#/files';
    if (location.hash !== canonical) history.replaceState(null, '', canonical);

    updateFilesChrome();
    renderFilesBody();
  } catch (e) {
    if (e.status === 401) return;
    S.files.loading = false;
    S.files.loadError = e.message;
    S.files.loadErrorCode = e.code || '';
    S.files.entries = [];
    updateFilesChrome();
    renderFilesBody();
  }
}

function renderFilesLoading(path) {
  const sub = $('#files-sub');
  if (sub) sub.textContent = 'Carregando arquivos…';
  renderCrumbs(path);
  renderFilters();
  const sel = $('#file-selection');
  if (sel) sel.innerHTML = '';
  const body = $('#files-body');
  if (!body) return;
  body.innerHTML = `<div class="file-table file-loading">${Array.from({ length: 7 }).map(() => `
    <div class="sk-row"><span class="sk av"></span><div style="flex:1"><div class="sk l1"></div><div class="sk l2"></div></div><div class="sk" style="width:70px;height:9px"></div></div>`).join('')}</div>`;
}

function updateFilesChrome() {
  const sub = $('#files-sub');
  if (sub) {
    if (S.files.loadError) {
      sub.textContent = 'Não foi possível abrir esta pasta.';
    } else if (S.files.loading) {
      sub.textContent = 'Carregando arquivos…';
    } else {
      const files = S.files.entries.filter(e => !e.is_dir);
      const folders = S.files.entries.filter(e => e.is_dir);
      const total = files.reduce((a, e) => a + (Number(e.size) || 0), 0);
      const parts = [`${folders.length} pasta(s)`, `${files.length} arquivo(s)`, fmtBytes(total)];
      if (!S.files.writable) parts.push('somente leitura');
      sub.textContent = parts.join(' · ');
    }
  }
  renderCrumbs(S.files.path);
  renderFilters();
  updateSortButton();
  updateHiddenButton();
  updateSearchState();
  renderSelectionBar();

  const up = $('#files-up');
  if (up) {
    up.disabled = !S.files.path;
    up.title = S.files.path ? 'Subir um nível' : 'Você já está na raiz';
  }

  const ro = !S.files.writable;
  $$('.files-actions [data-act="new-folder"], .files-actions [data-act="upload"], .files-actions [data-act="upload-menu"]').forEach(b => {
    b.disabled = ro;
    b.title = ro ? 'Esta pasta é somente leitura' : (b.dataset.act === 'upload-menu' ? 'Mais opções de envio' : '');
  });
}

function rootLabelFor(alias) {
  const list = (S.files && S.files.roots) || [];
  const found = list.find(r => r.name === alias);
  return found ? found.name : alias;
}

// Nome da raiz principal. Antes o rótulo "Armazenamento" era fixo, o que
// confundia quando a raiz principal é uma pasta dedicada (ex.: PocketServe).
function baseRootLabel() {
  return (S.files && S.files.rootLabel) || 'Armazenamento';
}

function renderCrumbs(path) {
  const el = $('#file-crumbs');
  if (!el) return;
  const parts = String(path || '').split('/').filter(Boolean);
  const roots = (S.files && S.files.roots) || [];
  // Primeiro segmento pode ser uma raiz secundária (ex.: "cartao"). Nesse caso
  // o apelido JÁ É a raiz: mostrá-lo depois de "vA/" sugeriria que o cartão
  // fica dentro do interno.
  const inAlias = parts.length && roots.some(r => r.name === parts[0]);
  const baseLabel = baseRootLabel();
  let html = '';
  if (!parts.length) {
    html = `<span class="crumb current">${ico('folder', 'root-ico')} ${esc(baseLabel)}</span>`;
  } else if (inAlias) {
    const alias = parts[0];
    const aliasHref = `#/files/${encodePath(alias)}`;
    if (parts.length === 1) {
      html = `<span class="crumb current">${ico('sd', 'root-ico')} ${esc(alias)}</span>`;
    } else {
      html = `<a href="${aliasHref}" title="${esc(alias)}">${ico('sd', 'root-ico')} ${esc(alias)}</a>`;
      let acc = alias;
      parts.slice(1).forEach((part, i) => {
        acc += '/' + part;
        html += '<span class="cr">/</span>';
        const last = i === parts.length - 2;
        if (last) {
          html += `<span class="crumb current" title="${esc(part)}">${esc(part)}</span>`;
        } else {
          html += `<a href="#/files/${encodePath(acc)}" title="${esc(part)}">${esc(part)}</a>`;
        }
      });
    }
  } else {
    html = `<a href="#/files" title="${esc(baseLabel)}">${ico('folder', 'root-ico')} ${esc(baseLabel)}</a>`;
    let acc = '';
    parts.forEach((part, i) => {
      acc += (acc ? '/' : '') + part;
      html += '<span class="cr">/</span>';
      if (i === parts.length - 1) {
        html += `<span class="crumb current" title="${esc(part)}">${esc(part)}</span>`;
      } else {
        html += `<a href="#/files/${encodePath(acc)}" title="${esc(part)}">${esc(part)}</a>`;
      }
    });
  }
  el.innerHTML = html;
  requestAnimationFrame(() => { el.scrollLeft = el.scrollWidth; });
}

function updateFilesViewButtons() {
  const list = $('#files-view-list');
  const grid = $('#files-view-grid');
  if (list) {
    list.classList.toggle('active', S.files.view !== 'grid');
    list.setAttribute('aria-pressed', S.files.view !== 'grid' ? 'true' : 'false');
  }
  if (grid) {
    grid.classList.toggle('active', S.files.view === 'grid');
    grid.setAttribute('aria-pressed', S.files.view === 'grid' ? 'true' : 'false');
  }
}

function setFilesView(view) {
  S.files.view = view === 'grid' ? 'grid' : 'list';
  try { localStorage.setItem('ps_view', S.files.view); } catch {}
  updateFilesViewButtons();
  renderFilesBody();
}

function currentFileEntries() {
  let base = (S.files.query && S.files.remoteSearch && Array.isArray(S.files.results))
    ? S.files.results
    : S.files.entries;
  const q = (S.files.query || '').toLowerCase();
  if (q) base = base.filter(e => String(e.name || '').toLowerCase().includes(q));
  if (S.files.filter && S.files.filter !== 'all') {
    base = base.filter(e => fileFilterMatch(e, S.files.filter));
  }
  return sortFileEntries(base);
}

function fileFilterMatch(entry, filter) {
  if (filter === 'folder') return !!entry.is_dir;
  if (filter === 'doc') return ['pdf', 'text', 'doc', 'sheet', 'slides'].includes(entry.kind || fileKind(entry.name, false));
  return (entry.kind || fileKind(entry.name, false)) === filter;
}

function sortFileEntries(list) {
  const dir = S.files.dir === 'desc' ? -1 : 1;
  const key = S.files.sort || 'name';
  const byName = (a, b) => String(a.name || '').localeCompare(String(b.name || ''), 'pt-BR', { numeric: true, sensitivity: 'base' });
  const cmp = (a, b) => {
    if (key === 'date') return (new Date(a.mtime || 0) - new Date(b.mtime || 0)) || byName(a, b);
    if (key === 'size') return ((Number(a.size) || 0) - (Number(b.size) || 0)) || byName(a, b);
    if (key === 'type') {
      const ka = a.kind || fileKind(a.name, a.is_dir);
      const kb = b.kind || fileKind(b.name, b.is_dir);
      return ka.localeCompare(kb, 'pt-BR') || byName(a, b);
    }
    return byName(a, b);
  };
  const folders = list.filter(e => e.is_dir).sort((a, b) => cmp(a, b) * dir);
  const files = list.filter(e => !e.is_dir).sort((a, b) => cmp(a, b) * dir);
  return folders.concat(files);
}

function findFileEntry(path) {
  return (S.files.results || []).find(e => e.path === path)
    || S.files.entries.find(e => e.path === path)
    || null;
}

function renderRootTabs() {
  const el = $('#root-tabs');
  if (!el) return;
  const roots = (S.files && S.files.roots) || [];
  if (!roots.length) { el.hidden = true; el.innerHTML = ''; return; }
  const current = String(S.files.path || '').split('/')[0];
  const isAlias = roots.some(r => r.name === current);
  const tabs = [
    { name: '', label: baseRootLabel(), icon: 'server' },
    ...roots.map(r => ({ name: r.name, label: r.name, icon: 'sd' }))
  ];
  el.hidden = false;
  el.innerHTML = tabs.map(t => {
    const active = t.name === '' ? !isAlias : current === t.name;
    const href = t.name ? `#/files/${encodePath(t.name)}` : '#/files';
    return `<a class="root-tab${active ? ' active' : ''}" href="${href}">${ico(t.icon)}<span>${esc(t.label)}</span></a>`;
  }).join('');
}

function renderFilesBody() {
  const body = $('#files-body');
  if (!body) return;
  renderRootTabs();

  if (S.files.loading) return;
  if (S.files.loadError) {
    const denied = S.files.loadErrorCode === 'permission';
    body.innerHTML = `<div class="file-table"><div class="file-empty">
      <div class="empty-ico">${ico(denied ? 'lock' : 'info')}</div>
      <b>${denied ? 'Pasta bloqueada pelo Android' : 'Pasta indisponível'}</b>
      <span>${esc(S.files.loadError)}</span>
      <div class="empty-actions">
        <a class="btn ghost small" href="#/files">Ir para o início</a>
        <button class="btn primary small" data-act="refresh">Tentar novamente</button>
      </div>
    </div></div>`;
    updateSearchState();
    return;
  }

  const entries = currentFileEntries();
  let html = '';

  if (S.files.query) {
    const searching = S.files.searching ? ' Buscando em todas as subpastas…' : '';
    const scope = S.files.remoteSearch ? 'em todas as subpastas' : 'nesta pasta';
    html += `<div class="results-banner">
      ${ico('search')}
      <span>${entries.length} resultado(s) para <b>“${esc(S.files.query)}”</b> ${scope}.${searching}</span>
      <button class="btn ghost small" id="files-clear-search">Limpar busca</button>
    </div>`;
  } else if (S.files.filter && S.files.filter !== 'all' && !entries.length) {
    html += `<div class="file-table"><div class="file-empty">
      <div class="empty-ico">${ico('filter')}</div>
      <b>Nenhum item neste filtro</b>
      <span>Não há itens desta categoria nesta pasta.</span>
      <div class="empty-actions"><button class="btn ghost small" id="files-clear-filter">Ver todos os itens</button></div>
    </div></div>`;
  }

  if (!entries.length && !html) {
    if (S.files.query) {
      html += `<div class="file-table"><div class="file-empty">
        <div class="empty-ico">${ico('search')}</div>
        <b>Nada encontrado</b>
        <span>Nenhum arquivo corresponde a “${esc(S.files.query)}”.</span>
        <div class="empty-actions"><button class="btn ghost small" id="files-clear-search">Limpar busca</button></div>
      </div></div>`;
    } else {
      const ro = !S.files.writable;
      html += `<div class="file-table"><div class="file-empty">
        <div class="empty-ico">${ico('folder')}</div>
        <b>${S.files.path ? 'Esta pasta está vazia' : 'Nenhum arquivo visível'}</b>
        <span>${ro ? 'Esta pasta é somente leitura.' : 'Arraste arquivos para cá ou use os botões abaixo para começar.'}</span>
        <div class="empty-actions">
          <button class="btn ghost small" data-act="new-folder" ${ro ? 'disabled' : ''}>${ico('folderPlus')} Nova pasta</button>
          <button class="btn primary small" data-act="upload" ${ro ? 'disabled' : ''}>${ico('upload')} Enviar arquivos</button>
        </div>
      </div></div>`;
    }
  } else if (entries.length) {
    html += S.files.view === 'grid' ? renderFilesGrid(entries) : renderFilesTable(entries);
  }

  body.innerHTML = html;

  const clear = $('#files-clear-search');
  if (clear) clear.addEventListener('click', clearSearch);
  const clearFilter = $('#files-clear-filter');
  if (clearFilter) clearFilter.addEventListener('click', () => {
    S.files.filter = 'all';
    renderFilters();
    renderFilesBody();
  });

  bindFileEntryActions(body);
  syncFileSelection();
  updateSearchState();
}

function clearSearch() {
  const input = $('#file-search');
  if (input) {
    input.value = '';
    input.dispatchEvent(new Event('input'));
  }
}

function renderFilesTable(entries) {
  const allSelected = entries.length > 0 && entries.every(e => S.files.selection.has(e.path));
  const rows = entries.map(e => {
    const kind = e.kind || fileKind(e.name, e.is_dir);
    const selected = S.files.selection.has(e.path);
    return `<div class="file-row${selected ? ' selected' : ''}" data-path="${esc(e.path)}" tabindex="0" title="${esc(e.name)}">
      <span class="fcheck"><input type="checkbox" data-check="${esc(e.path)}" ${selected ? 'checked' : ''} aria-label="Selecionar ${esc(e.name)}"></span>
      <span class="fname" data-open="${esc(e.path)}">
        <span class="file-ico ${kind}">${ico(kindIcon(kind))}</span>
        <span class="fname-text"><b>${esc(e.name)}</b><span>${e.is_dir ? 'Pasta' : esc(fmtBytes(e.size))} · ${esc(fmtDateTime(e.mtime))}${esc(remoteLocation(e))}</span></span>
      </span>
      <span class="fsize">${e.is_dir ? '—' : esc(fmtBytes(e.size))}</span>
      <span class="fdate">${esc(fmtDateTime(e.mtime))}</span>
      <button class="icon-btn kebab" data-menu="${esc(e.path)}" aria-label="Opções de ${esc(e.name)}">${ico('moreVertical')}</button>
    </div>`;
  }).join('');

  return `<div class="file-table">
    <div class="file-head">
      <span><input type="checkbox" id="files-select-all" ${allSelected ? 'checked' : ''} aria-label="Selecionar todos"></span>
      <span><button data-sort="name">Nome ${sortArrow('name')}</button></span>
      <span><button data-sort="size">Tamanho ${sortArrow('size')}</button></span>
      <span><button data-sort="date">Modificado ${sortArrow('date')}</button></span>
      <span></span>
    </div>
    ${rows}
  </div>`;
}

function renderFilesGrid(entries) {
  const items = entries.map(e => {
    const kind = e.kind || fileKind(e.name, e.is_dir);
    const selected = S.files.selection.has(e.path);
    let thumb;
    if (!e.is_dir && kind === 'image' && /\.(jpe?g|png|gif)$/i.test(e.name)) {
      thumb = `<img src="/api/thumb?path=${encodeURIComponent(e.path)}" alt="" loading="lazy" decoding="async">`;
    } else {
      thumb = `<span class="file-ico ${kind}">${ico(kindIcon(kind))}</span>`;
    }
    return `<div class="grid-item${selected ? ' selected' : ''}" data-path="${esc(e.path)}" tabindex="0" title="${esc(e.name)}">
      <label class="grid-check"><input type="checkbox" data-check="${esc(e.path)}" ${selected ? 'checked' : ''} aria-label="Selecionar ${esc(e.name)}"></label>
      <span class="grid-actions"><button class="icon-btn" data-menu="${esc(e.path)}" aria-label="Opções de ${esc(e.name)}">${ico('moreVertical')}</button></span>
      <div class="grid-thumb">${thumb}</div>
      <div class="grid-meta"><b>${esc(e.name)}</b><span>${e.is_dir ? 'Pasta' : esc(fmtBytes(e.size))} · ${esc(fmtDate(e.mtime))}${esc(remoteLocation(e))}</span></div>
    </div>`;
  }).join('');
  return `<div class="file-grid">${items}</div>`;
}

function sortArrow(key) {
  if ((S.files.sort || 'name') !== key) return '';
  return ico(S.files.dir === 'desc' ? 'arrowDown' : 'arrowUp');
}

function bindFileEntryActions(root) {
  $$('.file-row[data-path], .grid-item[data-path]', root).forEach(el => {
    el.addEventListener('click', e => {
      if (e.target.closest('[data-check], [data-menu], .fcheck, .grid-check, .grid-actions')) return;
      const entry = findFileEntry(el.dataset.path);
      if (entry) openEntry(entry);
    });
    el.addEventListener('keydown', e => {
      if (!['Enter', ' '].includes(e.key) || e.target.closest('input,button')) return;
      e.preventDefault();
      const entry = findFileEntry(el.dataset.path);
      if (entry) openEntry(entry);
    });
  });
  $$('[data-menu]', root).forEach(el => {
    el.addEventListener('click', e => {
      e.preventDefault();
      e.stopPropagation();
      const entry = findFileEntry(el.dataset.menu);
      if (entry) openFileMenu(el, entry);
    });
  });
  $$('[data-sort]', root).forEach(btn => {
    btn.addEventListener('click', () => {
      const key = btn.dataset.sort;
      if (S.files.sort === key) S.files.dir = S.files.dir === 'desc' ? 'asc' : 'desc';
      else {
        S.files.sort = key;
        S.files.dir = key === 'name' ? 'asc' : 'desc';
      }
      persistFilesPrefs();
      updateFilesChrome();
      renderFilesBody();
    });
  });
  const selectAll = $('#files-select-all', root);
  if (selectAll) {
    selectAll.addEventListener('change', () => {
      const entries = currentFileEntries();
      if (selectAll.checked) entries.forEach(e => S.files.selection.add(e.path));
      else entries.forEach(e => S.files.selection.delete(e.path));
      S.files.lastChecked = null;
      syncFileSelection();
      renderSelectionBar();
    });
  }
  $$('.grid-thumb img', root).forEach(img => {
    img.addEventListener('error', () => {
      const item = img.closest('.grid-item');
      const entry = item && findFileEntry(item.dataset.path);
      const kind = entry ? (entry.kind || fileKind(entry.name, false)) : 'file';
      if (img.parentElement) img.parentElement.innerHTML = `<span class="file-ico ${kind}">${ico(kindIcon(kind))}</span>`;
    });
  });
}

function openEntry(entry) {
  if (entry.is_dir) openFolder(entry.path);
  else previewFile(entry);
}

function handleFilesChange(e) {
  const cb = e.target.closest('[data-check]');
  if (!cb) return;
  const path = cb.dataset.check;
  if (cb.checked) S.files.selection.add(path);
  else S.files.selection.delete(path);
  S.files.lastChecked = path;
  syncFileSelection();
  renderSelectionBar();
}

function syncFileSelection() {
  $$('#files-body .file-row, #files-body .grid-item').forEach(el => {
    const path = el.dataset.path;
    const on = S.files.selection.has(path);
    el.classList.toggle('selected', on);
    const cb = el.querySelector('[data-check]');
    if (cb) cb.checked = on;
  });
  const selAll = $('#files-select-all');
  if (selAll) {
    const entries = currentFileEntries();
    selAll.checked = entries.length > 0 && entries.every(e => S.files.selection.has(e.path));
  }
}

/* ---------------- filters / preferences ---------------- */

function persistFilesPrefs() {
  try {
    localStorage.setItem('ps_sort', S.files.sort || 'name');
    localStorage.setItem('ps_sort_dir', S.files.dir || 'asc');
    localStorage.setItem('ps_hidden', S.files.showHidden ? '1' : '0');
    localStorage.setItem('ps_filter', S.files.filter || 'all');
  } catch {}
}

function renderFilters() {
  const el = $('#file-filters');
  if (!el) return;
  const counts = { all: S.files.entries.length, folder: 0, image: 0, video: 0, audio: 0, doc: 0, archive: 0 };
  S.files.entries.forEach(e => {
    const kind = e.kind || fileKind(e.name, e.is_dir);
    if (e.is_dir) counts.folder++;
    if (counts[kind] !== undefined) counts[kind]++;
    if (['pdf', 'text', 'doc', 'sheet', 'slides'].includes(kind)) counts.doc++;
  });
  el.innerHTML = FILE_FILTERS.map(f => {
    const active = (S.files.filter || 'all') === f.id;
    const n = counts[f.id] || 0;
    return `<button class="filter-chip${active ? ' active' : ''}" data-filter="${f.id}">${f.label}<b>${n}</b></button>`;
  }).join('');
}

function updateSortButton() {
  const label = $('#files-sort-label');
  if (!label) return;
  const item = FILE_SORTS.find(s => s.id === (S.files.sort || 'name')) || FILE_SORTS[0];
  label.textContent = item.label;
  const btn = $('#files-sort');
  if (btn) btn.title = `Ordenar por ${item.label.toLowerCase()} (${S.files.dir === 'desc' ? 'decrescente' : 'crescente'})`;
}

function updateHiddenButton() {
  const btn = $('#files-hidden');
  if (!btn) return;
  btn.classList.toggle('active', !!S.files.showHidden);
  btn.innerHTML = ico(S.files.showHidden ? 'eye' : 'eyeOff');
  btn.title = S.files.showHidden ? 'Ocultar arquivos ocultos' : 'Mostrar arquivos ocultos';
  btn.setAttribute('aria-label', btn.title);
}

function updateSearchState() {
  const wrap = $('#file-search-wrap');
  if (wrap) wrap.classList.toggle('busy', !!S.files.searching);
  const clear = $('#file-search-clear');
  if (clear) clear.hidden = !(($('#file-search') || {}).value || '').trim();
  updateFilesViewButtons();
}

/* ---------------- selection bar ---------------- */

function renderSelectionBar() {
  const el = $('#file-selection');
  const page = $('#page-files');
  if (!el) return;
  const paths = [...S.files.selection];
  if (page) page.classList.toggle('selection-on', paths.length > 0);
  if (!paths.length) { el.innerHTML = ''; return; }

  const single = paths.length === 1 ? findFileEntry(paths[0]) : null;
  el.innerHTML = `<div class="selection-bar">
    <b>${paths.length} selecionado(s)</b>
    <button class="btn ghost" data-sel="download" aria-label="Baixar">${ico('download')} <span class="btn-label">Baixar</span></button>
    <button class="btn ghost" data-sel="move" aria-label="Mover">${ico('move')} <span class="btn-label">Mover</span></button>
    <button class="btn ghost" data-sel="copy" aria-label="Copiar">${ico('copy')} <span class="btn-label">Copiar</span></button>
    <button class="btn ghost" data-sel="share" ${paths.length === 1 ? '' : 'hidden'} aria-label="Compartilhar">${ico('share')} <span class="btn-label">Compartilhar</span></button>
    <button class="btn danger" data-sel="delete" aria-label="Excluir">${ico('trash')} <span class="btn-label">Excluir</span></button>
    <button class="btn ghost" data-sel="clear" aria-label="Limpar seleção">${ico('x')} <span class="btn-label">Limpar</span></button>
  </div>`;

  el.querySelectorAll('[data-sel]').forEach(btn => btn.addEventListener('click', () => {
    const action = btn.dataset.sel;
    if (action === 'clear') {
      S.files.selection.clear();
      syncFileSelection();
      renderSelectionBar();
    } else if (action === 'download') downloadSelection(paths);
    else if (action === 'move') moveEntries(paths);
    else if (action === 'copy') copyEntries(paths);
    else if (action === 'share') { if (single) shareDialog(single); }
    else if (action === 'delete') deleteEntries(paths);
  }));
}

function downloadSelection(paths) {
  if (!paths.length) return;
  if (paths.length === 1) {
    const entry = findFileEntry(paths[0]);
    if (entry && !entry.is_dir) {
      window.location.href = `/api/download?path=${encodeURIComponent(paths[0])}`;
      return;
    }
  }
  const qs = paths.map(p => `path=${encodeURIComponent(p)}`).join('&');
  window.location.href = `/api/zip?${qs}`;
}

/* ---------------- folder navigation / search ---------------- */

function openFolder(path) {
  S.files.query = '';
  S.files.results = null;
  S.files.remoteSearch = false;
  S.files.searching = false;
  navigate('files', path);
}

async function runFileSearch(q, seq) {
  try {
    const data = await api(`/api/search?q=${encodeURIComponent(q)}&path=${encodeURIComponent(S.files.path)}`);
    if (seq !== S.files.searchSeq || S.files.query !== q) return;
    S.files.results = Array.isArray(data.entries) ? data.entries : [];
    S.files.remoteSearch = true;
    S.files.searching = false;
    renderFilesBody();
  } catch (e) {
    if (seq !== S.files.searchSeq) return;
    S.files.searching = false;
    S.files.remoteSearch = false;
    renderFilesBody();
    if (e.status !== 401) toast(e.message, 'error', 'Busca');
  }
}

/* ---------------- folder / file creation ---------------- */

async function createFolderDialog(path) {
  const name = await promptDialog('Nova pasta', 'Nome da pasta', '', {
    ok: 'Criar pasta',
    placeholder: 'Ex.: Fotos, Documentos, Backup'
  });
  if (!name) return;
  try {
    const res = await api('/api/mkdir', { method: 'POST', json: { path: path || '', name } });
    toast(`Pasta “${name}” criada.`, 'ok');
    if (S.page === 'files' && S.files.path === (path || '')) {
      S.files.filter = 'all';
      loadFiles(S.files.path, { keepSelection: true });
    } else if (S.page !== 'files') {
      navigate('files', path || '');
    }
    return res;
  } catch (e) {
    toast(e.message, 'error', 'Não foi possível criar');
  }
}

function baseName(path) {
  return String(path || '').split('/').filter(Boolean).pop() || '';
}

function parentDir(path) {
  const parts = String(path || '').split('/').filter(Boolean);
  parts.pop();
  return parts.join('/') || baseRootLabel();
}

function remoteLocation(e) {
  if (!S.files.query || !S.files.remoteSearch) return '';
  return e.path && e.path.includes('/') ? ` · em ${parentDir(e.path)}` : '';
}

/* ---------------- rename / delete / move / copy ---------------- */

async function renameEntry(entry) {
  if (!entry) return;
  const oldName = entry.name || baseName(entry.path);
  const name = await promptDialog('Renomear', 'Novo nome', oldName, { ok: 'Renomear', select: true });
  if (!name || name === oldName) return;
  try {
    await api('/api/rename', { method: 'POST', json: { path: entry.path, name } });
    toast('Item renomeado.', 'ok');
    loadFiles(S.files.path);
  } catch (e) {
    toast(e.message, 'error', 'Não foi possível renomear');
  }
}

async function deleteEntries(paths) {
  paths = (paths || []).filter(Boolean);
  if (!paths.length) return;
  // A exclusão vai para a lixeira quando ela está ativa; dizer "não pode ser
  // desfeita" nesse caso assusta sem motivo e esconde o caminho de volta.
  const trashDays = Number((S.files && typeof S.files.trashDays === 'number')
    ? S.files.trashDays
    : ((S.settings && S.settings.trash_days) || 0));
  const toTrash = trashDays > 0;
  const what = paths.length === 1
    ? `O item <b>${esc(baseName(paths[0]))}</b>`
    : `<b>${paths.length} itens</b>`;
  const title = toTrash ? 'Mover para a lixeira?' : 'Excluir definitivamente?';
  const detail = toTrash
    ? `${what} vai para a lixeira e pode ser restaurado por ${trashDays} dia(s).`
    : `${what} será excluído definitivamente. Esta ação não pode ser desfeita.`;
  const ok = await confirmDialog(title, detail, { ok: toTrash ? 'Mover' : 'Excluir', danger: true });
  if (!ok) return;
  try {
    const res = await api('/api/delete', { method: 'POST', json: { paths } });
    const trashed = Number(res.trashed || 0);
    const deleted = Number(res.deleted || 0);
    if (trashed > 0 && trashed === deleted) {
      toast(`${trashed} item(ns) na lixeira.`, 'ok', 'Dá para restaurar');
    } else if (trashed > 0) {
      toast(`${trashed} na lixeira, ${deleted - trashed} excluído(s) definitivamente.`, 'info');
    } else {
      toast(`${deleted} item(ns) excluído(s).`, 'ok');
    }
    S.files.selection.clear();
    loadFiles(S.files.path);
  } catch (e) {
    toast(e.message, 'error', 'Não foi possível excluir');
  }
}

async function moveEntries(paths) {
  paths = (paths || []).filter(Boolean);
  if (!paths.length) return;
  const dest = await pickFolder('Mover itens', S.files.path, 'Mover para cá');
  if (dest === null) return;
  if (paths.every(p => (p.split('/').slice(0, -1).join('/') || '') === dest)) {
    toast('Escolha uma pasta diferente da atual.', 'info', 'Mover');
    return;
  }
  try {
    const res = await api('/api/move', { method: 'POST', json: { paths, dest } });
    toast(`${res.moved || 0} item(ns) movido(s).`, 'ok', 'Mover');
    S.files.selection.clear();
    loadFiles(S.files.path);
  } catch (e) {
    toast(e.message, 'error', 'Não foi possível mover');
  }
}

async function copyEntries(paths) {
  paths = (paths || []).filter(Boolean);
  if (!paths.length) return;
  const dest = await pickFolder('Copiar itens', S.files.path, 'Copiar para cá');
  if (dest === null) return;
  try {
    const res = await api('/api/copy', { method: 'POST', json: { paths, dest } });
    toast(`${res.copied || 0} item(ns) copiado(s).`, 'ok', 'Copiar');
    S.files.selection.clear();
    loadFiles(S.files.path);
  } catch (e) {
    toast(e.message, 'error', 'Não foi possível copiar');
  }
}

function pickFolder(title, start, actionLabel) {
  return new Promise(resolve => {
    let current = start || '';
    let settled = false;
    const finish = value => {
      if (settled) return;
      settled = true;
      resolve(value);
    };

    openModal({
      title: esc(title),
      sub: 'Escolha a pasta de destino',
      body: `<div class="tiny" id="picker-current" style="margin-bottom:7px"></div>
        <div class="picker-roots" id="picker-roots"></div>
        <div class="picker-list" id="picker-list"><div class="picker-row"><span class="tiny">Carregando…</span></div></div>`,
      footer: `<button class="btn ghost" data-close>Cancelar</button>
        <button class="btn primary" id="picker-ok">${ico('check')} ${esc(actionLabel)}</button>`,
      onClose: () => finish(null)
    });

    $('#picker-ok').addEventListener('click', () => {
      finish(current);
      closeModal();
    });

    // Atalhos para cada volume configurado. Sem isto o seletor só navegava
    // dentro da raiz atual: era impossível mover ou copiar para o cartão
    // estando no interno (e vice-versa).
    function renderPickerRoots() {
      const box = $('#picker-roots');
      if (!box) return;
      const roots = (S.files && S.files.roots) || [];
      const head = String(current || '').split('/')[0];
      const tabs = [
        { name: '', label: baseRootLabel(), icon: 'server' },
        ...roots.map(r => ({ name: r.name, label: r.name, icon: 'sd' }))
      ];
      box.innerHTML = tabs.map(t => {
        const active = t.name === '' ? !roots.some(r => r.name === head) : head === t.name;
        return `<button type="button" class="root-tab${active ? ' active' : ''}" data-pick-root="${esc(t.name)}">${ico(t.icon)}<span>${esc(t.label)}</span></button>`;
      }).join('');
      $$('[data-pick-root]', box).forEach(btn => {
        btn.addEventListener('click', () => go(btn.dataset.pickRoot || ''));
      });
    }

    async function go(path) {
      const list = $('#picker-list');
      if (!list) return;
      list.innerHTML = `<div class="picker-row"><span class="tiny">Carregando…</span></div>`;
      try {
        const qs = path ? `?path=${encodeURIComponent(path)}` : '';
        const data = await api(`/api/files${qs}`);
        current = data.path || '';
        if (Array.isArray(data.roots)) S.files.roots = data.roots;
        const here = $('#picker-current');
        if (here) here.innerHTML = `Destino: <b>${esc(current || baseRootLabel())}</b>`;
        renderPickerRoots();

        const dirs = (data.entries || []).filter(e => e.is_dir);
        let rows = '';
        if (current) {
          rows += `<div class="picker-row" data-pick-go="${esc(data.parent || '')}">${ico('arrowUp')}<b>Subir um nível</b></div>`;
          rows += `<div class="picker-row" data-pick-go="">${ico('folder')}<b>${esc(baseRootLabel())} (raiz)</b></div>`;
        }
        rows += dirs.map(d => `<div class="picker-row" data-pick-go="${esc(d.path)}">
          <span class="file-ico folder">${ico('folder')}</span><b>${esc(d.name)}</b>${ico('chevron')}
        </div>`).join('');
        if (!dirs.length && !current) {
          rows += `<div class="picker-row"><span class="tiny">Nenhuma subpasta.</span></div>`;
        }
        list.innerHTML = rows || `<div class="picker-row"><span class="tiny">Nenhuma subpasta.</span></div>`;
        $$('[data-pick-go]', list).forEach(row => {
          row.addEventListener('click', () => go(row.dataset.pickGo || ''));
        });
      } catch (e) {
        list.innerHTML = `<div class="picker-row"><span class="tiny">${esc(e.message)}</span></div>`;
      }
    }

    go(current);
  });
}

/* ---------------- context menu ---------------- */

function openFileMenu(anchor, entry) {
  const kind = entry.kind || fileKind(entry.name, entry.is_dir);
  const items = [
    { label: entry.is_dir ? 'Abrir pasta' : 'Visualizar', icon: entry.is_dir ? 'folder' : 'eye', action: () => openEntry(entry) }
  ];
  if (!entry.is_dir) {
    items.push({ label: 'Baixar', icon: 'download', action: () => { window.location.href = `/api/download?path=${encodeURIComponent(entry.path)}`; } });
    items.push({ label: 'Abrir em nova aba', icon: 'external', action: () => window.open(`/api/raw?path=${encodeURIComponent(entry.path)}`, '_blank', 'noopener') });
  } else {
    items.push({ label: 'Baixar pasta (.zip)', icon: 'archive', action: () => { window.location.href = `/api/zip?path=${encodeURIComponent(entry.path)}`; } });
  }
  items.push({ label: 'Compartilhar', icon: 'share', action: () => shareDialog(entry) });
  items.push({ sep: true });
  items.push({ label: 'Renomear', icon: 'edit', action: () => renameEntry(entry) });
  items.push({ label: 'Mover para…', icon: 'move', action: () => moveEntries([entry.path]) });
  items.push({ label: 'Copiar para…', icon: 'copy', action: () => copyEntries([entry.path]) });
  if (!entry.is_dir && /\.zip$/i.test(entry.name)) {
    items.push({ label: 'Extrair ZIP', icon: 'archive', action: () => unzipEntry(entry) });
  }
  items.push({ label: 'Detalhes', icon: 'info', action: () => showDetails(entry) });
  if (!entry.is_dir) {
    items.push({ label: 'Versões (backup)', icon: 'clock', action: () => showVersions(entry) });
  }
  items.push({ sep: true });
  items.push({ label: 'Excluir', icon: 'trash', danger: true, action: () => deleteEntries([entry.path]) });
  openCtx(anchor, items, { title: entry.name });
}

function showDetails(entry) {
  const kind = entry.kind || fileKind(entry.name, entry.is_dir);
  const rows = [
    ['Nome', entry.name],
    ['Caminho', entry.path || '(raiz)'],
    ['Tipo', entry.is_dir ? 'Pasta' : `${kind}${entry.mime ? ' · ' + entry.mime : ''}`],
    ['Tamanho', entry.is_dir ? '—' : fmtBytes(entry.size)],
    ['Modificado', fmtDateTime(entry.mtime)],
    ['Link direto', entry.is_dir ? '—' : `/api/raw?path=${encodeURIComponent(entry.path)}`]
  ];
  openModal({
    title: 'Detalhes do item',
    sub: `Informações de ${entry.name}`,
    body: `<div class="detail-list">${rows.map(([k, v]) => `<div class="detail-row"><span>${esc(k)}</span><b>${esc(v)}</b></div>`).join('')}</div>`,
    footer: `<button class="btn ghost" data-copy="${esc(entry.path)}">${ico('copy')} Copiar caminho</button><button class="btn primary" data-close>Fechar</button>`
  });
}

async function unzipEntry(entry) {
  try {
    const res = await api('/api/unzip', { method: 'POST', json: { path: entry.path } });
    toast(`Conteúdo extraído em “${baseName(res.path || '')}”.`, 'ok', 'ZIP extraído');
    loadFiles(S.files.path);
  } catch (e) {
    toast(e.message, 'error', 'Não foi possível extrair');
  }
}

/* ---------------- sharing ---------------- */

function shareDialog(entry) {
  if (!entry) return;
  const isDir = !!entry.is_dir;
  openModal({
    title: 'Compartilhar',
    sub: `${isDir ? 'Pasta' : 'Arquivo'} · ${entry.name}`,
    body: `<div class="field">
        <label for="share-exp">Validade do link</label>
        <select class="select" id="share-exp">
          <option value="0">Nunca expira</option>
          <option value="1">1 hora</option>
          <option value="24">24 horas</option>
          <option value="168" selected>7 dias</option>
          <option value="720">30 dias</option>
        </select>
      </div>
      <div class="field">
        <label for="share-pw">Senha do link <span class="tiny">(opcional)</span></label>
        <input class="input" id="share-pw" type="password" minlength="4" autocomplete="new-password" placeholder="Mínimo de 4 caracteres">
      </div>
      <p class="hint">Qualquer pessoa com o link poderá ${isDir ? 'navegar e baixar os arquivos da pasta' : 'baixar o arquivo'}. Você pode revogar o acesso quando quiser.</p>`,
    footer: `<button class="btn ghost" data-close>Cancelar</button>
      <button class="btn primary" id="share-create">${ico('link')} Criar link</button>`
  });

  $('#share-create').addEventListener('click', async () => {
    const btn = $('#share-create');
    const expires = Number($('#share-exp').value) || 0;
    const password = $('#share-pw').value;
    if (password && password.length < 4) {
      toast('A senha do link precisa ter pelo menos 4 caracteres.', 'info', 'Senha curta');
      $('#share-pw').focus();
      return;
    }
    btn.disabled = true;
    btn.textContent = 'Criando…';
    try {
      const res = await api('/api/shares', {
        method: 'POST',
        json: { path: entry.path, expires_hours: expires, password }
      });
      closeModal();
      const url = new URL(res.url || `/s/${res.share.token}`, location.origin).href;
      toast('Link de compartilhamento criado.', 'ok', 'Pronto para enviar');
      qrDialog('Link de compartilhamento', url);
    } catch (e) {
      btn.disabled = false;
      btn.innerHTML = `${ico('link')} Criar link`;
      toast(e.message, 'error', 'Não foi possível compartilhar');
    }
  });
}

/* ---------------- preview / editor ---------------- */

function previewFile(entry) {
  if (!entry) return;
  if (typeof entry === 'string') {
    entry = findFileEntry(entry) || { path: entry, name: baseName(entry), is_dir: false, kind: fileKind(baseName(entry), false) };
  }
  if (entry.is_dir) { openFolder(entry.path); return; }

  const token = ++filesPreviewSeq;
  const kind = entry.kind || fileKind(entry.name, false);
  const raw = `/api/raw?path=${encodeURIComponent(entry.path)}`;
  const download = `/api/download?path=${encodeURIComponent(entry.path)}`;
  const canExtract = /\.zip$/i.test(entry.name || '');
  let body = '';
  let flush = true;

  if (kind === 'image') {
    body = `<img class="preview-media fill" src="${raw}" alt="${esc(entry.name)}">`;
  } else if (kind === 'video') {
    body = `<video class="preview-media fill" src="${raw}" controls autoplay playsinline preload="metadata"></video>`;
  } else if (kind === 'audio') {
    body = `<div style="width:100%;padding:34px;text-align:center">
      <div class="file-ico audio" style="margin:0 auto 15px;width:52px;height:52px;border-radius:13px">${ico('audio')}</div>
      <div style="font-size:13px;font-weight:600;margin-bottom:15px;word-break:break-word">${esc(entry.name)}</div>
      <audio controls autoplay style="width:min(520px,100%)" src="${raw}"></audio>
    </div>`;
  } else if (kind === 'pdf') {
    body = `<iframe class="preview-frame" src="${raw}" title="${esc(entry.name)}"></iframe>`;
  } else if (kind === 'text') {
    body = `<div id="pv-text-body" style="width:100%;height:100%;display:grid;place-items:center"><span class="spinner"></span></div>`;
  } else {
    flush = false;
    body = `<div class="preview-unsupported">
      ${ico(kindIcon(kind))}
      <b style="display:block;font-size:14px;color:var(--text);margin-bottom:5px">Prévia não disponível</b>
      <span>Este tipo de arquivo não pode ser visualizado no navegador.</span>
      <div style="margin-top:16px"><a class="btn primary small" href="${download}">${ico('download')} Baixar arquivo</a></div>
    </div>`;
  }

  openModal({
    title: esc(entry.name),
    sub: `${fileBadge(kind, entry.name)} · ${fmtBytes(entry.size)} · ${fmtDateTime(entry.mtime)}`,
    body,
    flush,
    preview: true,
    footer: `<a class="btn ghost" href="${download}">${ico('download')} Baixar</a>
      ${canExtract ? `<button class="btn ghost" id="pv-unzip">${ico('archive')} Extrair ZIP</button>` : ''}
      ${kind === 'text' ? `<button class="btn ghost" id="pv-edit" hidden>${ico('edit')} Editar</button>` : ''}
      <button class="btn ghost" id="pv-details">${ico('info')} Detalhes</button>
      <button class="btn ghost" id="pv-share">${ico('share')} Compartilhar</button>
      <button class="btn primary" data-close>Fechar</button>`
  });

  const shareBtn = $('#pv-share');
  if (shareBtn) shareBtn.addEventListener('click', () => shareDialog(entry));
  const detailsBtn = $('#pv-details');
  if (detailsBtn) detailsBtn.addEventListener('click', () => showDetails(entry));
  const unzipBtn = $('#pv-unzip');
  if (unzipBtn) unzipBtn.addEventListener('click', () => { closeModal(); unzipEntry(entry); });

  if (kind === 'text') {
    api(`/api/text?path=${encodeURIComponent(entry.path)}`).then(data => {
      if (token !== filesPreviewSeq) return;
      const holder = $('#pv-text-body');
      if (!holder) return;
      if (data.binary) {
        holder.innerHTML = `<div class="preview-unsupported">${ico('file')}<b style="display:block;color:var(--text);margin-bottom:4px">Arquivo binário</b><span>Não é possível exibir este arquivo como texto.</span></div>`;
        return;
      }
      const notice = data.truncated ? `<div class="tiny" style="padding:8px 16px;border-bottom:1px solid var(--line)">Mostrando apenas os primeiros 2 MB do arquivo.</div>` : '';
      holder.outerHTML = `<div style="width:100%;height:100%;overflow:auto">${notice}<pre class="preview-text">${esc(data.content)}</pre></div>`;
      const edit = $('#pv-edit');
      if (edit && !data.truncated) {
        edit.hidden = false;
        edit.addEventListener('click', () => openTextEditor(entry, data.content));
      }
    }).catch(e => {
      if (token !== filesPreviewSeq) return;
      const holder = $('#pv-text-body');
      if (holder) holder.innerHTML = `<div class="preview-unsupported">${ico('info')}<b style="display:block;color:var(--text);margin-bottom:4px">Não foi possível abrir</b><span>${esc(e.message)}</span></div>`;
    });
  }
}

function openTextEditor(entry, content) {
  openModal({
    title: `Editar · ${esc(entry.name)}`,
    sub: esc(entry.path),
    body: `<textarea class="preview-textarea" id="pv-editor" spellcheck="false" autocapitalize="off" autocomplete="off">${esc(content)}</textarea>`,
    flush: true,
    preview: true,
    footer: `<button class="btn ghost" data-close>Cancelar</button>
      <button class="btn primary" id="pv-save">${ico('check')} Salvar</button>`
  });

  $('#pv-save').addEventListener('click', async () => {
    const btn = $('#pv-save');
    const text = $('#pv-editor').value;
    btn.disabled = true;
    btn.textContent = 'Salvando…';
    try {
      await api('/api/save', { method: 'POST', json: { path: entry.path, content: text } });
      toast('Arquivo salvo.', 'ok');
      closeModal();
      if (S.page === 'files') loadFiles(S.files.path, { keepSelection: true });
    } catch (e) {
      btn.disabled = false;
      btn.innerHTML = `${ico('check')} Salvar`;
      toast(e.message, 'error', 'Não foi possível salvar');
    }
  });
}

/* ---------------- versões (backup) ---------------- */

async function showVersions(entry) {
  if (!entry) return;
  openModal({
    title: 'Versões',
    sub: entry.name,
    body: `<div id="ver-body"><span class="tiny">Carregando…</span></div>`,
    footer: `<button class="btn ghost" data-close>Fechar</button>`
  });
  try {
    const h = await api(`/api/versions?path=${encodeURIComponent(entry.path)}`);
    renderVersions(h, entry);
  } catch (e) {
    const box = $('#ver-body');
    if (box) box.innerHTML = `<p class="muted">${esc(e.message)}</p>`;
  }
}

function renderVersions(h, entry) {
  const box = $('#ver-body');
  if (!box) return;

  if (h.note) {
    box.innerHTML = `<p class="muted" style="margin:0">${esc(h.note)}</p>`;
    return;
  }
  if (!h.versions || !h.versions.length) {
    box.innerHTML = `<p class="muted" style="margin:0">Nenhuma versão encontrada.</p>`;
    return;
  }

  const rows = h.versions.map(v => {
    const badge = v.current ? `<span class="chip ok">atual</span>` : '';
    const broken = v.recoverable ? '' : `<span class="chip warn">indisponível</span>`;
    return `<div class="ver-row">
      <div class="ver-meta">
        <b>${esc(v.at)}</b> ${badge}${broken}
        <span class="tiny mono">${esc(v.sha.slice(0, 12))} · ${esc(fmtBytes(v.size))}</span>
      </div>
      <div class="ver-actions">
        <a class="btn small ghost" href="/api/versions/download?snapshot=${encodeURIComponent(v.snapshot)}&path=${encodeURIComponent(h.path)}">${ico('download')} Baixar</a>
        ${v.current ? '' : `<button class="btn small primary" data-restore="${esc(v.snapshot)}">${ico('refresh')} Restaurar</button>`}
      </div>
    </div>`;
  }).join('');

  box.innerHTML = `
    <p class="tiny" style="margin:0 0 10px">${esc(h.distinct)} versão(ões) diferente(s) nos snapshots de backup. Restaurar move a versão atual para a lixeira.</p>
    <div class="ver-list">${rows}</div>`;

  $$('[data-restore]', box).forEach(b => {
    b.addEventListener('click', () => restoreVersion(h.path, b.dataset.restore));
  });
}

async function restoreVersion(path, snapshot) {
  const ok = await confirmDialog('Restaurar esta versão?',
    `O arquivo <b>${esc(path)}</b> volta ao conteúdo de ${esc(snapshot)}. ` +
    `A versão atual vai para a lixeira, não é perdida.`,
    { ok: 'Restaurar', danger: true });
  if (!ok) return;
  try {
    await api('/api/versions/restore', { method: 'POST', json: { snapshot, path } });
    toast('Versão restaurada.', 'ok');
    closeModal();
    if (S.page === 'files') loadFiles(S.files.path, { keepSelection: true });
  } catch (e) {
    toast(e.message, 'error', 'Não foi possível restaurar');
  }
}
