# Guía Maestra de Estructura, Documentación, Skills y Distribución para el Proyecto HTTPaaS

Esta guía contiene las instrucciones detalladas para inicializar, estructurar y desarrollar el nuevo proyecto **«Servicio Administrado de Páginas y Aplicaciones Web (HTTPaaS)»** correspondiente a la **Nota Parcial #3 de Computación en la Nube (2026-2)**.

El nuevo proyecto adopta y refina el estándar de ingeniería, calidad y automatización de `vm-console-automation`, aplicando **Arquitectura Hexagonal en Go**, ciclo de desarrollo **Docs → Tests → Implementación (TDD)**, gestión automatizada con **Taskfile**, documentación técnica en **Markdown + PlantUML + OpenAPI**, informe académico formal en **LaTeX (IEEEtran)** y un conjunto de **Skills para Agentes de IA (`.agents/skills`)** adaptadas al hipervisor VirtualBox con discos multiconexión y al servidor DNS Bind9.

> **Nota:** las direcciones `192.168.10.x` de esta guía son ilustrativas. La implementación deriva la subred de la interfaz host-only que asigna VirtualBox (DNS en `.10`, plantilla en `.11` y DHCP en `.10–.199`).

---

## 1. Contexto del Proyecto Destino (HTTPaaS)

### 1.1 Objetivo del Sistema
Construir una plataforma local de tipo **PaaS / IaaS** que permita a usuarios solicitar, mediante un panel web en Go, el aprovisionamiento automatizado y no interactivo de servidores HTTP Apache independientes. Cada instancia se creará a partir de una máquina virtual base con disco en **modo multiconexión**, se le asignará un nombre de host único registrado dinámicamente en un servidor **DNS Bind9** local (`cloud.local.`), y se le desplegará el contenido web suministrado por el usuario en formato `.zip`.

### 1.2 Componentes de la Infraestructura
```
                      +------------------------------------------+
                      |         Equipo Anfitrión (Host)          |
                      |   Orquestador HTTPaaS en Go (:8080)      |
                      +--------------------+---------------------+
                                           |
                    +----------------------+----------------------+
                    | Red host-only (subred derivada por VBox)    |
                    |                                             |
   +----------------+----------------+          +-----------------+----------------+
   |  Servidor DNS                   |          | Disco base multiconexión          |
   |  VM: ns (Debian 13)             |          | ApacheServer.vdi (multiattach)    |
   |  IP fija: subred + .10          |          +--------+----------------+---------+
   |  Dominio: cloud.local.          |                   |                |
   +----------------+----------------+                   | Diferencial    | Diferencial
                    | Registros dinámicos                v                v
                    |                      +-------------+----+ +---------+----+
                    +--------------------> | Instancia 1      | | Instancia 2  |
                    |                      | paisajes         | | negocios     |
                    +--------------------> | DHCP .100-.199   | | DHCP .100-.199|
                                           | Apache + web.zip | | Apache + .zip|
                                           +------------------+ +--------------+
```

### 1.3 Casos de Uso Fundamentales
1. **Aprovisionar Instancia:**
   - El usuario envía nombre de host (ej. `paisajes`) y archivo `.zip` con el sitio web.
   - El orquestador crea la VM en VirtualBox reutilizando el disco multiconexión `ApacheServer.vdi`.
   - Se configura la identidad de red (hostname y dirección dinámica por DHCP en el rango `.100-.199` de la subred derivada).
   - Se registra el registro `A` en el servidor DNS Bind9 (`paisajes.cloud.local.` -> `<ip-dhcp>`).
   - Se transfiere y descomprime el archivo `.zip` en `/var/www/vhosts/<host>/public/`.
   - La instancia queda visible en el dashboard con enlace activo `http://paisajes.cloud.local`.
2. **Consultar Instancias:**
   - Panel de control en tiempo real con listado de instancias, hostname, IP, fecha/hora de creación y estado de salud.
3. **Eliminar Instancia:**
   - Destrucción de la VM, eliminación de los diferenciales de disco, baja del registro DNS y liberación de IP.

---

## 2. Distribución del Código y Estructura del Repositorio

La solución debe organizarse bajo **Arquitectura Hexagonal (Puertos y Adaptadores)** estricta, desacoplando el núcleo de negocio de `VBoxManage`, comandos SSH o protocolos DNS.

