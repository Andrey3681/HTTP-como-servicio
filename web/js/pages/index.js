// pages/index.js - Lógica de la página principal (Instancias).
(function () {
  function init() {
    var U = window.HTTPaaS || {};
    var esc = U.escapeHtml || function (s) { return String(s == null ? '' : s); };
    var escA = U.escapeAttr || esc;
    var showMsg = U.showMessage || function () {};
    var hideMsg = U.hideMessage || function () {};
    var showLoad = U.showLoading || function () {};

    var provisionForm = document.getElementById('provisionForm');
    if (!provisionForm || provisionForm.dataset.bound) return; // no estamos en index o ya enlazado
    provisionForm.dataset.bound = '1';

    var hostnameInput = document.getElementById('hostnameInput');
    var zipInput = document.getElementById('zipInput');
    var zipDropZone = document.getElementById('zipDropZone');
    var zipFileName = document.getElementById('zipFileName');
    var checkHostBtn = document.getElementById('checkHostBtn');
    var instancesList = document.getElementById('instancesList');
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

    function setBusy(el, busy) {
      if (!el) return;
      el.disabled = !!busy;
      el.setAttribute('aria-busy', busy ? 'true' : 'false');
    }

    function setZipUI(file, errText) {
      if (!zipDropZone || !zipFileName) return;
      zipDropZone.classList.toggle('has-file', !!file && !errText);
      zipDropZone.classList.toggle('has-error', !!errText);
      zipFileName.textContent = errText ? errText : (file ? file.name : 'Ningún archivo seleccionado');
    }
    function applyZipFile(file) {
      if (!zipInput) return;
      if (!file) { setZipUI(null); return; }
      var ok = /\.zip$/i.test(file.name || '');
      if (!ok) {
        zipInput.value = '';
        setZipUI(null, 'El archivo debe ser .zip');
        showMsg('El archivo debe tener extensión .zip.', 'error');
        return;
      }
      try {
        var dt = new DataTransfer();
        dt.items.add(file);
        zipInput.files = dt.files;
      } catch (e) { /* el navegador no permite asignar; el input ya puede tenerlo si vino por change */ }
      setZipUI(file);
    }
    if (zipDropZone && zipInput && !zipDropZone.dataset.bound) {
      zipDropZone.dataset.bound = '1';
      zipDropZone.addEventListener('click', function (e) { if (e.target !== zipInput) zipInput.click(); });
      zipDropZone.addEventListener('keydown', function (e) {
        if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); zipInput.click(); }
      });
      zipInput.addEventListener('change', function () {
        var f = zipInput.files && zipInput.files[0];
        if (f) applyZipFile(f); else setZipUI(null);
      });
      ['dragenter', 'dragover'].forEach(function (ev) {
        zipDropZone.addEventListener(ev, function (e) { e.preventDefault(); e.stopPropagation(); zipDropZone.classList.add('is-dragover'); });
      });
      ['dragleave', 'dragend'].forEach(function (ev) {
        zipDropZone.addEventListener(ev, function (e) { e.preventDefault(); e.stopPropagation(); zipDropZone.classList.remove('is-dragover'); });
      });
      zipDropZone.addEventListener('drop', function (e) {
        e.preventDefault(); e.stopPropagation(); zipDropZone.classList.remove('is-dragover');
        var f = e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0];
        if (f) applyZipFile(f);
      });
    }
    provisionForm.addEventListener('reset', function () { setZipUI(null); });

    if (checkHostBtn) checkHostBtn.addEventListener('click', async function () {
      var hostname = (hostnameInput && hostnameInput.value || '').trim().toLowerCase();
      if (!hostname) { showMsg('Por favor ingrese un nombre de host.', 'error'); return; }
      if (!/^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/.test(hostname)) { showMsg('El nombre debe tener 3–32 caracteres (minúsculas, números y guiones).', 'error'); return; }
      setBusy(checkHostBtn, true);
      try {
        var res = await fetch('/api/instancias/' + encodeURIComponent(hostname));
        if (res.status === 404) showMsg('El nombre de host "' + hostname + '" está disponible.', 'success');
        else if (res.ok) showMsg('El nombre de host "' + hostname + '" ya está en uso.', 'error');
        else showMsg('No se pudo verificar la disponibilidad.', 'error');
      } catch (e) { showMsg('Backend no accesible. El formato de "' + hostname + '" es válido.', 'info'); }
      finally { setBusy(checkHostBtn, false); }
    });

    var submitBtn = provisionForm.querySelector('button[type="submit"]');
    provisionForm.addEventListener('submit', async function (e) {
      e.preventDefault();
      var hostname = (hostnameInput && hostnameInput.value || '').trim().toLowerCase();
      var file = zipInput && zipInput.files && zipInput.files[0];
      if (!hostname) { showMsg('Ingrese el nombre del host.', 'error'); return; }
      if (!file) { showMsg('Seleccione un archivo comprimido .zip.', 'error'); return; }
      if (!file.name.toLowerCase().endsWith('.zip')) { showMsg('El archivo debe tener extensión .zip.', 'error'); return; }
      var fd = new FormData(); fd.append('nombre', hostname); fd.append('archivo', file);
      showLoad(true, 'Aprovisionando VM "' + hostname + '" y registrando DNS…'); hideMsg();
      setBusy(submitBtn, true); setBusy(checkHostBtn, true);
      try {
        var res = await fetch('/api/instancias', { method: 'POST', body: fd });
        var result = await res.json().catch(function () { return {}; });
        if (!res.ok) throw new Error(result.error || 'Error al aprovisionar la instancia.');
        showMsg('Instancia "' + hostname + '" aprovisionada con éxito.', 'success');
        provisionForm.reset(); fetchInstances(); checkHostDns();
      } catch (err) { showMsg('Error en el aprovisionamiento: ' + err.message, 'error'); }
      finally { setBusy(submitBtn, false); setBusy(checkHostBtn, false); showLoad(false); }
    });

    function renderInstances(list) {
      if (!instancesList) return;
      if (!list || !list.length) {
        instancesList.innerHTML = '<div class="p-4 text-center text-[13px] text-on-surface-variant">No hay instancias aprovisionadas en este momento.</div>';
        if (instancesCount) instancesCount.textContent = '0'; if (instancesBindCount) instancesBindCount.textContent = 'Bind9: 0 hosts'; return;
      }
      if (instancesCount) instancesCount.textContent = String(list.length);
      if (instancesBindCount) instancesBindCount.textContent = 'Bind9: ' + list.length + ' hosts';
      instancesList.innerHTML = list.map(function (inst) {
        var id = inst.id || inst.nombre || '', nombre = inst.nombre || id, fqdn = inst.fqdn || (nombre + '.cloud.local');
        var enlace = inst.enlace || ('http://' + fqdn), ip = inst.ip || '192.168.10.x', estado = (inst.estado || 'ACTIVA').toUpperCase();
        var ok = estado === 'ACTIVA', dotCls = ok ? 'bg-[#16a34a]' : (estado === 'ERROR' ? 'bg-[#dc2626]' : 'bg-[#f59e0b]');
        var stateColor = ok ? 'text-[#16a34a]' : (estado === 'ERROR' ? 'text-[#ba1a1a]' : 'text-[#b45309]');
        return '<div class="p-3 hover:bg-surface-bright transition-colors flex flex-col gap-2">' +
          '<div class="flex items-center justify-between"><div class="flex items-center gap-2"><span class="w-2 h-2 rounded-full ' + dotCls + '"></span><span class="text-[13px] font-semibold text-on-surface mono">' + esc(nombre) + '</span></div>' +
          '<span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-xl border-standard bg-surface-container-lowest text-[11px] font-semibold mono ' + stateColor + '">● ' + esc(estado === 'ACTIVA' ? 'RUNNING' : estado) + '</span></div>' +
          '<div class="flex items-center justify-between text-on-surface-variant text-[11px]"><a class="inline-flex items-center gap-1 text-[12px] hover:underline url-service-link truncate mono" href="' + escA(enlace) + '" target="_blank" rel="noopener noreferrer"><span>' + esc(enlace.replace(/^https?:\/\//, '')) + '</span><span class="material-symbols-outlined text-[14px]">open_in_new</span></a><span class="mono text-outline shrink-0">' + esc(ip) + '</span></div>' +
          '<div class="flex justify-between items-center pt-1"><a class="text-[11px] text-primary-container hover:underline mono" href="/detalle.html?id=' + encodeURIComponent(id) + '">Ver detalle</a>' +
          '<button class="inline-flex items-center gap-1 px-2.5 py-1 bg-secondary-container hover:bg-[#b91c1c] text-white text-xs rounded-xl font-medium" type="button" data-del="' + escA(id) + '" data-fqdn="' + escA(fqdn) + '" data-host="' + escA(nombre) + '"><span class="material-symbols-outlined text-[14px]">delete</span> Eliminar</button></div></div>';
      }).join('');
      instancesList.querySelectorAll('[data-del]').forEach(function (btn) {
        btn.addEventListener('click', function () { openDeleteModal(btn.getAttribute('data-fqdn'), btn.getAttribute('data-host'), btn.getAttribute('data-del')); });
      });
    }
    function applyFilter() {
      var q = (instancesFilter && instancesFilter.value || '').trim().toLowerCase();
      if (!q) { renderInstances(allInstances); return; }
      renderInstances(allInstances.filter(function (i) { return ((i.nombre||'')+' '+(i.fqdn||'')+' '+(i.ip||'')+' '+(i.id||'')).toLowerCase().indexOf(q)!==-1; }));
    }
    if (instancesFilter) instancesFilter.addEventListener('input', applyFilter);
    async function checkHostDns() {
      var banner = document.getElementById('hostDnsWarning');
      var text = document.getElementById('hostDnsWarningText');
      if (!banner) return;
      try {
        var res = await fetch('/api/dns/anfitrion');
        var data = await res.json().catch(function () { return {}; });
        if (data && data.resuelve) {
          banner.classList.add('hidden');
          if (text) text.textContent = '';
        } else {
          if (text) text.textContent = (data && data.mensaje) || 'El resolver del anfitrión no resuelve la zona cloud.local.';
          banner.classList.remove('hidden');
        }
      } catch (err) {
        banner.classList.add('hidden');
      }
    }

    var configureHostDnsBtn = document.getElementById('configureHostDnsBtn');
    if (configureHostDnsBtn) configureHostDnsBtn.addEventListener('click', async function () {
      setBusy(configureHostDnsBtn, true);
      hideMsg();
      showLoad(true, 'Configurando el DNS del anfitrión…');
      try {
        var res = await fetch('/api/dns/anfitrion/configurar', { method: 'POST' });
        var data = await res.json().catch(function () { return {}; });
        if (!res.ok) throw new Error(data.error || 'No se pudo configurar el DNS del anfitrión.');
        showMsg('DNS del anfitrión configurado. Ya puedes abrir http://<host>.cloud.local.', 'success');
        await checkHostDns();
      } catch (err) {
        showMsg('No se pudo configurar el DNS: ' + err.message, 'error');
      } finally {
        setBusy(configureHostDnsBtn, false);
        showLoad(false);
      }
    });

    async function fetchInstances() {
      if (!instancesList) return;
      setBusy(refreshInstancesBtn, true);
      try {
        var res = await fetch('/api/instancias');
        if (!res.ok) throw new Error('estado ' + res.status);
        var data = await res.json();
        allInstances = Array.isArray(data.datos) ? data.datos : (Array.isArray(data) ? data : []);
      } catch (e) {
        allInstances = [];
        showMsg('No se pudieron cargar las instancias (backend inaccesible o error).', 'error');
      } finally { setBusy(refreshInstancesBtn, false); }
      applyFilter();
    }

    function modalFocusables() {
      if (!deleteModal) return [];
      return Array.prototype.slice.call(deleteModal.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'))
        .filter(function (el) { return !el.disabled && el.offsetParent !== null; });
    }
    function openDeleteModal(fqdn, host, id) {
      pendingDelete = { fqdn: fqdn, host: host, id: id || host };
      if (modalQuestion) modalQuestion.textContent = '¿Eliminar instancia ' + (fqdn || host) + '?';
      if (modalPath) modalPath.textContent = '/var/www/vhosts/' + (host || '') + '/';
      if (modalConf) modalConf.textContent = host || ''; if (modalNsupdate) modalNsupdate.textContent = host || '';
      if (deleteModal) {
        modalReturnFocus = document.activeElement;
        deleteModal.classList.remove('hidden');
        var panel = document.getElementById('delete-modal-panel');
        var focusTarget = deleteCancelBtn || (modalFocusables()[0]) || panel;
        if (focusTarget && focusTarget.focus) focusTarget.focus();
      }
    }
    window.openDeleteModal = openDeleteModal; window.deleteInstance = function(id){ openDeleteModal((id||'')+'.cloud.local', id, id); };
    function closeDeleteModal() {
      pendingDelete = null;
      if (deleteModal) deleteModal.classList.add('hidden');
      var ret = modalReturnFocus; modalReturnFocus = null;
      if (ret && ret.focus) try { ret.focus(); } catch (e) {}
    }
    if (deleteCancelBtn) deleteCancelBtn.addEventListener('click', closeDeleteModal);
    if (deleteModal) deleteModal.addEventListener('click', function(e){ if(e.target===deleteModal) closeDeleteModal(); });
    if (deleteModal) deleteModal.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { e.preventDefault(); closeDeleteModal(); return; }
      if (e.key !== 'Tab') return;
      var f = modalFocusables(); if (!f.length) return;
      var first = f[0], last = f[f.length - 1];
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    });
    window.addEventListener('keydown', function(e){ if (e.key === 'Escape' && deleteModal && !deleteModal.classList.contains('hidden')) closeDeleteModal(); });
    if (deleteConfirmBtn) deleteConfirmBtn.addEventListener('click', async function(){
      var target = pendingDelete; closeDeleteModal(); if (!target) return;
      showLoad(true, 'Destruyendo VM "' + target.host + '" y liberando DNS…');
      setBusy(deleteConfirmBtn, true);
      try {
        var res = await fetch('/api/instancias/' + encodeURIComponent(target.id), { method: 'DELETE' });
        if (!res.ok) { var err = await res.json().catch(function () { return {}; }); throw new Error(err.error || 'Error al eliminar.'); }
        showMsg('Instancia "' + target.host + '" eliminada exitosamente.', 'success');
        if (window.refreshHeaderHealth) window.refreshHeaderHealth();
        fetchInstances();
      } catch (err) { showMsg('Error al eliminar: ' + err.message, 'error'); }
      finally { setBusy(deleteConfirmBtn, false); showLoad(false); }
    });
    if (refreshInstancesBtn) refreshInstancesBtn.addEventListener('click', fetchInstances);
    fetchInstances();
    checkHostDns();
  }
  window.HTTPaaSPages = window.HTTPaaSPages || {};
  window.HTTPaaSPages.index = init;
})();
