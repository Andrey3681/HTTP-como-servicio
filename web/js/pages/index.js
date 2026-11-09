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

    if (checkHostBtn) checkHostBtn.addEventListener('click', async function () {
      var hostname = (hostnameInput && hostnameInput.value || '').trim().toLowerCase();
      if (!hostname) { showMsg('Por favor ingrese un nombre de host.', 'error'); return; }
      if (!/^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/.test(hostname)) { showMsg('El nombre debe tener 3–32 caracteres (minúsculas, números y guiones).', 'error'); return; }
      try {
        var res = await fetch('/api/instancias/' + encodeURIComponent(hostname));
        if (res.status === 404) showMsg('El nombre de host "' + hostname + '" está disponible.', 'success');
        else if (res.ok) showMsg('El nombre de host "' + hostname + '" ya está en uso.', 'error');
        else showMsg('No se pudo verificar la disponibilidad.', 'error');
      } catch (e) { showMsg('Backend no accesible. El formato de "' + hostname + '" es válido.', 'info'); }
    });

    provisionForm.addEventListener('submit', async function (e) {
      e.preventDefault();
      var hostname = (hostnameInput && hostnameInput.value || '').trim().toLowerCase();
      var file = zipInput && zipInput.files && zipInput.files[0];
      if (!hostname) { showMsg('Ingrese el nombre del host.', 'error'); return; }
      if (!file) { showMsg('Seleccione un archivo comprimido .zip.', 'error'); return; }
      if (!file.name.toLowerCase().endsWith('.zip')) { showMsg('El archivo debe tener extensión .zip.', 'error'); return; }
      var fd = new FormData(); fd.append('nombre', hostname); fd.append('archivo', file);
      showLoad(true, 'Aprovisionando VM "' + hostname + '" y registrando DNS…'); hideMsg();
      try {
        var res = await fetch('/api/instancias', { method: 'POST', body: fd });
        var result = await res.json().catch(function () { return {}; });
        if (!res.ok) throw new Error(result.error || 'Error al aprovisionar la instancia.');
        showMsg('Instancia "' + hostname + '" aprovisionada con éxito.', 'success');
        provisionForm.reset(); fetchInstances();
      } catch (err) { showMsg('Error en el aprovisionamiento: ' + err.message, 'error'); }
      finally { showLoad(false); }
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
    async function fetchInstances() {
      if (!instancesList) return;
      try { var res = await fetch('/api/instancias'); if (!res.ok) throw new Error(); var data = await res.json(); allInstances = Array.isArray(data.datos) ? data.datos : (Array.isArray(data)?data:[]); } catch(e){ allInstances=[]; }
      applyFilter();
    }
    function openDeleteModal(fqdn, host, id) {
      pendingDelete = { fqdn: fqdn, host: host, id: id || host };
      if (modalQuestion) modalQuestion.textContent = '¿Eliminar instancia ' + (fqdn || host) + '?';
      if (modalPath) modalPath.textContent = '/var/www/vhosts/' + (host || '') + '/';
      if (modalConf) modalConf.textContent = host || ''; if (modalNsupdate) modalNsupdate.textContent = host || '';
      if (deleteModal) deleteModal.classList.remove('hidden');
    }
    window.openDeleteModal = openDeleteModal; window.deleteInstance = function(id){ openDeleteModal((id||'')+'.cloud.local', id, id); };
    function closeDeleteModal(){ pendingDelete=null; if(deleteModal) deleteModal.classList.add('hidden'); }
    if (deleteCancelBtn) deleteCancelBtn.addEventListener('click', closeDeleteModal);
    if (deleteModal) deleteModal.addEventListener('click', function(e){ if(e.target===deleteModal) closeDeleteModal(); });
    window.addEventListener('keydown', function(e){ if(e.key==='Escape') closeDeleteModal(); });
    if (deleteConfirmBtn) deleteConfirmBtn.addEventListener('click', async function(){
      var target=pendingDelete; closeDeleteModal(); if(!target) return;
      showLoad(true, 'Destruyendo VM "'+target.host+'" y liberando DNS…');
      try { var res=await fetch('/api/instancias/'+encodeURIComponent(target.id),{method:'DELETE'}); if(!res.ok){ var err=await res.json().catch(function(){return {};}); throw new Error(err.error||'Error al eliminar.'); } showMsg('Instancia "'+target.host+'" eliminada exitosamente.','success'); fetchInstances(); }
      catch(err){ showMsg('Error al eliminar: '+err.message,'error'); } finally{ showLoad(false); }
    });
    if (refreshInstancesBtn) refreshInstancesBtn.addEventListener('click', fetchInstances);
    fetchInstances();
  }
  window.HTTPaaSPages = window.HTTPaaSPages || {};
  window.HTTPaaSPages.index = init;
})();
