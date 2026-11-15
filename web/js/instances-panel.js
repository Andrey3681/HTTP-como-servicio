// instances-panel.js - Panel compartido de instancias activas (listado global).
(function () {
  var U = window.HTTPaaS || {};
  var esc = U.escapeHtml || function (s) { return String(s == null ? '' : s); };
  var escA = U.escapeAttr || esc;
  var cachedInstances = null;
  var cachedHostDnsOk = null;
  var lastFetchAt = 0;

  function panelHTML() {
    return '' +
      '<section class="bg-surface-container-lowest border-standard rounded-xl overflow-hidden flex flex-col justify-between h-full" id="instances-panel">' +
        '<div>' +
          '<div id="nsStoppedWarning" class="hidden mx-4 mt-3 rounded-xl border border-orange-300 bg-orange-50 p-3" role="alert">' +
            '<div class="flex items-start gap-2.5">' +
              '<span class="material-symbols-outlined text-orange-600 text-[20px] shrink-0" aria-hidden="true">power_off</span>' +
              '<div class="flex-1 min-w-0">' +
                '<p class="m-0 text-[13px] font-semibold text-orange-950">Servidor DNS (ns) apagado</p>' +
                '<p class="m-0 mt-1 text-[12.5px] leading-relaxed text-orange-900">La máquina virtual ns está detenida en VirtualBox.</p>' +
              '</div>' +
              '<div class="flex items-center gap-1.5 shrink-0">' +
                '<button class="inline-flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-[12.5px] font-semibold text-white transition-colors disabled:opacity-60" id="startNsBtn" type="button" style="background:#059669;border:1px solid #047857;"><span class="material-symbols-outlined text-[15px]">power_settings_new</span> Encender ns</button>' +
              '</div>' +
            '</div>' +
          '</div>' +
          '<div id="hostDnsWarning" class="hidden mx-4 mt-3 rounded-xl border border-orange-300 bg-orange-50 p-3" role="alert">' +
            '<div class="flex items-start gap-2.5">' +
              '<span class="material-symbols-outlined text-orange-600 text-[20px] shrink-0" aria-hidden="true">warning</span>' +
              '<div class="flex-1 min-w-0">' +
                '<p class="m-0 text-[13px] font-semibold text-orange-950">DNS del anfitrión no configurado</p>' +
                '<p class="m-0 mt-1 text-[12.5px] leading-relaxed text-orange-900" id="hostDnsWarningText"></p>' +
              '</div>' +
              '<div class="flex items-center gap-1.5 shrink-0">' +
                '<button class="inline-flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-[12.5px] font-semibold text-white transition-colors disabled:opacity-60" id="configureHostDnsBtn" type="button" style="background:#ea580c;border:1px solid #c2410c;">Configurar DNS</button>' +
              '</div>' +
            '</div>' +
          '</div>' +
          '<div class="p-4 border-b-standard flex flex-col gap-3 bg-surface-container-lowest">' +
            '<div class="flex items-center justify-between gap-2">' +
              '<div class="flex items-center gap-2">' +
                '<h2 class="text-[16px] font-semibold text-on-surface">Instancias activas</h2>' +
                '<span class="px-2 py-0.5 rounded-xl border-standard bg-surface-container text-[11px] font-semibold text-on-surface mono" id="instancesCount">0</span>' +
              '</div>' +
              '<button class="px-2 py-1 border-standard rounded-xl bg-surface-container-lowest hover:bg-surface-container-low text-on-surface text-[12px] flex items-center gap-1" id="refreshInstancesBtn" title="Actualizar instancias" aria-label="Actualizar instancias" type="button">' +
                '<span class="material-symbols-outlined text-[15px]">refresh</span>' +
              '</button>' +
            '</div>' +
            '<div class="relative w-full">' +
              '<span class="material-symbols-outlined absolute left-2.5 top-1/2 -translate-y-1/2 text-outline text-[16px]">search</span>' +
              '<input id="instancesFilter" aria-label="Filtrar instancias" class="w-full pl-8 pr-3 py-1 bg-surface-container-lowest border-standard rounded-xl text-[12px] text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary-container focus:outline-none" placeholder="Filtrar..." type="text">' +
            '</div>' +
          '</div>' +
          '<div class="divide-y divide-[#d0d7de]" id="instancesList">' +
            '<div class="p-4 text-center text-[13px] text-on-surface-variant">Cargando instancias…</div>' +
          '</div>' +
        '</div>' +
        '<div class="p-3 bg-surface-container-low border-t-standard text-on-surface-variant text-[11px] mono">' +
          '<span id="instancesBindCount">0 instancias</span>' +
        '</div>' +
      '</section>';
  }

  function ensureOverlays() {
    if (!document.getElementById('delete-modal')) {
      var wrap = document.createElement('div');
      wrap.innerHTML =
        '<div aria-labelledby="modal-title" aria-modal="true" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-[rgba(31,35,40,0.4)] hidden" id="delete-modal" role="dialog">' +
          '<div id="delete-modal-panel" class="w-full max-w-md bg-surface-container-lowest border-standard rounded-xl p-6 relative" tabindex="-1">' +
            '<div class="flex items-start gap-3 pb-4 border-b-standard">' +
              '<div class="w-9 h-9 rounded-xl bg-error-container border-standard flex items-center justify-center text-secondary shrink-0"><span class="material-symbols-outlined">warning</span></div>' +
              '<div><h3 class="text-[16px] font-semibold text-on-surface" id="modal-title">Confirmar eliminación</h3></div>' +
            '</div>' +
            '<div class="py-4 space-y-3">' +
              '<p class="text-sm text-on-surface font-semibold" id="modal-question">¿Eliminar instancia?</p>' +
              '<p class="text-[12px] text-on-surface-variant leading-relaxed">Ruta: <code class="mono" id="modal-path">—</code></p>' +
              '<span id="modal-conf" class="hidden"></span><span id="modal-nsupdate" class="hidden"></span>' +
            '</div>' +
            '<div class="flex items-center justify-end gap-3 pt-3 border-t-standard">' +
              '<button class="px-4 py-2 border-standard bg-surface-container-lowest hover:bg-surface-container-low text-on-surface text-[14px] rounded-xl font-medium" id="deleteCancelBtn" type="button">Cancelar</button>' +
              '<button class="px-4 py-2 bg-secondary-container hover:bg-[#b91c1c] text-white text-sm rounded-xl font-semibold flex items-center gap-1.5" id="deleteConfirmBtn" type="button"><span class="material-symbols-outlined text-[16px]">delete_forever</span> Eliminar</button>' +
            '</div>' +
          '</div>' +
        '</div>';
      document.body.appendChild(wrap.firstElementChild);
    }
    if (!document.getElementById('instance-detail-drawer')) {
      var d = document.createElement('div');
      d.innerHTML =
        '<div id="instance-detail-overlay" class="instance-drawer-overlay" aria-hidden="true"></div>' +
        '<aside id="instance-detail-drawer" class="instance-drawer" aria-hidden="true" aria-labelledby="instance-detail-title" role="dialog" aria-modal="true" tabindex="-1">' +
          '<div class="instance-drawer-header"><div><h2 id="instance-detail-title">Detalle de instancia</h2></div><button id="instance-detail-close" class="instance-drawer-close" type="button" aria-label="Cerrar detalle"><span class="material-symbols-outlined" aria-hidden="true">close</span></button></div>' +
          '<div class="instance-drawer-body">' +
            '<div id="instance-detail-loading" class="empty-state" style="display:none;">Cargando detalle…</div>' +
            '<div id="instance-detail-error" class="empty-state" style="display:none;">No se pudo cargar el detalle.</div>' +
            '<div id="instance-detail-content" style="display:none;">' +
              '<div class="pb-4 border-b-standard">' +
                '<div class="flex items-center justify-between gap-2 mb-2">' +
                  '<h3 class="text-[14px] font-semibold m-0">Acciones</h3>' +
                  '<span id="d-actions-msg" class="text-[12px] text-on-surface-variant" role="status" aria-live="polite"></span>' +
                '</div>' +
                '<div class="space-y-2">' +
                  '<button id="d-power-btn" class="btn-flat w-full justify-start" type="button"><span class="material-symbols-outlined text-[16px]">power_settings_new</span> <span id="d-power-label">Encender instancia</span></button>' +
                  '<button id="d-restart-btn" class="btn-flat w-full justify-start" type="button"><span class="material-symbols-outlined text-[16px]">restart_alt</span> Reiniciar servicios</button>' +
                  '<div class="action-accordion">' +
                    '<button id="d-ssh-btn" class="btn-flat w-full justify-between action-toggle" type="button" aria-expanded="false" aria-controls="d-ssh-panel">' +
                      '<span class="inline-flex items-center gap-2"><span class="material-symbols-outlined text-[16px]">terminal</span> Acceso SSH</span>' +
                      '<span class="material-symbols-outlined text-[16px] action-chevron" aria-hidden="true">expand_more</span>' +
                    '</button>' +
                    '<div id="d-ssh-panel" class="hidden mt-2 rounded-xl border-standard p-3">' +
                      '<div class="kv-label mb-1">Comando SSH</div>' +
                      '<pre class="code-block" id="d-ssh-command" style="white-space:pre-wrap;word-break:break-all;"></pre>' +
                      '<p id="d-ssh-note" class="text-[12px] text-on-surface-variant mt-2"></p>' +
                      '<div class="flex justify-end mt-2"><button id="d-ssh-copy" class="btn-flat" type="button"><span class="material-symbols-outlined text-[16px]">content_copy</span> Copiar comando</button></div>' +
                    '</div>' +
                  '</div>' +
                  '<div class="action-accordion">' +
                    '<button id="d-redeploy-btn" class="btn-flat w-full justify-between action-toggle" type="button" aria-expanded="false" aria-controls="d-redeploy-panel">' +
                      '<span class="inline-flex items-center gap-2"><span class="material-symbols-outlined text-[16px]">upload_file</span> Actualizar despliegue</span>' +
                      '<span class="material-symbols-outlined text-[16px] action-chevron" aria-hidden="true">expand_more</span>' +
                    '</button>' +
                    '<div id="d-redeploy-panel" class="hidden mt-2 rounded-xl border-standard p-3">' +
                      '<div id="d-redeploy-drop" class="zip-dropzone compact" tabindex="0" role="button" aria-label="Seleccionar archivo .zip" aria-describedby="d-redeploy-file-name">' +
                        '<input class="zip-dropzone-input" id="d-redeploy-file" type="file" accept=".zip,application/zip">' +
                        '<span class="material-symbols-outlined zip-dropzone-icon" aria-hidden="true">upload_file</span>' +
                        '<p class="zip-dropzone-title">Arrastra o selecciona un .zip</p>' +
                        '<p id="d-redeploy-file-name" class="zip-dropzone-file">Ningún archivo seleccionado</p>' +
                      '</div>' +
                      '<div class="flex justify-end mt-2"><button id="d-redeploy-apply" class="btn-flat primary" type="button">Actualizar despliegue</button></div>' +
                    '</div>' +
                  '</div>' +
                  '<button id="d-delete-btn" class="btn-flat danger w-full justify-start" type="button"><span class="material-symbols-outlined text-[16px]">delete</span> Eliminar</button>' +
                '</div>' +
              '</div>' +
              '<div class="kv-grid mt-4">' +
                '<div class="kv"><div class="kv-label">ID / Nombre</div><div class="kv-value mono" id="d-id">—</div></div>' +
                '<div class="kv"><div class="kv-label">FQDN</div><div class="kv-value mono" id="d-fqdn">—</div></div>' +
                '<div class="kv"><div class="kv-label">IP</div><div class="kv-value mono" id="d-ip">—</div></div>' +
                '<div class="kv"><div class="kv-label">Estado</div><div class="kv-value" id="d-estado">—</div></div>' +
                '<div class="kv"><div class="kv-label">Enlace</div><div class="kv-value" id="d-enlace">—</div></div>' +
                '<div class="kv"><div class="kv-label">Creado en</div><div class="kv-value mono" id="d-creado">—</div></div>' +
              '</div>' +
            '</div>' +
          '</div>' +
        '</aside>';
      while (d.firstChild) document.body.appendChild(d.firstChild);
    }
    if (!document.getElementById('loadingOverlay')) {
      var l = document.createElement('div');
      l.innerHTML = '<div id="loadingOverlay" class="loading-overlay" role="status" aria-live="polite" aria-atomic="true" aria-hidden="true" aria-busy="false"><div class="bg-surface-container-lowest border-standard rounded-xl p-6 flex items-center gap-4 w-full max-w-md"><span class="material-symbols-outlined text-primary-container loading-spinner" aria-hidden="true">progress_activity</span><div class="min-w-0"><p id="loadingText" class="text-on-surface text-[14px] m-0">Procesando…</p></div></div></div>';
      document.body.appendChild(l.firstElementChild);
    }
  }

  function mount() {
    var host = document.getElementById('instances-panel-mount');
    if (!host || host.dataset.ready) return;
    host.dataset.ready = '1';
    host.innerHTML = panelHTML();
    ensureOverlays();
  }

  function init() {
    mount();
    var instancesList = document.getElementById('instancesList');
    if (!instancesList || instancesList.dataset.bound) return;
    instancesList.dataset.bound = '1';

    var showMsg = (U.showMessage || function () {});
    var showLoad = (U.showLoading || function () {});
    var instancesCount = document.getElementById('instancesCount');
    var instancesBindCount = document.getElementById('instancesBindCount');
    var instancesFilter = document.getElementById('instancesFilter');
    var refreshInstancesBtn = document.getElementById('refreshInstancesBtn');
    var deleteModal = document.getElementById('delete-modal');
    var deleteCancelBtn = document.getElementById('deleteCancelBtn');
    var deleteConfirmBtn = document.getElementById('deleteConfirmBtn');
    var modalQuestion = document.getElementById('modal-question');
    var modalPath = document.getElementById('modal-path');
    var modalConf = document.getElementById('modal-conf');
    var modalNsupdate = document.getElementById('modal-nsupdate');
    var allInstances = [];
    var pendingDelete = null;
    var modalReturnFocus = null;
    var loadError = false;

    function setBusy(el, busy) { if (!el) return; el.disabled = !!busy; el.setAttribute('aria-busy', busy ? 'true' : 'false'); }

    function renderInstances(list) {
      if (!instancesList) return;
      if (!list || !list.length) {
        if (loadError) {
          instancesList.innerHTML = '<div class="p-4 text-center text-[13px] text-on-surface-variant"><strong class="text-on-surface">No se pudieron cargar las instancias.</strong><br><span>Comprueba la conexión con el sistema y pulsa “Actualizar”.</span></div>';
        } else if ((instancesFilter && instancesFilter.value || '').trim()) {
          instancesList.innerHTML = '<div class="p-4 text-center text-[13px] text-on-surface-variant">No hay instancias que coincidan con el filtro.</div>';
        } else {
          instancesList.innerHTML = '<div class="p-4 text-center text-[13px] text-on-surface-variant"><strong class="text-on-surface">Aún no hay instancias.</strong><br><span>Crea una desde “Agregar” y aparecerá aquí con su estado y acceso.</span></div>';
        }
        if (instancesCount) instancesCount.textContent = '0';
        if (instancesBindCount) instancesBindCount.textContent = '0 instancias';
        return;
      }
      if (instancesCount) instancesCount.textContent = String(list.length);
      if (instancesBindCount) instancesBindCount.textContent = list.length + ' instancia' + (list.length === 1 ? '' : 's');
      instancesList.innerHTML = list.map(function (inst) {
        var id = inst.id || inst.nombre || '', nombre = inst.nombre || id, fqdn = inst.fqdn || (nombre + '.cloud.local');
        var enlace = inst.enlace || ('http://' + fqdn), ip = inst.ip || '—', estado = (inst.estado || 'ACTIVA').toUpperCase();
        var ok = estado === 'ACTIVA', dotCls = ok ? 'bg-[#16a34a]' : (estado === 'ERROR' ? 'bg-[#dc2626]' : 'bg-[#f59e0b]');
        var stateColor = ok ? 'text-[#16a34a]' : (estado === 'ERROR' ? 'text-[#ba1a1a]' : 'text-[#b45309]');
        var powerBtn = !ok && (estado === 'DETENIDA' || estado === 'ERROR')
          ? '<button class="inline-flex items-center gap-1.5 px-3 py-1.5 text-white text-[12px] font-semibold rounded-xl" style="background:#059669;border:1px solid #047857;" type="button" data-power-on="' + escA(id) + '" title="Encender instancia"><span class="material-symbols-outlined text-[15px]">power_settings_new</span> Encender</button>'
          : '';
        return '<div class="p-3 hover:bg-surface-bright transition-colors flex flex-col gap-2">' +
          '<div class="flex items-center justify-between"><div class="flex items-center gap-2"><span class="w-2 h-2 rounded-full ' + dotCls + '"></span><span class="text-[13px] font-semibold text-on-surface mono">' + esc(nombre) + '</span></div>' +
          '<span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-xl border-standard bg-surface-container-lowest text-[11px] font-semibold mono ' + stateColor + '">● ' + esc(estado === 'ACTIVA' ? 'RUNNING' : estado) + '</span></div>' +
          '<div class="flex items-center justify-between text-on-surface-variant text-[11px]"><a class="inline-flex items-center gap-1 text-[12px] hover:underline url-service-link truncate mono" href="' + escA(enlace) + '" target="_blank" rel="noopener noreferrer"><span>' + esc(enlace.replace(/^https?:\/\//, '')) + '</span><span class="material-symbols-outlined text-[14px]">open_in_new</span></a><span class="mono text-outline shrink-0">' + esc(ip) + '</span></div>' +
          '<div class="flex justify-end items-center gap-2 pt-1">' +
          powerBtn +
          '<button class="inline-flex items-center gap-1.5 px-3 py-1.5 text-white text-[12px] font-semibold rounded-xl" style="background:rgb(15,118,110);border:1px solid rgb(13,95,88);" type="button" data-detail="' + escA(id) + '" title="Ver detalle de la instancia"><span class="material-symbols-outlined text-[15px]">visibility</span> Ver detalle</button>' +
          '</div></div>';
      }).join('');
      instancesList.querySelectorAll('[data-detail]').forEach(function (btn) {
        btn.addEventListener('click', function () { if (window.openInstanceDrawer) window.openInstanceDrawer(btn.getAttribute('data-detail')); });
      });
      instancesList.querySelectorAll('[data-power-on]').forEach(function (btn) {
        btn.addEventListener('click', async function () {
          var id = btn.getAttribute('data-power-on');
          if (!id) return;
          setBusy(btn, true);
          showLoad(true, 'Encendiendo instancia…');
          try {
            var res = await fetch('/api/instancias/' + encodeURIComponent(id) + '/activar', { method: 'POST', headers: { 'Accept': 'application/json' } });
            var data = await res.json().catch(function () { return {}; });
            if (!res.ok) throw new Error(data.error || ('HTTP ' + res.status));
            showMsg('Instancia encendida.', 'success');
            await fetchInstances();
          } catch (err) {
            showMsg('No se pudo encender la instancia: ' + err.message, 'error');
            setBusy(btn, false);
          } finally {
            showLoad(false);
          }
        });
      });
    }

    function applyFilter() {
      var q = (instancesFilter && instancesFilter.value || '').trim().toLowerCase();
      if (!q) { renderInstances(allInstances); return; }
      renderInstances(allInstances.filter(function (i) { return ((i.nombre||'')+' '+(i.fqdn||'')+' '+(i.ip||'')+' '+(i.id||'')).toLowerCase().indexOf(q)!==-1; }));
    }

    function modalFocusables() {
      if (!deleteModal) return [];
      return Array.prototype.slice.call(deleteModal.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])')).filter(function (el) { return !el.disabled && el.offsetParent !== null; });
    }
    function trapModalFocus(e) {
      if (!deleteModal || deleteModal.classList.contains('hidden') || e.key !== 'Tab') return;
      var nodes = modalFocusables();
      if (!nodes.length) return;
      var first = nodes[0], last = nodes[nodes.length - 1];
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    }
    function openDeleteModal(fqdn, host, id) {
      pendingDelete = { fqdn: fqdn, host: host, id: id || host };
      if (modalQuestion) modalQuestion.textContent = '¿Eliminar instancia ' + (fqdn || host) + '?';
      if (modalPath) modalPath.textContent = '/var/www/vhosts/' + (host || '') + '/';
      if (modalConf) modalConf.textContent = host || '';
      if (modalNsupdate) modalNsupdate.textContent = host || '';
      if (deleteModal) {
        modalReturnFocus = document.activeElement;
        deleteModal.classList.remove('hidden');
        document.body.classList.add('httpaas-modal-open');
        document.addEventListener('keydown', trapModalFocus);
        var focusTarget = deleteCancelBtn || (modalFocusables()[0]);
        if (focusTarget && focusTarget.focus) focusTarget.focus();
      }
    }
    window.openDeleteModal = openDeleteModal;
    function closeDeleteModal() {
      pendingDelete = null;
      document.removeEventListener('keydown', trapModalFocus);
      if (deleteModal) deleteModal.classList.add('hidden');
      if (!document.querySelector('.instance-drawer.open')) document.body.classList.remove('httpaas-modal-open');
      var ret = modalReturnFocus; modalReturnFocus = null;
      if (ret && ret.focus) try { ret.focus(); } catch (e) {}
    }

    async function fetchInstances() {
      setBusy(refreshInstancesBtn, true);
      loadError = false;
      try {
        var res = await fetch('/api/instancias');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        var data = await res.json();
        allInstances = Array.isArray(data.datos) ? data.datos : (Array.isArray(data) ? data : []);
        cachedInstances = allInstances;
        lastFetchAt = Date.now();
      } catch (e) {
        allInstances = cachedInstances || [];
        loadError = !cachedInstances;
      }
      finally { setBusy(refreshInstancesBtn, false); }
      applyFilter();
    }
    window.refreshInstancesPanel = fetchInstances;

    async function checkHostDns() {
      var nsBanner = document.getElementById('nsStoppedWarning');
      var startNsBtn = document.getElementById('startNsBtn');
      var banner = document.getElementById('hostDnsWarning');
      var text = document.getElementById('hostDnsWarningText');
      var configureHostDnsBtn = document.getElementById('configureHostDnsBtn');
      if (!banner) return;
      try {
        var res = await fetch('/api/dns/anfitrion');
        var data = await res.json().catch(function () { return {}; });
        if (nsBanner) {
          if (data && data.ns_activa === false) nsBanner.classList.remove('hidden');
          else nsBanner.classList.add('hidden');
        }
        if (data && data.resuelve) { banner.classList.add('hidden'); if (text) text.textContent = ''; }
        else { if (text) text.textContent = (data && data.mensaje) || 'Configura el DNS del anfitrión.'; banner.classList.remove('hidden'); }
      } catch (err) {
        banner.classList.add('hidden');
        if (nsBanner) nsBanner.classList.add('hidden');
      }
      if (startNsBtn && !startNsBtn.dataset.bound) {
        startNsBtn.dataset.bound = '1';
        startNsBtn.addEventListener('click', async function () {
          setBusy(startNsBtn, true); showLoad(true, 'Encendiendo servidor DNS (ns)…');
          try {
            var res = await fetch('/api/dns/encender', { method: 'POST', headers: { 'Accept': 'application/json' } });
            var data = await res.json().catch(function () { return {}; });
            if (!res.ok) throw new Error(data.error || 'Error al encender ns');
            showMsg('Servidor DNS (ns) encendido.', 'success');
            await checkHostDns();
          } catch (err) { showMsg('Error: ' + err.message, 'error'); }
          finally { setBusy(startNsBtn, false); showLoad(false); }
        });
      }
      if (configureHostDnsBtn && !configureHostDnsBtn.dataset.bound) {
        configureHostDnsBtn.dataset.bound = '1';
        configureHostDnsBtn.addEventListener('click', async function () {
          setBusy(configureHostDnsBtn, true); showLoad(true, 'Configurando DNS…');
          try {
            var res = await fetch('/api/dns/anfitrion/configurar', { method: 'POST' });
            var data = await res.json().catch(function () { return {}; });
            if (!res.ok) throw new Error(data.error || 'Error');
            showMsg('DNS configurado.', 'success'); await checkHostDns();
          } catch (err) { showMsg('Error: ' + err.message, 'error'); }
          finally { setBusy(configureHostDnsBtn, false); showLoad(false); }
        });
      }
    }

    if (instancesFilter) instancesFilter.addEventListener('input', applyFilter);
    if (refreshInstancesBtn) refreshInstancesBtn.addEventListener('click', function(){ fetchInstances(); checkHostDns(); });
    if (deleteCancelBtn) deleteCancelBtn.addEventListener('click', closeDeleteModal);
    if (deleteModal) deleteModal.addEventListener('click', function(e){ if(e.target===deleteModal) closeDeleteModal(); });
    if (deleteConfirmBtn) deleteConfirmBtn.addEventListener('click', async function(){
      var target = pendingDelete; closeDeleteModal(); if (!target) return;
      showLoad(true, 'Eliminando instancia…');
      try {
        var res = await fetch('/api/instancias/' + encodeURIComponent(target.id), { method: 'DELETE' });
        if (!res.ok) { var err = await res.json().catch(function () { return {}; }); throw new Error(err.error || 'Error al eliminar.'); }
        showMsg('Instancia eliminada.', 'success');
        if (window.closeInstanceDrawer) window.closeInstanceDrawer();
        fetchInstances(); checkHostDns();
      } catch (err) { showMsg('Error: ' + err.message, 'error'); }
      finally { showLoad(false); }
    });
    window.addEventListener('keydown', function(e){ if (e.key === 'Escape' && deleteModal && !deleteModal.classList.contains('hidden')) closeDeleteModal(); });
    if (cachedInstances) { allInstances = cachedInstances; applyFilter(); }
    var now = Date.now();
    if (!cachedInstances || (now - lastFetchAt) > 15000) {
      fetchInstances();
      checkHostDns();
    }
  }

  window.HTTPaaSInstancesPanel = { mount: mount, ensureOverlays: ensureOverlays, init: init };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init); else init();
})();
