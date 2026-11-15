// pages/salud.js
(function () {
  function init() {
    var esc = (window.HTTPaaS && window.HTTPaaS.escapeHtml) || function (s) { return s == null ? '' : String(s); };
    var raw = document.getElementById('healthRaw'), btn = document.getElementById('checkBtn');
    if (!raw || raw.dataset.bound) return;
    raw.dataset.bound = '1';

    function setText(id, value) {
      var el = document.getElementById(id);
      if (el) el.textContent = value == null || value === '' ? '—' : String(value);
    }
    function setDesc(id, value) {
      var el = document.getElementById(id);
      if (el) el.textContent = value || '';
    }
    function setPill(id, state, text) {
      var el = document.getElementById(id); if (!el) return;
      var dot = state === 'ok' ? 'dot' : state === 'err' ? 'dot err' : 'dot warn';
      var cls = state === 'ok' ? 'badge-ok' : state === 'err' ? 'badge-err' : 'badge-warn';
      el.className = 'pill-state ' + cls;
      el.innerHTML = '<span class="' + dot + '"></span><span>' + esc(text) + '</span>';
    }
    function setReadiness(kind, title, message) {
      var box = document.getElementById('health-readiness');
      var t = document.getElementById('health-readiness-title');
      var m = document.getElementById('health-readiness-message');
      if (t) t.textContent = title;
      if (m) m.textContent = message;
      if (!box) return;
      box.classList.remove('readiness-ok', 'readiness-warn', 'readiness-err', 'readiness-checking');
      if (kind) box.classList.add('readiness-' + kind);
    }

    async function fetchJSON(url) {
      var res = await fetch(url, { headers: { 'Accept': 'application/json' } });
      var data = await res.json().catch(function () { return {}; });
      return { ok: res.ok, status: res.status, data: data };
    }

    async function check() {
      if (btn) { btn.disabled = true; btn.setAttribute('aria-busy', 'true'); }
      setReadiness('checking', 'Comprobando el entorno…', 'Consultando dependencias, instancias y resolución DNS. Esto puede tardar unos segundos.');
      setPill('h-overall', 'warn', 'Comprobando…');
      setPill('h-vbox', 'warn', 'Comprobando…');
      setPill('h-bind9', 'warn', 'Comprobando…');
      setDesc('h-overall-desc', 'Resumen del estado reportado por el sistema.');
      setDesc('h-vbox-desc', 'Necesario para crear y administrar máquinas virtuales.');
      setDesc('h-bind9-desc', 'Necesario para resolver los nombres *.cloud.local.');

      var summary = {};
      var overallOk = false, vboxOk = false, bindOk = false;
      var hostResolves = null, hostMsg = '';
      var instCount = null, dnsCount = null, instOk = false, dnsOk = false;

      try {
        var health = await fetchJSON('/api/salud');
        var data = health.data || {};
        overallOk = health.ok && (data.estado === 'ok' || data.estado === 'OK');
        vboxOk = data.vboxmanage === true;
        bindOk = data.bind9 === true;
        setPill('h-overall', overallOk ? 'ok' : (health.ok ? 'err' : 'err'), overallOk ? 'Operativo' : (health.ok ? (data.estado || 'Con incidencias') : 'No disponible'));
        setPill('h-vbox', vboxOk ? 'ok' : data.vboxmanage === false ? 'err' : 'warn', vboxOk ? 'Disponible' : data.vboxmanage === false ? 'No disponible' : 'Sin datos');
        setPill('h-bind9', bindOk ? 'ok' : data.bind9 === false ? 'err' : 'warn', bindOk ? 'Disponible' : data.bind9 === false ? 'No disponible' : 'Sin datos');
        setDesc('h-overall-desc', overallOk ? 'Las dependencias principales responden correctamente.' : (health.ok ? 'Hay componentes que requieren atención.' : 'No se pudo consultar el estado del sistema.'));
        setDesc('h-vbox-desc', vboxOk ? 'VirtualBox responde y puede administrar VMs.' : data.vboxmanage === false ? 'VirtualBox no está disponible. Revisa la instalación y permisos en el anfitrión.' : 'No se pudo determinar el estado de VirtualBox.');
        setDesc('h-bind9-desc', bindOk ? 'El servicio DNS está disponible.' : data.bind9 === false ? 'La máquina virtual DNS (ns) está apagada. Pulsa “Encender ns” para iniciarla.' : 'No se pudo determinar el estado de DNS.');
        var bindAction = document.getElementById('h-bind9-action');
        if (bindAction) bindAction.classList.toggle('hidden', !(data.bind9 === false || data.ns_activa === false));
        summary.salud = health.ok ? data : { error: true, estado_http: health.status, data: data };
      } catch (e) {
        setPill('h-overall', 'err', 'No disponible');
        setPill('h-vbox', 'warn', 'Sin datos');
        setPill('h-bind9', 'warn', 'Sin datos');
        setDesc('h-overall-desc', 'No se pudo conectar con la API de salud. Verifica que el orquestador esté en ejecución.');
        summary.salud = { estado: 'error', detalle: e.message };
      }

      try {
        var inst = await fetchJSON('/api/instancias');
        var list = inst.ok ? (Array.isArray(inst.data.datos) ? inst.data.datos : (Array.isArray(inst.data) ? inst.data : [])) : [];
        instCount = inst.ok ? list.length : null;
        instOk = inst.ok;
        setText('health-instances', inst.ok ? (list.length + ' instancia' + (list.length === 1 ? '' : 's')) : 'No disponible');
        setDesc('health-instances-desc', inst.ok ? (list.length ? 'Instancias registradas en el sistema.' : 'Aún no hay instancias. Puedes crear una desde “Agregar”.') : 'No se pudo consultar el listado de instancias.');
        summary.instancias = inst.ok ? { total: list.length } : { error: true, estado_http: inst.status };
      } catch (e) {
        setText('health-instances', 'No disponible');
        setDesc('health-instances-desc', 'No se pudo consultar el listado de instancias.');
        summary.instancias = { error: e.message };
      }

      try {
        var dns = await fetchJSON('/api/dns/registros');
        var regs = dns.ok ? (Array.isArray(dns.data.registros) ? dns.data.registros : []) : [];
        dnsCount = dns.ok ? regs.length : null;
        dnsOk = dns.ok;
        setText('health-dns-records', dns.ok ? (regs.length + ' registro' + (regs.length === 1 ? '' : 's')) : 'No disponible');
        setDesc('health-dns-records-desc', dns.ok ? (regs.length ? 'Registros A visibles en la zona local.' : 'No hay registros A publicados todavía.') : 'No se pudo consultar la zona DNS.');
        summary.dns = dns.ok ? { zona: dns.data.zona, total: regs.length } : { error: true, estado_http: dns.status };
      } catch (e) {
        setText('health-dns-records', 'No disponible');
        setDesc('health-dns-records-desc', 'No se pudo consultar la zona DNS.');
        summary.dns = { error: e.message };
      }

      try {
        var host = await fetchJSON('/api/dns/anfitrion');
        var h = host.data || {};
        hostResolves = host.ok ? !!h.resuelve : null;
        hostMsg = (h && h.mensaje) || '';
        if (host.ok && h.resuelve) {
          setText('health-host-dns', 'Resuelve' + (h.ip ? (' (' + h.ip + ')') : ''));
          setDesc('health-host-dns-desc', 'El anfitrión puede resolver nombres del dominio local.');
        } else if (host.ok) {
          setText('health-host-dns', 'No resuelve');
          setDesc('health-host-dns-desc', hostMsg || 'Configura el DNS del anfitrión desde “Configurar sistemas” o el aviso del panel de instancias para acceder a *.cloud.local.');
        } else {
          setText('health-host-dns', 'No disponible');
          setDesc('health-host-dns-desc', 'No se pudo comprobar la resolución DNS del anfitrión.');
        }
        summary.resolucion_anfitrion = host.ok ? h : { error: true, estado_http: host.status, data: h };
      } catch (e) {
        hostResolves = null;
        setText('health-host-dns', 'No disponible');
        setDesc('health-host-dns-desc', 'No se pudo comprobar la resolución DNS del anfitrión.');
        summary.resolucion_anfitrion = { error: e.message };
      }

      // Preparación general y siguiente acción
      var readyCore = overallOk && vboxOk && bindOk;
      if (readyCore && hostResolves === false) {
        setReadiness('warn', 'Entorno operativo con pendiente de configuración', 'VirtualBox y DNS están disponibles, pero el anfitrión no resuelve *.cloud.local. Configura el DNS del anfitrión para abrir los sitios desde este equipo.');
      } else if (readyCore) {
        setReadiness('ok', 'Entorno listo para operar', (instCount != null ? ('Hay ' + instCount + ' instancia' + (instCount === 1 ? '' : 's') + ' registrada' + (instCount === 1 ? '' : 's') + '. ') : '') + 'Puedes crear o administrar instancias con normalidad.');
      } else if (!instOk && !dnsOk && !overallOk) {
        setReadiness('err', 'No se pudo verificar el entorno', 'Revisa que el orquestador esté en ejecución y vuelve a comprobar. Si el problema persiste, abre “Detalles técnicos”.');
      } else {
        var issues = [];
        if (!vboxOk) issues.push('VirtualBox');
        if (!bindOk) issues.push('DNS (Bind9)');
        if (!overallOk && vboxOk && bindOk) issues.push('estado general');
        setReadiness('err', 'El entorno necesita atención', (issues.length ? ('Revisa: ' + issues.join(', ') + '. ') : '') + 'Corrige los puntos marcados y vuelve a comprobar.');
      }

      if (raw) raw.textContent = JSON.stringify(summary, null, 2);
      if (window.refreshHeaderHealth) window.refreshHeaderHealth();
      if (btn) { btn.disabled = false; btn.setAttribute('aria-busy', 'false'); }
    }

    var startNsBtn = document.getElementById('startNsHealthBtn');
    if (startNsBtn && !startNsBtn.dataset.bound) {
      startNsBtn.dataset.bound = '1';
      startNsBtn.addEventListener('click', async function () {
        startNsBtn.disabled = true;
        setDesc('h-bind9-desc', 'Encendiendo máquina virtual DNS (ns)…');
        try {
          var res = await fetch('/api/dns/encender', { method: 'POST', headers: { 'Accept': 'application/json' } });
          var body = await res.json().catch(function () { return {}; });
          if (!res.ok) throw new Error(body.error || ('HTTP ' + res.status));
          await check();
          if (window.refreshInstancesPanel) window.refreshInstancesPanel();
        } catch (e) {
          setDesc('h-bind9-desc', 'No se pudo encender ns: ' + e.message);
        } finally {
          startNsBtn.disabled = false;
        }
      });
    }
    if (btn) btn.addEventListener('click', check);
    check();
  }
  window.HTTPaaSPages = window.HTTPaaSPages || {}; window.HTTPaaSPages.salud = init;
})();
