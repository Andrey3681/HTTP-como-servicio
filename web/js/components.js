// components.js - Componentes compartidos aislados (header/footer) para todas las páginas.
(function () {
  function currentPath() {
    var p = location.pathname.replace(/\/$/, '') || '/index.html';
    if (p === '/' || p === '') p = '/index.html';
    return p;
  }
  function active(path) { return currentPath().indexOf(path) !== -1 ? ' class="active"' : ''; }

  async function refreshHeaderHealth() {
    var pill = document.getElementById('header-health-pill');
    if (!pill) return;
  }

  function mountHeader() {
    var mount = document.getElementById('site-header-mount');
    if (!mount || mount.dataset.ready) return;
    mount.dataset.ready = '1';
    mount.innerHTML =
      '<header class="site-header">' +
        '<div class="flex items-center gap-3">' +
          '<div class="brand"><span class="brand-wordmark">HTTPaaS<span class="brand-dot" aria-hidden="true"></span></span></div>' +
        '</div>' +
        '<div class="flex items-center gap-2">' +
          '<button id="setup-open-btn" class="header-config-btn" type="button" title="Configurar sistemas" aria-label="Configurar sistemas">' +
            '<span class="material-symbols-outlined" aria-hidden="true">settings_suggest</span>' +
            '<span>Configurar sistemas</span>' +
          '</button>' +
        '</div>' +
      '</header>';
    var setupBtn = document.getElementById('setup-open-btn');
    if (setupBtn) {
      setupBtn.addEventListener('click', function () { if (window.openSetupWizard) window.openSetupWizard(); });
    }
  }

  function mountDock() {
    if (document.getElementById('side-dock')) return;
    var dock = document.createElement('nav');
    dock.id = 'side-dock';
    dock.className = 'side-dock';
    dock.setAttribute('aria-label', 'Módulos'); // aria-label="Módulos"
    dock.innerHTML =
      '<a data-path="index.html" href="/index.html" title="Agregar"><span class="material-symbols-outlined" aria-hidden="true">add_circle</span><span>Agregar</span></a>' +
      '<a data-path="dns.html" href="/dns.html" title="DNS"><span class="material-symbols-outlined" aria-hidden="true">language</span><span>DNS</span></a>' +
      '<a data-path="vhosts.html" href="/vhosts.html" title="VHosts"><span class="material-symbols-outlined" aria-hidden="true">public</span><span>VHosts</span></a>' +
      '<a data-path="salud.html" href="/salud.html" title="Salud"><span class="material-symbols-outlined" aria-hidden="true">monitor_heart</span><span>Salud</span></a>';
    document.body.appendChild(dock);
    updateActiveNav();
  }

  function mountFooter() {
    var mount = document.getElementById('site-footer-mount');
    if (!mount || mount.dataset.ready) return;
    mount.dataset.ready = '1';
    mount.innerHTML =
      '<footer class="relative w-full px-4 sm:px-6 py-2 shrink-0 z-[60]" style="background-color:rgb(36,40,50);border-top:1px solid rgb(51,56,66);color:#e2e8f0;">' +
        '<div class="max-w-7xl mx-auto flex items-center justify-between gap-3 text-[11px] mono">' +
          '<div id="footer-latest-log" class="flex-1 min-w-0 truncate text-[#d4d4d8]" aria-live="polite" title="Último log">Esperando logs…</div>' +
          '<div class="flex items-center justify-end gap-3 shrink-0">' +
            '<div id="logs-toggle-mount"></div>' +
          '</div>' +
        '</div>' +
      '</footer>';
    startFooterLiveLog();
  }

  function escHtml(s) {
    var H = (window.HTTPaaS && window.HTTPaaS.escapeHtml);
    if (H) return H(s);
    return String(s == null ? '' : s).replace(/[&<>"\']/g, function (c) {
      return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c];
    });
  }

  function formatFooterLog(entry) {
    if (!entry) return 'Esperando logs…';
    var ts = '—';
    if (entry.ts) {
      try { var d = new Date(entry.ts); if (!isNaN(d.getTime())) ts = d.toLocaleTimeString('es-CO', { hour12: false }); } catch (e) { ts = String(entry.ts); }
    }
    var lvl = entry.nivel || 'INFO';
    var comp = entry.componente || 'system';
    var msg = entry.mensaje || '';
    return ts + ' ' + lvl + ' [' + comp + '] ' + msg;
  }

  function setFooterLatestLog(entry) {
    var el = document.getElementById('footer-latest-log');
    if (!el) return;
    var text = formatFooterLog(entry);
    el.textContent = text;
    el.title = text;
  }

  function startFooterLiveLog() {
    if (window.__httpaasFooterLogStarted) return;
    window.__httpaasFooterLogStarted = true;

    fetch('/api/logs?limit=1', { headers: { 'Accept': 'application/json' } })
      .then(function (res) { return res.ok ? res.json() : null; })
      .then(function (data) {
        var items = data && Array.isArray(data.datos) ? data.datos : [];
        if (items.length) setFooterLatestLog(items[items.length - 1]);
      })
      .catch(function () {});

    if (typeof EventSource === 'undefined') return;
    try {
      var es = new EventSource('/api/logs/stream');
      es.onmessage = function (ev) {
        try { setFooterLatestLog(JSON.parse(ev.data)); } catch (e) {}
      };
    } catch (e) {}
  }

  function updateActiveNav() {
    var p = currentPath();
    var dock = document.getElementById('side-dock');
    if (!dock) return;
    dock.querySelectorAll('a[data-path]').forEach(function (a) {
      var href = a.getAttribute('href') || '';
      var on = (p === href) || (p.indexOf(href.replace('/', '')) !== -1 && href !== '/');
      if (href === '/index.html' && (p === '/index.html' || p === '/' || p === '')) on = true;
      a.classList.toggle('active', on);
      if (on) a.setAttribute('aria-current', 'page');
      else a.removeAttribute('aria-current');
    });
  }

  function init() {
    mountHeader();
    mountDock();
    mountFooter();
    updateActiveNav();
    if (window.mountLogsDock) window.mountLogsDock();
  }
  window.updateActiveNav = updateActiveNav;
  window.refreshHeaderHealth = refreshHeaderHealth;
  window.HTTPaaSComponents = { init: init, updateActiveNav: updateActiveNav, refreshHeaderHealth: refreshHeaderHealth };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init); else init();
})();
