// pages/detalle.js - Drawer lateral de detalle de instancia (reemplaza la página /detalle.html).
(function () {
  var U = window.HTTPaaS || {};
  var esc = U.escapeHtml || function (s) { return s == null ? '' : String(s); };
  var escA = U.escapeAttr || esc;
  var fmt = U.formatFecha || function (s) { return s || '—'; };
  var bound = false;
  var lastFocus = null;

  function $(id) { return document.getElementById(id); }
  function set(id, v) { var el = $(id); if (el) el.textContent = (v == null || v === '') ? '—' : v; }

  function render(inst) {
    if (!inst) return;
    var nombre = inst.nombre || inst.id || '—';
    var fqdn = inst.fqdn || (nombre + '.cloud.local');
    var ip = inst.ip || '—';
    var estado = (inst.estado || '—').toUpperCase();
    var ok = estado === 'ACTIVA';
    var dot = ok ? 'dot' : estado === 'ERROR' ? 'dot err' : 'dot warn';
    var cls = ok ? 'badge-ok' : estado === 'ERROR' ? 'badge-err' : 'badge-warn';
    set('d-id', inst.id || nombre);
    set('d-fqdn', fqdn);
    set('d-ip', ip);
    var eEst = $('d-estado');
    if (eEst) eEst.innerHTML = '<span class="pill-state ' + cls + '"><span class="' + dot + '"></span><span>' + esc(estado) + '</span></span>';
    var enlace = inst.enlace || ('http://' + fqdn);
    var eEn = $('d-enlace');
    if (eEn) eEn.innerHTML = '<a class="url-service-link mono" href="' + escA(enlace) + '" target="_blank" rel="noopener noreferrer">' + esc(enlace) + '</a>';
    set('d-creado', fmt(inst.creado_en));
    set('d-vhost-name', fqdn);
    set('d-vhost-dir', nombre);
    set('d-vhost-log', nombre);
    set('d-dns-name', nombre);
    set('d-dns-ip', String(ip).replace('/24', ''));
  }

  function showState(state) {
    var loading = $('instance-detail-loading');
    var error = $('instance-detail-error');
    var content = $('instance-detail-content');
    if (loading) loading.style.display = state === 'loading' ? 'block' : 'none';
    if (error) error.style.display = state === 'error' ? 'block' : 'none';
    if (content) content.style.display = state === 'ready' ? 'block' : 'none';
  }

  function openDrawer() {
    var drawer = $('instance-detail-drawer');
    var overlay = $('instance-detail-overlay');
    if (!drawer || !overlay) return;
    lastFocus = document.activeElement;
    drawer.classList.add('open');
    overlay.classList.add('open');
    drawer.setAttribute('aria-hidden', 'false');
    overlay.setAttribute('aria-hidden', 'false');
    document.body.classList.add('httpaas-modal-open');
    var closeBtn = $('instance-detail-close');
    if (closeBtn) closeBtn.focus();
  }

  function closeDrawer() {
    var drawer = $('instance-detail-drawer');
    var overlay = $('instance-detail-overlay');
    if (!drawer || !overlay || !drawer.classList.contains('open')) return;
    drawer.classList.remove('open');
    overlay.classList.remove('open');
    drawer.setAttribute('aria-hidden', 'true');
    overlay.setAttribute('aria-hidden', 'true');
    if (!document.getElementById('delete-modal') || document.getElementById('delete-modal').classList.contains('hidden')) {
      document.body.classList.remove('httpaas-modal-open');
    }
    if (lastFocus && lastFocus.focus) {
      try { lastFocus.focus(); } catch (e) {}
    }
    lastFocus = null;
  }

  async function openInstanceDrawer(id) {
    if (!id) return;
    bind();
    openDrawer();
    showState('loading');
    try {
      var res = await fetch('/api/instancias/' + encodeURIComponent(id), { headers: { 'Accept': 'application/json' } });
      var data = await res.json().catch(function () { return {}; });
      if (!res.ok) throw new Error((data && data.error) || ('estado ' + res.status));
      render(data.datos || data);
      showState('ready');
    } catch (e) {
      var err = $('instance-detail-error');
      if (err) err.textContent = e && e.message ? ('No se pudo cargar el detalle de la instancia (' + e.message + '). Cierra el panel y vuelve a intentarlo desde el listado.') : 'No se pudo cargar el detalle de la instancia. Cierra el panel y vuelve a intentarlo.';
      showState('error');
    }
  }

  function bind() {
    if (bound) return;
    var drawer = $('instance-detail-drawer');
    var overlay = $('instance-detail-overlay');
    var closeBtn = $('instance-detail-close');
    if (!drawer || !overlay) return;
    bound = true;
    if (closeBtn) closeBtn.addEventListener('click', closeDrawer);
    overlay.addEventListener('click', closeDrawer);
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && drawer.classList.contains('open')) {
        e.preventDefault();
        closeDrawer();
      }
    });
  }

  function openFromHash() {
    var m = (location.hash || '').match(/[#&?]detalle=([^&]+)/);
    if (m && m[1]) {
      try { openInstanceDrawer(decodeURIComponent(m[1])); } catch (e) { openInstanceDrawer(m[1]); }
    }
  }

  window.openInstanceDrawer = openInstanceDrawer;
  window.HTTPaaSPages = window.HTTPaaSPages || {};
  window.HTTPaaSPages.detalle = function () { bind(); openFromHash(); };
  window.addEventListener('hashchange', openFromHash);
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', openFromHash); else openFromHash();
})();
