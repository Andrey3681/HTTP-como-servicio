// app.js - Lógica del Dashboard de Gestión de Páginas y Aplicaciones Web (HTTPaaS)

document.addEventListener('DOMContentLoaded', () => {
  const provisionForm = document.getElementById('provisionForm');
  const hostnameInput = document.getElementById('hostnameInput');
  const zipInput = document.getElementById('zipInput');
  const checkHostBtn = document.getElementById('checkHostBtn');
  const statusMessage = document.getElementById('statusMessage');
  const instancesTableBody = document.getElementById('instancesTableBody');
  const loadingOverlay = document.getElementById('loadingOverlay');
  const loadingText = document.getElementById('loadingText');

  // Cargar instancias al inicio
  fetchInstances();

  // Botón "Aceptar": Valida el formato y disponibilidad del host
  checkHostBtn.addEventListener('click', async () => {
    const hostname = hostnameInput.value.trim().toLowerCase();
    if (!hostname) {
      showMessage('Por favor ingrese un nombre de host.', 'error');
      return;
    }

    const hostRegex = /^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/;
    if (!hostRegex.test(hostname)) {
      showMessage('El nombre de host debe tener entre 3 y 32 caracteres (solo minúsculas, números y guiones).', 'error');
      return;
    }

    try {
      const response = await fetch(`/api/instancias/${encodeURIComponent(hostname)}`);
      if (response.status === 404) {
        showMessage(`El nombre de host "${hostname}" está disponible.`, 'success');
      } else if (response.ok) {
        showMessage(`El nombre de host "${hostname}" ya está en uso. Elija otro.`, 'error');
      } else {
        showMessage('No se pudo verificar la disponibilidad del host.', 'info');
      }
    } catch (err) {
      showMessage(`El nombre "${hostname}" parece válido. Verifique al publicar.`, 'info');
    }
  });

  // Envío del formulario: Publicar
  provisionForm.addEventListener('submit', async (e) => {
    e.preventDefault();

    const hostname = hostnameInput.value.trim().toLowerCase();
    const file = zipInput.files[0];

    if (!hostname) {
      showMessage('Ingrese el nombre del host.', 'error');
      return;
    }
    if (!file) {
      showMessage('Seleccione un archivo comprimido .zip.', 'error');
      return;
    }
    if (!file.name.endsWith('.zip')) {
      showMessage('El archivo debe tener extensión .zip.', 'error');
      return;
    }

    const formData = new FormData();
    formData.append('nombre', hostname);
    formData.append('archivo', file);

    showLoading(true, `Aprovisionando VM "${hostname}" en VirtualBox y registrando en DNS...`);
    hideMessage();

    try {
      const response = await fetch('/api/instancias', {
        method: 'POST',
        body: formData,
      });

      const result = await response.json();
      if (!response.ok) {
        throw new Error(result.error || 'Error al aprovisionar la instancia.');
      }

      showMessage(`¡Instancia "${hostname}" aprovisionada con éxito!`, 'success');
      provisionForm.reset();
      fetchInstances();
    } catch (err) {
      showMessage(`Error en el aprovisionamiento: ${err.message}`, 'error');
    } finally {
      showLoading(false);
    }
  });

  // Consultar instancias
  async function fetchInstances() {
    try {
      const res = await fetch('/api/instancias');
      if (!res.ok) throw new Error('Error al consultar instancias.');
      const data = await res.json();
      renderInstances(data.datos || []);
    } catch (err) {
      console.error(err);
    }
  }

  // Renderizar tabla
  function renderInstances(instances) {
    if (!instances || instances.length === 0) {
      instancesTableBody.innerHTML = `
        <tr class="empty-row">
          <td colspan="5">No hay instancias aprovisionadas en este momento.</td>
        </tr>
      `;
      return;
    }

    instancesTableBody.innerHTML = instances.map(inst => {
      const enlace = inst.enlace || `http://${inst.nombre}.cloud.local`;
      const ip = inst.ip || '192.168.10.x';
      const fecha = formatFecha(inst.creado_en);

      return `
        <tr>
          <td><a href="${enlace}" target="_blank" rel="noopener noreferrer">${enlace}</a></td>
          <td>${ip}</td>
          <td>${inst.nombre}</td>
          <td>${fecha}</td>
          <td>
            <button class="btn btn-danger" onclick="deleteInstance('${inst.id || inst.nombre}')">Eliminar</button>
          </td>
        </tr>
      `;
    }).join('');
  }

  // Eliminar instancia
  window.deleteInstance = async function(id) {
    if (!confirm(`¿Está seguro de que desea eliminar y destruir la instancia "${id}"?`)) {
      return;
    }

    showLoading(true, `Destruyendo VM "${id}" y liberando registro DNS...`);
    try {
      const res = await fetch(`/api/instancias/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      });
      if (!res.ok) {
        const errorData = await res.json().catch(() => ({}));
        throw new Error(errorData.error || 'Error al eliminar la instancia.');
      }
      showMessage(`Instancia "${id}" eliminada exitosamente.`, 'success');
      fetchInstances();
    } catch (err) {
      showMessage(`Error al eliminar: ${err.message}`, 'error');
    } finally {
      showLoading(false);
    }
  };

  function showMessage(text, type) {
    statusMessage.textContent = text;
    statusMessage.className = `status-message status-${type}`;
    statusMessage.style.display = 'block';
  }

  function hideMessage() {
    statusMessage.style.display = 'none';
  }

  function showLoading(show, text = '') {
    loadingText.textContent = text;
    loadingOverlay.style.display = show ? 'flex' : 'none';
  }

  function formatFecha(isoString) {
    if (!isoString) return '---';
    try {
      const date = new Date(isoString);
      const pad = (n) => String(n).padStart(2, '0');
      const yyyy = date.getFullYear();
      const mm = pad(date.getMonth() + 1);
      const dd = pad(date.getDate());
      const hh = pad(date.getHours());
      const min = pad(date.getMinutes());
      const ss = pad(date.getSeconds());
      return `${yyyy}-${mm}-${dd} ${hh}:${min}:${ss}`;
    } catch {
      return isoString;
    }
  }
});

