# Lista de Seguimiento y Tareas (TODO) - HTTPaaS

## Objetivos del Proyecto (según la Especificación)

### Objetivo General
Diseñar e implementar un entorno local automatizado para el aprovisionamiento y despliegue de servicios HTTP con Apache Web Server, integrados en un dominio DNS gestionado con Bind9, empleando infraestructura virtualizada y técnicas básicas de automatización que simulen el comportamiento de una plataforma real de servicios en la nube.

### Objetivos Específicos
- [x] **OE1:** Comprender el funcionamiento de un servidor DNS y desplegar el servidor Bind9 para gestionar los nombres de host de la zona local `cloud.local.`, incluyendo el registro automático de nuevos hosts mediante actualizaciones dinámicas.
- [x] **OE2:** Desarrollar los mecanismos de aprovisionamiento y gestión del servicio, automatizando la creación, configuración y eliminación de instancias del servidor HTTP Apache.
- [x] **OE3:** Automatizar el despliegue de las páginas o aplicaciones web solicitadas por el usuario a partir de archivos `.zip` en instancias Apache con discos en modo multiconexión.

## 1. Infraestructura Base y Virtualización
> Automatizable desde el preparación de primer arranque (`docs/setup-inicial.md`, `/api/setup/iniciar`).
- [x] Desplegar y configurar la máquina virtual dedicada `ns` (Debian 13.0 CLI).
  - [x] Usar la subred host-only asignada por VirtualBox (DNS derivado en `.10`).
  - [x] Instalar y configurar Bind9 (`named`) para la zona `cloud.local.`.
  - [x] Habilitar actualizaciones dinámicas (`allow-update` por subred y `nsupdate` desde el orquestador).
- [x] Configurar la máquina virtual plantilla base `ApacheServer` (Debian 13.0 CLI).
  - [x] Instalar Apache Web Server y verificar que inicie automáticamente.
  - [x] Habilitar SSH con la clave del laboratorio (Guest Additions no es necesario: la imagen incluye el módulo `vboxguest`, pero no el servicio de usuario).
  - [x] Habilitar servidor SSH y permisos de escritura en `/var/www/vhosts`.
  - [x] Marcar el disco `ApacheServer.vdi` en modo multiconexión (`multiattach`):
    `VBoxManage modifymedium disk ApacheServer.vdi --type multiattach`.

## 2. Dominio y Puertos Hexagonales (Go)
- [x] Definir modelos de dominio e invariantes (`internal/core/domain`).
- [x] Definir contratos de puertos (`internal/core/ports`).
- [x] Implementar pruebas unitarias de invariantes del dominio (`domain_test.go`).

## 3. Adaptadores Secundarios (Infraestructura)
- [x] Adaptador `localproc` (`ports.Executor` sobre `os/exec`).
- [x] Adaptador `vbox` (`ports.Hypervisor` sobre `VBoxManage` CLI con `sync.Mutex`).
- [x] Adaptador `bind9` (`ports.DnsProvider` sobre `nsupdate` o SSH).
- [x] Adaptador `deployer` (`ports.Deployer` sobre `guestcontrol` o SSH/SCP).
- [x] Adaptador `filestore` (`ports.Repository` sobre JSON atómico / SQLite).

## 4. Servicios de Aplicación y Orquestación
- [x] Implementar servicio de orquestación transaccional (`internal/services/orchestrator.go`).
- [x] Implementar política de rollback si falla la asignación de IP, registro DNS o despliegue.
- [x] Implementar servicio de consulta y eliminación de instancias (`internal/services/instance.go`).
- [x] Suite de pruebas de servicios con mocks/fakes de puertos (`orchestrator_test.go`).

## 5. Adaptadores Primarios e Interfaz Web
- [x] Implementar handlers REST (`internal/handlers/instance_handler.go`, `dns_handler.go`).
- [x] Implementar middleware de logging, CORS y recuperación ante pánicos.
- [x] Implementar sistema de logs estructurado (`internal/logging`, `ports.EventLogger`): anillo en memoria, `GET/DELETE /api/logs`, SSE en `/api/logs/stream` con reanudación por `Last-Event-ID`, y sanitización de mensajes `500`.
- [x] Construir panel web estático en `web/` (HTML, CSS y JS).
  - [x] Formulario de carga con campo de texto y selector de `.zip`.
  - [x] Validación asíncrona de disponibilidad de subdominio.
  - [x] Tabla de instancias activas con enlace `http://<host>.cloud.local` (target="_blank").
  - [x] Botón rojo para eliminar instancia con diálogo de confirmación.
  - [x] Dock global de logs conectado a `/api/logs` y `/api/logs/stream`.

## 6. Documentación e Informe Académico
- [x] Crear especificación arquitectónica (`docs/arquitectura.md`).
- [x] Crear especificación de diseño (`docs/diseno-solucion.md`).
- [x] Crear especificación OpenAPI (`docs/openapi.yaml`).
- [x] Crear inventario de endpoints (`docs/api/endpoints.md`).
- [x] Documentar módulos web y sistema de logs (`docs/modulos-web.md`, `docs/sistema-logs.md`).
- [x] Crear diagramas PlantUML (`docs/diagramas/`).
- [x] Plantilla del informe IEEEtran en LaTeX (`docs/informe/`).
- [x] Compilar informe en PDF (`task informe`, `docs/informe/main.pdf`).
- [x] Migrar `.golangci.yml` al esquema v2 (golangci-lint 2.x) y resolver los hallazgos de `errcheck`.
- [x] Diseñar el configuración inicial (`docs/setup-inicial.md`) con contrato HTTP y pasos.
- [x] Implementar la preparación: backend (`services.SetupService`, `adapters/provisioner`), endpoints y ventana de primer arranque con progreso por pasos.
- [x] Alinear el adaptador `vbox` con la realidad: red host-only y discos diferenciales explícitos.
- [ ] Ejecutar la preparación en el host y registrar las evidencias (ISO, VMs, multiattach, Bind9 resolviendo).
- [ ] Redactar detalles finales del informe tras desplegar el laboratorio físico.
- [ ] Tomar capturas de pantalla de evidencias y almacenarlas en `docs/informe/images/`.

## 7. Brechas Conocidas (aceptadas)
- [ ] `/api/salud` reporta `bind9: true` de forma estática; el contrato OpenAPI indica verificación real. Pendiente de decisión: sondear el proveedor DNS (requiere docs → tests → implementación) o ajustar el contrato.
- [ ] `GET /api/vhosts` no existe: la vista M5 deriva sus datos del listado de instancias.
- [ ] Logs solo en memoria (sin persistencia en disco ni exportación).
