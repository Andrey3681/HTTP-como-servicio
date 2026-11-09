// pages/dns.js
(function () {
  function init() {
    var U = window.HTTPaaS || {}, esc = U.escapeHtml || function (s) { return s == null ? '' : String(s); };
    var tbody = document.getElementById('dnsTableBody'), zone = document.getElementById('dnsZone'), btn = document.getElementById('refreshDnsBtn');
    if (!tbody || tbody.dataset.bound) return;
    tbody.dataset.bound = '1';
    async function load() {
      tbody.innerHTML = '<tr><td colspan="4" class="empty-state">Cargando registros…</td></tr>';
      try {
        var res = await fetch('/api/dns/registros'); if (!res.ok) throw new Error('HTTP ' + res.status);
        var data = await res.json(); if (zone && data.zona) zone.textContent = data.zona;
        var regs = Array.isArray(data.registros) ? data.registros : [];
        if (!regs.length) { tbody.innerHTML = '<tr><td colspan="4" class="empty-state">No hay registros A en la zona.</td></tr>'; return; }
        tbody.innerHTML = regs.map(function (r) { return '<tr><td class="mono">' + esc(r.nombre||'—') + '</td><td><span class="pill-state">' + esc(r.tipo||'A') + '</span></td><td class="mono">' + esc(r.ip||'—') + '</td><td class="mono muted">' + esc(String(r.ttl!=null?r.ttl:'—')) + '</td></tr>'; }).join('');
      } catch (e) { tbody.innerHTML = '<tr><td colspan="4" class="empty-state">No se pudieron cargar los registros (backend no accesible).</td></tr>'; }
    }
    if (btn) btn.addEventListener('click', load);
    load();
  }
  window.HTTPaaSPages = window.HTTPaaSPages || {}; window.HTTPaaSPages.dns = init;
})();
