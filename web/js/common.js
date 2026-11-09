// common.js - utilidades compartidas entre páginas HTTPaaS
(function () {
  function escapeHtml(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c];
    });
  }
  function escapeAttr(s) { return escapeHtml(s).replace(/`/g, '&#96;'); }
  function formatFecha(isoString) {
    if (!isoString) return '—';
    try {
      var d = new Date(isoString), p = function (n) { return String(n).padStart(2, '0'); };
      return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes()) + ':' + p(d.getSeconds());
    } catch (e) { return isoString; }
  }
  function showMessage(text, type) {
    var el = document.getElementById('statusMessage');
    if (!el) return;
    el.textContent = text;
    el.className = 'status-' + (type || 'info');
    el.style.display = 'block';
  }
  function hideMessage() { var el = document.getElementById('statusMessage'); if (el) el.style.display = 'none'; }
  function showLoading(show, text) {
    var ov = document.getElementById('loadingOverlay'), tx = document.getElementById('loadingText');
    if (tx) tx.textContent = text || 'Procesando operación…';
    if (ov) {
      ov.classList.toggle('open', !!show);
      ov.setAttribute('aria-hidden', show ? 'false' : 'true');
      ov.setAttribute('aria-busy', show ? 'true' : 'false');
    }
  }
  window.HTTPaaS = { escapeHtml: escapeHtml, escapeAttr: escapeAttr, formatFecha: formatFecha, showMessage: showMessage, hideMessage: hideMessage, showLoading: showLoading };
})();