### 2.1 Árbol de Directorios del Repositorio

```text
httpaas-orchestrator/
├── .agents/                               # Habilidades y contexto para Agentes de IA
│   └── skills/
│       ├── golang-engineering/            # Reglas de código Go, Effective Go y arquitectura
│       ├── go-testing/                    # TDD, pruebas por tablas y dobles de prueba
│       ├── virtualbox-multiattach/        # Control de VBoxManage y discos multiconexión
│       └── bind9-dns-automation/          # Automatización de DNS dinámico en Bind9
├── cmd/
│   └── server/
│       └── main.go                        # Punto de entrada y composición (DI)
├── data/                                  # Almacenamiento local (base SQLite/JSON de instancias)
├── docs/                                  # Documentación técnica e informe académico
│   ├── api/                               # Documentación Go autogenerada con gomarkdoc
│   ├── diagramas/                         # Diagramas PlantUML (.puml) y render (.png)
│   ├── informe/                           # Informe en LaTeX IEEEtran para entrega académica
│   │   ├── body/                          # Capítulos modulares del documento
│   │   ├── images/                        # Capturas de pantalla y evidencias
│   │   ├── .latexmkrc                     # Configuración de compilación (LuaLaTeX)
│   │   ├── main.tex                       # Archivo maestro del informe
│   │   └── ref.bib                        # Referencias bibliográficas BibTeX
│   ├── parcial/                           # Enunciado y rúbrica del Parcial #3
│   ├── arquitectura.md                    # Diseño hexagonal y límites arquitectónicos
│   ├── diseno-solucion.md                 # Especificación funcional de módulos y flujos
│   ├── openapi.yaml                       # Contrato formal OpenAPI 3.0 de la API
│   ├── TODO.md                            # Seguimiento de tareas y lista de verificación
│   └── verificacion.md                    # Protocolo de pruebas de aceptación y verificación
├── internal/
│   ├── core/                              # Núcleo puro (sin dependencias externas)
│   │   ├── domain/                        # Entidades del dominio y errores de negocio
│   │   │   ├── instance.go                # Entidad Instancia (ID, Hostname, IP, Estado, CreadoEn)
│   │   │   ├── dns.go                     # Entidad Registro DNS (Nombre, Zona, IP, TTL)
│   │   │   ├── webpackage.go              # Metadatos del paquete comprimido (.zip)
│   │   │   ├── errors.go                  # Errores centinela del dominio
│   │   │   └── domain_test.go             # Pruebas unitarias de invariantes del dominio
│   │   └── ports/                         # Interfaces que definen los contratos del núcleo
│   │       ├── hypervisor.go              # Puerto para gestión de VMs y ciclo de vida
│   │       ├── dns.go                     # Puerto para registro/baja en servidor DNS
│   │       ├── deployer.go                # Puerto para transferencia y despliegue de archivos
│   │       ├── repository.go              # Puerto para persistencia del inventario de instancias
│   │       └── executor.go                # Puerto para ejecución de comandos en el host
│   ├── adapters/                          # Implementaciones concretas de los puertos
│   │   ├── localproc/                     # Adaptador ports.Executor sobre os/exec
│   │   │   ├── localproc.go
│   │   │   └── localproc_test.go
│   │   ├── vbox/                          # Adaptador ports.Hypervisor sobre VBoxManage CLI
│   │   │   ├── vbox.go                    # Manejo de multiattach, createvm, startvm, control
│   │   │   ├── parser.go                  # Parseo de showvminfo y guestproperty
│   │   │   └── vbox_test.go               # Pruebas unitarias con mocks de ports.Executor
│   │   ├── bind9/                         # Adaptador ports.DnsProvider (nsupdate o SSH)
│   │   │   ├── bind9.go                   # Gestión dinámica de registros en cloud.local.
│   │   │   └── bind9_test.go
│   │   ├── deployer/                      # Adaptador ports.Deployer (GuestControl o SSH/SCP)
│   │   │   ├── deployer.go                # Envío de .zip y extracción en /var/www/vhosts/<host>/public
│   │   │   └── deployer_test.go
│   │   └── filestore/                     # Adaptador ports.Repository (JSON atómico o SQLite)
│   │       ├── filestore.go
│   │       └── filestore_test.go
│   ├── services/                          # Casos de uso de la aplicación
│   │   ├── orchestrator.go                # Orquestador transaccional de aprovisionamiento
│   │   ├── orchestrator_test.go           # Pruebas del flujo completo con dobles de prueba
│   │   ├── instance.go                    # Consulta y eliminación de instancias
│   │   ├── instance_test.go
│   │   ├── dns.go                         # Servicios de resolución y verificación DNS
│   │   └── dns_test.go
│   └── handlers/                          # Adaptadores primarios (HTTP REST + SSE)
│       ├── doc.go
│       ├── instance_handler.go            # Endpoints POST /api/instancias, GET, DELETE
│       ├── instance_handler_test.go
│       ├── dns_handler.go                 # Endpoints de consulta de registros de zona
│       ├── dns_handler_test.go
│       ├── middleware.go                  # Logging, recuperación de pánico, CORS
│       └── respuesta.go                   # Estructuración estándar de respuestas JSON
├── scripts/                               # Scripts utilitarios de entorno y verificación
│   ├── dev-doctor.sh                      # Chequeo de binarios (vboxmanage, bind9, go, etc.)
│   ├── configure-bind9-vm.sh              # Script de aprovisionamiento de la VM ns
│   └── prepare-base-vm.sh                 # Script para crear la VM base y disco multiconexión
├── web/                                   # Frontend web (Dashboard)
│   ├── index.html                         # Vista del dashboard (diseño según mockup de parcial)
│   ├── css/
│   │   └── styles.css
│   └── js/
│       └── app.js                         # Lógica de subida multipart, tabla y eliminación
├── .gitignore
├── .golangci.yml                          # Reglas estrictas de calidad estática
├── .gomarkdoc.yml                         # Configuración de generación de documentación
├── AGENTS.md                              # Reglas maestras de comportamiento para IA
├── go.mod
├── go.sum
├── LICENSE                                # Licencia Apache 2.0
├── README.md                              # Guía rápida de instalación y ejecución
└── Taskfile.yml                           # Orquestador del ciclo de desarrollo (CI/CD local)
```

