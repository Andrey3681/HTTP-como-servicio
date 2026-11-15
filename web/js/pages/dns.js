// pages/dns.js
(function () {
  function init() {
    var U = window.HTTPaaS || {}, esc = U.escapeHtml || function (s) { return s == null ? '' : String(s); };
    var tbody = document.getElementById('dnsTableBody'), zone = document.getElementById('dnsZone'), btn = document.getElementById('refreshDnsBtn');
    if (!tbody || tbody.dataset.bound) return;
    tbody.dataset.bound = '1';
    async function load() {
      if (btn) { btn.disabled = true; btn.setAttribute('aria-busy', 'true'); }
      tbody.innerHTML = '<tr><td colspan="4" class="empty-state">Cargando registros DNS…</td></tr>';
      try {
        var res = await fetch('/api/dns/registros');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        var data = await res.json();
        if (zone && data.zona) zone.textContent = data.zona;
        var regs = Array.isArray(data.registros) ? data.registros : [];
        if (!regs.length) {
          tbody.innerHTML = '<tr><td colspan="4" class="empty-state"><strong>No hay registros A en la zona.</strong><br><span class="text-on-surface-variant">Los registros se crean automáticamente al aprovisionar instancias. Si esperabas ver alguno, pulsa “Actualizar”.</span></td></tr>';
          return;
        }
        tbody.innerHTML = regs.map(function (r) {
          return '<tr><td class="mono">' + esc(r.nombre||'—') + '</td><td><span class="pill-state">' + esc(r.tipo||'A') + '</span></td><td class="mono">' + esc(r.ip||'—') + '</td><td class="mono muted">' + esc(String(r.ttl!=null?r.ttl:'—')) + '</td></tr>';
        }).join('');
      } catch (e) {
        tbody.innerHTML = '<tr><td colspan="4" class="empty-state"><strong>No se pudieron cargar los registros DNS.</strong><br><span class="text-on-surface-variant">Verifica que el sistema esté disponible y que DNS/Bind9 esté operativo en “Salud”, luego pulsa “Actualizar”.</span></td></tr>';
      } finally {
        if (btn) { btn.disabled = false; btn.setAttribute('aria-busy', 'false'); }
      }
    }
    if (btn) btn.addEventListener('click', load);
    load();
  }
  window.HTTPaaSPages = window.HTTPaaSPages || {}; window.HTTPaaSPages.dns = init;
})();
