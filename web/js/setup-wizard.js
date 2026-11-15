// setup-wizard.js - Preparación inicial del laboratorio HTTPaaS.
// Pantalla completa por pasos: bienvenida → credenciales → resumen →
// ejecución automática. Usa las clases y tokens del resto de la interfaz.
(function () {
  var STYLE_ID = 'setup-wizard-style';
  var ICONS = {
    PENDIENTE: 'radio_button_unchecked',
    EN_CURSO: 'progress_activity',
    COMPLETADO: 'check_circle',
    ERROR: 'error',
    OMITIDO: 'skip_next'
  };

  var root = null;
  var stream = null;
  var lastState = null;
  var streamErrors = 0;
  var elapsedSince = {};
  var elapsedTicker = null;
  var logStream = null;
  var logLastID = 0;
  var logLines = [];
  var userOpened = false;

  function esc(s) {
    var H = (window.HTTPaaS && window.HTTPaaS.escapeHtml);
    if (H) return H(s);
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c];
    });
  }

  function ensureStyles() {
    if (document.getElementById(STYLE_ID)) return;
    var s = document.createElement('style');
    s.id = STYLE_ID;
    s.textContent =
      'body.setup-gate{overflow:hidden!important;}' +
      'body.setup-gate #site-header-mount,body.setup-gate > main,body.setup-gate #site-footer-mount{display:none!important;}' +
      '.setup-page{position:fixed;inset:0;z-index:90;overflow:auto;}' +
      '.setup-progress-fill{transition:width .4s ease;}' +
      '.setup-track.indeterminate .setup-progress-fill{width:35%!important;animation:setup-slide 1.4s ease-in-out infinite;}' +
      '@keyframes setup-slide{0%{left:-35%}100%{left:100%}}' +
      '@media (prefers-reduced-motion:reduce){.setup-track.indeterminate .setup-progress-fill{animation:none;left:0;width:100%!important;opacity:.45}}';
    document.head.appendChild(s);
  }

  function navMarkup() {
    return '' +
      '<div class="flex flex-wrap gap-2" aria-label="Pasos">' +
        '<span class="setup-nav px-2 py-1 rounded-xl border-standard text-[11px] mono text-on-surface-variant" data-nav="credenciales">1 · Credenciales</span>' +
        '<span class="setup-nav px-2 py-1 rounded-xl border-standard text-[11px] mono text-on-surface-variant" data-nav="resumen">2 · Confirmar</span>' +
        '<span class="setup-nav px-2 py-1 rounded-xl border-standard text-[11px] mono text-on-surface-variant" data-nav="ejecucion">Preparación</span>' +
      '</div>';
  }

  function mount() {
    if (document.getElementById('setup-wizard')) return;
    ensureStyles();

    var wrap = document.createElement('div');
    wrap.innerHTML =
      '<div class="setup-page bg-surface-container-lowest text-on-surface" id="setup-wizard" hidden>' +
        '<div class="max-w-4xl mx-auto px-4 sm:px-6 py-8">' +
          '<div id="setup-header-bar" class="flex items-center gap-3 pb-4 border-b-standard">' +
            '<div class="w-9 h-9 rounded-xl bg-surface-container-low border-standard flex items-center justify-center text-primary-container shrink-0">' +
              '<span class="material-symbols-outlined text-[18px]" aria-hidden="true">settings</span>' +
            '</div>' +
            '<span class="text-[13px] font-semibold" id="setup-title">Configuración</span>' +
            '<span class="setup-live ml-auto text-[11px] mono text-primary-container items-center gap-1.5" id="setup-live" hidden><span class="w-1.5 h-1.5 rounded-full bg-[#f59e0b]" id="setup-live-dot" aria-hidden="true"></span><span id="setup-live-text">en vivo</span></span>' +
            '<button id="setup-close-btn" class="ml-auto hidden items-center justify-center w-8 h-8 border-standard rounded-xl bg-white hover:bg-surface-container-low" type="button" aria-label="Cerrar"><span class="material-symbols-outlined text-[18px]">close</span></button>' +
          '</div>' +
          '<div class="mt-4 mb-5" id="setup-nav-wrap">' + navMarkup() + '</div>' +

          '<section class="setup-screen hidden" data-screen="bienvenida" aria-labelledby="setup-h-bienvenida">' +
            '<div class="bg-surface-container-lowest border-standard rounded-xl p-6 space-y-4">' +
              '<h1 class="text-[16px] font-semibold" id="setup-h-bienvenida">Preparación del entorno</h1>' +
              '<p class="text-[12px] text-on-surface-variant">Configura el entorno inicial para comenzar a usar la consola.</p>' +
              '<div class="flex justify-end"><button class="px-3 py-1.5 text-white text-[13px] font-semibold rounded-xl" style="background:rgb(15,118,110);border:1px solid rgb(13,95,88);" id="setup-begin-btn" type="button">Comenzar</button></div>' +
            '</div>' +
          '</section>' +

          '<section class="setup-screen hidden" data-screen="credenciales" aria-labelledby="setup-h-credenciales">' +
            '<div class="bg-surface-container-lowest border-standard rounded-xl p-6 space-y-4">' +
              '<div><h1 class="text-[16px] font-semibold" id="setup-h-credenciales">Credenciales</h1></div>' +
              '<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">' +
                '<div><label class="text-[12px] font-semibold block mb-1" for="setup-user">Usuario</label><input class="w-full border-standard rounded-xl bg-surface-container-lowest px-3 py-2 text-[13px] mono text-on-surface focus:outline-none focus:ring-2 focus:ring-primary-container" id="setup-user" type="text" autocomplete="username" spellcheck="false" placeholder="http"></div>' +
                '<div><label class="text-[12px] font-semibold block mb-1" for="setup-pass">Contraseña</label><input class="w-full border-standard rounded-xl bg-surface-container-lowest px-3 py-2 text-[13px] mono text-on-surface focus:outline-none focus:ring-2 focus:ring-primary-container" id="setup-pass" type="password" autocomplete="new-password"></div>' +
              '</div>' +
              '<div><label class="text-[12px] font-semibold block mb-1" for="setup-pass2">Repetir contraseña</label><input class="w-full border-standard rounded-xl bg-surface-container-lowest px-3 py-2 text-[13px] mono text-on-surface focus:outline-none focus:ring-2 focus:ring-primary-container" id="setup-pass2" type="password" autocomplete="new-password"></div>' +
              '<p class="setup-error hidden whitespace-pre-wrap break-words text-[12px] rounded-xl border-standard px-3 py-2 text-error" id="setup-error-creds" role="alert"></p>' +
              '<div class="flex justify-between gap-2"><button class="px-3 py-1.5 border-standard rounded-xl text-[13px] font-medium" data-back="bienvenida" type="button">Atrás</button><button class="px-3 py-1.5 text-white text-[13px] font-semibold rounded-xl" style="background:rgb(15,118,110);border:1px solid rgb(13,95,88);" id="setup-next-creds" type="button">Siguiente</button></div>' +
            '</div>' +
          '</section>' +

          '<section class="setup-screen hidden" data-screen="resumen" aria-labelledby="setup-h-resumen">' +
            '<div class="bg-surface-container-lowest border-standard rounded-xl p-6 space-y-4">' +
              '<h1 class="text-[16px] font-semibold" id="setup-h-resumen">Confirmar</h1>' +
              '<dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-[12px]">' +
                '<dt class="text-on-surface-variant">Usuario</dt><dd class="mono break-all" id="setup-sum-usuario">—</dd>' +
                '<dt class="text-on-surface-variant">Red</dt><dd class="mono" id="setup-sum-red">—</dd>' +
                '<dt class="text-on-surface-variant">DNS</dt><dd class="mono" id="setup-sum-dns">—</dd>' +
              '</dl>' +
              '<p class="setup-error hidden whitespace-pre-wrap break-words text-[12px] rounded-xl border-standard px-3 py-2 text-error" id="setup-error-resumen" role="alert"></p>' +
              '<div class="flex justify-between gap-2"><button class="px-3 py-1.5 border-standard rounded-xl text-[13px] font-medium" data-back="credenciales" type="button">Atrás</button><button class="px-3 py-1.5 text-white text-[13px] font-semibold rounded-xl" style="background:rgb(15,118,110);border:1px solid rgb(13,95,88);" id="setup-start-btn" type="button">Comenzar</button></div>' +
            '</div>' +
          '</section>' +

          '<section class="setup-screen hidden" data-screen="ejecucion" aria-labelledby="setup-h-ejecucion">' +
            '<div class="bg-surface-container-lowest border-standard rounded-xl p-6 space-y-4">' +
              '<h1 class="text-[16px] font-semibold" id="setup-h-ejecucion">Preparando el entorno</h1>' +
              '<div class="flex items-center gap-3">' +
                '<div class="setup-track relative flex-1 h-2 rounded-full bg-surface-container overflow-hidden" id="setup-progress-track" role="progressbar" aria-label="Progreso" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0"><span class="setup-progress-fill absolute inset-y-0 left-0 w-0" style="background:rgb(15,118,110)" id="setup-progress-fill"></span></div>' +
                '<span class="text-[12px] mono text-primary-container min-w-[3.5ch] text-right" id="setup-progress-text">0%</span>' +
              '</div>' +
              '<ol class="space-y-1" id="setup-steps" aria-live="polite"></ol>' +
              '<details class="mt-2" open>' +
                '<summary class="flex items-center justify-between gap-2 text-[12px] font-semibold cursor-pointer select-none"><span>Registro</span><button class="text-[11px] mono font-medium px-2 py-0.5 rounded-xl border-standard bg-surface-container-lowest text-on-surface-variant" id="setup-copy-log" type="button">Copiar</button></summary>' +
                '<pre class="mt-1.5 h-48 overflow-y-auto overflow-x-hidden text-[10.5px] mono bg-surface-container-low rounded-xl border-standard p-2 whitespace-pre-wrap break-words" id="setup-log" role="log" aria-label="Registro de la preparación"></pre>' +
              '</details>' +
              '<p class="setup-error hidden whitespace-pre-wrap break-words text-[12px] rounded-xl border-standard px-3 py-2 text-error" id="setup-error" role="alert"></p>' +
              '<div class="flex justify-between gap-2"><button class="px-3 py-1.5 border-standard rounded-xl text-[13px] font-medium" id="setup-reset-btn" type="button">Reiniciar</button><button class="px-3 py-1.5 text-white text-[13px] font-semibold rounded-xl hidden" style="background:rgb(15,118,110);border:1px solid rgb(13,95,88);" id="setup-retry-btn" type="button">Reintentar</button></div>' +
            '</div>' +
          '</section>' +

          '<section class="setup-screen hidden" data-screen="listo" aria-labelledby="setup-h-listo">' +
            '<div class="bg-surface-container-lowest border-standard rounded-xl p-6 space-y-4">' +
              '<h1 class="text-[16px] font-semibold" id="setup-h-listo">Entorno listo</h1>' +
              '<div class="flex justify-between gap-2"><button class="px-3 py-1.5 border-standard rounded-xl text-[13px] font-medium" id="setup-restart-done-btn" type="button">Reiniciar</button><button class="px-3 py-1.5 text-white text-[13px] font-semibold rounded-xl" style="background:rgb(15,118,110);border:1px solid rgb(13,95,88);" id="setup-done-btn" type="button">Ir a la consola</button></div>' +
            '</div>' +
          '</section>' +

          '<section class="setup-screen hidden" data-screen="sistema">' +
            '<div class="space-y-6">' +
              '<div class="bg-surface-container-lowest border-standard rounded-xl p-6 space-y-4">' +
                '<h2 class="text-[15px] font-semibold m-0">Restablecer credenciales</h2>' +
                '<div class="grid grid-cols-1 md:grid-cols-2 gap-4">' +
                  '<div><label class="text-[12px] font-semibold block mb-1" for="sys-user">Usuario</label><input class="w-full border-standard rounded-xl bg-surface-container-lowest px-3 py-2 text-[13px] mono text-on-surface" id="sys-user" type="text" autocomplete="username" spellcheck="false"></div>' +
                  '<div><label class="text-[12px] font-semibold block mb-1" for="sys-pass">Contraseña</label><input class="w-full border-standard rounded-xl bg-surface-container-lowest px-3 py-2 text-[13px] mono text-on-surface" id="sys-pass" type="password" autocomplete="new-password"></div>' +
                '</div>' +
                '<div><label class="text-[12px] font-semibold block mb-1" for="sys-pass2">Repetir contraseña</label><input class="w-full md:w-1/2 border-standard rounded-xl bg-surface-container-lowest px-3 py-2 text-[13px] mono text-on-surface" id="sys-pass2" type="password" autocomplete="new-password"></div>' +
                '<p id="sys-cred-msg" class="text-[12px] hidden"></p>' +
                '<div class="flex gap-2"><button class="px-3 py-1.5 text-white text-[13px] font-semibold rounded-xl" style="background:rgb(15,118,110);border:1px solid rgb(13,95,88);" id="sys-save-creds" type="button">Guardar credenciales</button></div>' +
              '</div>' +
              '<div class="bg-surface-container-lowest border-standard rounded-xl p-6 space-y-3">' +
                '<div class="flex items-center justify-between"><h2 class="text-[15px] font-semibold m-0">Hipervisor y entorno</h2><button class="btn-flat" id="sys-refresh" type="button"><span class="material-symbols-outlined text-[16px]">refresh</span> Actualizar</button></div>' +
                '<div class="grid grid-cols-1 md:grid-cols-3 gap-3">' +
                  '<div class="card"><div class="card-body"><div class="kv-label">VirtualBox</div><div class="kv-value" id="sys-vbox">—</div></div></div>' +
                  '<div class="card"><div class="card-body"><div class="kv-label">DNS</div><div class="kv-value" id="sys-bind9">—</div></div></div>' +
                  '<div class="card"><div class="card-body"><div class="kv-label">Red</div><div class="kv-value mono" id="sys-net">—</div></div></div>' +
                '</div>' +
                '<pre class="code-block" id="sys-env" style="max-height:220px;overflow:auto;">Cargando…</pre>' +
              '</div>' +
              '<div class="bg-surface-container-lowest border-standard rounded-xl p-6 space-y-3">' +
                '<h2 class="text-[15px] font-semibold m-0">Máquinas virtuales registradas</h2>' +
                '<div style="overflow-x:auto;"><table class="simple-table"><thead><tr><th>Nombre</th><th>FQDN</th><th>IP</th><th>Estado</th><th>Acciones</th></tr></thead><tbody id="sys-vms"><tr><td colspan="5" class="empty-state">Cargando…</td></tr></tbody></table></div>' +
              '</div>' +
              '<div class="flex justify-start items-center gap-2">' +
                '<button class="px-3 py-1.5 text-white text-[13px] font-semibold rounded-xl" style="background:#dc2626;border:1px solid #b91c1c;" id="sys-reset-open-btn" type="button">Restablecer sistema</button>' +
              '</div>' +
            '</div>' +
          '</section>' +
        '</div>' +
        '<div id="sys-reset-dialog" class="hidden fixed inset-0 z-[120]" role="dialog" aria-modal="true" aria-labelledby="sys-reset-title">' +
          '<div class="absolute inset-0 bg-black/40" data-sys-reset-cancel="backdrop"></div>' +
          '<div class="relative max-w-lg mx-auto mt-24 bg-surface-container-lowest border-standard rounded-xl p-6 space-y-4 shadow-lg">' +
            '<div class="flex items-start gap-3">' +
              '<span class="material-symbols-outlined text-error text-[22px]" aria-hidden="true">warning</span>' +
              '<div class="space-y-1">' +
                '<h2 id="sys-reset-title" class="text-[15px] font-semibold m-0">¿Seguro que deseas continuar?</h2>' +
                '<p class="text-[12px] text-on-surface-variant m-0"><strong class="text-error">Acción destructiva:</strong> se eliminarán las máquinas virtuales <span class="mono">ns</span> y <span class="mono">ApacheServer</span> y se reiniciará la preparación del entorno. La imagen y el VDI base se conservan, pero la configuración y el estado actual de esas VMs se perderán.</p>' +
              '</div>' +
            '</div>' +
            '<div>' +
              '<label class="text-[12px] font-semibold block mb-1" for="sys-reset-confirm-input">Escribe <span class="mono">Confirmar</span> para continuar</label>' +
              '<input id="sys-reset-confirm-input" class="w-full border-standard rounded-xl bg-surface-container-lowest px-3 py-2 text-[13px] mono text-on-surface" type="text" autocomplete="off" spellcheck="false" placeholder="Confirmar">' +
            '</div>' +
            '<p id="sys-reset-error" class="text-[12px] text-error hidden"></p>' +
            '<div class="flex justify-end gap-2">' +
              '<button id="sys-reset-cancel-btn" class="px-3 py-1.5 border-standard rounded-xl text-[13px] font-medium" type="button">Cancelar</button>' +
              '<button id="sys-reset-confirm-btn" class="px-3 py-1.5 text-white text-[13px] font-semibold rounded-xl opacity-50 cursor-not-allowed" style="background:#dc2626;border:1px solid #b91c1c;" type="button" disabled>Restablecer</button>' +
            '</div>' +
          '</div>' +
        '</div>' +
      '</div>';
    document.body.appendChild(wrap.firstChild);
    root = document.getElementById('setup-wizard');

    document.getElementById('setup-begin-btn').addEventListener('click', function () { showScreen('credenciales'); });
    document.getElementById('setup-next-creds').addEventListener('click', validateCreds);
    document.getElementById('setup-start-btn').addEventListener('click', startSetup);
    document.getElementById('setup-retry-btn').addEventListener('click', startSetup);
    document.getElementById('setup-reset-btn').addEventListener('click', resetSetup);
    document.getElementById('setup-restart-done-btn').addEventListener('click', resetSetup);
    document.getElementById('setup-done-btn').addEventListener('click', function () { if (lastState && lastState.completado) showScreen('sistema'); else finish(); });
    var closeBtn = document.getElementById('setup-close-btn');
    if (closeBtn) closeBtn.addEventListener('click', finish);

    var sysRefresh = document.getElementById('sys-refresh');
    if (sysRefresh) sysRefresh.addEventListener('click', loadSystemInfo);
    var sysSave = document.getElementById('sys-save-creds');
    if (sysSave) sysSave.addEventListener('click', saveSystemCreds);
    var sysResetOpen = document.getElementById('sys-reset-open-btn');
    if (sysResetOpen) sysResetOpen.addEventListener('click', openSystemResetDialog);
    var sysResetCancel = document.getElementById('sys-reset-cancel-btn');
    if (sysResetCancel) sysResetCancel.addEventListener('click', closeSystemResetDialog);
    var sysResetBackdrop = root.parentNode && root.parentNode.querySelector('[data-sys-reset-cancel="backdrop"]');
    if (sysResetBackdrop) sysResetBackdrop.addEventListener('click', closeSystemResetDialog);
    var sysResetInput = document.getElementById('sys-reset-confirm-input');
    if (sysResetInput) sysResetInput.addEventListener('input', updateSystemResetConfirmState);
    var sysResetConfirm = document.getElementById('sys-reset-confirm-btn');
    if (sysResetConfirm) sysResetConfirm.addEventListener('click', confirmSystemReset);
    var copyBtn = document.getElementById('setup-copy-log');
    if (copyBtn) {
      copyBtn.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        var textVal = logLines.join('\\n');
        var done = function () {
          copyBtn.textContent = 'Copiado';
          setTimeout(function () { copyBtn.textContent = 'Copiar'; }, 1500);
        };
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(textVal).then(done).catch(function () { fallbackCopy(textVal); done(); });
        } else {
          fallbackCopy(textVal);
          done();
        }
      });
    }
    root.querySelectorAll('[data-back]').forEach(function (btn) {
      btn.addEventListener('click', function () { showScreen(btn.getAttribute('data-back')); });
    });
  }

  function currentScreen() {
    var active = root && root.querySelector('.setup-screen.active');
    return active ? active.getAttribute('data-screen') : '';
  }

  function showScreen(name) {
    if (!root) return;
    root.querySelectorAll('.setup-screen').forEach(function (section) {
      section.classList.toggle('hidden', section.getAttribute('data-screen') !== name);
      section.classList.toggle('active', section.getAttribute('data-screen') === name);
    });
    var navWrap = document.getElementById('setup-nav-wrap');
    if (navWrap) navWrap.classList.toggle('hidden', name === 'sistema');
    var closeBtn = document.getElementById('setup-close-btn');
    var live = document.getElementById('setup-live');
    var headerBar = document.getElementById('setup-header-bar');
    if (closeBtn) {
      closeBtn.classList.toggle('hidden', name !== 'sistema');
      if (name === 'sistema') closeBtn.classList.add('inline-flex'); else closeBtn.classList.remove('inline-flex');
    }
    if (live) {
      if (name === 'sistema') {
        live.hidden = true;
        live.classList.add('hidden');
        live.classList.remove('inline-flex');
      }
    }
    if (headerBar) headerBar.classList.toggle('border-b-standard', name !== 'sistema');
    var title = document.getElementById('setup-title');
    if (title) title.textContent = name === 'sistema' ? 'Configurar sistemas' : 'Configuración';
    var order = ['credenciales', 'resumen'];
    var index = order.indexOf(name);
    root.querySelectorAll('[data-nav]').forEach(function (item) {
      var key = item.getAttribute('data-nav');
      item.classList.toggle('active', key === name);
      item.classList.toggle('done', order.indexOf(key) !== -1 && index !== -1 && order.indexOf(key) < index);
      item.classList.toggle('text-primary-container', key === name || item.classList.contains('done'));
      item.classList.toggle('font-semibold', key === name);
    });
    if (name === 'ejecucion') {
      root.querySelectorAll('[data-nav]').forEach(function (item) { item.classList.toggle('done', item.getAttribute('data-nav') !== 'ejecucion'); });
      connectLogStream();
    }
    if (name === 'listo') {
      root.querySelectorAll('[data-nav]').forEach(function (item) { item.classList.add('done'); item.classList.remove('active'); });
    }
    if (name === 'sistema') {
      loadSystemInfo();
    }
    if (name === 'credenciales') {
      var input = document.getElementById('setup-user');
      if (input && !input.value) input.focus();
    }
  }

  function errorBox(id, message) {
    var box = document.getElementById(id);
    if (!box) return;
    box.textContent = message || '';
    box.classList.toggle('hidden', !message);
  }

  function validateCreds() {
    var user = (document.getElementById('setup-user').value || '').trim();
    var pass = document.getElementById('setup-pass').value || '';
    var pass2 = document.getElementById('setup-pass2').value || '';
    if (!/^[a-z][a-z0-9_-]{2,31}$/.test(user)) {
      errorBox('setup-error-creds', 'Usuario inválido.');
      return;
    }
    if (!pass) {
      errorBox('setup-error-creds', 'La contraseña es obligatoria.');
      return;
    }
    if (pass !== pass2) {
      errorBox('setup-error-creds', 'Las contraseñas no coinciden.');
      return;
    }
    errorBox('setup-error-creds', '');
    updateSummary();
    showScreen('resumen');
  }

  function updateSummary() {
    var user = (document.getElementById('setup-user').value || '').trim();
    var sumUser = document.getElementById('setup-sum-usuario');
    if (sumUser) sumUser.textContent = user || '—';
    updateNetworkSummary(lastState);
  }

  function resetSetup() {
    if (!window.confirm('Se eliminarán las VMs ns y ApacheServer y se reiniciará la preparación. La imagen y el VDI base se conservan. ¿Continuar?')) return;
    var btn = document.getElementById('setup-reset-btn');
    if (btn) btn.disabled = true;
    fetch('/api/setup/reiniciar', { method: 'POST', headers: { 'Accept': 'application/json' } })
      .then(function (res) {
        return res.json().catch(function () { return {}; }).then(function (body) {
          if (!res.ok) throw new Error(body.error || ('HTTP ' + res.status));
          window.location.reload();
        });
      })
      .catch(function (err) {
        if (btn) btn.disabled = false;
        errorBox('setup-error', 'No se pudo reiniciar: ' + err.message);
      });
  }

  function startSetup() {
    var payload = {
      usuario: (document.getElementById('setup-user').value || '').trim(),
      contrasena: document.getElementById('setup-pass').value || ''
    };

    var startBtn = document.getElementById('setup-start-btn');
    if (startBtn) startBtn.disabled = true;
    errorBox('setup-error-resumen', '');

    fetch('/api/setup/configuracion', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
      body: JSON.stringify(payload)
    }).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (body) {
        if (!res.ok) throw new Error(body.error || ('HTTP ' + res.status));
        return body;
      });
    }).then(function () {
      return fetch('/api/setup/iniciar', { method: 'POST', headers: { 'Accept': 'application/json' } });
    }).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (body) {
        if (!res.ok && res.status !== 409) throw new Error(body.error || ('HTTP ' + res.status));
        connectStream();
        showScreen('ejecucion');
      });
    }).catch(function (err) {
      if (startBtn) startBtn.disabled = false;
      errorBox('setup-error-resumen', err.message);
    });
  }

  function updateNetworkSummary(state) {
    var red = state && state.red;
    var redEl = document.getElementById('setup-sum-red');
    var dnsEl = document.getElementById('setup-sum-dns');
    if (redEl) {
      redEl.textContent = (red && red.ip_anfitrion)
        ? red.ip_anfitrion + '/24 · DHCP ' + red.dhcp_inicio + '–' + red.dhcp_fin
        : 'pendiente';
    }
    if (dnsEl) {
      dnsEl.textContent = (red && red.ip_dns) ? 'ns.cloud.local · ' + red.ip_dns : 'pendiente';
    }
  }

  function applyGate(state) {    var gated = !state.completado;
    document.body.classList.toggle('setup-gate', gated);
  }

  function render(state) {
    lastState = state;
    applyGate(state);
    updateNetworkSummary(state);

    if (state.ejecutando) {
      if (root && root.hidden) { root.hidden = false; userOpened = true; }
      if (currentScreen() !== 'ejecucion') showScreen('ejecucion');
      setLive('', 'en vivo');
    } else if (state.completado) {
      document.body.classList.remove('setup-gate');
      if (userOpened) {
        var cs = currentScreen();
        if (cs === 'sistema') {
          var liveEl = document.getElementById('setup-live');
          if (liveEl) { liveEl.hidden = true; liveEl.classList.add('hidden'); liveEl.classList.remove('inline-flex'); }
        } else {
          if (cs !== 'listo') showScreen('listo');
          setLive('', 'en vivo');
        }
      } else if (root) {
        root.hidden = true;
      }
    } else if (state.error) {
      setLive('error', 'detenido');
      if (root && root.hidden) { root.hidden = false; userOpened = true; }
      if (currentScreen() !== 'ejecucion') showScreen('ejecucion');
    } else {
      var live = document.getElementById('setup-live');
      if (live) live.hidden = true;
    }

    if (state.configuracion && state.configuracion.usuario) {
      var user = document.getElementById('setup-user');
      if (user && !user.value) user.value = state.configuracion.usuario;
    }

    var stepsHost = document.getElementById('setup-steps');
    if (stepsHost) renderSteps(state, stepsHost);
  }

  function renderSteps(state, host) {
    var steps = state.pasos || [];
    var done = 0, partial = 0, running = null;
    host.innerHTML = steps.map(function (step) {
      if (step.estado === 'COMPLETADO' || step.estado === 'OMITIDO') done++;
      else if (step.estado === 'EN_CURSO') {
        running = step;
        if (step.progreso > 0) partial += step.progreso / 100;
      }
      var icon = ICONS[step.estado] || ICONS.PENDIENTE;
      var iconColor = step.estado === 'COMPLETADO' ? 'text-[#16a34a]' : (step.estado === 'ERROR' ? 'text-error' : 'text-outline');
      var indicator = step.estado === 'EN_CURSO'
        ? '<span class="inline-block w-4 h-4 rounded-full border-2 border-primary-container border-t-transparent animate-spin shrink-0 mt-0.5" aria-hidden="true"></span>'
        : '<span class="material-symbols-outlined text-[18px] ' + iconColor + '" aria-hidden="true">' + icon + '</span>';
      var bar = (step.estado === 'EN_CURSO' && step.progreso >= 0)
        ? '<div class="relative h-[3px] rounded-full bg-surface-container overflow-hidden mt-1"><span class="absolute inset-y-0 left-0" style="width:' + Math.max(2, step.progreso) + '%;background:rgb(15,118,110)"></span></div>' : '';
      var msg = step.mensaje ? '<div class="text-[11px] mono text-primary-container break-words">' + esc(step.mensaje) + '</div>' : '';
      return '' +
        '<li class="flex gap-2.5 p-2.5 rounded-xl ' + (step.estado === 'EN_CURSO' ? 'border-standard' : '') + (step.estado === 'ERROR' ? ' border-standard' : '') + '" data-estado="' + esc(step.estado) + '" data-setup-step="' + esc(step.id) + '">' +
          indicator +
          '<div class="flex-1 min-w-0"><div class="text-[12.5px] font-semibold">' + esc(step.nombre) + '<span class="setup-elapsed text-[11px] mono text-outline"></span></div>' + bar + msg + '</div>' +
        '</li>';
    }).join('');

    if (running) {
      if (!elapsedSince[running.id]) {
        var started = Date.parse(state.actualizado_en || '');
        elapsedSince[running.id] = isNaN(started) ? Date.now() : started;
      }
      ensureElapsedTicker();
    } else {
      stopElapsedTicker();
    }

    var percent = steps.length ? Math.round((done + partial) / steps.length * 100) : 0;
    if (state.completado) percent = 100;
    var fill = document.getElementById('setup-progress-fill');
    var text = document.getElementById('setup-progress-text');
    var track = document.getElementById('setup-progress-track');
    if (fill) fill.style.width = percent + '%';
    if (text) text.textContent = percent + '%';
    if (track) {
      var indeterminate = !!(state.ejecutando && running && running.progreso < 0 && percent < 100);
      track.classList.toggle('indeterminate', indeterminate);
      track.setAttribute('aria-valuenow', String(percent));
      track.setAttribute('aria-valuetext', indeterminate ? 'En curso' : percent + '%');
    }

    var errBox = document.getElementById('setup-error');
    if (errBox) {
      errBox.textContent = state.error || '';
      errBox.classList.toggle('hidden', !state.error);
    }
    var retry = document.getElementById('setup-retry-btn');
    if (retry) retry.classList.toggle('hidden', !state.error || state.ejecutando);
    var reset = document.getElementById('setup-reset-btn');
    if (reset) reset.disabled = !!state.ejecutando;
  }

  function ensureElapsedTicker() {
    if (elapsedTicker) return;
    elapsedTicker = setInterval(function () {
      var row = document.querySelector('.setup-step[data-estado="EN_CURSO"]');
      if (!row) return;
      var id = row.getAttribute('data-setup-step');
      if (!elapsedSince[id]) elapsedSince[id] = Date.now();
      var label = row.querySelector('.setup-elapsed');
      if (!label) return;
      var seconds = Math.floor((Date.now() - elapsedSince[id]) / 1000);
      label.textContent = ' · ' + Math.floor(seconds / 60) + 'm ' + String(seconds % 60).padStart(2, '0') + 's';
    }, 1000);
  }

  function stopElapsedTicker() {
    if (elapsedTicker) {
      clearInterval(elapsedTicker);
      elapsedTicker = null;
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

  function appendLogLine(entry) {
    if (!entry || typeof entry.id !== 'number' || entry.id <= logLastID) return;
    logLastID = entry.id;
    if ((entry.componente || '') === 'http') return;
    var pre = document.getElementById('setup-log');
    if (!pre) return;
    var ts = '';
    try { ts = new Date(entry.ts).toTimeString().slice(0, 8); } catch (e) { ts = ''; }
    logLines.push('[' + ts + '] ' + (entry.nivel || 'INFO') + ' ' + (entry.componente || '') + ' · ' + (entry.mensaje || ''));
    if (logLines.length > 300) logLines.shift();
    pre.textContent = logLines.join('\n');
    pre.scrollTop = pre.scrollHeight;
  }

  function connectLogStream() {
    if (logStream) return;
    fetch('/api/logs?limit=200', { headers: { 'Accept': 'application/json' } })
      .then(function (res) { return res.ok ? res.json() : null; })
      .then(function (data) {
        if (!data) return;
        (data.datos || []).forEach(appendLogLine);
      })
      .catch(function () { /* sin historial */ });

    logStream = new EventSource('/api/logs/stream');
    logStream.onmessage = function (event) {
      try { appendLogLine(JSON.parse(event.data)); } catch (e) { /* ignorar */ }
    };
    logStream.onerror = function () { /* EventSource reintenta solo */ };
  }

  function stopLogStream() {
    if (logStream) {
      logStream.close();
      logStream = null;
    }
  }

  function setLive(kind, text) {
    var el = document.getElementById('setup-live');
    var label = document.getElementById('setup-live-text');
    var dot = document.getElementById('setup-live-dot');
    if (!el || !label) return;
    el.hidden = false;
    el.classList.remove('hidden');
    el.classList.add('inline-flex');
    if (dot) {
      dot.className = 'w-1.5 h-1.5 rounded-full';
      if (kind === 'live') dot.classList.add('bg-[#10b981]');
      else if (kind === 'error') dot.classList.add('bg-[#ef4444]');
      else dot.classList.add('bg-[#f59e0b]');
    }
    label.textContent = text;
  }

  function connectStream() {
    if (stream || !('EventSource' in window)) return;
    stream = new EventSource('/api/setup/stream');
    streamErrors = 0;
    stream.onopen = function () { setLive('', 'en vivo'); streamErrors = 0; };
    stream.onmessage = function (event) {
      try { render(JSON.parse(event.data)); } catch (e) { /* payload inválido */ }
    };
    stream.onerror = function () {
      streamErrors++;
      setLive('error', 'reconectando');
      if (streamErrors > 8) stopStream();
    };
  }

  function stopStream() {
    if (stream) { stream.close(); stream = null; }
  }

  async function loadSystemInfo() {
    var vbox = document.getElementById('sys-vbox');
    var bind9 = document.getElementById('sys-bind9');
    var net = document.getElementById('sys-net');
    var env = document.getElementById('sys-env');
    var vms = document.getElementById('sys-vms');
    var sysUser = document.getElementById('sys-user');
    if (sysUser && lastState && lastState.configuracion && lastState.configuracion.usuario && !sysUser.value) {
      sysUser.value = lastState.configuracion.usuario;
    }
    var summary = {};
    try {
      var saludRes = await fetch('/api/salud', { headers: { 'Accept': 'application/json' } });
      var salud = await saludRes.json().catch(function () { return {}; });
      if (vbox) vbox.textContent = salud.vboxmanage === true ? 'Disponible' : salud.vboxmanage === false ? 'No disponible' : 'Sin datos';
      if (bind9) bind9.textContent = salud.bind9 === true ? 'Disponible' : salud.bind9 === false ? 'No disponible' : 'Sin datos';
      summary.salud = salud;
    } catch (e) {
      if (vbox) vbox.textContent = 'No disponible';
      if (bind9) bind9.textContent = 'No disponible';
      summary.salud = { error: e.message };
    }
    try {
      var stRes = await fetch('/api/setup/estado', { headers: { 'Accept': 'application/json' } });
      var st = await stRes.json().catch(function () { return {}; });
      var red = st.red || {};
      if (net) net.textContent = red.cidr ? (red.cidr + (red.ip_dns ? ' · DNS ' + red.ip_dns : '')) : '—';
      if (sysUser && st.configuracion && st.configuracion.usuario && !sysUser.value) sysUser.value = st.configuracion.usuario;
      summary.estado_setup = { completado: st.completado, red: st.red, imagen: st.imagen, configuracion: st.configuracion };
    } catch (e) {
      if (net) net.textContent = '—';
      summary.estado_setup = { error: e.message };
    }
    try {
      var instRes = await fetch('/api/instancias', { headers: { 'Accept': 'application/json' } });
      var instData = await instRes.json().catch(function () { return {}; });
      var list = instRes.ok ? (Array.isArray(instData.datos) ? instData.datos : (Array.isArray(instData) ? instData : [])) : [];
      if (vms) {
        if (!instRes.ok) {
          vms.innerHTML = '<tr><td colspan="5" class="empty-state">No disponible</td></tr>';
        } else {
          var nsRunning = !(summary.salud && summary.salud.ns_activa === false);
          var nsIP = (summary.estado_setup && summary.estado_setup.red && summary.estado_setup.red.ip_dns) || '—';
          var nsEstado = nsRunning ? 'ACTIVA' : 'DETENIDA';
          var nsAction = !nsRunning
            ? '<button class="btn-flat primary" type="button" data-sys-ns-power="ns"><span class="material-symbols-outlined text-[15px]">power_settings_new</span> Encender</button>'
            : '—';
          var rows = ['<tr><td class="mono">ns</td><td class="mono">ns.cloud.local</td><td class="mono">' + esc(nsIP) + '</td><td>' + esc(nsEstado) + '</td><td>' + nsAction + '</td></tr>'];
          rows = rows.concat(list.map(function (i) {
            var id = i.id || i.nombre || '';
            var estado = (i.estado || '—').toUpperCase();
            var action = (estado === 'DETENIDA' || estado === 'ERROR')
              ? '<button class="btn-flat primary" type="button" data-sys-power="' + esc(id) + '"><span class="material-symbols-outlined text-[15px]">power_settings_new</span> Encender</button>'
              : '—';
            return '<tr><td class="mono">' + esc(i.nombre || i.id || '—') + '</td><td class="mono">' + esc(i.fqdn || '—') + '</td><td class="mono">' + esc(i.ip || '—') + '</td><td>' + esc(estado) + '</td><td>' + action + '</td></tr>';
          }));
          vms.innerHTML = rows.join('');
          vms.querySelectorAll('[data-sys-ns-power]').forEach(function (btn) {
            btn.addEventListener('click', async function () {
              btn.disabled = true;
              btn.textContent = 'Encendiendo…';
              try {
                var res = await fetch('/api/dns/encender', { method: 'POST', headers: { 'Accept': 'application/json' } });
                if (!res.ok) throw new Error('HTTP ' + res.status);
                await loadSystemInfo();
                if (window.refreshInstancesPanel) window.refreshInstancesPanel();
              } catch (e) {
                btn.disabled = false;
                btn.textContent = 'Reintentar';
              }
            });
          });
          vms.querySelectorAll('[data-sys-power]').forEach(function (btn) {
            btn.addEventListener('click', async function () {
              var id = btn.getAttribute('data-sys-power');
              if (!id) return;
              btn.disabled = true;
              btn.textContent = 'Encendiendo…';
              try {
                var res = await fetch('/api/instancias/' + encodeURIComponent(id) + '/activar', { method: 'POST', headers: { 'Accept': 'application/json' } });
                if (!res.ok) throw new Error('HTTP ' + res.status);
                await loadSystemInfo();
                if (window.refreshInstancesPanel) window.refreshInstancesPanel();
              } catch (e) {
                btn.disabled = false;
                btn.textContent = 'Reintentar';
              }
            });
          });
        }
      }
      summary.instancias = { total: list.length };
    } catch (e) {
      if (vms) vms.innerHTML = '<tr><td colspan="5" class="empty-state">No disponible</td></tr>';
      summary.instancias = { error: e.message };
    }
    if (env) env.textContent = JSON.stringify(summary, null, 2);
  }

  async function saveSystemCreds() {
    var user = (document.getElementById('sys-user').value || '').trim();
    var pass = document.getElementById('sys-pass').value || '';
    var pass2 = document.getElementById('sys-pass2').value || '';
    var msg = document.getElementById('sys-cred-msg');
    var btn = document.getElementById('sys-save-creds');
    function showMsg(text, ok) {
      if (!msg) return;
      msg.textContent = text || '';
      msg.className = 'text-[12px] ' + (ok ? 'text-[#059669]' : 'text-error');
      msg.classList.toggle('hidden', !text);
    }
    if (!/^[a-z][a-z0-9_-]{2,31}$/.test(user)) { showMsg('Usuario inválido.', false); return; }
    if (!pass) { showMsg('La contraseña es obligatoria.', false); return; }
    if (pass !== pass2) { showMsg('Las contraseñas no coinciden.', false); return; }
    if (btn) { btn.disabled = true; btn.setAttribute('aria-busy', 'true'); }
    showMsg('', true);
    try {
      var res = await fetch('/api/setup/configuracion', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
        body: JSON.stringify({ usuario: user, contrasena: pass })
      });
      var body = await res.json().catch(function () { return {}; });
      if (!res.ok) throw new Error(body.error || ('HTTP ' + res.status));
      showMsg('Credenciales guardadas.', true);
      document.getElementById('sys-pass').value = '';
      document.getElementById('sys-pass2').value = '';
      if (lastState && lastState.configuracion) lastState.configuracion.usuario = user;
    } catch (e) {
      showMsg(e.message || 'No se pudieron guardar las credenciales.', false);
    } finally {
      if (btn) { btn.disabled = false; btn.setAttribute('aria-busy', 'false'); }
    }
  }

  var sysResetLastFocus = null;
  function sysResetFocusables() {
    var dlg = document.getElementById('sys-reset-dialog');
    if (!dlg) return [];
    return Array.prototype.slice.call(dlg.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])')).filter(function (el) { return !el.disabled && el.offsetParent !== null; });
  }
  function trapSysResetFocus(e) {
    var dlg = document.getElementById('sys-reset-dialog');
    if (!dlg || dlg.classList.contains('hidden')) return;
    if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeSystemResetDialog(); return; }
    if (e.key !== 'Tab') return;
    var nodes = sysResetFocusables();
    if (!nodes.length) return;
    var first = nodes[0], last = nodes[nodes.length - 1];
    if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
  }
  function openSystemResetDialog() {
    var dlg = document.getElementById('sys-reset-dialog');
    var input = document.getElementById('sys-reset-confirm-input');
    var err = document.getElementById('sys-reset-error');
    if (!dlg) return;
    sysResetLastFocus = document.activeElement;
    if (input) input.value = '';
    if (err) { err.textContent = ''; err.classList.add('hidden'); }
    updateSystemResetConfirmState();
    dlg.classList.remove('hidden');
    document.addEventListener('keydown', trapSysResetFocus, true);
    if (input) input.focus();
  }

  function closeSystemResetDialog() {
    var dlg = document.getElementById('sys-reset-dialog');
    var input = document.getElementById('sys-reset-confirm-input');
    var err = document.getElementById('sys-reset-error');
    document.removeEventListener('keydown', trapSysResetFocus, true);
    if (dlg) dlg.classList.add('hidden');
    if (input) input.value = '';
    if (err) { err.textContent = ''; err.classList.add('hidden'); }
    updateSystemResetConfirmState();
    if (sysResetLastFocus && sysResetLastFocus.focus) try { sysResetLastFocus.focus(); } catch (e) {}
    sysResetLastFocus = null;
  }

  function updateSystemResetConfirmState() {
    var input = document.getElementById('sys-reset-confirm-input');
    var btn = document.getElementById('sys-reset-confirm-btn');
    if (!btn) return;
    var ok = input && input.value === 'Confirmar';
    btn.disabled = !ok;
    btn.classList.toggle('opacity-50', !ok);
    btn.classList.toggle('cursor-not-allowed', !ok);
  }

  async function confirmSystemReset() {
    var input = document.getElementById('sys-reset-confirm-input');
    var btn = document.getElementById('sys-reset-confirm-btn');
    var err = document.getElementById('sys-reset-error');
    if (!input || input.value !== 'Confirmar') { updateSystemResetConfirmState(); return; }
    if (btn) { btn.disabled = true; btn.setAttribute('aria-busy', 'true'); }
    if (err) { err.textContent = ''; err.classList.add('hidden'); }
    try {
      var res = await fetch('/api/setup/reiniciar', { method: 'POST', headers: { 'Accept': 'application/json' } });
      var body = await res.json().catch(function () { return {}; });
      if (!res.ok) throw new Error(body.error || ('HTTP ' + res.status));
      window.location.reload();
    } catch (e) {
      if (err) { err.textContent = 'No se pudo restablecer: ' + (e.message || 'error'); err.classList.remove('hidden'); }
      if (btn) { btn.disabled = false; btn.setAttribute('aria-busy', 'false'); }
      updateSystemResetConfirmState();
    }
  }

  function open(mode) {
    mount();
    userOpened = true;
    var systemMode = mode === 'sistema' || (mode && mode.mode === 'sistema');
    root.hidden = false;
    if (systemMode || (lastState && lastState.completado)) {
      document.body.classList.remove('setup-gate');
      showScreen('sistema');
      return;
    }
    if (!lastState) showScreen('bienvenida');
    else if (lastState.ejecutando) showScreen('ejecucion');
    else showScreen('bienvenida');
    var begin = document.getElementById('setup-begin-btn');
    if (begin) begin.focus();
  }

  function finish() {
    if (lastState && !lastState.completado) return;
    userOpened = false;
    if (root) root.hidden = true;
    document.body.classList.remove('setup-gate');
    stopLogStream();
    stopElapsedTicker();
  }

  function init() {
    mount();
    fetch('/api/setup/estado', { headers: { 'Accept': 'application/json' } })
      .then(function (res) { if (!res.ok) throw new Error('HTTP ' + res.status); return res.json(); })
      .then(function (state) {
        render(state);
        document.documentElement.classList.remove('setup-checking');
        if (state.completado) {
          if (!userOpened && root) root.hidden = true;
          return;
        }
        connectStream();
        userOpened = true;
        root.hidden = false;
        if (state.ejecutando) {
          showScreen('ejecucion');
        } else {
          showScreen('bienvenida');
          var begin = document.getElementById('setup-begin-btn');
          if (begin) begin.focus();
        }
      })
      .catch(function () { document.documentElement.classList.remove('setup-checking'); });
  }

  window.openSetupWizard = open;
  window.HTTPaaSSetup = { refresh: init, open: open, close: finish };

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
