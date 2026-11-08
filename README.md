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
                    | Red Interna / Host-Only (192.168.10.0/24)   |
                    |                                             |
   +----------------+----------------+          +-----------------+----------------+
   |  Servidor DNS Dedicado (Bind9)  |          | Disco Base Multiconexión         |
   |  VM: ns (Debian 13.0 CLI)       |          | ApacheServer.vdi (multiattach)   |
   |  IP: 192.168.10.10/24           |          +--------+----------------+--------+
   |  Dominio: cloud.local.          |                   |                |
   +----------------+----------------+                   | Diferencial    | Diferencial
                    | Registros dinámicos                v                v
                    |                      +-------------+----+ +---------+----+
                    +--------------------> | Instancia 1:     | | Instancia 2: |
                    |                      | paisajes         | | negocios     |
                    +--------------------> | 192.168.10.30    | | 192.168.10.31|
                                           | Apache + web.zip | | Apache + .zip|
                                           +------------------+ +--------------+
```

## 📋 Requisitos Previos

- **Go**: 1.22+ (detectado `1.27.0`)
- **Oracle VirtualBox**: 7.0+ (con herramienta `VBoxManage` en el PATH)
- **Task**: Administrador de tareas `task`
- **VM Base**: Máquina virtual Debian 13 CLI con Apache instalado y su disco virtual configurado en modo `multiattach` (`ApacheServer.vdi`)
- **VM DNS**: Máquina virtual Debian 13 (`ns`) con Bind9 configurado en `192.168.10.10/24` para la zona `cloud.local.`

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

# Compilar binario de producción
task build
```

## 📁 Estructura del Proyecto

Organizado bajo **Arquitectura Hexagonal (Puertos y Adaptadores)**:

- `cmd/server/`: Punto de entrada (`main.go`) y composición de dependencias.
- `internal/core/domain/`: Entidades de negocio (`Instance`, `DnsRecord`, etc.) e invariantes.
- `internal/core/ports/`: Interfaces de puertos desacoplados (`Hypervisor`, `DnsProvider`, `Deployer`, `Repository`, `Executor`).
- `internal/adapters/`: Implementaciones concretas (`vbox`, `bind9`, `deployer`, `filestore`, `localproc`).
- `internal/services/`: Casos de uso de la aplicación (`Orchestrator`, `InstanceService`, `DnsService`).
- `internal/handlers/`: Controladores HTTP REST.
- `web/`: Frontend con panel de control web responsivo.
- `docs/`: Documentación técnica, diagramas PlantUML y reporte en LaTeX.
- `.agents/skills/`: Reglas y contexto técnico para agentes inteligentes.

## 📄 Licencia

Este proyecto está bajo la Licencia Apache 2.0. Consulta el archivo [LICENSE](LICENSE) para más detalles.