---

## 3. Estructura y Estándares de Documentación (`docs/`)

La carpeta `docs/` centraliza todo el ciclo de diseño, especificación, arquitectura y entrega académica del proyecto. Debe mantenerse sincronizada antes de codificar cada funcionalidad.

### 3.1 Documentos de Diseño y Arquitectura en Markdown

1. **`docs/arquitectura.md`**:
   - Describe el patrón de Arquitectura Hexagonal adoptado.
   - Define formalmente las interfaces de `internal/core/ports`:
     ```go
     // Hypervisor define las operaciones con el hipervisor VirtualBox.
     type Hypervisor interface {
         CreateInstanceFromBase(ctx context.Context, name string, baseDiskPath string) (domain.Instance, error)
         Start(ctx context.Context, vmID string) error
         GetIPAddress(ctx context.Context, vmID string) (netip.Addr, error)
         Destroy(ctx context.Context, vmID string) error
     }

     // DnsProvider define la interacción con el servidor Bind9.
     type DnsProvider interface {
         AddRecord(ctx context.Context, record domain.DnsRecord) error
         RemoveRecord(ctx context.Context, hostname string) error
         Resolve(ctx context.Context, hostname string) (netip.Addr, error)
     }

     // Deployer define la transferencia y despliegue del paquete web.
     type Deployer interface {
         DeployWebZip(ctx context.Context, targetIP netip.Addr, zipReader io.Reader) error
     }
     ```
   - Establece la regla de inversión de dependencias: el núcleo **jamás** importa adaptadores ni paquetes del sistema como `os/exec`.

2. **`docs/diseno-solucion.md`**:
   - Vistas arquitectónicas:
     - **Vista de Despliegue:** Mapa de la subred host-only derivada, servidor DNS (`subred + .10`), plantilla (`subred + .11`) y máquinas con IP dinámica (`.100-.199`).
     - **Vista de Procesos:** Flujo detallado paso a paso con manejo transaccional de fallos (rollback si falla el DNS o la subida de archivos).
     - **Vista de Datos:** Modelo de la instancia y persistencia del inventario.
     - **Vista de Seguridad y Validación:** Reglas de nombres de host (solo caracteres alfanuméricos y guiones, sin caracteres especiales, longitud 3-32), validación de archivos zip (detección de archivos maliciosos, límites de tamaño).

