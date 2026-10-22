// FINDEROAD 화면 — made by. Nevertheless_D
'use strict';

const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];
const TOKEN = new URLSearchParams(location.search).get('t') || '';
const NATIVE = !!(window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.fr);

// ---------------------------------------------------------------- 분류 (확장자 묶음)
const CATS = [
  { id: 'all', label: '전체', exts: '' },
  { id: 'video', label: '동영상', exts: 'mp4, mov, m4v, mkv, avi, webm, wmv' },
  { id: 'image', label: '이미지', exts: 'jpg, jpeg, png, heic, gif, webp, tiff, bmp, svg' },
  { id: 'doc', label: '문서', exts: 'pdf, docx, doc, hwp, hwpx, xlsx, xls, pptx, ppt, key, pages, numbers, txt, md, rtf, csv' },
  { id: 'audio', label: '음악', exts: 'mp3, m4a, wav, aac, flac, aiff' },
  { id: 'archive', label: '압축', exts: 'zip, dmg, 7z, rar, tar, gz' },
  { id: 'code', label: '코드', exts: 'swift, js, ts, py, go, java, c, cpp, h, html, css, json, sh' },
];
const KIND_LABEL = { video: '동영상', image: '이미지', doc: '문서', audio: '오디오', archive: '압축 파일', code: '소스 코드', app: '응용 프로그램', folder: '폴더', other: '파일' };
const IMG_EXT = new Set('jpg jpeg png gif webp heic heif bmp tif tiff svg avif ico'.split(' '));
const VIDEO_PLAY = new Set('mp4 mov m4v webm'.split(' '));
const AUDIO_PLAY = new Set('mp3 m4a wav aac flac aif aiff'.split(' '));
const TEXT_EXT = new Set('txt md markdown csv tsv json js mjs ts jsx tsx py go swift java kt c cpp h hpp cs html htm css scss xml yml yaml sh zsh log ini conf cfg sql rb php rs toml srt vtt'.split(' '));
const KEYS = [
  ['⌘F', '검색창으로'], ['Space', '미리보기 (Quick Look)'], ['⌘C', '복사'], ['⌘V', '붙여넣기'],
  ['⌘⇧M', '이동'], ['⌘⌫', '휴지통으로'], ['⌘⌥⌫', '즉시 삭제'], ['⌘Z', '되돌리기'],
];

const S = {
  appearance: 'system', scope: 'all', folders: [], hidden: false, autoUpdate: true, home: '', build: '',
  items: [], total: 0, capped: false, method: '', ms: 0, notes: [], searched: false, searching: false, error: '',
  sel: new Set(), anchor: null, cursor: null, sort: { key: 'mtime', dir: -1 }, undo: null, update: null,
};

// ---------------------------------------------------------------- 엔진 · 앱 연결

async function api(path, body, opts = {}) {
  const init = { method: body === undefined ? 'GET' : 'POST', headers: { 'x-token': TOKEN }, signal: opts.signal };
  if (body !== undefined) {
    init.headers['content-type'] = 'application/json';
    init.body = JSON.stringify(body);
  }
  let res;
  try {
    res = await fetch(path, init);
  } catch (e) {
    if (e.name === 'AbortError') throw e;
    engineDown();
    throw new Error('엔진과 연결이 끊겼어요');
  }
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `문제가 생겼어요 (${res.status})`);
  return data;
}

async function native(cmd, args = {}) {
  if (!NATIVE) return null;
  try {
    return await window.webkit.messageHandlers.fr.postMessage({ cmd, ...args });
  } catch (e) {
    clientLog(`native ${cmd}: ${e && e.message || e}`);
    return null;
  }
}

const fileURL = (p) => `/api/file?t=${TOKEN}&path=${encodeURIComponent(p)}`;
const thumbURL = (p, s = 640) => `/api/thumb?t=${TOKEN}&s=${s}&path=${encodeURIComponent(p)}`;

let offlineTimer = null;
function engineDown() {
  $('#offline').hidden = false;
  if (offlineTimer) return;
  offlineTimer = setInterval(async () => {
    try {
      const r = await fetch('/api/ping', { headers: { 'x-token': TOKEN } });
      if (r.ok) {
        clearInterval(offlineTimer);
        offlineTimer = null;
        $('#offline').hidden = true;
      }
    } catch { /* 계속 기다려요 */ }
  }, 1500);
}

const logQueue = [];
function clientLog(msg) {
  logQueue.push(String(msg).slice(0, 500));
  if (logQueue.length === 1) {
    setTimeout(() => {
      const items = logQueue.splice(0);
      fetch('/api/clientlog', { method: 'POST', headers: { 'x-token': TOKEN, 'content-type': 'application/json' }, body: JSON.stringify({ items }) }).catch(() => {});
    }, 800);
  }
}
window.addEventListener('error', (e) => clientLog(`${e.message} @ ${e.filename}:${e.lineno}`));
window.addEventListener('unhandledrejection', (e) => clientLog(`promise: ${e.reason && e.reason.message || e.reason}`));

// ---------------------------------------------------------------- 도우미

const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

