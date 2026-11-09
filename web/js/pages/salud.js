// pages/salud.js
(function () {
  function init() {
    var esc = (window.HTTPaaS && window.HTTPaaS.escapeHtml) || function (s) { return s == null ? '' : String(s); };
    var raw = document.getElementById('healthRaw'), btn = document.getElementById('checkBtn');
    if (!raw || raw.dataset.bound) return;
    raw.dataset.bound = '1';
    function setPill(id, state, text) {
      var el = document.getElementById(id); if (!el) return;
      var dot = state === 'ok' ? 'dot' : state === 'err' ? 'dot err' : 'dot warn';
      var cls = state === 'ok' ? 'badge-ok' : state === 'err' ? 'badge-err' : 'badge-warn';
      el.className = 'pill-state ' + cls; el.innerHTML = '<span class="' + dot + '"></span><span>' + esc(text) + '</span>';
    }
    async function check() {
      try {
        var res = await fetch('/api/salud'); var data = await res.json().catch(function () { return {}; });
        var overallOk = res.ok && (data.estado === 'ok' || data.estado === 'OK');
        setPill('h-overall', overallOk ? 'ok' : 'err', overallOk ? 'OK' : (data.estado || 'ERROR'));
        setPill('h-vbox', data.vboxmanage === true ? 'ok' : data.vboxmanage === false ? 'err' : 'warn', data.vboxmanage === true ? 'Disponible' : data.vboxmanage === false ? 'No disponible' : 'Sin datos');
        setPill('h-bind9', data.bind9 === true ? 'ok' : data.bind9 === false ? 'err' : 'warn', data.bind9 === true ? 'Disponible' : data.bind9 === false ? 'No disponible' : 'Sin datos');
        if (raw) raw.textContent = JSON.stringify(data, null, 2);
      } catch (e) {
        var fb = { estado: 'error', vboxmanage: null, bind9: null, detalle: e.message };
        setPill('h-overall', 'err', 'ERROR'); setPill('h-vbox', 'warn', 'Sin datos'); setPill('h-bind9', 'warn', 'Sin datos');
        if (raw) raw.textContent = JSON.stringify(fb, null, 2);
      }
    }
    if (btn) btn.addEventListener('click', check);
    check();
  }
  window.HTTPaaSPages = window.HTTPaaSPages || {}; window.HTTPaaSPages.salud = init;
})();