3. **`docs/openapi.yaml`**:
   - Especificación formal OpenAPI 3.0 de la API REST en español:
     - `POST /api/instancias`: Recibe `multipart/form-data` con campos `nombre` (string) y `archivo` (file `.zip`). Retorna `201 Created` con la entidad de la instancia.
     - `GET /api/instancias`: Lista todas las instancias activas (`200 OK`).
     - `GET /api/instancias/{id}`: Detalle de una instancia por ID o nombre (`200 OK`).
     - `DELETE /api/instancias/{id}`: Destruye la VM, desvincula del DNS y elimina del registro (`200 OK` o `204 No Content`).
     - `GET /api/dns/registros`: Lista los registros de la zona `cloud.local.` (`200 OK`).
     - `GET /api/salud`: Estado de los servicios auxiliares (`vboxmanage`, `bind9`).

4. **`docs/TODO.md`**:
   - Lista de verificación alineada con la rúbrica del Parcial #3:
     - [ ] Despliegue y configuración de la VM DNS dedicada (`ns`, Bind9, zona `cloud.local.`).
     - [ ] Configuración del disco base `ApacheServer.vdi` en modo multiconexión (`multiattach`).
     - [ ] Implementación de adaptadores (`vbox`, `bind9`, `deployer`, `localproc`).
     - [ ] Orquestador de aprovisionamiento y pruebas unitarias/integración.
     - [ ] Interfaz web con dashboard y enlaces directos (`http://<host>.cloud.local`).
     - [ ] Redacción del informe académico en LaTeX y recopilación de evidencias fotográficas.

5. **`docs/verificacion.md`**:
   - Matriz de pruebas end-to-end con comandos de prueba manuales y reproducibles (`curl`, `dig`, `host`, `VBoxManage`).

### 3.2 Diagramas con PlantUML (`docs/diagramas/`)

Todo diagrama se versiona en código PlantUML (`.puml`) y se compila a imagen `.png`:

1. **`docs/diagramas/escenario.puml`**:
   Diagrama de componentes y red que ilustra el Host, la red interna virtual, la VM `ns` con Bind9, el disco multiconexión y las VMs instanciadas.
2. **`docs/diagramas/flujo-aprovisionamiento.puml`**:
   Diagrama de secuencia UML del caso de uso de aprovisionamiento:
   `Usuario -> Web -> Handler -> Orchestrator -> [VBox Adapter, Bind9 Adapter, Deployer Adapter] -> Instancia Apache`.
3. **`docs/diagramas/flujo-eliminacion.puml`**:
   Diagrama de secuencia UML de la baja controlada de recursos.

### 3.3 Informe Académico Formal en LaTeX (`docs/informe/`)

El informe se diseña para compilarse con **LuaLaTeX** vía `latexmk` utilizando el formato **IEEEtran**, respondiendo a los entregables exigidos por la cátedra:

```text
docs/informe/
├── body/
│   ├── 01-introduccion.tex               # Contexto de HTTPaaS, objetivos y alcance
│   ├── 02-marco-teorico.tex               # Virtualización, discos multiconexión, DNS dinámico y Bind9
│   ├── 03-diseno-solucion.tex             # Arquitectura hexagonal, diseño de red y flujo de orquestación
│   ├── 04-implementacion.tex              # Backend en Go, adaptadores de VBoxManage y frontend
│   ├── 05-pruebas-evidencias.tex          # Capturas paso a paso del aprovisionamiento y navegación
│   └── 06-problemas-mejoras-conclusiones.tex # Dificultades técnicas, lecciones aprendidas y trabajo futuro
├── images/                                # Capturas requeridas en la entrega
│   ├── escenario.png
│   ├── flujo-aprovisionamiento.png
│   ├── captura-dashboard-vacio.png
│   ├── captura-formulario-subida.png
│   ├── captura-instancia-paisajes.png
│   ├── captura-instancia-negocios.png
│   ├── captura-resolucion-dns.png
│   └── captura-web-navegador.png
├── .latexmkrc                             # Configuración de compilación
├── main.tex                               # Documento principal
└── ref.bib                                # Referencias IEEE
```