function displayPath(p) {
  if (S.home && (p === S.home || p.startsWith(S.home + '/'))) return '~' + p.slice(S.home.length);
  return p;
}
const dirname = (p) => p.replace(/\/[^/]*$/, '') || '/';
const basename = (p) => p.replace(/^.*\//, '');

function fmtSize(n) {
  if (n == null || n < 0) return '—';
  if (n < 1000) return `${n} B`;
  if (n < 1e6) return `${Math.round(n / 1e3)} KB`;
  if (n < 1e9) { const v = n / 1e6; return `${v < 100 ? v.toFixed(1) : Math.round(v)} MB`; }
  return `${(n / 1e9).toFixed(2)} GB`;
}

const timeFmt = new Intl.DateTimeFormat('ko-KR', { hour: 'numeric', minute: '2-digit' });
function dayDiff(d) {
  const a = new Date(); a.setHours(0, 0, 0, 0);
  const b = new Date(d); b.setHours(0, 0, 0, 0);
  return Math.round((a - b) / 864e5);
}
function fmtDate(ms) {
  if (!ms) return '—';
  const d = new Date(ms);
  const diff = dayDiff(d);
  if (diff === 0) return `오늘 ${timeFmt.format(d)}`;
  if (diff === 1) return `어제 ${timeFmt.format(d)}`;
  return `${d.getFullYear()}. ${d.getMonth() + 1}. ${d.getDate()}.`;
}
function fmtDateLong(ms) {
  if (!ms) return '—';
  const d = new Date(ms);
  return `${d.getFullYear()}년 ${d.getMonth() + 1}월 ${d.getDate()}일 ${timeFmt.format(d)}`;
}
function fmtDuration(sec) {
  sec = Math.round(sec);
  const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60), s = sec % 60;
  const pad = (x) => String(x).padStart(2, '0');
  return h ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

function extList(s) {
  return [...new Set(String(s).toLowerCase().split(/[\s,;，、]+/).map((x) => x.replace(/^\*?\.*/, '')).filter(Boolean))];
}
const sameSet = (a, b) => a.length === b.length && a.every((x) => b.includes(x));

const ICON_FOLDER = '<svg aria-hidden="true" viewBox="0 0 16 16" class="ic"><path d="M2 4.5A1.5 1.5 0 0 1 3.5 3h2.6l1.4 1.5h5A1.5 1.5 0 0 1 14 6v5.5A1.5 1.5 0 0 1 12.5 13h-9A1.5 1.5 0 0 1 2 11.5z"/></svg>';
const ICON_X = '<svg aria-hidden="true" width="12" height="12" viewBox="0 0 16 16" class="ic" style="stroke-width:1.7"><path d="M4 4l8 8M12 4l-8 8"/></svg>';
const ICON_MUSIC = '<svg aria-hidden="true" viewBox="0 0 16 16" class="ic"><path d="M6 12.5V3.5l7-1.5v9"/><circle cx="4.5" cy="12.5" r="1.5"/><circle cx="11.5" cy="11" r="1.5"/></svg>';

function tileHTML(it) {
  if (it.isDir && it.kind === 'folder') return `<span class="tile">${ICON_FOLDER}</span>`;
  const e = (it.ext || '?').toUpperCase();
  return `<span class="tile">${esc(e.length > 5 ? e.slice(0, 4) : e)}</span>`;
}

// ---------------------------------------------------------------- 알림 · 상태 표시줄

let toastTimer = null;
function toast(msg, opts = {}) {
  const t = $('#toast');
  $('#toastMsg').textContent = msg;
  t.classList.toggle('err', !!opts.err);
  const act = $('#toastAct');
  if (opts.action) {
    act.hidden = false;
    act.textContent = opts.action;
    act.onclick = () => { t.hidden = true; opts.onAction(); };
  } else {
    act.hidden = true;
  }
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { t.hidden = true; }, opts.ms || (opts.action ? 7000 : 4200));
}
function setStatus(msg) { $('#statusMsg').textContent = msg; }

function renderStatusLeft() {
  const scope = S.scope === 'all' ? 'Mac 전체' : `폴더 ${S.folders.length}곳 · 하위 폴더 포함`;
  const how = { spotlight: 'Spotlight', walk: '폴더 직접 검색', mixed: 'Spotlight + 폴더 직접 검색' }[S.method];
  $('#statusLeft').textContent = how && S.searched ? `${scope} · ${how}` : scope;
}

// ---------------------------------------------------------------- 화면 모드

