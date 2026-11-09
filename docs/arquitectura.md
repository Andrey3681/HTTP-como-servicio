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
4. **Punto de Composición (`cmd/server/main.go`)**:
   - Es el único lugar donde se instancian los adaptadores concretos y se inyectan en los servicios de aplicación y controladores HTTP.

---

## 3. Contrato de Puertos (`internal/core/ports`)

### 3.1 Hipervisor (`Hypervisor`)
```go
// Hypervisor define las operaciones requeridas sobre el hipervisor VirtualBox.
type Hypervisor interface {
    // CreateInstanceFromBase crea una nueva VM a partir del disco base multiconexión.
    CreateInstanceFromBase(ctx context.Context, name string, baseDiskPath string) (domain.Instance, error)
    
    // Start inicia la máquina virtual en modo headless.
    Start(ctx context.Context, vmID string) error
    
    // GetIPAddress obtiene la dirección IP de la VM una vez que los servicios de red están activos.
    GetIPAddress(ctx context.Context, vmID string) (netip.Addr, error)
    
    // Stop apaga la VM de forma controlada o forzada.
    Stop(ctx context.Context, vmID string) error
    
    // Destroy desmantela la VM, liberando almacenamiento diferencial y registros en VirtualBox.
    Destroy(ctx context.Context, vmID string) error
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

### 3.3 Desplegador Web (`Deployer`)
```go
// Deployer transfiere y descomprime los paquetes web en la instancia Apache.
type Deployer interface {
    // DeployWebZip descomprime el archivo .zip en /var/www/html/ de la VM objetivo.
    DeployWebZip(ctx context.Context, targetIP netip.Addr, zipReader io.Reader) error
}
```

### 3.4 Repositorio de Persistencia (`Repository`)
```go
// Repository administra el inventario de instancias aprovisionadas.
type Repository interface {
    Save(ctx context.Context, instance domain.Instance) error
    FindByID(ctx context.Context, id string) (domain.Instance, error)
    FindByName(ctx context.Context, name string) (domain.Instance, error)
    FindAll(ctx context.Context) ([]domain.Instance, error)
    Delete(ctx context.Context, id string) error
}
```

### 3.5 Ejecutor de Comandos en el Host (`Executor`)
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

