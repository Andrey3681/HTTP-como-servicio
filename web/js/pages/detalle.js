// pages/detalle.js
(function () {
  function init() {
    var U = window.HTTPaaS || {}, esc = U.escapeHtml || function (s) { return s == null ? '' : String(s); }, escA = U.escapeAttr || esc, fmt = U.formatFecha || function (s) { return s || '—'; };
    var sel = document.getElementById('instanceSelect'), loadBtn = document.getElementById('loadBtn'), empty = document.getElementById('detailEmpty'), content = document.getElementById('detailContent');
    if (!sel || sel.dataset.bound) return;
    sel.dataset.bound = '1';
    function set(id, v) { var el = document.getElementById(id); if (el) el.textContent = (v == null || v === '') ? '—' : v; }
    function render(inst) {
      if (!inst) return;
      var nombre = inst.nombre || inst.id || '—', fqdn = inst.fqdn || (nombre + '.cloud.local'), ip = inst.ip || '—', estado = (inst.estado || '—').toUpperCase();
      var ok = estado === 'ACTIVA', dot = ok ? 'dot' : estado === 'ERROR' ? 'dot err' : 'dot warn', cls = ok ? 'badge-ok' : estado === 'ERROR' ? 'badge-err' : 'badge-warn';
      set('d-id', inst.id || nombre); set('d-fqdn', fqdn); set('d-ip', ip);
      var eEst = document.getElementById('d-estado'); if (eEst) eEst.innerHTML = '<span class="pill-state ' + cls + '"><span class="' + dot + '"></span><span>' + esc(estado) + '</span></span>';
      var enlace = inst.enlace || ('http://' + fqdn);
      var eEn = document.getElementById('d-enlace'); if (eEn) eEn.innerHTML = '<a class="url-service-link mono" href="' + escA(enlace) + '" target="_blank" rel="noopener">' + esc(enlace) + '</a>';
      set('d-creado', fmt(inst.creado_en)); set('d-vhost-name', fqdn); set('d-vhost-dir', nombre); set('d-vhost-log', nombre);
      set('d-dns-name', nombre); set('d-dns-ip', String(ip).replace('/24', ''));
      if (empty) empty.style.display = 'none'; if (content) content.style.display = 'block';
    }
    async function loadDetail(id) {
      if (!id) return;
      try { var res = await fetch('/api/instancias/' + encodeURIComponent(id)); var data = await res.json().catch(function () { return {}; }); if (!res.ok) throw new Error(); render(data.datos || data); } catch (e) {}
    }
    async function populate() {
      try {
        var res = await fetch('/api/instancias'); var data = await res.json().catch(function () { return {}; });
        var list = Array.isArray(data.datos) ? data.datos : (Array.isArray(data) ? data : []);
        sel.innerHTML = '<option value="">Seleccionar instancia…</option>' + list.map(function (i) { var id = i.id || i.nombre || ''; return '<option value="' + escA(id) + '">' + esc(i.fqdn || (i.nombre || id) + '.cloud.local') + '</option>'; }).join('');
        var q = new URLSearchParams(location.search).get('id'); if (q) { sel.value = q; loadDetail(q); }
      } catch (e) {}
    }
    if (loadBtn) loadBtn.addEventListener('click', function () { if (sel && sel.value) loadDetail(sel.value); });
    if (sel) sel.addEventListener('change', function () { if (sel.value) loadDetail(sel.value); });
    populate();
  }
  window.HTTPaaSPages = window.HTTPaaSPages || {}; window.HTTPaaSPages.detalle = init;
})();