const mq = window.matchMedia('(prefers-color-scheme: dark)');
function applyTheme() {
  const dark = S.appearance === 'dark' || (S.appearance === 'system' && mq.matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  $$('#themeSeg button').forEach((b) => b.setAttribute('aria-pressed', String(b.dataset.themePick === S.appearance)));
  const chrome = getComputedStyle(document.documentElement).getPropertyValue('--chrome').trim();
  native('appearance', { mode: S.appearance, color: chrome });
}
mq.addEventListener('change', () => { if (S.appearance === 'system') applyTheme(); });
$('#themeSeg').addEventListener('click', (e) => {
  const b = e.target.closest('button');
  if (!b) return;
  S.appearance = b.dataset.themePick;
  applyTheme();
  saveSettings({ appearance: S.appearance });
});

function saveSettings(patch) { api('/api/settings', patch).catch((e) => toast(e.message, { err: true })); }

// ---------------------------------------------------------------- 검색 범위

function renderScope() {
  $('#scopeAll').setAttribute('aria-pressed', String(S.scope === 'all'));
  $('#scopeFolders').setAttribute('aria-pressed', String(S.scope === 'folders'));
  $('#folderCount').textContent = S.folders.length;
  $('#folders').classList.toggle('off', S.scope !== 'folders');
  $('#folderList').innerHTML = S.folders.map((f, i) => `
    <div class="fd${f.exists === false ? ' missing' : ''}" title="${esc(f.path)}${f.exists === false ? ' (찾을 수 없음)' : ''}">
      ${ICON_FOLDER.replace('class="ic"', 'class="ic f" width="16" height="16"')}
      <span class="fd-t"><span class="fd-n">${esc(f.name)}</span><span class="fd-p">${esc(f.display || displayPath(f.path))}</span></span>
      <button type="button" class="x" data-rm="${i}" aria-label="${esc(f.name)} 폴더 빼기">${ICON_X}</button>
    </div>`).join('');
  $('#hiddenChk').checked = S.hidden;
  renderStatusLeft();
}

function setScope(scope) {
  if (S.scope === scope) return;
  S.scope = scope;
  renderScope();
  saveSettings({ scope });
  if (scope === 'folders' && !S.folders.length) { addFolders(); return; }
  if (S.searched) runSearch();
}
$('#scopeAll').addEventListener('click', () => setScope('all'));
$('#scopeFolders').addEventListener('click', () => setScope('folders'));
$('#folderList').addEventListener('click', (e) => {
  const b = e.target.closest('[data-rm]');
  if (!b) return;
  const f = S.folders.splice(+b.dataset.rm, 1)[0];
  renderScope();
  saveFolders();
  setStatus(`‘${f.name}’ 폴더를 검색 범위에서 뺐어요`);
  if (S.scope === 'folders' && S.searched && S.folders.length) runSearch();
});
$('#addFolder').addEventListener('click', () => addFolders());
$('#hiddenChk').addEventListener('change', (e) => {
  S.hidden = e.target.checked;
  saveSettings({ showHidden: S.hidden });
  if (S.searched) runSearch();
});

function saveFolders() { saveSettings({ folders: S.folders.map((f) => f.path) }); }

function addFolderPaths(paths) {
  let added = 0;
  for (const p of paths) {
    if (!p || S.folders.some((f) => f.path === p)) continue;
    S.folders.push({ path: p, name: basename(p) || p, display: displayPath(p), exists: true });
    added++;
  }
  if (!added) { setStatus('이미 추가된 폴더예요'); return; }
  S.scope = 'folders';
  renderScope();
  saveSettings({ folders: S.folders.map((f) => f.path), scope: 'folders' });
  setStatus(added === 1 ? `‘${basename(paths[paths.length - 1])}’ 폴더를 추가했어요` : `폴더 ${added}개를 추가했어요`);
  if (S.searched || qName.value.trim() || qExt.value.trim()) runSearch();
}

async function chooseFolders({ prompt, button, multiple }) {
  if (NATIVE) {
    const r = await native('chooseFolders', { prompt, button, multiple: !!multiple });
    return (r && r.paths) || [];
  }
  try {
    const r = await api('/api/choose-folder', { prompt, multiple: !!multiple });
    return r.paths || [];
  } catch (e) {
    toast(e.message, { err: true });
    return [];
  }
}

async function addFolders() {
  const paths = await chooseFolders({ prompt: '검색할 폴더를 골라 주세요 (여러 개 고를 수 있어요)', button: '추가', multiple: true });
  if (paths.length) addFolderPaths(paths);
  else if (S.scope === 'folders' && !S.folders.length) setStatus('검색할 폴더를 추가해 주세요');
}

// 맥 앱이 알려 줘요: Finder 에서 폴더를 끌어다 놓음 · 전체 화면
window.FR = {
  drop(paths) { if (paths && paths.length) addFolderPaths(paths); },
  dragging(on) { document.body.classList.toggle('drop-on', !!on); },
  fullscreen(on) { document.body.classList.toggle('fullscreen', !!on); setTimeout(reportDrag, 50); },
};

// ---------------------------------------------------------------- 분류 버튼 · 검색 칸

const qName = $('#qName');
const qExt = $('#qExt');

function currentCat() {
  const list = extList(qExt.value);
  if (!list.length) return 'all';
  const c = CATS.find((x) => x.exts && sameSet(extList(x.exts), list));
  return c ? c.id : null;
}

function renderChips() {
  const cur = currentCat();
  $('#chips').innerHTML = CATS.map((c) => `<button type="button" class="chip" data-cat="${c.id}" aria-pressed="${c.id === cur}" title="${esc(c.exts || '모든 확장자')}">${c.label}</button>`).join('');
}
$('#chips').addEventListener('click', (e) => {
  const b = e.target.closest('[data-cat]');
  if (!b) return;
  const c = CATS.find((x) => x.id === b.dataset.cat);
  qExt.value = c.exts;
  renderChips();
  if (S.scope === 'all' && !c.exts && !qName.value.trim()) {
    setStatus('모든 확장자 · 파일 이름을 입력해 주세요');
    clearResults();
    return;
  }
  runSearch();
});

let liveTimer = null;
function scheduleSearch() {
  clearTimeout(liveTimer);
  liveTimer = setTimeout(() => runSearch({ live: true }), 450);
}
qName.addEventListener('input', scheduleSearch);
qExt.addEventListener('input', () => { renderChips(); scheduleSearch(); });
$('#goBtn').addEventListener('click', () => runSearch());

// ---------------------------------------------------------------- 검색

let searchCtl = null;
let searchSeq = 0;

function clearResults() {
  searchCtl && searchCtl.abort();
  S.items = []; S.total = 0; S.searched = false; S.error = ''; S.sel.clear(); S.cursor = null; S.anchor = null; S.method = '';
  S.searching = false;
  renderTable(); renderPreview(); renderToolbar(); renderStatusLeft();
  $('#loading').hidden = true;
}

async function runSearch(opts = {}) {
  clearTimeout(liveTimer);
  const name = qName.value.trim();
  const ext = qExt.value.trim();
  const exts = extList(ext);
  if (S.scope === 'all' && !name && !exts.length) {
    if (!opts.live) toast('파일 이름이나 확장자를 입력해 주세요');
    clearResults();
    return;
  }
  if (S.scope === 'folders' && !S.folders.length) {
    toast('검색할 폴더를 추가해 주세요');
    return;
  }
  // 글자 하나로 Mac 전체를 뒤지면 너무 많아서, 입력 중에는 두 글자부터 찾아요 (Enter 는 바로)
  if (opts.live && S.scope === 'all' && !exts.length && [...name].length < 2) return;

  searchCtl && searchCtl.abort();
  const ctl = searchCtl = new AbortController();
  const seq = ++searchSeq;
  S.searching = true;
  $('#loading').hidden = false;
  if (!opts.quiet) setStatus('찾는 중…');
  renderEmpty();
  const keep = opts.keepSel ? new Set(S.sel) : null;
  try {
    const r = await api('/api/search', { name, ext, scope: S.scope, folders: S.folders.map((f) => f.path), hidden: S.hidden }, { signal: ctl.signal });
    if (seq !== searchSeq) return;
    S.items = r.items || [];
    S.total = r.total || 0;
    S.capped = !!r.capped;
    S.method = r.method || '';
    S.ms = r.ms || 0;
    S.notes = r.notes || [];
    S.searched = true;
    S.error = '';
    sortItems();
    if (keep) {
      S.sel = new Set(S.items.filter((i) => keep.has(i.path)).map((i) => i.path));
    } else {
      S.sel = new Set(S.items.length ? [S.items[0].path] : []);
      S.anchor = S.items.length ? 0 : null;
      S.cursor = S.anchor;
    }
    fixCursor();
    renderTable();
    renderPreview();
    renderToolbar();
    renderStatusLeft();
    if (!opts.quiet) {
      const secs = (S.ms / 1000).toFixed(S.ms < 10000 ? 2 : 0);
      setStatus(S.items.length ? `${S.total.toLocaleString()}개 찾음 · ${secs}초` + (S.notes.length ? ` · ${S.notes[0]}` : '') : (S.notes[0] || '찾은 파일이 없어요'));
    }
  } catch (e) {
    if (e.name === 'AbortError') return;
    if (seq !== searchSeq) return;
    S.items = []; S.total = 0; S.searched = true; S.error = e.message;
    S.sel.clear();
    renderTable(); renderPreview(); renderToolbar();
    setStatus(e.message);
  } finally {
    if (seq === searchSeq) {
      S.searching = false;
      $('#loading').hidden = true;
      renderEmpty();
    }
  }
}

// ---------------------------------------------------------------- 정렬 · 표

const SORT_LABEL = {
  name: ['이름 가나다순', '이름 역순'], dir: ['위치 가나다순', '위치 역순'],
  size: ['크기 작은 순', '크기 큰 순'], mtime: ['수정일 오래된 순', '수정일 최신순'],
};
const collator = new Intl.Collator('ko', { numeric: true, sensitivity: 'base' });
function sortItems() {
  const { key, dir } = S.sort;
  S.items.sort((a, b) => {
    let r;
    if (key === 'name') r = collator.compare(a.name, b.name);
    else if (key === 'dir') r = collator.compare(a.dir, b.dir) || collator.compare(a.name, b.name);
    else if (key === 'size') r = (a.size ?? -1) - (b.size ?? -1);
    else r = a.mtime - b.mtime;
    return r * dir;
  });
}
$('.thead').addEventListener('click', (e) => {
  const b = e.target.closest('[data-sort]');
  if (!b) return;
  const key = b.dataset.sort;
  if (S.sort.key === key) S.sort.dir *= -1;
  else S.sort = { key, dir: key === 'name' || key === 'dir' ? 1 : -1 };
  const cur = S.cursor != null && S.items[S.cursor] ? S.items[S.cursor].path : null;
  sortItems();
  S.cursor = cur ? S.items.findIndex((i) => i.path === cur) : null;
  S.anchor = S.cursor;
  renderTable();
  renderToolbar();
});

const tbody = $('#tbody');

function renderTable() {
  tbody.innerHTML = S.items.map((it, i) => `<div class="row" role="option" id="r${i}" data-i="${i}" aria-selected="${S.sel.has(it.path)}" title="${esc(it.path)}">` +
    `<span class="c-name">${tileHTML(it)}<span class="nm">${esc(it.name)}</span></span>` +
    `<span class="c-dir">${esc(it.dir)}</span><span class="c-size">${it.isDir ? '—' : fmtSize(it.size)}</span>` +
    `<span class="c-date">${fmtDate(it.mtime)}</span></div>`).join('');
  $$('.th').forEach((th) => {
    if (th.dataset.sort === S.sort.key) th.dataset.dir = String(S.sort.dir);
    else delete th.dataset.dir;
  });
  markCursor();
  renderEmpty();
}

function renderEmpty() {
  const el = $('#empty');
  if (S.items.length) { el.hidden = true; return; }
  el.hidden = false;
  if (S.searching) {
    el.innerHTML = '<b>찾는 중…</b>';
  } else if (S.error) {
    el.innerHTML = `<b>${esc(S.error)}</b>`;
  } else if (S.searched) {
    el.innerHTML = '<b>조건에 맞는 파일이 없어요</b><span>확장자나 검색 범위를 바꿔 보세요</span>' + (S.notes.length ? `<span>${esc(S.notes[0])}</span>` : '');
  } else {
    el.innerHTML = '<b>무엇을 찾을까요?</b><div class="tips">' +
      '<span>파일 이름을 쓰거나, 확장자 칸에 <code>mp4</code> 처럼 확장자만 써도 돼요</span>' +
      '<span>분류 버튼을 누르면 확장자가 자동으로 채워져요</span>' +
      '<span>입력하면 바로 찾아요 · Enter 로 다시 찾기</span></div>';
  }
}

function updateSelectionView() {
  for (const row of tbody.children) {
    const it = S.items[+row.dataset.i];
    row.setAttribute('aria-selected', String(!!it && S.sel.has(it.path)));
  }
  markCursor();
  renderToolbar();
  renderPreview();
}

function markCursor() {
  const prev = tbody.querySelector('.row.cursor');
  if (prev) prev.classList.remove('cursor');
  if (S.cursor == null) { tbody.removeAttribute('aria-activedescendant'); return; }
  const row = document.getElementById('r' + S.cursor);
  if (row) {
    row.classList.add('cursor');
    tbody.setAttribute('aria-activedescendant', row.id);
  }
}

function fixCursor() {
  if (S.cursor != null && (S.cursor < 0 || S.cursor >= S.items.length)) S.cursor = S.items.length ? S.items.length - 1 : null;
  if (S.anchor != null && (S.anchor < 0 || S.anchor >= S.items.length)) S.anchor = S.cursor;
}

function scrollToCursor() {
  const row = S.cursor != null && document.getElementById('r' + S.cursor);
  if (row) row.scrollIntoView({ block: 'nearest' });
}

const selItems = () => S.items.filter((i) => S.sel.has(i.path));
const selPaths = () => selItems().map((i) => i.path);

function selectRange(a, b, add) {
  const [lo, hi] = a < b ? [a, b] : [b, a];
  if (!add) S.sel.clear();
  for (let i = lo; i <= hi; i++) S.sel.add(S.items[i].path);
}

tbody.addEventListener('click', (e) => {
  const row = e.target.closest('.row');
  if (!row) return;
  const i = +row.dataset.i;
  const it = S.items[i];
  if (e.shiftKey && S.anchor != null) {
    selectRange(S.anchor, i, e.metaKey);
  } else if (e.metaKey) {
    if (S.sel.has(it.path)) S.sel.delete(it.path); else S.sel.add(it.path);
    S.anchor = i;
  } else {
    S.sel = new Set([it.path]);
    S.anchor = i;
  }
  S.cursor = i;
  tbody.focus({ preventScroll: true });
  updateSelectionView();
});
tbody.addEventListener('dblclick', (e) => {
  const row = e.target.closest('.row');
  if (!row) return;
  openItems([S.items[+row.dataset.i].path]);
});
tbody.addEventListener('contextmenu', (e) => {
  e.preventDefault();
  const row = e.target.closest('.row');
  if (!row) return;
  const i = +row.dataset.i;
  if (!S.sel.has(S.items[i].path)) {
    S.sel = new Set([S.items[i].path]);
    S.anchor = i;
  }
  S.cursor = i;
  updateSelectionView();
  showMenu(e.clientX, e.clientY);
});
document.addEventListener('contextmenu', (e) => {
  if (!e.target.closest('input, textarea, .pv-body')) e.preventDefault();
});

function moveCursor(delta, extend) {
  if (!S.items.length) return;
  let i = S.cursor == null ? (delta > 0 ? 0 : S.items.length - 1) : S.cursor + delta;
  i = Math.max(0, Math.min(S.items.length - 1, i));
  if (extend && S.anchor != null) {
    selectRange(S.anchor, i, false);
  } else {
    S.sel = new Set([S.items[i].path]);
    S.anchor = i;
  }
  S.cursor = i;
  updateSelectionView();
  scrollToCursor();
}

// ---------------------------------------------------------------- 도구 막대

function renderToolbar() {
  const n = S.sel.size;
  ['#opCopy', '#opMove', '#opTrash', '#opDelete'].forEach((id) => { $(id).disabled = n === 0; });
  let label = '결과';
  if (S.searched) {
    label = S.total > S.items.length ? `결과 ${S.total.toLocaleString()}개 중 최근 ${S.items.length.toLocaleString()}개` : `결과 ${S.items.length.toLocaleString()}개`;
    if (S.capped) label += ' (너무 많아 일부만)';
  }
  if (n > 1) label += ` · ${n}개 선택`;
  $('#resLabel').textContent = label;
  $('#sortLabel').textContent = S.items.length ? SORT_LABEL[S.sort.key][S.sort.dir === 1 ? 0 : 1] : '';
}
$('#opCopy').addEventListener('click', () => copySel());
$('#opPaste').addEventListener('click', () => paste());
$('#opMove').addEventListener('click', () => moveSel());
$('#opTrash').addEventListener('click', () => trashSel());
$('#opDelete').addEventListener('click', () => deleteSel());

// ---------------------------------------------------------------- 파일 작업

function reportErrors(r, okMsg) {
  const errs = r.errors || [];
  if (errs.length) {
    const first = errs[0];
    toast(`${basename(first.path)}: ${first.error}` + (errs.length > 1 ? ` 외 ${errs.length - 1}개` : ''), { err: true, ms: 7000 });
    return false;
  }
  if (okMsg) setStatus(okMsg);
  return true;
}

function removeItems(paths) {
  const gone = new Set(paths);
  const cur = S.cursor;
  S.items = S.items.filter((i) => !gone.has(i.path));
  S.total = Math.max(0, S.total - gone.size);
  for (const p of gone) S.sel.delete(p);
  if (!S.sel.size && S.items.length) {
    const i = Math.min(cur ?? 0, S.items.length - 1);
    S.sel.add(S.items[i].path);
    S.cursor = i;
    S.anchor = i;
  }
  fixCursor();
  renderTable();
  renderPreview();
  renderToolbar();
}

async function copySel() {
  const paths = selPaths();
  if (!paths.length) return;
  if (NATIVE) await native('pbWriteFiles', { paths });
  else await api('/api/clipboard', { paths });
  const what = paths.length === 1 ? `‘${basename(paths[0])}’` : `${paths.length}개`;
  setStatus(`${what} 복사함 · 붙여넣기(⌘V)를 누르면 넣을 폴더를 골라요`);
  toast(`${what}를 복사했어요` + (NATIVE ? ' · Finder 에서도 붙여넣을 수 있어요' : ''));
}

async function paste() {
  let paths = [];
  try {
    if (NATIVE) paths = ((await native('pbReadFiles')) || {}).paths || [];
    else paths = (await api('/api/clipboard')).paths || [];
  } catch (e) { toast(e.message, { err: true }); return; }
  if (!paths.length) { toast('붙여넣을 파일이 없어요. 먼저 파일을 복사(⌘C)해 주세요'); return; }
  const dests = await chooseFolders({ prompt: `${paths.length}개를 붙여넣을 폴더를 골라 주세요`, button: '여기에 붙여넣기' });
  if (!dests.length) return;
  setStatus('붙여넣는 중…');
  try {
    const r = await api('/api/transfer', { mode: 'copy', paths, dest: dests[0] });
    const n = (r.items || []).length;
    if (n) {
      S.undo = { type: 'paste', items: r.items, label: '붙여넣기' };
      const renamed = r.items.filter((m) => basename(m.to) !== basename(m.from)).length;
      reportErrors(r, `${n}개를 ‘${basename(dests[0])}’에 붙여넣었어요` + (renamed ? ` (같은 이름이 있어 ${renamed}개는 이름 뒤에 번호를 붙였어요)` : ''));
      toast(`${n}개를 ‘${basename(dests[0])}’에 붙여넣었어요`, { action: '되돌리기', onAction: undo });
      if (S.searched) runSearch({ keepSel: true, quiet: true });
    } else {
      reportErrors(r);
    }
  } catch (e) { toast(e.message, { err: true }); }
}

async function moveSel() {
  const paths = selPaths();
  if (!paths.length) return;
  const dests = await chooseFolders({ prompt: `${paths.length}개를 옮길 폴더를 골라 주세요`, button: '여기로 이동' });
  if (!dests.length) return;
  setStatus('옮기는 중…');
  try {
    const r = await api('/api/transfer', { mode: 'move', paths, dest: dests[0] });
    const items = r.items || [];
    if (items.length) {
      const map = new Map(items.map((m) => [m.from, m.to]));
      for (const it of S.items) {
        const to = map.get(it.path);
        if (!to) continue;
        const wasSel = S.sel.delete(it.path);
        it.path = to;
        it.name = basename(to).normalize('NFC');
        it.dir = displayPath(dirname(to)).normalize('NFC');
        if (wasSel) S.sel.add(to);
      }
      renderTable();
      renderPreview();
      S.undo = { type: 'move', items, label: '이동' };
      reportErrors(r, `${items.length}개를 ‘${basename(dests[0])}’(으)로 옮겼어요`);
      toast(`${items.length}개를 ‘${basename(dests[0])}’(으)로 옮겼어요`, { action: '되돌리기', onAction: undo });
    } else if (r.skipped && !(r.errors || []).length) {
      toast('이미 그 폴더에 있어요');
    } else {
      reportErrors(r);
    }
  } catch (e) { toast(e.message, { err: true }); }
}

async function trashSel() {
  const paths = selPaths();
  if (!paths.length) return;
  let r;
  try {
    r = NATIVE ? await native('trash', { paths }) : await api('/api/trash', { paths });
  } catch (e) { toast(e.message, { err: true }); return; }
  if (!r) { toast('휴지통으로 옮기지 못했어요', { err: true }); return; }
  const items = r.items || [];
  if (items.length) {
    removeItems(items.map((m) => m.from));
    S.undo = { type: 'trash', items, label: '휴지통으로 이동' };
    const what = items.length === 1 ? `‘${basename(items[0].from)}’` : `${items.length}개`;
    reportErrors(r, `${what}을(를) 휴지통으로 옮겼어요 · ⌘Z 로 되돌리기`);
    if (!(r.errors || []).length) toast(`${what}을(를) 휴지통으로 옮겼어요`, { action: '되돌리기', onAction: undo });
  } else {
    reportErrors(r);
  }
}

async function deleteSel() {
  const items = selItems();
  if (!items.length) return;
  const names = items.map((i) => i.name + (i.isDir ? '/' : ''));
  const ok = await confirmDialog({
    title: items.length === 1 ? `‘${items[0].name}’을(를) 바로 지울까요?` : `${items.length}개를 바로 지울까요?`,
    text: '휴지통을 거치지 않고 바로 지워서 되돌릴 수 없어요.' + (items.some((i) => i.isDir) ? '\n폴더는 안에 든 파일까지 모두 지워져요.' : ''),
    list: names.slice(0, 50).concat(names.length > 50 ? [`… 외 ${names.length - 50}개`] : []),
    yes: '영구 삭제',
  });
  if (!ok) return;
  setStatus('지우는 중…');
  try {
    const r = await api('/api/delete', { paths: items.map((i) => i.path) });
    const deleted = r.deleted || [];
    if (deleted.length) removeItems(deleted);
    reportErrors(r, `${deleted.length}개를 영구 삭제했어요`);
    if (deleted.length && !(r.errors || []).length) toast(`${deleted.length}개를 영구 삭제했어요`);
  } catch (e) { toast(e.message, { err: true }); }
}

async function undo() {
  const u = S.undo;
  if (!u) { toast('되돌릴 작업이 없어요'); return; }
  S.undo = null;
  try {
    if (u.type === 'trash' || u.type === 'move') {
      const r = await api('/api/move-back', { items: u.items });
      reportErrors(r, `${u.label}을(를) 되돌렸어요`);
    } else if (u.type === 'paste') {
      const paths = u.items.map((m) => m.to);
      const r = NATIVE ? await native('trash', { paths }) : await api('/api/trash', { paths });
      if (r) reportErrors(r, '붙여넣은 파일을 휴지통으로 옮겼어요');
    }
    toast('되돌렸어요');
    if (S.searched) runSearch({ keepSel: true, quiet: true });
  } catch (e) { toast(e.message, { err: true }); }
}

function openItems(paths) {
  if (!paths.length) return;
  api('/api/open', { paths }).catch((e) => toast(e.message, { err: true }));
  setStatus(paths.length === 1 ? `‘${basename(paths[0])}’을(를) 여는 중` : `${paths.length}개를 여는 중`);
}
function revealItems(paths) {
  if (!paths.length) return;
  api('/api/reveal', { paths }).catch((e) => toast(e.message, { err: true }));
}
async function quickLook() {
  const items = selItems();
  if (!items.length) return;
  if (!NATIVE) { toast('Quick Look 은 맥 앱에서 쓸 수 있어요'); return; }
  const cur = S.cursor != null ? S.items[S.cursor] : null;
  const index = Math.max(0, cur ? items.findIndex((i) => i.path === cur.path) : 0);
  native('quicklook', { paths: items.map((i) => i.path), index });
}
async function copyPathText() {
  const text = selPaths().join('\n');
  if (!text) return;
  if (NATIVE) await native('pbWriteText', { text });
  else await navigator.clipboard.writeText(text).catch(() => {});
  setStatus('경로를 복사했어요');
}

// ---------------------------------------------------------------- 확인 창 · 오른쪽 클릭 메뉴

let dlgResolve = null;
function confirmDialog({ title, text, list = [], yes = '확인' }) {
  $('#dlgTitle').textContent = title;
  $('#dlgText').textContent = text;
  $('#dlgList').innerHTML = list.map((x) => `<li>${esc(x)}</li>`).join('');
  $('#dlgYes').textContent = yes;
  $('#modal').hidden = false;
  $('#dlgNo').focus();
  return new Promise((res) => { dlgResolve = res; });
}
function closeDialog(v) {
  $('#modal').hidden = true;
  if (dlgResolve) { dlgResolve(v); dlgResolve = null; }
  tbody.focus({ preventScroll: true });
}
$('#dlgNo').addEventListener('click', () => closeDialog(false));
$('#dlgYes').addEventListener('click', () => closeDialog(true));
$('#modal').addEventListener('mousedown', (e) => { if (e.target.id === 'modal') closeDialog(false); });

function showMenu(x, y) {
  const n = S.sel.size;
  const m = $('#menu');
  const one = n === 1;
  m.innerHTML = [
    ['open', '열기', 'Enter'], ['ql', 'Quick Look', 'Space'], ['reveal', 'Finder에서 보기', '⌘R'], '-',
    ['copy', one ? '복사' : `${n}개 복사`, '⌘C'], ['move', '이동…', '⌘⇧M'], ['path', '경로 복사', ''], '-',
    ['trash', '휴지통으로', '⌘⌫', 'danger'], ['delete', '즉시 삭제…', '⌘⌥⌫', 'danger'],
  ].map((it) => it === '-' ? '<hr>' : `<button type="button" role="menuitem" data-act="${it[0]}" class="${it[3] || ''}"><span>${it[1]}</span><kbd>${it[2]}</kbd></button>`).join('');
  m.hidden = false;
  const r = m.getBoundingClientRect();
  m.style.left = Math.min(x, innerWidth - r.width - 8) + 'px';
  m.style.top = Math.min(y, innerHeight - r.height - 8) + 'px';
  m.querySelector('button').focus();
}
function closeMenu() { $('#menu').hidden = true; }
$('#menu').addEventListener('click', (e) => {
  const b = e.target.closest('[data-act]');
  if (!b) return;
  closeMenu();
  const act = b.dataset.act;
  ({ open: () => openItems(selPaths()), ql: quickLook, reveal: () => revealItems(selPaths()), copy: copySel, move: moveSel,
    path: copyPathText, trash: trashSel, delete: deleteSel })[act]();
});
document.addEventListener('mousedown', (e) => {
  if (!$('#menu').hidden && !e.target.closest('#menu')) closeMenu();
  if (!$('#updPop').hidden && !e.target.closest('.upd-wrap')) togglePop(false);
});
window.addEventListener('blur', closeMenu);

// ---------------------------------------------------------------- 미리보기

let pvSeq = 0;
function renderPreview() {
  const seq = ++pvSeq;
  const body = $('#pvBody');
  const items = selItems();
  if (!items.length) {
    body.innerHTML = `<div class="pv-empty">${S.items.length ? '파일을 선택하면 여기에 미리보기가 표시됩니다.' : '검색 결과에서 파일을 고르면 여기에 미리보기가 나와요.'}</div>`;
    return;
  }
  if (items.length > 1) {
    const files = items.filter((i) => !i.isDir);
    const sum = files.reduce((a, i) => a + Math.max(0, i.size || 0), 0);
    body.innerHTML = `<div class="multi">${items.slice(0, 9).map(tileHTML).join('')}</div>
      <span class="pv-name">${items.length}개 선택됨</span>
      <dl class="meta"><dt>크기</dt><dd class="m">${fmtSize(sum)}${files.length < items.length ? ` (폴더 ${items.length - files.length}개 제외)` : ''}</dd></dl>
      <div class="pv-actions"><button type="button" class="btn primary" data-pv="open">모두 열기</button><button type="button" class="btn" data-pv="reveal">Finder에서 보기</button></div>`;
    return;
  }
  const it = items[0];
  body.innerHTML = `<div class="media" id="pvMedia" title="클릭하거나 Space 를 누르면 크게 볼 수 있어요"></div>
    <span class="pv-name">${esc(it.name)}</span>
    <dl class="meta" id="pvMeta">${metaHTML(it, null)}</dl>
    <div class="pv-actions"><button type="button" class="btn primary" data-pv="open">열기</button><button type="button" class="btn" data-pv="reveal">Finder에서 보기</button></div>`;
  fillMedia($('#pvMedia'), it, seq);
  api(`/api/info?path=${encodeURIComponent(it.path)}`).then((info) => {
    if (seq !== pvSeq) return;
    $('#pvMeta').innerHTML = metaHTML(it, info);
    if (it.isDir && info.count != null) {
      const sub = $('#pvMedia .big span');
      if (sub) sub.textContent = info.count >= 0 ? `항목 ${info.count.toLocaleString()}개` : '열어 볼 수 없는 폴더';
    }
  }).catch((e) => {
    if (seq !== pvSeq) return;
    $('#pvMeta').innerHTML = `<dt>상태</dt><dd>${esc(e.message)}</dd>`;
  });
}

function metaHTML(it, info) {
  const m = (info && info.meta) || {};
  const rows = [];
  rows.push(['종류', esc(m.kind || (KIND_LABEL[it.kind] || '파일') + (it.ext && it.kind !== 'folder' ? ` (${it.ext.toUpperCase()})` : ''))]);
  if (it.isDir && it.kind === 'folder') rows.push(['항목', info && info.count >= 0 ? `${info.count.toLocaleString()}개` : '—']);
  else rows.push(['크기', `<span class="m">${fmtSize(it.size)}</span>`]);
  if (m.width) rows.push(['해상도', `<span class="m">${m.width} × ${m.height}</span>`]);
  if (m.duration) rows.push(['길이', `<span class="m">${fmtDuration(m.duration)}</span>`]);
  if (info && info.created) rows.push(['만든 날짜', fmtDateLong(info.created)]);
  rows.push(['수정일', fmtDateLong(it.mtime)]);
  rows.push(['위치', `<span class="m">${esc(it.dir)}</span>`]);
  return rows.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join('');
}

function bigTile(el, it, sub) {
  el.className = 'media';
  const inner = it.isDir && it.kind === 'folder' ? ICON_FOLDER : (it.kind === 'audio' ? ICON_MUSIC : `<b>${esc((it.ext || '?').toUpperCase().slice(0, 6))}</b>`);
  el.innerHTML = `<div class="big">${inner}<span>${esc(sub)}</span></div>`;
}

function tryThumb(el, it, seq, fallbackSub) {
  const img = new Image();
  img.alt = '';
  img.onload = () => { if (seq === pvSeq) { el.className = 'media'; el.innerHTML = ''; el.appendChild(img); } };
  img.onerror = () => { if (seq === pvSeq) bigTile(el, it, fallbackSub); };
  img.src = thumbURL(it.path);
}

function fillMedia(el, it, seq) {
  const ext = it.ext || '';
  if (it.isDir && it.kind === 'folder') { bigTile(el, it, '폴더'); return; }
  bigTile(el, it, '불러오는 중…');
  if (IMG_EXT.has(ext)) {
    const img = new Image();
    img.alt = it.name;
    img.onload = () => { if (seq === pvSeq) { el.className = 'media'; el.innerHTML = ''; el.appendChild(img); } };
    img.onerror = () => { if (seq === pvSeq) tryThumb(el, it, seq, 'Quick Look 미리보기'); };
    img.src = fileURL(it.path);
    return;
  }
  if (VIDEO_PLAY.has(ext)) {
    const v = document.createElement('video');
    v.controls = true;
    v.preload = 'metadata';
    v.playsInline = true;
    v.onerror = () => { if (seq === pvSeq) tryThumb(el, it, seq, '앱에서 열어 재생해 주세요'); };
    v.onloadedmetadata = () => { if (seq === pvSeq) { el.className = 'media dark'; } };
    v.src = fileURL(it.path);
    el.className = 'media dark';
    el.innerHTML = '';
    el.appendChild(v);
    return;
  }
  if (AUDIO_PLAY.has(ext)) {
    tryThumb(el, it, seq, '오디오');
    const a = document.createElement('audio');
    a.controls = true;
    a.preload = 'metadata';
    a.className = 'audio';
    a.src = fileURL(it.path);
    el.after(a);
    return;
  }
  if (TEXT_EXT.has(ext)) {
    api(`/api/text?path=${encodeURIComponent(it.path)}`).then((r) => {
      if (seq !== pvSeq) return;
      if (r.binary) { tryThumb(el, it, seq, 'Quick Look 미리보기'); return; }
      el.className = 'media text';
      el.innerHTML = '<pre></pre>';
      el.firstChild.textContent = r.text.slice(0, 4000) || '(빈 파일)';
    }).catch(() => { if (seq === pvSeq) bigTile(el, it, '미리보기를 만들 수 없어요'); });
    return;
  }
  tryThumb(el, it, seq, 'Space 로 Quick Look');
}

$('#pvBody').addEventListener('click', (e) => {
  const b = e.target.closest('[data-pv]');
  if (b) {
    if (b.dataset.pv === 'open') openItems(selPaths());
    else revealItems(selPaths());
    return;
  }
  if (e.target.closest('.media') && !e.target.closest('video')) quickLook();
});

// ---------------------------------------------------------------- 업데이트

let popTimer = null;
function togglePop(show) {
  const pop = $('#updPop');
  show = show ?? pop.hidden;
  pop.hidden = !show;
  $('#updBtn').setAttribute('aria-expanded', String(show));
  clearInterval(popTimer);
  if (show) {
    renderUpdate();
    if (!S.update || ['idle', 'error', 'uptodate'].includes(S.update.status)) checkUpdate();
    popTimer = setInterval(refreshUpdate, 1000);
  }
}
$('#updBtn').addEventListener('click', () => togglePop());

async function refreshUpdate() {
  try {
    const prev = S.update && S.update.status;
    S.update = await api('/api/update/state');
    renderUpdate();
    if (S.update.status === 'ready' && prev !== 'ready' && $('#updPop').hidden) {
      toast(`새 버전 v${S.update.installed} 준비 완료 · 다시 시작하면 적용돼요`, { action: '지금 다시 시작', onAction: restartForUpdate, ms: 12000 });
    }
  } catch { /* 엔진 연결 문제는 따로 표시해요 */ }
}
async function checkUpdate() {
  if (S.update) S.update.status = 'checking';
  renderUpdate();
  try { S.update = await api('/api/update/check'); } catch (e) { S.update = { ...(S.update || {}), status: 'error', error: e.message }; }
  renderUpdate();
}
async function restartForUpdate() {
  try {
    await api('/api/update/restart', {});
    setStatus('새 버전으로 다시 여는 중…');
  } catch (e) { toast(e.message, { err: true }); }
}

function agoText(ms) {
  if (!ms) return '';
  const s = (Date.now() - ms) / 1000;
  if (s < 60) return '방금 전';
  if (s < 3600) return `${Math.floor(s / 60)}분 전`;
  return fmtDate(ms);
}

function renderUpdate() {
  const u = S.update || { status: 'idle', current: S.build };
  const cur = u.current || S.build;
  $('#ver').textContent = `v${cur}`;
  $('#updVer').textContent = `v${cur}`;
  $('#updDot').hidden = !(u.status === 'available' || u.status === 'ready');
  $('#autoSw').setAttribute('aria-checked', String(S.autoUpdate));
  const latest = u.latest;
  let msg = '최신 버전을 사용 중입니다';
  let sub = u.lastCheck ? `마지막 확인 · ${agoText(u.lastCheck)}` : '';
  let act = '지금 확인';
  let actFn = checkUpdate;
  let busy = false;
  let notes = [];
  switch (u.status) {
    case 'checking': msg = '확인하는 중…'; sub = '새 버전이 있는지 보고 있어요'; busy = true; break;
    case 'available':
      msg = `새 버전 v${latest.version}이 있어요`; act = '지금 업데이트'; actFn = installUpdate; notes = latest.notes || []; break;
    case 'downloading': msg = `v${latest ? latest.version : ''} 받는 중… ${Math.round((u.progress || 0) * 100)}%`; busy = true; break;
    case 'installing': msg = '설치하는 중…'; busy = true; break;
    case 'ready':
      msg = `v${u.installed} 준비 완료`; sub = '다시 시작하면 새 버전이 적용돼요'; act = '지금 다시 시작'; actFn = restartForUpdate;
      notes = latest && latest.version === u.installed ? latest.notes || [] : []; break;
    case 'error': msg = '업데이트를 확인하지 못했어요'; sub = u.error || ''; act = '다시 시도'; break;
    case 'uptodate': if (latest && latest.version === cur) notes = latest.notes || []; break;
    case 'idle': sub = '자동 업데이트가 켜져 있으면 앱이 알아서 확인해요'; break;
  }
  $('#updMsg').textContent = msg;
  $('#updSub').textContent = sub;
  const bar = $('#updBar');
  bar.hidden = u.status !== 'downloading';
  bar.firstElementChild.style.width = `${Math.round((u.progress || 0) * 100)}%`;
  const ul = $('#updNotes');
  ul.hidden = !notes.length;
  ul.innerHTML = notes.map((n) => `<li>${esc(n)}</li>`).join('');
  const btn = $('#updAct');
  btn.textContent = busy ? '잠시만요…' : act;
  btn.disabled = busy;
  btn.onclick = actFn;
}
async function installUpdate() {
  try {
    await api('/api/update/install', {});
    S.update.status = 'downloading';
    renderUpdate();
  } catch (e) { toast(e.message, { err: true }); }
}
$('#autoSw').addEventListener('click', () => {
  S.autoUpdate = !S.autoUpdate;
  renderUpdate();
  saveSettings({ autoUpdate: S.autoUpdate });
  setStatus(S.autoUpdate ? '자동 업데이트를 켰어요' : '자동 업데이트를 껐어요');
});

// ---------------------------------------------------------------- 키보드

document.addEventListener('keydown', (e) => {
  const mod = e.metaKey || e.ctrlKey;
  if (!$('#modal').hidden) {
    if (e.key === 'Escape') { e.preventDefault(); closeDialog(false); }
    return;
  }
  if (!$('#menu').hidden) {
    if (e.key === 'Escape') { e.preventDefault(); closeMenu(); }
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      const bs = $$('#menu button');
      const i = bs.indexOf(document.activeElement);
      bs[(i + (e.key === 'ArrowDown' ? 1 : -1) + bs.length) % bs.length].focus();
    }
    return;
  }
  if (mod && e.code === 'KeyF') { e.preventDefault(); qName.focus(); qName.select(); return; }
  if (mod && e.code === 'KeyR') { e.preventDefault(); revealItems(selPaths()); return; }
  const inField = e.target.matches('input[type="text"], textarea');
  if (inField) {
    if (e.key === 'Enter') { e.preventDefault(); runSearch(); }
    else if (e.key === 'Escape') { e.target.blur(); }
    else if (e.key === 'ArrowDown' && S.items.length) { e.preventDefault(); tbody.focus(); if (S.cursor == null) moveCursor(1); }
    return;
  }
  if (e.key === 'Escape') {
    if (!$('#updPop').hidden) { togglePop(false); return; }
    return;
  }
  // 다른 곳의 글자를 골라 둔 상태면 ⌘C 는 그 글자를 복사해요
  const textSel = String(window.getSelection() || '').trim();
  if (mod && e.code === 'KeyC' && !e.shiftKey && !e.altKey) {
    if (textSel && !$('.pv-body').contains(document.activeElement) && document.activeElement !== tbody) return;
    if (textSel && $('.pv-body').contains(window.getSelection().anchorNode)) return;
    e.preventDefault(); copySel(); return;
  }
  if (mod && e.code === 'KeyV' && !e.shiftKey) { e.preventDefault(); paste(); return; }
  if (mod && e.shiftKey && e.code === 'KeyM') { e.preventDefault(); moveSel(); return; }
  if (mod && e.key === 'Backspace') { e.preventDefault(); if (e.altKey) deleteSel(); else trashSel(); return; }
  if (mod && e.code === 'KeyZ' && !e.shiftKey) { e.preventDefault(); undo(); return; }
  if (mod && e.code === 'KeyA') {
    e.preventDefault();
    S.sel = new Set(S.items.map((i) => i.path));
    updateSelectionView();
    return;
  }
  if (mod && e.code === 'KeyO') { e.preventDefault(); openItems(selPaths()); return; }
  if (e.key === ' ' && !e.target.closest('button')) { e.preventDefault(); quickLook(); return; }
  if (e.key === 'Enter' && !e.target.closest('button')) { e.preventDefault(); openItems(selPaths()); return; }
  if (e.key === 'ArrowDown') { e.preventDefault(); moveCursor(1, e.shiftKey); return; }
  if (e.key === 'ArrowUp') { e.preventDefault(); moveCursor(-1, e.shiftKey); return; }
  if (e.key === 'Home') { e.preventDefault(); moveCursor(-1e9, e.shiftKey); return; }
  if (e.key === 'End') { e.preventDefault(); moveCursor(1e9, e.shiftKey); return; }
});

