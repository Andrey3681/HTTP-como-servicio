// router.js - Navegación sin recarga completa (intercambia <main>, preserva header/footer/dock).
(function () {
  var PAGE_SCRIPTS = {
    index: '/js/pages/index.js',
    dns: '/js/pages/dns.js',
    salud: '/js/pages/salud.js',
    vhosts: '/js/pages/vhosts.js',
    detalle: '/js/pages/detalle.js'
  };
  var loaded = {};

  function pageKey(pathname) {
    var p = (pathname || location.pathname).split('?')[0].replace(/\/$/, '');
    if (p === '' || p === '/' || p.indexOf('index') !== -1) return 'index';
    if (p.indexOf('dns') !== -1) return 'dns';
    if (p.indexOf('vhosts') !== -1) return 'vhosts';
    if (p.indexOf('salud') !== -1) return 'salud';
    return 'index';
  }

  function runPage() {
    if (window.HTTPaaSInstancesPanel && window.HTTPaaSInstancesPanel.init) window.HTTPaaSInstancesPanel.init();
    var key = pageKey();
    var pages = window.HTTPaaSPages || {};
    if (typeof pages[key] === 'function') pages[key]();
  }

  async function navigate(href, push) {
    var url = new URL(href, location.origin);
    if (url.origin !== location.origin) { location.href = href; return; }
    try {
      var res = await fetch(url.pathname + url.search, { headers: { 'X-Requested-With': 'router' } });
      if (!res.ok) throw new Error();
      var html = await res.text();
      var doc = new DOMParser().parseFromString(html, 'text/html');
      var newMain = doc.querySelector('main');
      var curMain = document.querySelector('main');
      if (!newMain || !curMain) { location.href = href; return; }
      curMain.replaceWith(newMain);
      newMain.classList.add('page-enter');
      if (doc.title) document.title = doc.title;
      if (push !== false) history.pushState({ href: url.pathname + url.search }, '', url.pathname + url.search);
      // re-asegurar padding para dock
      var m = document.querySelector('main'); if (m) m.style.paddingBottom = '2rem';
      if (window.updateActiveNav) window.updateActiveNav();
      ensurePage(pageKey(), runPage);
      window.scrollTo(0, 0);
    } catch (e) { location.href = href; }
  }

  document.addEventListener('click', function (e) {
    var a = e.target && e.target.closest ? e.target.closest('a[href]') : null;
    if (!a) return;
    if (a.target === '_blank' || a.hasAttribute('download')) return;
    var href = a.getAttribute('href');
    if (!href || href.charAt(0) === '#' || href.indexOf('mailto:') === 0) return;
    var url = new URL(href, location.origin);
    if (url.origin !== location.origin) return;
    if (!/\.(html)(\?|$)/.test(url.pathname) && url.pathname !== '/' && url.pathname !== '') return;
    e.preventDefault();
    if (url.pathname + url.search === location.pathname + location.search) return;
    navigate(url.pathname + url.search, true);
  });

  window.addEventListener('popstate', function () { navigate(location.pathname + location.search, false); });

  function ensureScript(src, cb) {
    if (!src) { cb(); return; }
    if (loaded[src] || document.querySelector('script[src="' + src + '"]')) { cb(); return; }
    var s = document.createElement('script');
    s.src = src;
    s.onload = function () { loaded[src] = 1; cb(); };
    s.onerror = function () { loaded[src] = 1; cb(); };
    document.body.appendChild(s);
  }

  function ensurePage(key, cb) {
    // El drawer de detalle está disponible en todos los módulos.
    ensureScript(PAGE_SCRIPTS.detalle, function () {
      ensureScript(PAGE_SCRIPTS[key], cb);
    });
  }

  function boot() {
    ensurePage(pageKey(), runPage);
  }

  window.HTTPaaSRouter = { navigate: navigate, runPage: runPage };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot); else boot();
})();
