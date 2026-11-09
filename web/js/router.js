// router.js - Navegación sin recarga completa (intercambia <main>, preserva header/footer/dock).
(function () {
  var PAGE_SCRIPTS = ['/js/pages/index.js','/js/pages/dns.js','/js/pages/salud.js','/js/pages/detalle.js','/js/pages/vhosts.js','/js/pages/ayuda.js'];

  function pageKey(pathname) {
    var p = (pathname || location.pathname).split('?')[0].replace(/\/$/, '');
    if (p === '' || p === '/' || p.indexOf('index') !== -1) return 'index';
    if (p.indexOf('dns') !== -1) return 'dns';
    if (p.indexOf('vhosts') !== -1) return 'vhosts';
    if (p.indexOf('salud') !== -1) return 'salud';
    if (p.indexOf('detalle') !== -1) return 'detalle';
    if (p.indexOf('ayuda') !== -1) return 'ayuda';
    return 'index';
  }

  function runPage() {
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
      if (doc.title) document.title = doc.title;
      if (push !== false) history.pushState({ href: url.pathname + url.search }, '', url.pathname + url.search);
      // re-asegurar padding para dock
      var m = document.querySelector('main'); if (m) m.style.paddingBottom = '18rem';
      if (window.updateActiveNav) window.updateActiveNav();
      runPage();
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

  function boot() {
    // garantizar scripts de páginas disponibles
    var pending = 0, done = false;
    function ready() { if (done) return; done = true; runPage(); }
    function ensure(src, cb) {
      if (document.querySelector('script[src="' + src + '"]')) { cb(); return; }
      var s = document.createElement('script'); s.src = src; s.onload = cb; s.onerror = cb; document.body.appendChild(s);
    }
    var i = 0; (function next(){ if(i>=PAGE_SCRIPTS.length){ ready(); return; } ensure(PAGE_SCRIPTS[i++], next); })();
  }

  window.HTTPaaSRouter = { navigate: navigate, runPage: runPage };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot); else boot();
})();
