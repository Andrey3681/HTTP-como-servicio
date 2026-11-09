// components.js - Componentes compartidos aislados (header/footer) para todas las páginas.
(function () {
  function currentPath() {
    var p = location.pathname.replace(/\/$/, '') || '/index.html';
    if (p === '/' || p === '') p = '/index.html';
    return p;
  }
  function active(path) { return currentPath().indexOf(path) !== -1 ? ' class="active"' : ''; }

  function setHealth(state, text) {
    var pill = document.getElementById('header-health-pill');
    if (!pill) return;
    pill.dataset.healthState = state;
    var label = document.getElementById('header-health-text');
    if (label) label.textContent = text;
    pill.setAttribute('aria-label', 'Estado de salud: ' + text);
  }

  async function refreshHeaderHealth() {
    setHealth('unknown', 'Salud: verificando…');
    try {
      var res = await fetch('/api/salud', { headers: { 'Accept': 'application/json' } });
      var data = await res.json().catch(function () { return {}; });
      var overallOk = res.ok && (data.estado === 'ok' || data.estado === 'OK');
      if (overallOk) setHealth('ok', 'Salud: OK');
      else if (res.ok) setHealth('error', 'Salud: degradada');
      else setHealth('error', 'Salud: error');
    } catch (e) {
      setHealth('unreachable', 'Salud: sin conexión');
    }
  }

  function closeNav(nav, toggle) {
    if (!nav || !nav.classList.contains('is-open')) return;
    nav.classList.remove('is-open');
    if (toggle) toggle.setAttribute('aria-expanded', 'false');
  }

  function bindNavToggle() {
    var toggle = document.getElementById('nav-toggle');
    var nav = document.getElementById('site-nav');
    if (!toggle || !nav || toggle.dataset.bound) return;
    toggle.dataset.bound = '1';
    toggle.addEventListener('click', function () {
      var open = nav.classList.toggle('is-open');
      toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
      if (open) {
        var first = nav.querySelector('a');
        if (first) first.focus();
      }
    });
    nav.addEventListener('click', function (e) {
      if (e.target && e.target.tagName === 'A') closeNav(nav, toggle);
    });
    document.addEventListener('click', function (e) {
      if (!nav.classList.contains('is-open')) return;
      if (nav.contains(e.target) || toggle.contains(e.target)) return;
      closeNav(nav, toggle);
    });
    document.addEventListener('keydown', function (e) {
      if (e.key !== 'Escape') return;
      if (!nav.classList.contains('is-open')) return;
      closeNav(nav, toggle);
      toggle.focus();
    });
  }

  function mountHeader() {
    var mount = document.getElementById('site-header-mount');
    if (!mount || mount.dataset.ready) return;
    mount.dataset.ready = '1';
    mount.innerHTML =
      '<header class="site-header">' +
        '<div class="flex items-center gap-3">' +
          '<div class="brand"><span class="material-symbols-outlined" data-icon="dns">dns</span><span class="header-brand-outfit">HTTPaaS</span></div>' +
          '<div id="header-health-pill" class="header-pill" data-health-state="unknown" aria-live="polite" aria-label="Estado de salud: verificando">' +
            '<span class="header-health-dot" aria-hidden="true"></span><span id="header-health-text">Salud: verificando…</span>' +
          '</div>' +
        '</div>' +
        '<button id="nav-toggle" class="nav-toggle" type="button" aria-expanded="false" aria-controls="site-nav" aria-label="Abrir menú de navegación">' +
          '<span class="material-symbols-outlined" aria-hidden="true">menu</span>' +
        '</button>' +
        '<nav id="site-nav" class="site-nav" aria-label="Navegación principal">' +
          '<a' + active('index.html') + ' href="/index.html">Instancias</a>' +
          '<a' + active('dns.html') + ' href="/dns.html">DNS</a>' +
          '<a' + active('vhosts.html') + ' href="/vhosts.html">VHosts</a>' +
          '<a' + active('detalle.html') + ' href="/detalle.html">Detalle</a>' +
          '<a' + active('salud.html') + ' href="/salud.html">Salud</a>' +
          '<a' + active('ayuda.html') + ' href="/ayuda.html">Ayuda</a>' +
        '</nav>' +
        '<div class="hidden lg:flex items-center gap-3">' +
          '<div class="flex items-center gap-2 px-2.5 py-1 border border-blue-400/40 rounded-xl" style="background:linear-gradient(180deg, rgba(255,255,255,0.15) 0%, rgba(255,255,255,0.04) 100%);">' +
            '<span class="material-symbols-outlined text-blue-200">terminal</span>' +
            '<div class="flex flex-col text-left"><span class="text-[11px] font-semibold text-white leading-none">Nodo local</span><span class="text-[10px] text-blue-200 leading-tight">contexto lab</span></div>' +
          '</div>' +
        '</div>' +
      '</header>';
    bindNavToggle();
    refreshHeaderHealth();
  }

  function mountFooter() {
    var mount = document.getElementById('site-footer-mount');
    if (!mount || mount.dataset.ready) return;
    mount.dataset.ready = '1';
    mount.innerHTML =
      '<footer class="relative w-full bg-[#f8fafc] border-t-standard px-4 sm:px-6 py-2 shrink-0 z-[60]" style="border-top:1px solid rgba(19,78,74,0.2);">' +
        '<div class="max-w-7xl mx-auto flex flex-col md:flex-row items-center justify-between gap-3 text-[11px] mono text-on-surface-variant">' +
          '<div class="flex flex-wrap items-center justify-center md:justify-start gap-x-3 gap-y-1">' +
            '<div class="flex items-center gap-1.5"><span class="w-2 h-2 rounded-full bg-[#16a34a] inline-block" aria-hidden="true"></span><span class="font-medium text-on-surface">Zona DNS:</span><span class="text-outline">cloud.local</span></div>' +
            '<span class="text-outline hidden sm:inline">•</span>' +
            '<div class="flex items-center gap-1"><span class="text-outline">DocumentRoot:</span><span class="px-1 rounded bg-surface-container border-standard text-on-surface">/var/www/vhosts/</span></div>' +
          '</div>' +
          '<div class="flex flex-wrap items-center justify-center md:justify-end gap-3 shrink-0">' +
            '<div id="logs-toggle-mount"></div>' +
            '<span class="text-outline">|</span>' +
            '<span class="text-on-surface-variant">BIND <span class="text-on-surface">9.18.28-1</span></span><span class="text-outline">|</span><span class="text-on-surface-variant">Apache <span class="text-on-surface">2.4.58</span></span>' +
          '</div>' +
        '</div>' +
      '</footer>';
  }

  function updateActiveNav() {
    var mount = document.getElementById('site-header-mount');
    if (!mount) return;
    var p = currentPath();
    mount.querySelectorAll('nav a').forEach(function (a) {
      var href = a.getAttribute('href') || '';
      var on = (p === href) || (p.indexOf(href.replace('/','')) !== -1 && href !== '/');
      if (href === '/index.html' && (p === '/index.html' || p === '/' || p === '')) on = true;
      a.classList.toggle('active', on);
    });
  }

  function init() {
    mountHeader();
    mountFooter();
    if (window.mountLogsDock) window.mountLogsDock();
  }
  window.updateActiveNav = updateActiveNav;
  window.refreshHeaderHealth = refreshHeaderHealth;
  window.HTTPaaSComponents = { init: init, updateActiveNav: updateActiveNav, refreshHeaderHealth: refreshHeaderHealth };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init); else init();
})();