#### Contenido de `.latexmkrc`
```perl
$pdf_mode = 4; # Usar LuaLaTeX
$post_mode = 0;
$bibtex_use = 2; # Biber
@default_files = ('main.tex');
```

---

## 4. Habilidades para Agentes de IA (`.agents/skills/`)

Para garantizar que los agentes de IA (Antigravity, Claude, Copilot) operen con máxima fidelidad técnica, se deben definir habilidades específicas en `.agents/skills/`. Cada habilidad consta de un archivo `SKILL.md` con metadatos YAML frontmatter y documentación técnica de soporte.

### 4.1 Habilidad 1: `golang-engineering`
- **Ubicación:** `.agents/skills/golang-engineering/SKILL.md`
- **Propósito:** Gobernanza de estilo, buenas prácticas oficiales de Go y arquitectura hexagonal.
- **Directrices Clave:**
  - Identificadores de Go (paquetes, funciones, tipos, variables) siempre en **inglés**.
  - Comentarios, documentación exportada y mensajes de error al usuario en **español**.
  - Contrato JSON y rutas HTTP en **español** (`/api/instancias`, `json:"nombre"`).
  - Encabezado obligatorio de licencia **Apache 2.0** en cada archivo `.go`:
    ```go
    // SPDX-License-Identifier: Apache-2.0
    // Copyright 2026 Tu Nombre y Tu Compañero
    ```
  - Formateo estricto con `gofumpt` y verificación estática con `golangci-lint`.
  - Tratamiento idiomático de errores con `%w` e inspección con `errors.Is` / `errors.As`.

### 4.2 Habilidad 2: `go-testing`
- **Ubicación:** `.agents/skills/go-testing/SKILL.md`
- **Propósito:** Asegurar la disciplina TDD y la calidad de las pruebas.
- **Directrices Clave:**
  - Ciclo estricto: **Docs → Tests → Implementación**.
  - Las pruebas se escriben antes de codificar la lógica de negocio.
  - Uso intensivo de **Table-Driven Tests** con subpruebas nombradas (`t.Run(tc.name, ...)`).
  - Aserciones idiomáticas estándar con `t.Fatalf` y `t.Errorf` (sin librerías pesadas externas innecesarias).
  - Dobles de prueba para puertos (`fakeHypervisor`, `fakeDnsProvider`, `fakeDeployer`, `fakeExecutor`) para probar la lógica de `internal/services` sin necesidad de tener VirtualBox o Bind9 en ejecución en el entorno de pruebas unitarias.

### 4.3 Habilidad 3: `virtualbox-multiattach`
- **Ubicación:** `.agents/skills/virtualbox-multiattach/SKILL.md`
- **Propósito:** Automatización de Oracle VM VirtualBox 7.2 centrada en el aprovisionamiento con discos multiconexión (`multiattach`).
- **Directrices y Procedimientos Técnicos:**
  1. **Configuración de Disco Multiconexión:**
     ```bash
     # Marcar el disco base como multiattach
     VBoxManage modifymedium disk /ruta/ApacheServer.vdi --type multiattach
     ```
  2. **Creación y Registro de una Nueva Instancia:**
     ```bash
     # 1. Crear VM
     VBoxManage createvm --name "paisajes" --ostype "Debian_64" --register
     
     # 2. Configurar memoria, CPUs y red interna
     VBoxManage modifyvm "paisajes" --memory 1024 --cpus 1 --nic1 hostonly --hostonlyadapter1 vboxnet1
     
     # 3. Asociar controlador de almacenamiento y el disco multiconexión
     VBoxManage storagectl "paisajes" --name "SATA" --add sata --controller IntelAHCI
     VBoxManage storageattach "paisajes" --storagectl "SATA" --port 0 --device 0 --type hdd --medium /ruta/ApacheServer.vdi
     
     # 4. Iniciar en modo headless
     VBoxManage startvm "paisajes" --type headless
     ```
  3. **Concurrencia y Serialización:**
     - VirtualBox (`VBoxSVC`) no admite llamadas concurrentes que modifiquen el estado. El adaptador `internal/adapters/vbox` debe implementar un `sync.Mutex` global en su cliente para serializar toda invocación a `VBoxManage`.
  4. **Lectura de IP de la instancia:**
     - La imagen cloud no trae el servicio de usuario de Guest Additions, así que `guestproperty` no publica la IP. Se consulta la reserva/concesión DHCP por MAC (`dhcpserver findlease`) y, como respaldo, SSH con la clave del laboratorio.
     - `guestproperty` queda solo como último recurso si el invitado tiene Guest Additions completas.
  5. **Destrucción Limpia:**
     - Apagar la VM (`controlvm poweroff`), desasociar el medio de almacenamiento y desregistrar/borrar archivos (`unregistervm --delete`).

