// components.js - Componentes compartidos aislados (header/footer) para todas las páginas.
(function () {
  function currentPath() {
    var p = location.pathname.replace(/\/$/, '') || '/index.html';
    if (p === '/' || p === '') p = '/index.html';
    return p;
  }
  function active(path) { return currentPath().indexOf(path) !== -1 ? ' class="active"' : ''; }

  function mountHeader() {
    var mount = document.getElementById('site-header-mount');
    if (!mount || mount.dataset.ready) return;
    mount.dataset.ready = '1';
    mount.innerHTML =
      '<header class="site-header">' +
        '<div class="flex items-center gap-4">' +
          '<div class="brand"><span class="material-symbols-outlined" data-icon="dns">dns</span><span class="header-brand-outfit">HTTPaaS</span></div>' +
          '<div class="header-pill"><span class="w-2 h-2 rounded-full bg-[#16a34a]" style="box-shadow:0 0 4px #22c55e !important;"></span><span>Apache 2.4 + Bind9 Online</span></div>' +
        '</div>' +
        '<nav>' +
          '<a' + active('index.html') + ' href="/index.html">Instancias</a>' +
          '<a' + active('dns.html') + ' href="/dns.html">DNS</a>' +
          '<a' + active('vhosts.html') + ' href="/vhosts.html">VHosts</a>' +
          '<a' + active('detalle.html') + ' href="/detalle.html">Detalle</a>' +
          '<a' + active('salud.html') + ' href="/salud.html">Salud</a>' +
          '<a' + active('ayuda.html') + ' href="/ayuda.html">Ayuda</a>' +
        '</nav>' +
        '<div class="flex items-center gap-3">' +
          '<div class="hidden md:flex items-center gap-2 px-2.5 py-1 border border-blue-400/40 rounded-xl" style="background:linear-gradient(180deg, rgba(255,255,255,0.15) 0%, rgba(255,255,255,0.04) 100%);">' +
            '<span class="material-symbols-outlined text-blue-200">terminal</span>' +
            '<div class="flex flex-col text-left"><span class="text-[11px] font-semibold text-white leading-none">Nodo 01</span><span class="text-[10px] text-blue-200 leading-tight">us-east-metal</span></div>' +
          '</div>' +
        '</div>' +
      '</header>';
  }

  function mountFooter() {
    var mount = document.getElementById('site-footer-mount');
    if (!mount || mount.dataset.ready) return;
    mount.dataset.ready = '1';
    mount.innerHTML =
      '<footer class="relative w-full bg-[#f8fafc] border-t-standard px-4 sm:px-6 py-2 shrink-0 z-[60]" style="border-top:1px solid rgba(19,78,74,0.2);">' +
        '<div class="max-w-7xl mx-auto flex flex-col md:flex-row items-center justify-between gap-3 text-[11px] mono text-on-surface-variant">' +
          '<div class="flex flex-wrap items-center justify-center md:justify-start gap-x-3 gap-y-1">' +
            '<div class="flex items-center gap-1.5"><span class="w-2 h-2 rounded-full bg-[#16a34a] inline-block"></span><span class="font-medium text-on-surface">Bind9 DNS:</span><span class="text-outline">Zona cloud.local</span></div>' +
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
  window.HTTPaaSComponents = { init: init, updateActiveNav: updateActiveNav };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init); else init();
})();
