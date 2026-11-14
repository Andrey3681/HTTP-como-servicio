# Arquitectura del Sistema HTTPaaS

## 1. Visión General de la Arquitectura Hexagonal

El sistema **HTTPaaS** adopta el patrón de **Arquitectura Hexagonal (Puertos y Adaptadores)**. Su propósito fundamental es aislar completamente el núcleo de negocio de la infraestructura subyacente (VirtualBox CLI, Bind9 vía TSIG/SSH, comandos de sistema operativo y almacenamiento en disco).

```
                      +------------------------------------------+
                      |         Controladores HTTP (Web / API)   |
                      |             internal/handlers            |
                      +--------------------+---------------------+
                                           | invoca
                                           v
                      +------------------------------------------+
                      |        Servicios de Aplicación           |
                      |            internal/services             |
                      +----+---------------+---------------+-----+
                           |               |               |
                           v               v               v
                     [ Hypervisor ]  [ DnsProvider ]  [ Deployer ]  <- Puertos (ports)
                           ^               ^               ^
                           |               |               | implementan
    +----------------------+---------------+---------------+----------------------+
    | Adaptadores          |               |               |                      |
    | internal/adapters:   |               |               |                      |
    |                    [vbox]         [bind9]       [deployer]     [filestore]  |
    |                   VBoxManage      nsupdate/SSH   GuestControl   JSON/SQLite |
    +-----------------------------------------------------------------------------+
```

---

## 2. Invariante de Dependencias

1. **El Núcleo Puro (`internal/core`)**:
   - `internal/core/domain`: Define los modelos de datos de negocio (`Instance`, `DnsRecord`, `WebPackage`) y los errores centinela. No tiene dependencias de paquetes externos ni de adaptadores.
   - `internal/core/ports`: Define las interfaces abstractas que representan los requerimientos del dominio hacia el mundo exterior.
2. **Los Servicios de Aplicación (`internal/services`)**:
   - Orquestan la lógica de negocio consumiendo exclusivamente las interfaces de `ports`. No importan implementaciones concretas de `adapters`.
3. **Los Adaptadores (`internal/adapters`)**:
   - Implementan las interfaces de `ports`. Interactúan con herramientas externas (`VBoxManage`, `nsupdate`, `dig`, sistema de archivos).
   - `internal/adapters/provisioner` prepara el laboratorio real (red host-only, VMs, instalación desatendida, disco multiconexión) detrás del puerto `ports.SetupProvisioner`.
4. **Punto de Composición (`cmd/server/main.go`)**:
   - Es el único lugar donde se instancian los adaptadores concretos y se inyectan en los servicios de aplicación y controladores HTTP.
   - También compone el broker de observabilidad (`internal/logging.Broker`), que se inyecta como `ports.EventLogger` en adaptadores, servicios y handlers.

---

## 3. Contrato de Puertos (`internal/core/ports`)

### 3.1 Hipervisor (`Hypervisor`)
```go
// Hypervisor define las operaciones requeridas sobre el hipervisor VirtualBox.
type Hypervisor interface {
    // CreateInstanceFromBase aprovisiona una nueva VM enlazando el disco multiconexión base.
    CreateInstanceFromBase(ctx context.Context, name string, baseDiskPath string) (*domain.Instance, error)

    // Start inicia la máquina virtual en modo desatendido (headless).
    Start(ctx context.Context, vmName string) error

    // GetIPAddress sondea la dirección IP asignada a la VM mediante Guest Additions.
    GetIPAddress(ctx context.Context, vmName string) (netip.Addr, error)

    // Stop apaga la máquina virtual.
    Stop(ctx context.Context, vmName string) error

    // Destroy desmantela la VM, desasocia el disco diferencial y elimina los archivos físicos.
    Destroy(ctx context.Context, vmName string) error
}
```

### 3.2 Proveedor DNS (`DnsProvider`)
```go
// DnsProvider define la gestión y consulta de nombres en el servidor Bind9.
type DnsProvider interface {
    // AddRecord registra un registro A en la zona DNS.
    AddRecord(ctx context.Context, record domain.DnsRecord) error
    
    // RemoveRecord elimina un registro A previamente registrado.
    RemoveRecord(ctx context.Context, hostname string) error
    
    // Resolve verifica la resolución DNS directa del nombre indicado.
    Resolve(ctx context.Context, hostname string) (netip.Addr, error)

    // ListRecords consulta los registros A de la zona administrada.
    ListRecords(ctx context.Context) ([]domain.DnsRecord, error)
}
```

### 3.3 Registro de eventos (`EventLogger`)
```go
// EventLogger define la emisión de eventos estructurados de observabilidad.
type EventLogger interface {
    // Append registra un evento con nivel, componente, mensaje y campos opcionales.
    Append(level string, component string, message string, fields map[string]any)
}
```
`EventLogger` es un puerto transversal: servicios y adaptadores lo reciben por inyección (opcional en constructores cuando aplique) para emitir eventos estructurados sin acoplarse al broker/HTTP. La implementación concreta (`internal/logging.Broker`) se compone en `cmd/server/main.go`.

### 3.4 Desplegador Web (`Deployer`)
```go
// Deployer transfiere y descomprime los paquetes web en la instancia Apache.
type Deployer interface {
    // DeployWebZip transfiere y descomprime el contenido web en /var/www/html/ de la máquina virtual objetivo.
    DeployWebZip(ctx context.Context, targetIP netip.Addr, zipContent io.Reader, size int64) error
}
```