### 4.4 Habilidad 4: `bind9-dns-automation`
- **Ubicación:** `.agents/skills/bind9-dns-automation/SKILL.md`
- **Propósito:** Automatización del servidor DNS Bind9 para el registro dinámico de hosts en la zona `cloud.local.`.
- **Directrices y Procedimientos Técnicos:**
  1. **Estructura de Zona:**
     - Dominio raíz del laboratorio: `cloud.local.`
     - Servidor de nombres: `ns.cloud.local.` (`192.168.10.10`).
  2. **Mecanismos de Actualización Dinámica:**
     - **Opción A (Recomendada - RFC 2136 vía `nsupdate`):**
       Uso de `nsupdate` con clave TSIG compartida para insertar y remover registros `A` de forma transaccional y sin reiniciar el servicio `named`.
       ```text
       server 192.168.10.10
       zone cloud.local.
       update add paisajes.cloud.local. 300 A 192.168.10.30
       send
       ```
     - **Opción B (Automatización por SSH / Script Remoto):**
       Invocación de comandos en la VM `ns` mediante el puerto `Executor` o SSH para actualizar el archivo de zona y ejecutar `rndc reload cloud.local.`.
  3. **Verificación Automatizada de Resolución:**
     - Invocación de `dig @192.168.10.10 <hostname>.cloud.local +short` para confirmar la propagación antes de declarar completado el aprovisionamiento.
  4. **Eliminación de Registros:**
     - Eliminación idempotente del registro `A` cuando se elimina la instancia.

---

## 5. Reglas de Gobernanza para Agentes (`AGENTS.md`)

El archivo `AGENTS.md` en la raíz del repositorio rige el comportamiento de cualquier agente de inteligencia artificial que trabaje en el código.

### Contenido Mandatorio de `AGENTS.md`
```markdown
# Reglas de Ingeniería y Desarrollo para Agentes

## Nombres y Documentación
- Los identificadores de Go —paquetes, tipos, funciones, métodos, variables, campos y constantes— se escriben en inglés.
- Los comentarios, la documentación y los mensajes al usuario se escriben en español.
- Los nombres del contrato JSON y de las rutas de la API se mantienen en español; los tipos de Go usan nombres en inglés con la etiqueta `json` correspondiente.
- Las pruebas usan nombres y datos auxiliares en inglés. No se usa el nombre de una distribución como sinónimo de máquina virtual: se dice «máquina virtual» o «distribución basada en Debian».

## Flujo de Trabajo Obligatorio (TDD)
- Trabajar siempre en este orden: **docs, luego tests y por último implementación**.
- Antes de escribir código, escribir o actualizar primero la documentación correspondiente en `docs/`.
- Antes de implementar una funcionalidad, escribir primero sus pruebas unitarias.
- La implementación debe limitarse a hacer pasar las pruebas ya escritas.

## Estándares de Código y Licencia
- Cada archivo `.go` debe iniciar con el encabezado de la licencia Apache 2.0 con SPDX y copyright.
- El encabezado de licencia se separa por exactamente una línea en blanco del comentario del paquete.
- Cada función, método, tipo y variable exportada debe tener comentario de documentación que comience con su propio nombre.
- Formatear el código con `gofumpt` mediante `task fmt` antes de dar por terminada cualquier tarea.
- Verificar la calidad del código con `golangci-lint` mediante `task lint` antes de dar por terminada cualquier tarea.
- Ejecutar `task test` antes de considerar terminada cualquier tarea.
- No commitear cambios sin haber ejecutado `task check` con éxito.
- Usar los comandos del `Taskfile.yml` en lugar de invocar herramientas de Go directamente.
- No agregar dependencias externas nuevas a `go.mod` sin justificarla en `docs/arquitectura.md`.

## Arquitectura y Desacoplamiento
- Mantener la estructura hexagonal: `internal/core/{domain,ports}`, `internal/adapters`, `internal/services`, `internal/handlers` y `web/`.
- El núcleo (`internal/core`) nunca importa adaptadores concretos (`vbox`, `bind9`, etc.) ni librerías de terceros.
- La inyección de dependencias se realiza únicamente en `main.go`.
- Los handlers solo enrutan peticiones HTTP y validan entradas; la orquestación reside en `internal/services`.
- Toda interacción con procesos del host se canaliza por el puerto `Executor`.
- Toda interacción con VirtualBox debe estar sincronizada mediante exclusión mutua para evitar colisiones en `VBoxSVC`.
- Los archivos del informe LaTeX residen en `docs/informe` y se compilan con `latexmk` usando LuaLaTeX.
```

