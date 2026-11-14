# Preparación Inicial del Entorno

Este documento describe cómo HTTPaaS prepara el laboratorio la primera vez: descarga la imagen cloud de Debian 13, crea dos máquinas virtuales a partir de ella mediante semillas cloud-init y deja la plantilla lista para generar instancias con discos diferenciales.

> **Flujo del proyecto:** docs → tests → implementación.

---

## 1. Máquinas virtuales

| VM | Rol | Disco |
|---|---|---|
| `ns` | Bind9 para `cloud.local.` y actualizaciones dinámicas | Copia propia de la imagen (VDI) |
| `ApacheServer` | Plantilla: Apache2, Guest Additions y `/var/www/vhosts` | Copia propia en modo `multiattach`; las instancias son discos diferenciales |

Una sola imagen base: `debian-13-generic-amd64.raw`. Se descarga solo si no existe en el directorio de datos.

---

## 2. Flujo

1. **Imagen:** descarga la `.raw` oficial (~3 GB) en `<app-dir>/raw/`. Si ya está, se reutiliza.
2. **Red:** reutiliza la interfaz host-only de VirtualBox y configura DHCP en su subred. El DHCP no anuncia puerta de enlace ni DNS; las VM base llevan además una NIC NAT para descargar paquetes durante la preparación.
3. **VM `ns`:** duplica el VDI base (convertido una sola vez), redimensiona el disco, crea la ISO semilla NoCloud y arranca.
4. **Primer arranque de `ns`:** cloud-init crea el usuario, instala los paquetes y Guest Additions.
5. **Configuración de Bind9:** el orquestador ejecuta `scripts/guest-bind9.sh` con la IP del DNS y el CIDR derivados.
6. **VM plantilla:** mismo proceso con su propia semilla.
7. **Disco multiconexión:** desconecta, marca `multiattach`, reconecta y guarda la ruta.
8. **Verificación final.**

La imagen se convierte **una sola vez** a un VDI base reutilizable y cada VM recibe una copia:

```bash
VBoxManage convertfromraw debian-13-generic-amd64.raw debian-13-generic-amd64.vdi --format VDI
# por VM: copiar el VDI base y redimensionar la copia
VBoxManage modifymedium disk <vm>.vdi --resize 8192
```

Reiniciar la preparación elimina solo las copias y las semillas; el `.raw` y el VDI base se conservan.

---

## 3. Semillas cloud-init

Las plantillas viven en `scripts/seeds/`:

- `ns-user-data.yaml`
- `apache-user-data.yaml`

El orquestador sustituye `@@USUARIO@@` y `@@CONTRASENA@@` con los datos del formulario y genera:

```
<app-dir>/seeds/<vm>-user-data
<app-dir>/seeds/<vm>-meta-data
<app-dir>/seeds/<vm>-seed.iso      # NoCloud, volumen "cidata"
```

`meta-data`:

```
instance-id: iid-<hostname>-01
local-hostname: <hostname>
```

La ISO se construye con `genisoimage` (o `mkisofs` si el primero no está):

```bash
genisoimage -quiet -output <vm>-seed.iso -volid cidata -joliet -rock <vm>-user-data <vm>-meta-data
```

### 3.1 Paquetes

Cada semilla instala exactamente lo necesario:

- **`ns`:** `bind9`, `bind9-utils`, `bind9-dnsutils`, `virtualbox-guest-additions-iso`, `bzip2`, `tar`, `build-essential`, `linux-headers-amd64`.
- **Plantilla:** `apache2`, `unzip`, `virtualbox-guest-additions-iso`, `bzip2`, `tar`, `build-essential`, `linux-headers-amd64`.

Guest Additions se compilan desde la ISO que instala el paquete Debian. `unzip` es necesario para extraer el contenido `.zip` de cada instancia.

Las semillas además:

- habilitan `contrib` y `non-free` para el paquete de Guest Additions;
- crean el usuario con `sudo` sin contraseña (requisito de `guestcontrol`);
- dejan la red en DHCP (la reserva de IP se hace por MAC desde el host).

---

## 4. Pantallas

| Pantalla | Contenido |
|---|---|
| Bienvenida | Alcance y duración estimada |
| Credenciales | Usuario y contraseña del invitado |
| Confirmación | Usuario, red derivada y máquinas |
| Ejecución | Pasos automáticos con progreso, error y registro en vivo |
| Listo | Acceso a la consola |

La contraseña solo se mantiene en memoria y se usa para renderizar las semillas; no se persiste en disco.

---

## 5. Contrato HTTP

| Método | Ruta | Propósito |
|---|---|---|
| `GET` | `/api/setup/estado` | Estado completo (pasos, progreso, red, imagen y error) |
| `POST` | `/api/setup/configuracion` | Valida y guarda usuario y contraseña |
| `POST` | `/api/setup/iniciar` | Arranca la preparación (`409` si falta configuración o ya corre) |
| `POST` | `/api/setup/reiniciar` | Elimina las VMs base y sus semillas, limpia el progreso y redetecta (conserva imagen y red) |
| `GET` | `/api/setup/stream` | SSE con una instantánea del estado por cada cambio |

Mientras no esté completa, la consola queda bloqueada y la API rechaza escrituras con `409`.

---

## 6. Directorio de datos

Resolución: `HTTPAAS_DIR` → `/var/lib/httpaas` (si es escribible) → `$XDG_DATA_HOME/httpaas` → `~/.local/share/httpaas`.

Contenido:

- `raw/` — imagen `.raw` descargada.
- `seeds/` — user-data, meta-data e ISO de cada semilla.
- `instances.json` — inventario.
- `setup-estado.json` — avance de la preparación.

La aplicación no lee ni escribe fuera de este directorio.

---

## 7. Requisitos

- VirtualBox 7.x con `VBoxManage`.
- `genisoimage` o `mkisofs` para crear las semillas.
- Internet la primera vez (imagen y paquetes).
- Sin privilegios de administrador: no se reconfiguran interfaces del host.
- Las VM base usan una NIC NAT temporal para instalar paquetes; las instancias aprovisionadas solo llevan la NIC host-only.

---

## 8. Criterios de aceptación

- `POST /api/setup/configuracion` valida usuario y contraseña y no persiste la contraseña.
- La imagen se reutiliza si existe y solo se descarga si falta.
- Cada VM arranca con su semilla, obtiene IP por DHCP y Guest Additions responde.
- Bind9 resuelve `ns.cloud.local` y la plantilla queda en `multiattach`.
- Reiniciar el proceso retoma desde el primer paso no completado.
