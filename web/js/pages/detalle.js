// pages/detalle.js - Drawer lateral de detalle y acciones de una instancia.
(function () {
  var U = window.HTTPaaS || {};
  var esc = U.escapeHtml || function (s) { return s == null ? '' : String(s); };
  var escA = U.escapeAttr || esc;
  var fmt = U.formatFecha || function (s) { return s || '—'; };
  var bound = false;
  var lastFocus = null;
  var current = null;
  var sshLoaded = false;
  var actionsBusy = false;

  function $(id) { return document.getElementById(id); }
  function set(id, v) { var el = $(id); if (el) el.textContent = (v == null || v === '') ? '—' : v; }
  function actionMsg(text, kind) {
    var el = $('d-actions-msg');
    if (!el) return;
    el.textContent = text || '';
    el.className = 'text-[12px] ' + (kind === 'error' ? 'text-error' : kind === 'success' ? 'text-[#059669]' : 'text-on-surface-variant');
  }
  function setActionBusy(busy) {
    actionsBusy = !!busy;
    updateActions(current);
  }
  function updateActions(inst) {
    var active = !!(inst && String(inst.estado || '').toUpperCase() === 'ACTIVA' && inst.ip && inst.ip !== '—');
    ['d-restart-btn', 'd-ssh-btn', 'd-redeploy-btn', 'd-redeploy-apply'].forEach(function (id) {
      var el = $(id);
      if (!el) return;
      el.disabled = actionsBusy || !active;
      el.setAttribute('aria-disabled', String(actionsBusy || !active));
    });
    var del = $('d-delete-btn');
    if (del) {
      del.disabled = actionsBusy;
      del.setAttribute('aria-disabled', String(actionsBusy));
    }
  }

  function render(inst) {
    if (!inst) return;
    current = inst;
    sshLoaded = false;
    setAccordion('ssh', false);
    setAccordion('redeploy', false);
    setRedeployFile(null);
    actionMsg('', '');

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
    updateActions(inst);
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

  async function fetchJSON(url, options) {
    var res = await fetch(url, options);
    var data = await res.json().catch(function () { return {}; });
    if (!res.ok) throw new Error(data.error || ('HTTP ' + res.status));
    return data;
  }

  async function refreshCurrent() {
    if (!current) return;
    try {
      var data = await fetchJSON('/api/instancias/' + encodeURIComponent(current.id), { headers: { 'Accept': 'application/json' } });
      render(data.datos || data);
    } catch (e) { /* mantiene el detalle actual */ }
  }

  async function restartServices() {
    if (!current || actionsBusy) return;
    if (!window.confirm('¿Reiniciar los servicios web de “' + (current.nombre || current.id) + '”? El sitio quedará disponible de nuevo al terminar.')) return;
    setActionBusy(true);
    actionMsg('Reiniciando servicios…', 'info');
    try {
      await fetchJSON('/api/instancias/' + encodeURIComponent(current.id) + '/servicios/reiniciar', { method: 'POST', headers: { 'Accept': 'application/json' } });
      await refreshCurrent();
      actionMsg('Servicios reiniciados correctamente.', 'success');
    } catch (e) {
      actionMsg('No se pudieron reiniciar los servicios: ' + e.message, 'error');
    } finally {
      setActionBusy(false);
    }
  }

  function setAccordion(which, open) {
    var sshOpen = which === 'ssh' ? open : false;
    var redeployOpen = which === 'redeploy' ? open : false;
    var sshPanel = $('d-ssh-panel'), redeployPanel = $('d-redeploy-panel');
    if (sshPanel) sshPanel.classList.toggle('hidden', !sshOpen);
    if (redeployPanel) redeployPanel.classList.toggle('hidden', !redeployOpen);
    var sshBtn = $('d-ssh-btn'), redeployBtn = $('d-redeploy-btn');
    if (sshBtn) sshBtn.setAttribute('aria-expanded', String(sshOpen));
    if (redeployBtn) redeployBtn.setAttribute('aria-expanded', String(redeployOpen));
  }

  async function toggleSSH() {
    var panel = $('d-ssh-panel');
    if (!panel || !current) return;
    if (!panel.classList.contains('hidden')) { setAccordion('ssh', false); return; }
    setAccordion('ssh', true);
    if (sshLoaded) return;
    actionMsg('Cargando acceso SSH…', 'info');
    try {
      var data = await fetchJSON('/api/instancias/' + encodeURIComponent(current.id) + '/acceso', { headers: { 'Accept': 'application/json' } });
      var access = data.datos || data;
      set('d-ssh-command', access.comando || '');
      set('d-ssh-note', access.nota_clave || '');
      sshLoaded = true;
      actionMsg('', '');
    } catch (e) {
      setAccordion('ssh', false);
      actionMsg('No se pudo obtener el acceso SSH: ' + e.message, 'error');
    }
  }

  function copySSH() {
    var el = $('d-ssh-command');
    var text = el ? el.textContent : '';
    if (!text) return;
    var done = function () { actionMsg('Comando SSH copiado.', 'success'); };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done).catch(function () { fallbackCopy(text); done(); });
    } else {
      fallbackCopy(text);
      done();
    }
  }

  function fallbackCopy(text) {
    var area = document.createElement('textarea');
    area.value = text;
    area.style.position = 'fixed';
    area.style.opacity = '0';
    document.body.appendChild(area);
    area.select();
    try { document.execCommand('copy'); } catch (e) { /* sin portapapeles */ }
    document.body.removeChild(area);
  }

  function setRedeployFile(file) {
    var drop = $('d-redeploy-drop');
    var name = $('d-redeploy-file-name');
    if (drop) {
      drop.classList.toggle('has-file', !!file);
      drop.classList.remove('has-error');
    }
    if (name) name.textContent = file ? file.name : 'Ningún archivo seleccionado';
  }

  function acceptRedeployFile(file) {
    var input = $('d-redeploy-file');
    if (!file) {
      if (input) input.value = '';
      setRedeployFile(null);
      return;
    }
    if (!/\.zip$/i.test(file.name || '')) {
      if (input) input.value = '';
      setRedeployFile(null);
      var drop = $('d-redeploy-drop');
      if (drop) drop.classList.add('has-error');
      var name = $('d-redeploy-file-name');
      if (name) name.textContent = 'El archivo debe ser .zip';
      actionMsg('El archivo debe tener extensión .zip.', 'error');
      return;
    }
    try {
      if (input) {
        var dt = new DataTransfer();
        dt.items.add(file);
        input.files = dt.files;
      }
    } catch (e) { /* el input conserva la selección nativa */ }
    setRedeployFile(file);
    actionMsg('', '');
  }

  function toggleRedeploy() {
    var panel = $('d-redeploy-panel');
    if (panel) setAccordion('redeploy', panel.classList.contains('hidden'));
  }

  async function applyRedeploy() {
    if (!current || actionsBusy) return;
    var input = $('d-redeploy-file');
    var file = input && input.files && input.files[0];
    if (!file) { actionMsg('Selecciona un archivo .zip.', 'error'); return; }
    if (!/\.zip$/i.test(file.name || '')) { actionMsg('El archivo debe tener extensión .zip.', 'error'); return; }
    if (!window.confirm('¿Reemplazar el contenido publicado de “' + (current.nombre || current.id) + '” con el nuevo .zip?')) return;

    setActionBusy(true);
    actionMsg('Actualizando despliegue…', 'info');
    var fd = new FormData();
    fd.append('archivo', file);
    try {
      await fetchJSON('/api/instancias/' + encodeURIComponent(current.id) + '/despliegue', { method: 'POST', body: fd });
      if (input) input.value = '';
      setRedeployFile(null);
      setAccordion('redeploy', false);
      await refreshCurrent();
      actionMsg('Despliegue actualizado correctamente.', 'success');
      if (window.refreshInstancesPanel) window.refreshInstancesPanel();
    } catch (e) {
      actionMsg('No se pudo actualizar el despliegue: ' + e.message, 'error');
    } finally {
      setActionBusy(false);
    }
  }

  function deleteCurrent() {
    if (!current) return;
    if (window.openDeleteModal) {
      window.openDeleteModal(current.fqdn || (current.nombre + '.cloud.local'), current.nombre || current.id, current.id || current.nombre);
    }
  }

  async function openInstanceDrawer(id) {
    if (!id) return;
    bind();
    openDrawer();
    showState('loading');
    try {
      var data = await fetchJSON('/api/instancias/' + encodeURIComponent(id), { headers: { 'Accept': 'application/json' } });
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
    var restart = $('d-restart-btn');
    if (restart) restart.addEventListener('click', restartServices);
    var ssh = $('d-ssh-btn');
    if (ssh) ssh.addEventListener('click', toggleSSH);
    var copy = $('d-ssh-copy');
    if (copy) copy.addEventListener('click', copySSH);
    var redeploy = $('d-redeploy-btn');
    if (redeploy) redeploy.addEventListener('click', toggleRedeploy);
    var apply = $('d-redeploy-apply');
    if (apply) apply.addEventListener('click', applyRedeploy);
    var drop = $('d-redeploy-drop');
    var fileInput = $('d-redeploy-file');
    if (drop && fileInput) {
      drop.addEventListener('click', function (e) { if (e.target !== fileInput) fileInput.click(); });
      drop.addEventListener('keydown', function (e) {
        if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); fileInput.click(); }
      });
      fileInput.addEventListener('change', function () { acceptRedeployFile(fileInput.files && fileInput.files[0]); });
      ['dragenter', 'dragover'].forEach(function (ev) {
        drop.addEventListener(ev, function (e) { e.preventDefault(); e.stopPropagation(); drop.classList.add('is-dragover'); });
      });
      ['dragleave', 'dragend'].forEach(function (ev) {
        drop.addEventListener(ev, function (e) { e.preventDefault(); e.stopPropagation(); drop.classList.remove('is-dragover'); });
      });
      drop.addEventListener('drop', function (e) {
        e.preventDefault();
        e.stopPropagation();
        drop.classList.remove('is-dragover');
        acceptRedeployFile(e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0]);
      });
    }
    var del = $('d-delete-btn');
    if (del) del.addEventListener('click', deleteCurrent);
  }

  function openFromHash() {
    var m = (location.hash || '').match(/[#&?]detalle=([^&]+)/);
    if (m && m[1]) {
      try { openInstanceDrawer(decodeURIComponent(m[1])); } catch (e) { openInstanceDrawer(m[1]); }
    }
  }

  window.openInstanceDrawer = openInstanceDrawer;
  window.closeInstanceDrawer = closeDrawer;
  window.HTTPaaSPages = window.HTTPaaSPages || {};
  window.HTTPaaSPages.detalle = function () { bind(); openFromHash(); };
  window.addEventListener('hashchange', openFromHash);
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', openFromHash); else openFromHash();
})();