// ---------------------------------------------------------------- 창 끌기 영역 (맥 앱)

function reportDrag() {
  if (!NATIVE) return;
  const rects = $$('[data-drag]').map((el) => {
    const r = el.getBoundingClientRect();
    return { x: r.left, y: r.top, w: r.width, h: r.height };
  });
  native('dragRegions', { rects });
}

// ---------------------------------------------------------------- 시작

function renderKeys() {
  $('#keys').innerHTML = KEYS.map(([k, d]) => `<div class="key"><span>${d}</span><kbd>${k}</kbd></div>`).join('');
}

async function init() {
  document.body.classList.toggle('native', NATIVE);
  renderKeys();
  renderChips();
  renderTable();
  renderPreview();
  renderToolbar();
  try {
    const st = await api('/api/state');
    S.appearance = st.appearance || 'system';
    S.scope = st.scope || 'all';
    S.folders = st.folders || [];
    S.hidden = !!st.showHidden;
    S.autoUpdate = st.autoUpdate !== false;
    S.home = st.home || '';
    S.build = st.build || '';
    S.update = st.update || null;
  } catch (e) {
    setStatus(e.message);
  }
  applyTheme();
  renderScope();
  renderUpdate();
  qName.focus();
  if (NATIVE) {
    new ResizeObserver(reportDrag).observe($('#hdr'));
    window.addEventListener('resize', reportDrag);
    reportDrag();
    native('ready', {});
  }
  setTimeout(refreshUpdate, 10000);
  setInterval(refreshUpdate, 60000);
}
init();
