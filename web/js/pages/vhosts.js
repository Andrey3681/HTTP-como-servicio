// pages/vhosts.js - M5: VHosts/Apache (derivado de instancias; listo para GET /api/vhosts).
(function () {
  function init() {
    var U = window.HTTPaaS || {}, esc = U.escapeHtml || function (s) { return s == null ? '' : String(s); }, escA = U.escapeAttr || esc;
    var tbody = document.getElementById('vhostsTableBody'), btn = document.getElementById('refreshVhostsBtn'), filter = document.getElementById('vhostsFilter'), message = document.getElementById('vhostsMessage');
    if (!tbody || tbody.dataset.bound) return;
    tbody.dataset.bound = '1';
    var all = [];
    var loadError = false;

    function showMessage(text, kind) {
      if (!message) return;
      message.textContent = text || '';
      message.className = 'px-4 py-2 text-[12px] ' + (kind === 'error' ? 'text-error' : kind === 'success' ? 'text-[#059669]' : 'text-on-surface-variant') + (text ? '' : ' hidden');
    }

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
        var id = inst.id || inst.nombre || '', nombre = inst.nombre || inst.id || '', fqdn = inst.fqdn || (nombre + '.cloud.local');
        var enlace = inst.enlace || ('http://' + fqdn), estado = (inst.estado || 'ACTIVA').toUpperCase();
        var ok = estado === 'ACTIVA', dot = ok ? 'dot' : estado === 'ERROR' ? 'dot err' : 'dot warn', cls = ok ? 'badge-ok' : estado === 'ERROR' ? 'badge-err' : 'badge-warn';
        var toggleable = estado === 'ACTIVA' || estado === 'DETENIDA';
        return '<tr>' +
          '<td class="mono">' + esc(fqdn) + '</td>' +
          '<td class="mono muted">/var/www/vhosts/' + esc(nombre) + '/</td>' +
          '<td><a class="url-service-link mono" href="' + escA(enlace) + '" target="_blank" rel="noopener">' + esc(enlace) + '</a></td>' +
          '<td><span class="pill-state ' + cls + '"><span class="' + dot + '"></span><span>' + esc(estado) + '</span></span></td>' +
          '<td><div class="flex items-center gap-2">' +
            '<label class="vhost-switch" title="' + (ok ? 'Desactivar instancia' : 'Activar instancia') + '">' +
              '<input type="checkbox" data-power="' + escA(id) + '"' + (ok ? ' checked' : '') + (toggleable ? '' : ' disabled') + ' aria-label="' + (ok ? 'Desactivar' : 'Activar') + ' ' + escA(fqdn) + '">' +
              '<span class="vhost-switch-slider" aria-hidden="true"></span>' +
            '</label>' +
          '</div></td>' +
        '</tr>';
      }).join('');
      tbody.querySelectorAll('[data-power]').forEach(function (input) {
        input.addEventListener('change', function () { togglePower(input); });
      });
    }

    async function togglePower(input) {
      var id = input.getAttribute('data-power');
      var activate = input.checked;
      var inst = all.filter(function (i) { return (i.id || i.nombre) === id; })[0];
      input.disabled = true;
      showMessage(activate ? 'Activando instancia…' : 'Desactivando instancia…', 'info');
      try {
        var res = await fetch('/api/instancias/' + encodeURIComponent(id) + (activate ? '/activar' : '/desactivar'), {
          method: 'POST',
          headers: { 'Accept': 'application/json' }
        });
        var data = await res.json().catch(function () { return {}; });
        if (!res.ok) throw new Error(data.error || ('HTTP ' + res.status));
        if (inst) {
          var updated = data.datos || {};
          inst.estado = updated.estado || (activate ? 'ACTIVA' : 'DETENIDA');
          inst.ip = updated.ip || inst.ip;
        }
        showMessage(activate ? 'Instancia activada.' : 'Instancia desactivada.', 'success');
        if (window.refreshInstancesPanel) window.refreshInstancesPanel();
        apply();
      } catch (e) {
        input.checked = !activate;
        showMessage('No se pudo ' + (activate ? 'activar' : 'desactivar') + ': ' + e.message, 'error');
        input.disabled = false;
      }
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