---

## 6. Automatización del Ciclo de Vida (`Taskfile.yml`)

El proyecto debe gobernarse mediante un `Taskfile.yml` robusto que estandarice las tareas del desarrollador y del agente:

```yaml
version: "3"

vars:
  BIN: httpaas-server
  PKGS: ./...

tasks:
  setup:
    desc: Descargar herramientas de desarrollo y dependencias de Go
    cmds:
      - go mod download
      - go install github.com/mvdan/gofumpt@latest
      - go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
      - go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest

  doctor:
    desc: Verificar herramientas del sistema (VBoxManage, dig, etc.)
    cmd: ./scripts/dev-doctor.sh

  run:
    desc: Ejecutar el servidor orquestador HTTPaaS
    cmds:
      - task: fmt
      - go run ./cmd/server

  test:
    desc: Formatear y ejecutar la suite completa de pruebas unitarias
    cmds:
      - task: fmt
      - go test {{.PKGS}} -cover -count=1

  fmt:
    desc: Formatear código Go con gofumpt estricto
    cmd: gofumpt -l -w .

  lint:
    desc: Análisis estático de código con golangci-lint
    cmd: golangci-lint run {{.PKGS}}

  doc:
    desc: Regenerar documentación Markdown de la API Go con gomarkdoc
    cmds:
      - rm -rf docs/api
      - gomarkdoc -o "docs/api/{{.Dir}}.md" ./internal/...
      - gomarkdoc -o docs/api/main.md ./cmd/...

  openapi:
    desc: Validar el contrato OpenAPI con Redocly
    cmd: npx --yes @redocly/cli@latest lint docs/openapi.yaml

  diagramas:
    desc: Renderizar diagramas PlantUML a PNG y sincronizar con el informe
    cmds:
      - plantuml -tpng docs/diagramas/*.puml
      - cp docs/diagramas/*.png docs/informe/images/ 2>/dev/null || true

  informe:
    desc: Compilar el informe en LaTeX con LuaLaTeX y biblatex
    dir: docs/informe
    cmds:
      - latexmk -lualatex main.tex

  informe-clean:
    desc: Limpiar artefactos temporales de compilación de LaTeX
    dir: docs/informe
    cmds:
      - latexmk -C

  check:
    desc: Validación integral previa a commit (fmt + lint + test + doc)
    cmds:
      - task: fmt
      - task: lint
      - task: test
      - task: doc

  build:
    desc: Compilar el binario ejecutable final
    cmd: go build -o {{.BIN}} ./cmd/server
```

---

## 7. Diseño del Frontend (Web Dashboard)

Conforme a las diapositivas y requerimientos del proyecto parcial, la interfaz web debe ser intuitiva, responsiva y cumplir con los siguientes elementos visuales y funcionales:

### 7.1 Formulario de Gestión de Páginas y Aplicaciones Web
- **Campo «Nombre del host»:** Caja de texto para ingresar el subdominio deseado (ej. `paisajes`, `negocios`).
- **Botón «Aceptar»:** Valida la disponibilidad del nombre de host en el DNS y en el inventario.
- **Campo «Contenido web (comprimido)»:** Selector de archivos para cargar paquetes `.zip`.
- **Botón «Publicar»:** Dispara el proceso de aprovisionamiento automatizado mostrando un indicador de progreso.