### 3.5 Repositorio de Persistencia (`Repository`)
```go
// Repository administra la persistencia del inventario de instancias aprovisionadas.
type Repository interface {
    Save(ctx context.Context, instance domain.Instance) error
    FindByID(ctx context.Context, id string) (*domain.Instance, error)
    FindAll(ctx context.Context) ([]domain.Instance, error)
    Delete(ctx context.Context, id string) error
}
```

### 3.6 Configuración inicial (`SetupProvisioner` e `ImageStore`)
```go
// SetupProvisioner abstrae la preparación real de la infraestructura del laboratorio.
type SetupProvisioner interface {
    Inspect(ctx context.Context, cfg domain.SetupConfig) (domain.EnvironmentStatus, error)
    EnsureNetwork(ctx context.Context, cfg domain.SetupConfig) (string, error)
    CreateVM(ctx context.Context, cfg domain.SetupConfig, vmName, netInterface string) (string, error)
    StartUnattendedInstall(ctx context.Context, cfg domain.SetupConfig, vmName, hostname, isoPath string, progress ProgressFunc) error
    WaitGuestReady(ctx context.Context, vmName string, timeout time.Duration) error
    RunGuestScript(ctx context.Context, cfg domain.SetupConfig, vmName, localScript string) error
    ReserveDHCPAddress(ctx context.Context, cfg domain.SetupConfig, vmName, ip string) error
    MarkBaseMultiattach(ctx context.Context, cfg domain.SetupConfig, vmName string) (string, error)
}

// ImageStore localiza o descarga la imagen cloud del sistema invitado.
type ImageStore interface {
    Locate(ctx context.Context, dir string) (string, bool)
    Download(ctx context.Context, url, dir string, progress ProgressFunc) (string, error)
}
```
El servicio `services.SetupService` orquesta los pasos, mantiene el estado y lo publica por SSE. El detalle de pasos y del contrato HTTP está en [`docs/setup-inicial.md`](setup-inicial.md).

### 3.7 Ejecutor de Comandos en el Host (`Executor`)
```go
// Executor abstrae la invocación de procesos del sistema operativo host.
type Executor interface {
    Execute(ctx context.Context, command string, args ...string) (stdout []byte, stderr []byte, err error)
}
```

---

## 4. Estrategia de Concurrencia y Sincronización

`VBoxManage` se comunica con el servicio central `VBoxSVC`. Invocaciones concurrentes que alteren el estado de las máquinas virtuales causan colisiones y bloqueos (`E_FAIL`, `VBOX_E_OBJECT_IN_USE`). Por lo tanto:
- El adaptador `internal/adapters/vbox` debe implementar un cerrojo de exclusión mutua global (`sync.Mutex`) que serialice todas las operaciones de modificación de VMs.
- Las consultas de solo lectura (`showvminfo`, `guestproperty get`) pueden optimizarse o ejecutarse bajo lectura protegida.

---

## 5. Manejo Transaccional de Aprovisionamiento (Rollback)

Si alguna etapa del aprovisionamiento falla (creación de VM -> asignación de red -> registro DNS -> despliegue web), el orquestador (`Orchestrator`) ejecuta una rutina de compensación inversa (Rollback):
1. Si falla el despliegue del `.zip`: Se elimina el registro DNS y se destruye la VM creada.
2. Si falla el registro DNS: Se destruye la VM creada.
3. Si falla la obtención de IP: Se destruye la VM creada.
Esto garantiza que la infraestructura no quede en un estado inconsistente ni consuma memoria RAM o almacenamiento innecesario.

---

## 6. Infraestructura Real: Red y Estrategia de Discos

### 6.1 Red host-only

La red del proyecto se materializa como **interfaz host-only** de VirtualBox, no como red interna: el navegador del anfitrión debe alcanzar `http://<host>.cloud.local`. No se impone una subred: se usa la que VirtualBox asigna a la interfaz (por ejemplo `192.168.57.1/24`) y de ella se derivan el servidor DNS (`.10`), la IP de la plantilla (`.11`) y el rango DHCP (`.10–.199`).

El adaptador `vbox` resuelve la interfaz host-only real (`list hostonlyifs`, ignorando la red por defecto `192.168.56.1`) y configura cada instancia con `--nic1 hostonly --hostonlyadapter1 <interfaz>`. Si no existe, el aprovisionamiento falla con un mensaje que remite a la preparación inicial.

### 6.2 Discos diferenciales explícitos

`ApacheServer.vdi` se marca como `multiattach` (acción que exige desconectar el medio) y permanece como plantilla de solo lectura. Cada instancia crea su capa escribible con `VBoxManage createmedium disk --filename <carpeta-vm>/<instancia>.vdi --diffparent <ruta-ApacheServer.vdi> --format VDI` y conecta únicamente ese diferencial. No se clona el disco base: se comparte y cada VM escribe en su propio archivo, que desaparece al destruir la instancia (`unregistervm --delete`).

### 6.3 Preparación del laboratorio

La primera vez, la preparación (`services.SetupService` + `adapters/provisioner`) crea la red, descarga la imagen cloud de Debian 13 si falta, la convierte a VDI, genera las semillas cloud-init, arranca `ns` y `ApacheServer`, ejecuta los scripts de invitado (`scripts/guest-bind9.sh`, `scripts/guest-apache.sh`) y marca el disco base. El avance se persiste en `setup-estado.json` y el orquestador usa la ruta del disco base como origen de los diferenciales.

