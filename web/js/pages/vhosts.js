// pages/vhosts.js - M5: VHosts/Apache (derivado de instancias; listo para GET /api/vhosts).
(function () {
  function init() {
    var U = window.HTTPaaS || {}, esc = U.escapeHtml || function (s) { return s == null ? '' : String(s); }, escA = U.escapeAttr || esc;
    var tbody = document.getElementById('vhostsTableBody'), btn = document.getElementById('refreshVhostsBtn'), filter = document.getElementById('vhostsFilter');
    if (!tbody || tbody.dataset.bound) return;
    tbody.dataset.bound = '1';
    var all = [];
    var loadError = false;

    function render(list) {
      if (!list || !list.length) {
        if (loadError) {
          tbody.innerHTML = '<tr><td colspan="5" class="empty-state"><strong>No se pudieron cargar los VHosts.</strong><br><span class="text-on-surface-variant">Verifica que el sistema esté disponible y vuelve a intentarlo con “Actualizar”.</span></td></tr>';
        } else if ((filter && filter.value || '').trim()) {
          tbody.innerHTML = '<tr><td colspan="5" class="empty-state">No hay resultados para el filtro aplicado.</td></tr>';
        } else {
          tbody.innerHTML = '<tr><td colspan="5" class="empty-state"><strong>No hay VHosts para mostrar.</strong><br><span class="text-on-surface-variant">Los VirtualHosts aparecen cuando aprovisionas instancias desde “Agregar”.</span></td></tr>';
        }
        return;
      }
      tbody.innerHTML = list.map(function (inst) {
        var nombre = inst.nombre || inst.id || '', fqdn = inst.fqdn || (nombre + '.cloud.local');
        var enlace = inst.enlace || ('http://' + fqdn), estado = (inst.estado || 'ACTIVA').toUpperCase();
        var ok = estado === 'ACTIVA', dot = ok ? 'dot' : estado === 'ERROR' ? 'dot err' : 'dot warn', cls = ok ? 'badge-ok' : estado === 'ERROR' ? 'badge-err' : 'badge-warn';
        return '<tr>' +
          '<td class="mono">' + esc(fqdn) + '</td>' +
          '<td class="mono muted">/var/www/vhosts/' + esc(nombre) + '/</td>' +
          '<td><a class="url-service-link mono" href="' + escA(enlace) + '" target="_blank" rel="noopener">' + esc(enlace) + '</a></td>' +
          '<td><span class="pill-state ' + cls + '"><span class="' + dot + '"></span><span>' + esc(estado) + '</span></span></td>' +
          '<td><a class="btn-flat" style="height:28px;padding:0 10px;font-size:12px;" href="/index.html#detalle=' + encodeURIComponent(inst.id || nombre) + '">Ver detalle</a></td>' +
        '</tr>';
      }).join('');
    }
    function apply() {
      var q = (filter && filter.value || '').trim().toLowerCase();
      render(!q ? all : all.filter(function (i) { return ((i.fqdn||'')+' '+(i.nombre||'')).toLowerCase().indexOf(q) !== -1; }));
    }
    async function load() {
      if (btn) { btn.disabled = true; btn.setAttribute('aria-busy', 'true'); }
      tbody.innerHTML = '<tr><td colspan="5" class="empty-state">Cargando VHosts…</td></tr>';
      loadError = false;
      try {
        var res = await fetch('/api/instancias');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        var data = await res.json().catch(function(){return {};});
        all = Array.isArray(data.datos) ? data.datos : (Array.isArray(data) ? data : []);
      } catch (e) {
        all = [];
        loadError = true;
      } finally {
        if (btn) { btn.disabled = false; btn.setAttribute('aria-busy', 'false'); }
      }
      apply();
    }
    if (btn) btn.addEventListener('click', load);
    if (filter) filter.addEventListener('input', apply);
    load();
  }
  window.HTTPaaSPages = window.HTTPaaSPages || {}; window.HTTPaaSPages.vhosts = init;
})();
