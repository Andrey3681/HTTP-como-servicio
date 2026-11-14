# HTTPaaS - Servicio Administrado de Páginas y Aplicaciones Web

Sistema de aprovisionamiento automatizado y no interactivo de instancias de servidor web Apache sobre máquinas virtuales Oracle VirtualBox con discos en modo multiconexión (`multiattach`) y registro dinámico en DNS Bind9 (`cloud.local.`), desarrollado en Go bajo Arquitectura Hexagonal.

Proyecto correspondiente a la **Nota Parcial #3 de Computación en la Nube (2026-2)**.

---

## 🚀 Arquitectura General

```
                      +------------------------------------------+
                      |         Equipo Anfitrión (Host)          |
                      |   Orquestador Web en Go (HTTPaaS Core)   |
                      |          Puerto Web: :8080               |
                      +--------------------+---------------------+
                                           |
                    +----------------------+----------------------+
                    | Red Host-Only (subred asignada por VirtualBox) |
                    |                                             |
   +----------------+----------------+          +-----------------+----------------+
   |  Servidor DNS Dedicado (Bind9)  |          | Disco Base Multiconexión         |
   |  VM: ns (Debian 13.0 CLI)       |          | ApacheServer.vdi (multiattach)   |
   |  IP: subred + .10 (derivada)     |          +--------+----------------+--------+
   |  Dominio: cloud.local.          |                   |                |
   +----------------+----------------+                   | Diferencial    | Diferencial
                    | Registros dinámicos                v                v
                    |                      +-------------+----+ +---------+----+
                    +--------------------> | Instancia 1:     | | Instancia 2: |
                    |                      | paisajes         | | negocios     |
                    +--------------------> | subred .100      | | subred .101  |
                                           | Apache + web.zip | | Apache + .zip|
                                           +------------------+ +--------------+
```

## 📋 Requisitos Previos

- **Go**: 1.22+ (detectado `1.27.0`)
- **Oracle VirtualBox**: 7.0+ (con herramienta `VBoxManage` en el PATH)
- **Task**: Administrador de tareas `task`
- **VM Base**: Máquina virtual Debian 13 CLI con Apache instalado y su disco virtual configurado en modo `multiattach` (`ApacheServer.vdi`)
- **VM DNS**: Máquina virtual Debian 13 (`ns`) con Bind9 en la IP derivada de la red host-only (`.10`) para la zona `cloud.local.`

## 🧭 Preparación inicial del laboratorio

La consola permanece **bloqueada** hasta que el laboratorio esté preparado: una pantalla guía el proceso (bienvenida → credenciales → confirmación → ejecución) y la API rechaza escrituras (`409`) mientras falte.

1. Pide el **usuario y la contraseña** del invitado; se usan en las semillas cloud-init.
2. Descarga la imagen cloud oficial de Debian 13 (`debian-13-generic-amd64.raw`, ~3 GB) si no existe en el directorio de datos.
3. Convierte la imagen a VDI con `VBoxManage convertfromraw` y redimensiona el disco.
4. Genera una ISO semilla por VM (`genisoimage`, volumen `cidata`) desde `scripts/seeds/`.
5. Arranca **`ns`** y la plantilla **`ApacheServer`**; cloud-init crea el usuario, instala los paquetes y Guest Additions.
6. Ejecuta `scripts/guest-bind9.sh` y `scripts/guest-apache.sh` para la zona DNS y el vhost.
7. Marca el disco de la plantilla en `multiattach` y guarda su ruta para los diferenciales.

### Directorio de datos

| Orden | Ruta | Uso |
|---|---|---|
| 1 | `HTTPAAS_DIR` | anula cualquier otra resolución |
| 2 | `/var/lib/httpaas` | si existe y es escribible |
| 3 | `$XDG_DATA_HOME/httpaas` o `~/.local/share/httpaas` | ejecución sin privilegios |

Contenido: `raw/` (imagen descargada), `seeds/` (user-data, meta-data e ISOs), `instances.json` y `setup-estado.json`. La aplicación no lee ni escribe fuera de este directorio.

El progreso se muestra paso a paso con un registro en vivo y se puede reabrir desde **Configuración** en el pie. Contrato completo en [`docs/setup-inicial.md`](docs/setup-inicial.md).

Flags relevantes: `-app-dir`, `-raw-url`, `-guest-user`, `-guest-pass`, `-install-timeout`, `-setup-state`.

## 🛠️ Comandos de Desarrollo (Taskfile)

```bash
# Verificar herramientas del sistema
task doctor

# Ejecutar el servidor orquestador
task run

# Ejecutar suite de pruebas unitarias
task test

# Formatear código Go
task fmt

# Análisis estático (linter)
task lint

# Validación integral antes de commit
task check

# Regenerar documentación Markdown de paquetes Go
task doc

# Compilar el informe LaTeX (docs/informe/main.pdf)
task informe

# Compilar binario de producción
task build
```

## 📁 Estructura del Proyecto

Organizado bajo **Arquitectura Hexagonal (Puertos y Adaptadores)**:

- `cmd/server/`: Punto de entrada (`main.go`) y composición de dependencias.
- `internal/core/domain/`: Entidades de negocio (`Instance`, `DnsRecord`, etc.) e invariantes.
- `internal/core/ports/`: Interfaces de puertos desacoplados (`Hypervisor`, `DnsProvider`, `Deployer`, `Repository`, `Executor`, `EventLogger`).
- `internal/adapters/`: Implementaciones concretas (`vbox`, `bind9`, `deployer`, `filestore`, `localproc`).
- `internal/logging/`: Modelo de eventos y broker en memoria (anillo + suscripción SSE) que implementa `ports.EventLogger`.
- `internal/services/`: Casos de uso de la aplicación (`Orchestrator`, `InstanceService`).
- `internal/handlers/`: Controladores HTTP REST (instancias, DNS, salud, logs) y middleware.
- `web/`: Frontend modular (páginas, componentes compartidos, router y dock de logs).
- `docs/`: Documentación técnica, diagramas PlantUML y reporte en LaTeX.
- `.agents/skills/`: Reglas y contexto técnico para agentes inteligentes.

## 📄 Licencia

Este proyecto está bajo la Licencia Apache 2.0. Consulta el archivo [LICENSE](LICENSE) para más detalles.

