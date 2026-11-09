// logs-dock.js - Componente compartido: dock de logs. Abre/cierra con animación,
// persiste estado y se conecta a GET /api/logs + GET /api/logs/stream (SSE).
(function () {
  var STYLE_ID = 'logs-dock-anim-style';
  var MAX_LINES = 400;

  function ensureStyles() {
    if (document.getElementById(STYLE_ID)) return;
    var s = document.createElement('style');
    s.id = STYLE_ID;
    s.textContent =
      '#system-logs-dock{transition:transform .28s ease; will-change:transform;}' +
      '#system-logs-dock.logs-closed{transform:translateY(calc(100% + 2rem)); pointer-events:none;}' +
      '#system-logs-dock.logs-open{transform:translateY(0); pointer-events:auto;}' +
      '.log-line{white-space:pre-wrap;word-break:break-word;padding:1px 0;border-bottom:1px solid rgba(51,56,66,.35);}' +
      '.log-ts{color:#94a3b8;} .log-comp{color:#a5b4fc;} .log-msg{color:#e2e8f0;}' +
      '.log-lvl-DEBUG{color:#94a3b8;} .log-lvl-INFO{color:#34d399;} .log-lvl-WARN{color:#fbbf24;} .log-lvl-ERROR{color:#f87171;}' +
      '.logs-state-dot{width:6px;height:6px;border-radius:9999px;display:inline-block;background:#fbbf24;}' +
      '.logs-state-live .logs-state-dot{background:#10b981;} .logs-state-error .logs-state-dot{background:#f87171;}';
    document.head.appendChild(s);
  }

  function esc(s) {
    var H = (window.HTTPaaS && window.HTTPaaS.escapeHtml);
    if (H) return H(s);
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c];
    });
  }

  function mountToggle() {
    var mount = document.getElementById('logs-toggle-mount');
    if (!mount || mount.dataset.ready) return;
    mount.dataset.ready = '1';
    mount.innerHTML =
      '<button id="toggle-logs-btn" class="inline-flex items-center gap-1.5 px-2.5 py-1 text-[#e2e8f0] mono text-[11px] transition-colors cursor-pointer" title="Mostrar/Ocultar logs del sistema" aria-label="Mostrar u ocultar logs del sistema" aria-expanded="true" aria-controls="system-logs-dock" style="background-color:rgb(36,40,50);border:1px solid rgb(51,56,66);border-radius:4px;" type="button">' +
        '<span class="material-symbols-outlined text-[14px] text-[#94a3b8]">terminal</span>' +
        '<span class="font-semibold">Logs del Sistema</span>' +
        '<span id="logs-conn-toggle" class="px-1.5 py-0.2 text-[10px] text-[#e2e8f0] font-semibold flex items-center gap-1 logs-state-connecting" data-logs-conn="toggle" style="background-color:#27272a;border:1px solid #3f3f46;border-radius:4px;"><span class="logs-state-dot animate-pulse" aria-hidden="true"></span><span class="logs-state-text">conectando</span></span>' +
        '<span class="material-symbols-outlined text-[14px] ml-0.5 text-[#94a3b8]" id="dock-chevron-icon" aria-hidden="true">expand_more</span>' +
      '</button>';
  }

  function mountDock() {
    if (document.getElementById('system-logs-dock')) return;
    ensureStyles();
    var main = document.querySelector('main');
    if (main) main.style.paddingBottom = '18rem';
    var wrap = document.createElement('div');
    wrap.innerHTML =
      '<div class="fixed bottom-8 left-0 right-0 z-40 flex flex-col text-[#e2e8f0] select-none rounded-t border-t shadow-lg logs-open" id="system-logs-dock" style="height:260px;background-color:rgb(30,34,41);border-top:1px solid rgb(51,56,66);border-top-left-radius:4px;border-top-right-radius:4px;">' +
        '<div class="w-full h-2 cursor-row-resize flex items-center justify-center group" id="logs-resize-handle" role="separator" aria-orientation="horizontal" aria-label="Redimensionar panel de logs (flechas arriba/abajo)" tabindex="0" style="background-color:rgb(36,40,50);border-bottom:1px solid rgb(51,56,66);" title="Arrastrar o usar flechas para redimensionar"><div class="w-12 h-1 rounded bg-[#71717a] group-hover:bg-[#a1a1aa]"></div></div>' +
        '<div class="h-9 px-3 sm:px-4 flex items-center justify-between text-xs mono shrink-0 gap-2" style="background-color:rgb(36,40,50);border-bottom:1px solid rgb(51,56,66);">' +
          '<div class="flex items-center gap-2 min-w-0">' +
            '<span class="material-symbols-outlined text-[16px] text-[#94a3b8]" aria-hidden="true">terminal</span>' +
            '<span class="font-semibold text-[#e2e8f0]">Logs</span>' +
            '<span id="logs-conn-dock" class="px-1.5 py-0.5 text-[10px] flex items-center gap-1 font-semibold logs-state-connecting" data-logs-conn="dock" style="background-color:#1e2229;color:#e2e8f0;border:1px solid #333842;border-radius:4px;"><span class="logs-state-dot animate-pulse" aria-hidden="true"></span><span class="logs-state-text">conectando</span></span>' +
          '</div>' +
          '<div class="flex items-center gap-2 shrink-0">' +
            '<button class="px-2 py-0.5 flex items-center gap-1 text-[11px] text-[#d4d4d8] hover:text-white" id="logs-clear-btn" type="button" aria-label="Limpiar logs" style="background-color:#1e2229;border:1px solid #333842;border-radius:4px;"><span class="material-symbols-outlined text-[13px]" aria-hidden="true">mop</span><span class="hidden sm:inline">Limpiar</span></button>' +
            '<button class="p-1 text-[#94a3b8] hover:text-white hover:bg-[#2e3440]" id="logs-close-btn" type="button" title="Cerrar logs" aria-label="Cerrar panel de logs" style="border-radius:4px;"><span class="material-symbols-outlined text-[16px]" aria-hidden="true">keyboard_arrow_down</span></button>' +
          '</div>' +
        '</div>' +
        '<div class="flex-1 overflow-y-auto p-3 mono text-[11px] leading-relaxed select-text" id="terminal-output-container" role="log" aria-live="polite" aria-label="Salida de logs" style="background-color:rgb(22,24,29);color:rgb(212,212,216);"></div>' +
      '</div>';
    document.body.appendChild(wrap.firstChild);
  }

  var STORAGE_KEY = 'httpaas.logsDock.open';
  function readOpen() { try { var v = localStorage.getItem(STORAGE_KEY); return v === null ? true : v === '1'; } catch (e) { return true; } }
  function writeOpen(on) { try { localStorage.setItem(STORAGE_KEY, on ? '1' : '0'); } catch (e) {} }

  function formatTs(ts) {
    if (!ts) return '—';
    try { var d = new Date(ts); if (isNaN(d.getTime())) return String(ts); return d.toLocaleTimeString('es-CO', { hour12: false }); } catch (e) { return String(ts); }
  }

  function renderEntry(entry) {
    var nivel = esc(entry.nivel || 'INFO');
    var lvlCls = 'log-lvl-' + nivel;
    var campos = '';
    if (entry.campos && typeof entry.campos === 'object') {
      try { campos = ' ' + esc(JSON.stringify(entry.campos)); } catch (e) { campos = ''; }
    }
    return '<div class="log-line" data-log-id="' + esc(entry.id) + '">' +
      '<span class="log-ts">' + esc(formatTs(entry.ts)) + '</span> ' +
      '<span class="' + lvlCls + '">' + nivel + '</span> ' +
      '<span class="log-comp">[' + esc(entry.componente || 'system') + ']</span> ' +
      '<span class="log-msg">' + esc(entry.mensaje || '') + campos + '</span>' +
    '</div>';
  }

  function setConnState(state, label) {
    var els = document.querySelectorAll('[data-logs-conn]');
    var cls = state === 'live' ? 'logs-state-live' : state === 'error' ? 'logs-state-error' : 'logs-state-connecting';
    for (var i = 0; i < els.length; i++) {
      els[i].classList.remove('logs-state-live', 'logs-state-error', 'logs-state-connecting');
      els[i].classList.add(cls);
      var t = els[i].querySelector('.logs-state-text');
      if (t) t.textContent = label;
      var dot = els[i].querySelector('.logs-state-dot');
      if (dot) dot.classList.toggle('animate-pulse', state !== 'live');
    }
  }

  function bind() {
    var dock = document.getElementById('system-logs-dock');
    if (!dock || dock.dataset.bound) return;
    dock.dataset.bound = '1';
    var handle = document.getElementById('logs-resize-handle');
    var toggleBtn = document.getElementById('toggle-logs-btn');
    var closeBtn = document.getElementById('logs-close-btn');
    var chev = document.getElementById('dock-chevron-icon');
    var clearBtn = document.getElementById('logs-clear-btn');
    var term = document.getElementById('terminal-output-container');
    var open = readOpen(), prevH = 260, dragging = false, startY = 0, startH = 260;
    var lastId = 0, es = null;

    function atBottom() { return term ? (term.scrollHeight - term.scrollTop - term.clientHeight < 40) : true; }
    function scrollBottom() { if (term) term.scrollTop = term.scrollHeight; }
    function trimLines() {
      if (!term) return;
      while (term.children.length > MAX_LINES) term.removeChild(term.firstChild);
    }
    function appendEntries(entries) {
      if (!term || !entries || !entries.length) return;
      var stick = atBottom();
      var html = '';
      for (var i = 0; i < entries.length; i++) {
        var e = entries[i];
        if (!e || typeof e.id !== 'number') continue;
        if (e.id <= lastId) continue;
        lastId = e.id;
        html += renderEntry(e);
      }
      if (html) {
        term.insertAdjacentHTML('beforeend', html);
        trimLines();
        if (stick) scrollBottom();
      }
    }

    async function loadRecent() {
      setConnState('connecting', 'conectando');
      try {
        var res = await fetch('/api/logs?limit=200', { headers: { 'Accept': 'application/json' } });
        if (!res.ok) throw new Error('HTTP ' + res.status);
        var data = await res.json();
        var items = data && Array.isArray(data.datos) ? data.datos : [];
        appendEntries(items);
      } catch (err) {
        if (term && !term.children.length) {
          term.innerHTML = '<div class="log-line" style="color:#f87171;">No se pudieron cargar logs recientes (/api/logs). ' + esc(err && err.message) + '</div>';
        }
      }
    }

    function connectStream() {
      if (typeof EventSource === 'undefined') { setConnState('error', 'sin SSE'); return; }
      if (es) try { es.close(); } catch (e) {}
      setConnState('connecting', 'conectando');
      es = new EventSource('/api/logs/stream');
      es.onopen = function () { setConnState('live', 'live'); };
      es.onmessage = function (ev) {
        try { var entry = JSON.parse(ev.data); appendEntries([entry]); } catch (e) {}
      };
      es.onerror = function () {
        // EventSource reconecta automáticamente; reflejar estado.
        if (es && es.readyState === EventSource.CONNECTING) setConnState('connecting', 'reconectando');
        else if (es && es.readyState === EventSource.CLOSED) setConnState('error', 'desconectado');
        else setConnState('error', 'error');
      };
    }

    function clampHeight(h) {
      var nh = h; if (nh < 120) nh = 120; if (nh > window.innerHeight - 100) nh = window.innerHeight - 100; return nh;
    }
    function setOpen(on, persist) {
      open = !!on;
      dock.classList.toggle('logs-open', open);
      dock.classList.toggle('logs-closed', !open);
      if (chev) chev.textContent = open ? 'expand_more' : 'expand_less';
      if (toggleBtn) toggleBtn.setAttribute('aria-expanded', open ? 'true' : 'false');
      if (persist !== false) writeOpen(open);
    }
    window.toggleLogsDock = function () { setOpen(!open); };
    window.closeLogsDock = function () { setOpen(false); };
    window.openLogsDock = function () { setOpen(true); };
    // Restaurar estado persistido (sin reescribir innecesariamente)
    setOpen(open, false);
    if (toggleBtn) toggleBtn.addEventListener('click', window.toggleLogsDock);
    if (closeBtn) closeBtn.addEventListener('click', window.closeLogsDock);
    if (clearBtn) clearBtn.addEventListener('click', function () { if (term) term.innerHTML = ''; });
    if (handle) {
      handle.addEventListener('mousedown', function (e) { if (!open) return; dragging = true; startY = e.clientY; startH = parseInt(getComputedStyle(dock).height, 10); document.body.style.userSelect = 'none'; document.body.style.cursor = 'row-resize'; });
      window.addEventListener('mousemove', function (e) { if (!dragging) return; var nh = clampHeight(startH + (startY - e.clientY)); dock.style.height = nh + 'px'; prevH = nh; });
      window.addEventListener('mouseup', function () { if (dragging) { dragging = false; document.body.style.userSelect = ''; document.body.style.cursor = ''; } });
      handle.addEventListener('keydown', function (e) {
        if (!open) return;
        if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return;
        e.preventDefault();
        var cur = parseInt(getComputedStyle(dock).height, 10) || 260;
        var delta = e.key === 'ArrowUp' ? 24 : -24;
        var nh = clampHeight(cur + delta);
        dock.style.height = nh + 'px'; prevH = nh;
      });
    }

    loadRecent().then(connectStream).catch(connectStream);
  }

  function init() { mountToggle(); mountDock(); bind(); }
  window.mountLogsDock = init;
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init); else init();
})();
