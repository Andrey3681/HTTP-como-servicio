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
        '<div class="max-w-3xl mx-auto px-4 sm:px-6 py-8">' +
          '<div class="flex items-center gap-3 pb-4 border-b-standard">' +
            '<div class="w-9 h-9 rounded-xl bg-surface-container-low border-standard flex items-center justify-center text-primary-container shrink-0">' +
              '<span class="material-symbols-outlined text-[18px]" aria-hidden="true">settings</span>' +
            '</div>' +
            '<span class="text-[13px] font-semibold">HTTPaaS</span>' +
            '<span class="setup-live ml-auto text-[11px] mono text-primary-container items-center gap-1.5" id="setup-live" hidden><span class="w-1.5 h-1.5 rounded-full bg-[#f59e0b]" id="setup-live-dot" aria-hidden="true"></span><span id="setup-live-text">en vivo</span></span>' +
          '</div>' +
          '<div class="mt-4 mb-5">' + navMarkup() + '</div>' +

          '<section class="setup-screen hidden" data-screen="bienvenida" aria-labelledby="setup-h-bienvenida">' +
            '<div class="bg-surface-container-lowest border-standard rounded-xl p-6 space-y-4">' +
              '<h1 class="text-[16px] font-semibold" id="setup-h-bienvenida">Preparación del entorno</h1>' +
              '<p class="text-[12px] text-on-surface-variant">Se descarga la imagen cloud de Debian 13 (~3 GB) y se crean la red, el servidor DNS y la plantilla Apache. Tarda entre 15 y 30 minutos.</p>' +
              '<div class="flex justify-end"><button class="px-3 py-1.5 text-white text-[13px] font-semibold rounded-xl" style="background:rgb(15,118,110);border:1px solid rgb(13,95,88);" id="setup-begin-btn" type="button">Comenzar</button></div>' +
            '</div>' +
          '</section>' +

          '<section class="setup-screen hidden" data-screen="credenciales" aria-labelledby="setup-h-credenciales">' +
            '<div class="bg-surface-container-lowest border-standard rounded-xl p-6 space-y-4">' +
              '<div><h1 class="text-[16px] font-semibold" id="setup-h-credenciales">Credenciales del invitado</h1>' +
              '<p class="text-[12px] text-on-surface-variant mt-1">Se crean en ambas máquinas.</p></div>' +
              '<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">' +
                '<div><label class="text-[12px] font-semibold block mb-1" for="setup-user">Usuario</label><input class="w-full border-standard rounded-xl bg-surface-container-lowest px-3 py-2 text-[13px] mono text-on-surface focus:outline-none focus:ring-2 focus:ring-primary-container" id="setup-user" type="text" autocomplete="username" spellcheck="false" placeholder="http"><p class="text-[11px] text-outline mt-1">Minúsculas, números, guion o guion bajo.</p></div>' +
                '<div><label class="text-[12px] font-semibold block mb-1" for="setup-pass">Contraseña</label><input class="w-full border-standard rounded-xl bg-surface-container-lowest px-3 py-2 text-[13px] mono text-on-surface focus:outline-none focus:ring-2 focus:ring-primary-container" id="setup-pass" type="password" autocomplete="new-password"><p class="text-[11px] text-outline mt-1">No se guarda en disco.</p></div>' +
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
                '<dt class="text-on-surface-variant">Máquinas</dt><dd class="mono">ns + ApacheServer</dd>' +
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
              '<p class="text-[12px] text-on-surface-variant">Ya puedes aprovisionar instancias.</p>' +
              '<div class="flex justify-between gap-2"><button class="px-3 py-1.5 border-standard rounded-xl text-[13px] font-medium" id="setup-restart-done-btn" type="button">Reiniciar</button><button class="px-3 py-1.5 text-white text-[13px] font-semibold rounded-xl" style="background:rgb(15,118,110);border:1px solid rgb(13,95,88);" id="setup-done-btn" type="button">Ir a la consola</button></div>' +
            '</div>' +
          '</section>' +
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
    document.getElementById('setup-done-btn').addEventListener('click', finish);
    var copyBtn = document.getElementById('setup-copy-log');
    if (copyBtn) {
      copyBtn.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        var text = logLines.join('\n');
        var done = function () {
          copyBtn.textContent = 'Copiado';
          setTimeout(function () { copyBtn.textContent = 'Copiar'; }, 1500);
        };
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(text).then(done).catch(function () { fallbackCopy(text); done(); });
        } else {
          fallbackCopy(text);
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
    });
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
    }
    if (name === 'listo') {
      root.querySelectorAll('[data-nav]').forEach(function (item) { item.classList.add('done'); item.classList.remove('active'); });
    }
    if (name === 'credenciales') {
      var input = document.getElementById('setup-user');
      if (input && !input.value) input.focus();
    }
    if (name === 'ejecucion') connectLogStream();
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
    if (!window.confirm('Se eliminarán las VMs ns y ApacheServer y se reiniciará la preparación. La imagen descargada se conserva. ¿Continuar?')) return;
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
      if (currentScreen() !== 'ejecucion' && currentScreen() !== 'listo') showScreen('ejecucion');
      setLive('', 'en vivo');
    } else {
      if (state.completado) {
        setLive('live', 'completado');
        showScreen('listo');
      } else if (state.error) {
        setLive('error', 'detenido');
        if (currentScreen() !== 'ejecucion') showScreen('ejecucion');
      } else {
        var live = document.getElementById('setup-live');
        if (live) live.hidden = true;
      }
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

  function open() {
    mount();
    root.hidden = false;
    if (!lastState) showScreen('bienvenida');
    else if (lastState.completado) showScreen('listo');
    else if (lastState.ejecutando) showScreen('ejecucion');
    else showScreen('bienvenida');
    var begin = document.getElementById('setup-begin-btn');
    if (begin) begin.focus();
  }

  function finish() {
    if (lastState && !lastState.completado) return;
    if (root) root.hidden = true;
    document.body.classList.remove('setup-gate');
    stopLogStream();
    stopElapsedTicker();
  }

  function init() {
    mount();
    connectStream();
    fetch('/api/setup/estado', { headers: { 'Accept': 'application/json' } })
      .then(function (res) { if (!res.ok) throw new Error('HTTP ' + res.status); return res.json(); })
      .then(function (state) {
        render(state);
        document.documentElement.classList.remove('setup-checking');
        if (state.completado) {
          root.hidden = true;
          return;
        }
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
