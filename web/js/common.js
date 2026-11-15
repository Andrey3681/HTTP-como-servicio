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
  var loadingTimer = null, loadingBase = '', loadingStarted = 0;

  function paintLoading() {
    var tx = document.getElementById('loadingText');
    if (!tx) return;
    var secs = Math.floor((Date.now() - loadingStarted) / 1000);
    tx.textContent = loadingBase + ' · ' + secs + ' s';
  }

  function showLoading(show, text) {
    var ov = document.getElementById('loadingOverlay');
    if (show) {
      loadingBase = text || 'Procesando operación…';
      loadingStarted = Date.now();
      paintLoading();
      if (!loadingTimer) loadingTimer = setInterval(paintLoading, 1000);
    } else if (loadingTimer) {
      clearInterval(loadingTimer);
      loadingTimer = null;
    }
    if (ov) {
      ov.classList.toggle('open', !!show);
      ov.setAttribute('aria-hidden', show ? 'false' : 'true');
      ov.setAttribute('aria-busy', show ? 'true' : 'false');
    }
  }
  window.HTTPaaS = { escapeHtml: escapeHtml, escapeAttr: escapeAttr, formatFecha: formatFecha, showMessage: showMessage, hideMessage: hideMessage, showLoading: showLoading };
})();

// Tooltip propio para elementos con data-tooltip (visible al pasar el ratón o
// al enfocar con el teclado, sin depender del tooltip nativo del navegador).
(function () {
  var tip = null;

  function ensureTip() {
    if (!tip) {
      tip = document.createElement('div');
      tip.className = 'httpaas-tooltip';
      tip.setAttribute('role', 'tooltip');
      document.body.appendChild(tip);
    }
    return tip;
  }

  function place(el) {
    var text = el.getAttribute('data-tooltip') || '';
    if (!text) return;
    var t = ensureTip();
    t.textContent = text;
    t.classList.add('open');
    var r = el.getBoundingClientRect();
    var left = r.left + r.width / 2 - t.offsetWidth / 2;
    left = Math.max(8, Math.min(left, window.innerWidth - t.offsetWidth - 8));
    var top = r.top - t.offsetHeight - 8;
    if (top < 8) top = r.bottom + 8;
    t.style.left = left + 'px';
    t.style.top = top + 'px';
  }

  function hide() {
    if (tip) tip.classList.remove('open');
  }

  function target(node) {
    return node && node.closest ? node.closest('[data-tooltip]') : null;
  }

  function bind() {
    document.addEventListener('pointerover', function (e) {
      var el = target(e.target);
      if (el) place(el);
    });
    document.addEventListener('pointerout', function (e) {
      var el = target(e.target);
      if (!el) return;
      if (e.relatedTarget && el.contains(e.relatedTarget)) return;
      hide();
    });
    document.addEventListener('focusin', function (e) {
      var el = target(e.target);
      if (el) place(el);
    });
    document.addEventListener('focusout', hide);
    window.addEventListener('scroll', hide, true);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', bind);
  } else {
    bind();
  }
})();
