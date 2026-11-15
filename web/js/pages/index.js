// pages/index.js - Formulario de aprovisionamiento (Agregar) y arranque del panel compartido.
(function () {
  function init() {
    var U = window.HTTPaaS || {};
    var showMsg = U.showMessage || function () {};
    var hideMsg = U.hideMessage || function () {};
    var showLoad = U.showLoading || function () {};

    if (window.HTTPaaSInstancesPanel && window.HTTPaaSInstancesPanel.init) {
      window.HTTPaaSInstancesPanel.init();
    }

    var provisionForm = document.getElementById('provisionForm');
    if (!provisionForm || provisionForm.dataset.bound) return;
    provisionForm.dataset.bound = '1';

    function setBusy(el, busy) { if (!el) return; el.disabled = !!busy; el.setAttribute('aria-busy', busy ? 'true' : 'false'); }

    var hostnameInput = document.getElementById('hostnameInput');
    var zipInput = document.getElementById('zipInput');
    var zipDropZone = document.getElementById('zipDropZone');
    var zipFileName = document.getElementById('zipFileName');
    var checkHostBtn = document.getElementById('checkHostBtn');

    function setZipUI(file, errText) {
      if (!zipDropZone || !zipFileName) return;
      zipDropZone.classList.toggle('has-file', !!file && !errText);
      zipDropZone.classList.toggle('has-error', !!errText);
      zipFileName.textContent = errText ? errText : (file ? file.name : 'Ningún archivo seleccionado');
    }
    function applyZipFile(file) {
      if (!zipInput) return;
      if (!file) { setZipUI(null); return; }
      if (!/\.zip$/i.test(file.name || '')) { zipInput.value = ''; setZipUI(null, 'El archivo debe ser .zip'); showMsg('El archivo debe ser .zip.', 'error'); return; }
      try { var dt = new DataTransfer(); dt.items.add(file); zipInput.files = dt.files; } catch (e) {}
      setZipUI(file);
    }
    if (zipDropZone && zipInput) {
      zipDropZone.addEventListener('click', function (e) { if (e.target !== zipInput) zipInput.click(); });
      zipDropZone.addEventListener('keydown', function (e) { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); zipInput.click(); } });
      zipInput.addEventListener('change', function () { var f = zipInput.files && zipInput.files[0]; if (f) applyZipFile(f); else setZipUI(null); });
      ['dragenter','dragover'].forEach(function (ev) { zipDropZone.addEventListener(ev, function (e) { e.preventDefault(); e.stopPropagation(); zipDropZone.classList.add('is-dragover'); }); });
      ['dragleave','dragend'].forEach(function (ev) { zipDropZone.addEventListener(ev, function (e) { e.preventDefault(); e.stopPropagation(); zipDropZone.classList.remove('is-dragover'); }); });
      zipDropZone.addEventListener('drop', function (e) { e.preventDefault(); e.stopPropagation(); zipDropZone.classList.remove('is-dragover'); var f = e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0]; if (f) applyZipFile(f); });
    }
    provisionForm.addEventListener('reset', function () { setZipUI(null); });

    if (checkHostBtn) checkHostBtn.addEventListener('click', async function () {
      var hostname = (hostnameInput && hostnameInput.value || '').trim().toLowerCase();
      if (!hostname) { showMsg('Ingresa un nombre para verificar si está disponible.', 'error'); return; }
      if (!/^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/.test(hostname)) { showMsg('El nombre no es válido. Usa 3–32 caracteres en minúsculas, números o guiones (sin empezar ni terminar con guion).', 'error'); return; }
      setBusy(checkHostBtn, true);
      showMsg('Verificando disponibilidad…', 'info');
      try {
        var res = await fetch('/api/instancias/' + encodeURIComponent(hostname));
        if (res.status === 404) showMsg('“' + hostname + '” está disponible. Puedes aprovisionarlo.', 'success');
        else if (res.ok) showMsg('“' + hostname + '” ya está en uso. Elige otro nombre.', 'error');
        else showMsg('No se pudo verificar la disponibilidad ahora. Puedes intentarlo de nuevo o continuar si estás seguro.', 'error');
      } catch (e) { showMsg('No se pudo verificar la disponibilidad (sin conexión con el sistema). Intenta nuevamente.', 'error'); }
      finally { setBusy(checkHostBtn, false); }
    });

    var submitBtn = provisionForm.querySelector('button[type="submit"]');
    provisionForm.addEventListener('submit', async function (e) {
      e.preventDefault();
      var hostname = (hostnameInput && hostnameInput.value || '').trim().toLowerCase();
      var file = zipInput && zipInput.files && zipInput.files[0];
      if (!hostname) { showMsg('Ingresa un nombre para la instancia.', 'error'); return; }
      if (!/^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/.test(hostname)) { showMsg('El nombre no es válido. Usa 3–32 caracteres en minúsculas, números o guiones (sin empezar ni terminar con guion).', 'error'); return; }
      if (!file) { showMsg('Selecciona un archivo .zip con el contenido de tu sitio.', 'error'); return; }
      var fd = new FormData(); fd.append('nombre', hostname); fd.append('archivo', file);
      showLoad(true, 'Aprovisionando "' + hostname + '"… Esto puede tardar varios minutos.'); hideMsg();
      setBusy(submitBtn, true); setBusy(checkHostBtn, true);
      try {
        var res = await fetch('/api/instancias', { method: 'POST', body: fd });
        var result = await res.json().catch(function () { return {}; });
        if (!res.ok) throw new Error(result.error || 'No se pudo completar el aprovisionamiento.');
        showMsg('Instancia "' + hostname + '" creada. Cuando esté activa, podrás abrirla desde el listado o en http://' + hostname + '.cloud.local.', 'success');
        provisionForm.reset();
        if (window.refreshInstancesPanel) window.refreshInstancesPanel();
      } catch (err) {
        showMsg('No se pudo aprovisionar: ' + err.message + '. Revisa el nombre, el archivo .zip y el estado del sistema en "Salud", y vuelve a intentarlo.', 'error');
      }
      finally { setBusy(submitBtn, false); setBusy(checkHostBtn, false); showLoad(false); }
    });
  }
  window.HTTPaaSPages = window.HTTPaaSPages || {}; window.HTTPaaSPages.index = init;
})();
