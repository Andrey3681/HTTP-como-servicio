// pages/dns.js
(function () {
  function init() {
    var U = window.HTTPaaS || {}, esc = U.escapeHtml || function (s) { return s == null ? '' : String(s); }, escA = U.escapeAttr || esc;
    var tbody = document.getElementById('dnsTableBody'), zone = document.getElementById('dnsZone'), btn = document.getElementById('refreshDnsBtn'), message = document.getElementById('dnsMessage');
    if (!tbody || tbody.dataset.bound) return;
    tbody.dataset.bound = '1';

    var renameModal = document.getElementById('dns-rename-modal');
    var renameInput = document.getElementById('dns-rename-input');
    var renameError = document.getElementById('dns-rename-error');
    var renameCurrent = document.getElementById('dns-rename-current');
    var renameConfirm = document.getElementById('dns-rename-confirm');
    var renameCancel = document.getElementById('dns-rename-cancel');
    var renameOld = null;
    var renameReturnFocus = null;

    function showMessage(text, kind) {
      if (!message) return;
      message.textContent = text || '';
      message.className = 'px-4 py-2 text-[12px] ' + (kind === 'error' ? 'text-error' : kind === 'success' ? 'text-[#059669]' : 'text-on-surface-variant') + (text ? '' : ' hidden');
    }
    function shortName(name) {
      var n = String(name || '').trim().replace(/\.$/, '');
      return n.replace(/\.cloud\.local$/i, '');
    }
    function setRenameError(text) {
      if (!renameError) return;
      renameError.textContent = text || '';
      renameError.classList.toggle('hidden', !text);
    }

    async function load() {
      if (btn) { btn.disabled = true; btn.setAttribute('aria-busy', 'true'); }
      tbody.innerHTML = '<tr><td colspan="5" class="empty-state">Cargando registros DNS…</td></tr>';
      try {
        var res = await fetch('/api/dns/registros');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        var data = await res.json();
        if (zone && data.zona) zone.textContent = data.zona;
        var regs = Array.isArray(data.registros) ? data.registros : [];
        if (!regs.length) {
          tbody.innerHTML = '<tr><td colspan="5" class="empty-state"><strong>No hay registros A en la zona.</strong><br><span class="text-on-surface-variant">Los registros se crean automáticamente al aprovisionar instancias. Si esperabas ver alguno, pulsa “Actualizar”.</span></td></tr>';
          return;
        }
        tbody.innerHTML = regs.map(function (r) {
          var name = r.nombre || '—';
          return '<tr>' +
            '<td class="mono">' + esc(name) + '</td>' +
            '<td><span class="pill-state">' + esc(r.tipo || 'A') + '</span></td>' +
            '<td class="mono">' + esc(r.ip || '—') + '</td>' +
            '<td class="mono muted">' + esc(String(r.ttl != null ? r.ttl : '—')) + '</td>' +
            '<td><button class="btn-flat" style="height:28px;padding:0 10px;font-size:12px;" type="button" data-rename="' + escA(name) + '"><span class="material-symbols-outlined text-[14px]">edit</span> Renombrar</button></td>' +
          '</tr>';
        }).join('');
        tbody.querySelectorAll('[data-rename]').forEach(function (button) {
          button.addEventListener('click', function () { openRename(button.getAttribute('data-rename')); });
        });
      } catch (e) {
        tbody.innerHTML = '<tr><td colspan="5" class="empty-state"><strong>No se pudieron cargar los registros DNS.</strong><br><span class="text-on-surface-variant">Verifica que el sistema esté disponible y que DNS/Bind9 esté operativo en “Salud”, luego pulsa “Actualizar”.</span></td></tr>';
      } finally {
        if (btn) { btn.disabled = false; btn.setAttribute('aria-busy', 'false'); }
      }
    }

    function openRename(name) {
      if (!renameModal) return;
      renameOld = name;
      renameReturnFocus = document.activeElement;
      if (renameCurrent) renameCurrent.textContent = 'Registro actual: ' + shortName(name) + '.cloud.local';
      if (renameInput) {
        renameInput.value = shortName(name);
        renameInput.focus();
        renameInput.select();
      }
      setRenameError('');
      renameModal.classList.remove('hidden');
    }

    function closeRename() {
      if (!renameModal) return;
      renameModal.classList.add('hidden');
      setRenameError('');
      renameOld = null;
      if (renameReturnFocus && renameReturnFocus.focus) {
        try { renameReturnFocus.focus(); } catch (e) {}
      }
      renameReturnFocus = null;
    }

    async function submitRename() {
      if (!renameOld || !renameInput) return;
      var newName = (renameInput.value || '').trim().toLowerCase();
      if (!/^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/.test(newName)) {
        setRenameError('Usa 3–32 caracteres en minúsculas, números o guiones (sin empezar ni terminar con guion).');
        return;
      }
      setRenameError('');
      if (renameConfirm) { renameConfirm.disabled = true; renameConfirm.setAttribute('aria-busy', 'true'); }
      try {
        var res = await fetch('/api/dns/registros', {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
          body: JSON.stringify({ nombre: renameOld, nuevo_nombre: newName })
        });
        var data = await res.json().catch(function () { return {}; });
        if (!res.ok) throw new Error(data.error || ('HTTP ' + res.status));
        closeRename();
        showMessage('Registro DNS renombrado correctamente.', 'success');
        await load();
      } catch (e) {
        setRenameError(e.message || 'No se pudo renombrar el registro.');
      } finally {
        if (renameConfirm) { renameConfirm.disabled = false; renameConfirm.setAttribute('aria-busy', 'false'); }
      }
    }

    if (btn) btn.addEventListener('click', load);
    if (renameCancel) renameCancel.addEventListener('click', closeRename);
    if (renameConfirm) renameConfirm.addEventListener('click', submitRename);
    if (renameInput) {
      renameInput.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') { e.preventDefault(); submitRename(); }
        if (e.key === 'Escape') { e.preventDefault(); closeRename(); }
      });
    }
    if (renameModal) {
      renameModal.addEventListener('click', function (e) { if (e.target === renameModal) closeRename(); });
    }
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && renameModal && !renameModal.classList.contains('hidden')) closeRename();
    });
    load();
  }
  window.HTTPaaSPages = window.HTTPaaSPages || {}; window.HTTPaaSPages.dns = init;
})();