### 7.2 Tablero de Control de Instancias Disponibles
Tabla dinámica actualizada que muestra las instancias activas:
- **URL / Enlace Directo:** Hipervínculo con formato `http://<host>.cloud.local` que abre el sitio desplegado en una nueva pestaña.
- **Dirección IP:** Dirección IPv4 asignada a la instancia (ej. `192.168.10.30/24`).
- **Nombre de Host:** Nombre identificador único.
- **Fecha y Hora de Creación:** Timestamp legible (ej. `2026-04-22 17:02:03`).
- **Acción «Eliminar»:** Botón de acción (color rojo) que solicita confirmación y desmantela la máquina virtual y su registro DNS.

---

## 8. Guía Paso a Paso para Crear e Inicializar el Nuevo Repositorio

Sigue esta secuencia de comandos para inicializar el proyecto desde cero:

### Paso 1: Inicialización del Repositorio y Módulo de Go
```bash
# Crear directorio raíz del nuevo proyecto
mkdir httpaas-orchestrator
cd httpaas-orchestrator

# Inicializar Git y Go Modules
git init
go mod init github.com/tu-usuario/httpaas-orchestrator
```

### Paso 2: Creación de la Estructura de Carpetas
```bash
# Crear árbol hexagonal y documentación
mkdir -p cmd/server
mkdir -p internal/core/domain internal/core/ports
mkdir -p internal/adapters/localproc internal/adapters/vbox internal/adapters/bind9 internal/adapters/deployer internal/adapters/filestore
mkdir -p internal/services internal/handlers
mkdir -p docs/api docs/diagramas docs/informe/body docs/informe/images docs/parcial
mkdir -p .agents/skills/golang-engineering/references
mkdir -p .agents/skills/go-testing/references
mkdir -p .agents/skills/virtualbox-multiattach/references
mkdir -p .agents/skills/bind9-dns-automation/references
mkdir -p scripts web/css web/js data
```

### Paso 3: Configurar Archivos Base de Gobernanza y Calidad
1. Crear `AGENTS.md` copiando la plantilla de la **Sección 5**.
2. Crear `Taskfile.yml` copiando la especificación de la **Sección 6**.
3. Crear `.gitignore` ignorando binarios, archivos `.vdi` temporales, paquetes `.zip` subidos y temporales de LaTeX (`*.aux`, `*.fls`, `*.fdb_latexmk`, `*.log`, `*.out`, `*.toc`).
4. Crear `.golangci.yml` configurando los linters estándar (`errcheck`, `govet`, `unused`, `gofumpt`, `revive`).

### Paso 4: Redactar los Documentos de Diseño (`docs/`)
Antes de codificar, crear:
- `docs/arquitectura.md` con la definición de interfaces.
- `docs/diseno-solucion.md` con los casos de uso.
- `docs/diagramas/escenario.puml` con el diagrama de red.
- `docs/openapi.yaml` con el contrato HTTP.

### Paso 5: Implementar las Habilidades en `.agents/skills/`
Crear los archivos `SKILL.md` para cada una de las 4 habilidades descritas en la **Sección 4**, asegurando que los agentes cuenten con el contexto de VirtualBox 7.2, Bind9 dinámico y Go Hexagonal.

### Paso 6: Ciclo TDD de Implementación
1. Implementar pruebas de dominio (`internal/core/domain/domain_test.go`).
2. Implementar tipos de dominio (`internal/core/domain/instance.go`, etc.).
3. Escribir pruebas unitarias de adaptadores con dobles de prueba.
4. Implementar los adaptadores de `localproc`, `vbox`, `bind9` y `filestore`.
5. Implementar y probar el orquestador transaccional (`internal/services/orchestrator.go`).
6. Implementar los handlers HTTP y la interfaz web en `web/`.
7. Ejecutar `task check` para validar formato, linting y pruebas al 100%.

### Paso 7: Despliegue de Pruebas y Redacción del Informe
1. Arrancar el laboratorio con VirtualBox y la VM `ns`.
2. Probar el aprovisionamiento de las instancias de prueba (`paisajes`, `negocios`).
3. Capturar las pantallas de evidencia y guardarlas en `docs/informe/images/`.
4. Completar los capítulos del informe en `docs/informe/body/`.
5. Compilar el PDF final con `task informe`.

