// logs-dock.js - Componente compartido: dock de logs (Stitch). Abre/cierra con animación de deslizamiento.
(function () {
  var STYLE_ID = 'logs-dock-anim-style';
  function ensureStyles() {
    if (document.getElementById(STYLE_ID)) return;
    var s = document.createElement('style');
    s.id = STYLE_ID;
    s.textContent =
      '#system-logs-dock{transition:transform .28s ease; will-change:transform;}' +
      '#system-logs-dock.logs-closed{transform:translateY(calc(100% + 2rem)); pointer-events:none;}' +
      '#system-logs-dock.logs-open{transform:translateY(0); pointer-events:auto;}';
    document.head.appendChild(s);
  }

  function mountToggle() {
    var mount = document.getElementById('logs-toggle-mount');
    if (!mount || mount.dataset.ready) return;
    mount.dataset.ready = '1';
    mount.innerHTML =
      '<button id="toggle-logs-btn" class="inline-flex items-center gap-1.5 px-2.5 py-1 text-[#e2e8f0] mono text-[11px] transition-colors cursor-pointer" title="Mostrar/Ocultar logs del sistema" style="background-color:rgb(36,40,50);border:1px solid rgb(51,56,66);border-radius:4px;" type="button">' +
        '<span class="material-symbols-outlined text-[14px] text-[#94a3b8]">terminal</span>' +
        '<span class="font-semibold">Logs del Sistema</span>' +
        '<span class="px-1.5 py-0.2 text-[10px] text-[#e2e8f0] font-semibold flex items-center gap-1" style="background-color:#27272a;border:1px solid #3f3f46;border-radius:4px;"><span class="w-1.5 h-1.5 rounded-full bg-[#10b981] animate-pulse"></span>live</span>' +
        '<span class="material-symbols-outlined text-[14px] ml-0.5 text-[#94a3b8]" id="dock-chevron-icon">expand_more</span>' +
      '</button>';
  }

  function mountDock() {
    if (document.getElementById('system-logs-dock')) return;
    ensureStyles();
    var main = document.querySelector('main');
    if (main) main.style.paddingBottom = '18rem';
    var wrap = document.createElement('div');
    wrap.innerHTML =
      '<div class="fixed bottom-8 left-0 right-0 z-40 flex flex-col text-[#e2e8f0] select-none rounded-t border-t shadow-lg logs-open" id="system-logs-dock" style="height:260px;background-color:rgb(30,34,41);border-top:1px solid rgb(51,56,66);border-top-left-radius:4px;border-top-right-radius:4px;">' +
        '<div class="w-full h-2 cursor-row-resize flex items-center justify-center group" id="logs-resize-handle" style="background-color:rgb(36,40,50);border-bottom:1px solid rgb(51,56,66);" title="Arrastrar para redimensionar"><div class="w-12 h-1 rounded bg-[#71717a] group-hover:bg-[#a1a1aa]"></div></div>' +
        '<div class="h-9 px-3 sm:px-4 flex items-center justify-between text-xs mono shrink-0 gap-2" style="background-color:rgb(36,40,50);border-bottom:1px solid rgb(51,56,66);">' +
          '<div class="flex items-center gap-2 min-w-0">' +
            '<span class="material-symbols-outlined text-[16px] text-[#94a3b8]">terminal</span>' +
            '<span class="font-semibold text-[#e2e8f0]">Logs</span>' +
            '<span class="px-1.5 py-0.5 text-[10px] flex items-center gap-1 font-semibold" style="background-color:#1e2229;color:#e2e8f0;border:1px solid #333842;border-radius:4px;"><span class="w-1.5 h-1.5 rounded-full bg-[#10b981] animate-pulse"></span>live</span>' +
          '</div>' +
          '<div class="flex items-center gap-2 shrink-0">' +
            '<button class="px-2 py-0.5 flex items-center gap-1 text-[11px] text-[#d4d4d8] hover:text-white" id="logs-clear-btn" type="button" style="background-color:#1e2229;border:1px solid #333842;border-radius:4px;"><span class="material-symbols-outlined text-[13px]">mop</span><span class="hidden sm:inline">Limpiar</span></button>' +
            '<button class="p-1 text-[#94a3b8] hover:text-white hover:bg-[#2e3440]" id="logs-close-btn" type="button" title="Cerrar logs" style="border-radius:4px;"><span class="material-symbols-outlined text-[16px]">keyboard_arrow_down</span></button>' +
          '</div>' +
        '</div>' +
        '<div class="flex-1 overflow-y-auto p-3 mono text-[11px] leading-relaxed select-text" id="terminal-output-container" style="background-color:rgb(22,24,29);color:rgb(212,212,216);"></div>' +
      '</div>';
    document.body.appendChild(wrap.firstChild);
  }

  function bind() {
    var dock = document.getElementById('system-logs-dock');
    if (!dock || dock.dataset.bound) return;
    dock.dataset.bound = '1';
    var handle = document.getElementById('logs-resize-handle');
    var toggleBtn = document.getElementById('toggle-logs-btn');
    var closeBtn = document.getElementById('logs-close-btn');
    var chev = document.getElementById('dock-chevron-icon');
    var clearBtn = document.getElementById('logs-clear-btn');
    var term = document.getElementById('terminal-output-container');
    var open = true, prevH = 260, dragging = false, startY = 0, startH = 260;

    function setOpen(on) {
      open = !!on;
      dock.classList.toggle('logs-open', open);
      dock.classList.toggle('logs-closed', !open);
      if (chev) chev.textContent = open ? 'expand_more' : 'expand_less';
    }
    window.toggleLogsDock = function () { setOpen(!open); };
    window.closeLogsDock = function () { setOpen(false); };
    window.openLogsDock = function () { setOpen(true); };
    if (toggleBtn) toggleBtn.addEventListener('click', window.toggleLogsDock);
    if (closeBtn) closeBtn.addEventListener('click', window.closeLogsDock);
    if (clearBtn) clearBtn.addEventListener('click', function () { if (term) term.innerHTML = ''; });
    if (handle) {
      handle.addEventListener('mousedown', function (e) { if (!open) return; dragging = true; startY = e.clientY; startH = parseInt(getComputedStyle(dock).height, 10); document.body.style.userSelect = 'none'; document.body.style.cursor = 'row-resize'; });
      window.addEventListener('mousemove', function (e) { if (!dragging) return; var nh = startH + (startY - e.clientY); if (nh < 120) nh = 120; if (nh > window.innerHeight - 100) nh = window.innerHeight - 100; dock.style.height = nh + 'px'; prevH = nh; });
      window.addEventListener('mouseup', function () { if (dragging) { dragging = false; document.body.style.userSelect = ''; document.body.style.cursor = ''; } });
    }
  }

  function init() { mountToggle(); mountDock(); bind(); }
  window.mountLogsDock = init;
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init); else init();
})();
