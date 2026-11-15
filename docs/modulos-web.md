# Especificación de Módulos Web — HTTPaaS

> **Estado:** propuesta funcional/técnica para la interfaz web (`web/`).  
> **Fuente normativa principal (enunciado):** `docs/specs/Nota parcial 3 - Computacion en la Nube.md` (HTTP como servicio / HTTPaaS).  
> **Fuentes técnicas de soporte:** `docs/arquitectura.md`, `docs/diseno-solucion.md`, `docs/sistema-logs.md`, `docs/openapi.yaml`, `docs/verificacion.md`, `docs/TODO.md`.  
> **Alcance:** define *qué módulos web deben existir* y *qué funciones, formularios, botones y elementos* debe tener cada uno, sustentado en requisitos funcionales (RF) y no funcionales (RNF). No se limita a lo ya implementado.

---

## 1. Contexto del sistema (derivado de specs)

Según el enunciado (`Nota parcial 3`), el proyecto implementa un servicio administrado de servidores HTTP (HTTPaaS) sobre infraestructura virtualizada local: **VirtualBox** (plantilla Debian 13 CLI con Apache, disco en modo **multiconexión**), **Bind9** (dominio/zona local, p. ej. `cloud.local.`, con registro dinámico de hosts) y una **aplicación web de gestión en Go** que actúa como punto de acceso/orquestador. La webapp debe recibir solicitudes de aprovisionamiento, cargar un `.zip` con el contenido, crear/configurar VMs, actualizar DNS, desplegar en Apache y **mostrar en un dashboard** las instancias activas con **nombre de host, dirección IP y enlace** de acceso (preferiblemente en nueva pestaña). Los procedimientos de creación/configuración/despliegue deben estar **automatizados** (sin interacción manual del usuario durante el aprovisionamiento).

El diseño técnico (`arquitectura.md`/`diseno-solucion.md`) concreta el flujo transaccional y sus compensaciones (rollback): crear VM → iniciar → obtener IP → registrar DNS → desplegar `.zip` → persistir; ante fallos en DNS/deploy/IP se destruye/compensa. Topología de referencia: orquestador en el host (`localhost:8080`), VM DNS `ns` con IP derivada de la subred host-only (`.10`, zona `cloud.local.`), instancias dinámicas en el rango DHCP de esa subred con FQDN `<host>.cloud.local`.

---

## 2. Requisitos funcionales (RF)

| ID | Requisito funcional | Fuente |
|---|---|---|
| RF-01 | **Dashboard de instancias:** listar todas las instancias aprovisionadas/activas mostrando **nombre de host, dirección IP y enlace** de acceso (y datos operativos: FQDN, estado, fecha). | `Nota parcial 3` (webapp: “mostrar en un dashboard todas las instancias… con el nombre de host, la dirección IP asignada y el enlace”); `openapi.yaml` `GET /api/instancias`; `TODO` 5.33/39; `verificacion` 2.5 |
| RF-02 | **Solicitud de aprovisionamiento:** permitir al usuario solicitar un nuevo servicio HTTP y cargar un **archivo `.zip`** con el contenido; el sistema orquestador debe **crear/configurar la VM** (disco multiconexión), ajustar red, **actualizar automáticamente el DNS** (Bind9, zona local) y **desplegar** el contenido en Apache. | `Nota parcial 3` (webapp Go: recibir solicitudes, cargar `.zip`, crear/configurar VMs, actualizar DNS, desplegar); `openapi.yaml` `POST /api/instancias`; `arquitectura` 4/5; `diseno-solucion` §2; `TODO` 5.37 |
| RF-03 | Validar la entrada de aprovisionamiento: hostname único en el dominio, patrón `^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$` (3–32 caracteres), sin colisión con reservados (`ns`, `router`, `gateway`, `localhost`) ni con nombres existentes; archivo `.zip` requerido y validado (p. ej. tamaño/seguridad de rutas). | `Nota parcial 3` (hostname único, `.zip`); `openapi.yaml` (pattern, required); `diseno-solucion` §4.1/4.2 |
| RF-04 | Verificar disponibilidad de un nombre de host antes de aprovisionar (comprobación previa/validación asíncrona). | `TODO` 5.38; apoyado en `GET /api/instancias/{id}` (404 = no existe); coherente con hostname único (`Nota parcial 3`) |
| RF-05 | Consultar el detalle de una instancia por identificador/nombre. | `openapi.yaml` `GET /api/instancias/{id}`; `diseno-solucion` §3 (`Instance`) |
| RF-06 | Eliminar una instancia: apagar/destruir VM, eliminar diferenciales, desvincular registro DNS en Bind9; confirmar la acción y reflejar el resultado. | `openapi.yaml` `DELETE /api/instancias/{id}`; `TODO` 5.40; `verificacion` 2.7 |
| RF-07 | Consultar registros DNS de tipo A activos en la zona local (p. ej. `cloud.local.`), evidenciando el registro dinámico de hosts. | `Nota parcial 3` (Bind9/zona local, registro dinámico); `openapi.yaml` `GET /api/dns/registros`; `diseno-solucion` §1 |
| RF-08 | Comprobar la salud/disponibilidad del entorno (orquestador/dependencias: `VBoxManage`, `Bind9`). | `openapi.yaml` `GET /api/salud`; `verificacion` 2.1 |
| RF-09 | Proveer acceso directo al servicio publicado de cada instancia (`http://<fqdn>`) abriéndolo **preferiblemente en una nueva pestaña** del navegador. | `Nota parcial 3` (enlace “preferiblemente en una nueva pestaña”); `TODO` 5.39; `openapi` `Instancia.enlace/fqdn` |
| RF-10 | Presentar retroalimentación de operaciones de larga duración y de errores de negocio/infraestructura (creación, eliminación, fallos DNS/VBox/deploy) con mensajes claros. | `arquitectura` §5 (rollback/consistencia) + necesidad operativa; `openapi` `400/404/500` |
| RF-11 | Refrescar/sincronizar vistas de datos (instancias, DNS, salud) bajo demanda. | Necesidad operativa derivada de estado dinámico (`diseno-solucion`/`openapi`) |
| RF-12 | Mostrar el contexto operativo relevante (zona DNS, red, DocumentRoot, FQDN `.cloud.local`) para orientar al usuario. | `Nota parcial 3`/`diseno-solucion` §1/§2; coherencia con `openapi` |
| RF-13 | (Evolutivo, sustentado en specs) Visualizar el estado/configuración asociada a Apache VHost y al registro DNS de una instancia (referencia de `ServerName`/DocumentRoot y registro A) para diagnóstico. | `Nota parcial 3` (hostname único, directorio de contenido, despliegue por instancia); `arquitectura` §1; `diseno-solucion` §1/§2; nav Stitch (“VHosts”) |
| RF-15 | **Configuración inicial del entorno:** preparación de primer arranque que descarga/reutiliza la imagen de Debian, crea la red host-only, prepara las VMs `ns` y `ApacheServer` con semillas cloud-init, configura Bind9/Apache y marca el disco plantilla en modo multiconexión, mostrando progreso por paso. | `docs/setup-inicial.md`; `openapi.yaml` `/api/setup/estado`, `/api/setup/iniciar`, `/api/setup/stream` |
| RF-14 | Consultar eventos/logs de aprovisionamiento y operación (Apache/Bind9/orquestador) para observabilidad y soporte. | Observabilidad ante flujo transaccional/rollback (`arquitectura` §5); `docs/sistema-logs.md`; `GET/DELETE /api/logs`, `GET /api/logs/stream` |

> Nota: RF-13 no tiene endpoint dedicado en `openapi.yaml` y se especifica como vista **preparada para conexión** (datos derivados de instancias + contrato futuro). RF-14 ya cuenta con contrato e implementación mediante `GET/DELETE /api/logs` y `GET /api/logs/stream` (ver `docs/sistema-logs.md`).

---

## 3. Requisitos no funcionales (RNF)

| ID | Requisito no funcional | Sustento / implicación en UI |
|---|---|---|
| RNF-01 | **Usabilidad y claridad operativa:** interfaz orientada a operación técnica (DevOps/admin), con jerarquía clara y baja carga cognitiva. | `design_theme` Stitch (utilitario, preciso); formularios/acciones etiquetados en español. |
| RNF-02 | **Consistencia visual:** diseño coherente en todas las vistas (cabecera, navegación, tipografía, componentes, paleta). | Sistema de diseño Stitch (Plus Jakarta Sans/JetBrains Mono, `border-standard`, header teal) y **componentes aislados** compartidos (sin duplicación). |
| RNF-03 | **Retroalimentación y percepción de progreso:** operaciones que tocan VBox/DNS/deploy son potencialmente lentas; la UI debe indicar “en curso”, éxito o error y evitar doble envío. | RF-02/06/10; overlay/estado, deshabilitar acción durante ejecución. |
| RNF-03b | **Automatización del aprovisionamiento (backend), disparo desde UI:** la creación/configuración/despliegue se ejecutan automáticamente tras la solicitud, sin pasos manuales intermedios; la UI solo captura datos, dispara y reporta estado. | `Nota parcial 3` (automatización/evitar interacción del usuario en procedimientos; webapp como orquestador). |
| RNF-04 | **Prevención de errores y seguridad operativa:** validar formato/tamaño/obligatoriedad en cliente; confirmar acciones destructivas; no exponer secretos. | RF-03/06; `diseno-solucion` §4 (hostname, zip, path traversal — validación fuerte en backend; cliente como primera barrera). |
| RNF-05 | **Manejo de errores y estados vacíos:** mensajes comprensibles ante `400/404/500` o backend inaccesible; estados “sin datos/pendiente/no disponible”. | `openapi` errores; RF-10/11. |
| RNF-06 | **Trazabilidad RF↔UI↔API:** cada acción/elemento relevante mapea a un RF y, cuando exista, a un endpoint. | `openapi.yaml` como contrato. |
| RNF-07 | **Rendimiento percibido:** navegación fluida; evitar recargas completas innecesarias preservando shell (header/footer/logs). | Implementado vía router de intercambio de `<main>` + componentes persistentes. |
| RNF-08 | **Accesibilidad básica:** `lang="es"`, etiquetas asociadas a campos, `aria-live`/roles en feedback y modales, foco/teclado (Escape cierra modal/dock), contraste legible. | Buenas prácticas aplicadas a formularios, modal y estados. |
| RNF-09 | **Compatibilidad/responsividad:** usable en escritorio y adaptable a anchos menores. | Layout responsive (grids/stack) coherente con Stitch desktop y comportamiento móvil básico. |
| RNF-10 | **Mantenibilidad (sin duplicación):** header, footer y dock de logs como componentes aislados; lógica de página en módulos por vista. | Decisión de UI ya adoptada; evita divergencia. |
| RNF-11 | **Resiliencia ante indisponibilidad:** si el backend no responde, la UI se mantiene visible/usable en modo informativo (estados “no accesible/sin datos”), sin romper la navegación. | RF-08/10/11. |
| RNF-12 | **Seguridad de contenido en UI:** escapar datos renderizados (nombres, FQDN, IP, mensajes) para evitar inyección en el DOM. | Datos dinámicos desde API; `escapeHtml` en renderizado. |

---

## 4. Mapa de módulos web

| Módulo | Ruta/vista | Propósito | RF principales | API |
|---|---|---|---|---|
| M1. Instancias (Consola de aprovisionamiento) | `/index.html` | Crear y listar instancias; punto de entrada operativo. | RF-01,02,03,04,06,09,10,11,12 | `GET/POST /api/instancias`, `GET/DELETE /api/instancias/{id}` |
| M2. Detalle de instancia (drawer lateral) | Drawer en `/index.html` (se abre con `id` de instancia) | Inspección de una instancia (datos, estado, VHost/DNS de referencia) sin abandonar el listado. | RF-05,09,10,12,13 | `GET /api/instancias/{id}` (+ futuro VHost/DNS por instancia) |
| M3. DNS / Zonas | `/dns.html` | Visibilidad de registros A en `cloud.local.`. | RF-07,10,11,12 | `GET /api/dns/registros` |
| M4. Salud / Diagnóstico | `/salud.html` | Estado de dependencias del entorno. | RF-08,10,11 | `GET /api/salud` |
| M5. VHosts / Apache (vista operativa) | `/vhosts.html` | Visibilidad de VirtualHosts/ServerName/DocumentRoot por instancia (y estado lógico). | RF-13,12,10,11 | *futuro* `GET /api/vhosts` o derivado de instancias |
| M6. Operaciones / Eventos (logs) | componente `logs-dock` (global) *(extensible a `/eventos.html`)* | Observabilidad de eventos de aprovisionamiento/operación. | RF-14,10 | `GET/DELETE /api/logs`, `GET /api/logs/stream` (SSE) |
| M8. Preparación del entorno | página completa `setup-wizard` (bloquea la consola hasta completar) | Prepara el laboratorio real guiando al usuario paso a paso. | RF-15,10 | `GET /api/setup/estado`, `POST /api/setup/configuracion`, `POST /api/setup/iniciar`, `GET /api/setup/stream` |

Componentes transversales (presentes en todos los módulos, RNF-02/07/10): **header** (marca, nav, estado de servicios/nodo), **footer** (contexto zona/DocumentRoot/versiones + toggle de logs), **dock de logs** (M6), **router** (navegación sin recarga), utilidades comunes (escape, formato fecha, feedback/loading).

---

## 5. Especificación por módulo

### M1. Instancias — Consola de aprovisionamiento (`/index.html`)
- **Objetivo:** ejecutar RF-02/RF-04 y dar visibilidad/gestión (RF-01/06/09/11) en una sola consola.
- **Trazabilidad:** RF-01,02,03,04,06,09,10,11,12; RNF-01–06,07,08,11,12.
- **Funciones:**
  - Cargar y renderizar el listado de instancias; filtrar por hostname/FQDN/IP; refrescar.
  - Validar formato de hostname en cliente (patrón/longitud) y verificar disponibilidad (RF-04).
  - Enviar aprovisionamiento `multipart/form-data` (`nombre`, `archivo`) y manejar `201/400/500`.
  - Solicitar confirmación y ejecutar eliminación; actualizar listado y feedback.
  - Navegar al detalle de una instancia; abrir enlace publicado en nueva pestaña.
- **Formulario (Aprovisionar instancia):**
  - Campo **Nombre de host** (texto, requerido, `name=nombre`, patrón hostname, placeholder `mi-app`, prefijo `http://`, sufijo fijo `.cloud.local` mostrado). *Elemento:* input + addons/prefijo-sufijo + etiqueta + ayuda.
  - Campo **Contenido web** (archivo, requerido, `name=archivo`, `accept=.zip,application/zip`). *Elemento:* input file + ayuda (despliegue en `/var/www/vhosts/<host>/`).
  - *Elementos de contexto:* badge/ayuda `AUTO-VHOST-DNS`, zona/red/DocumentRoot. La explicación “Cómo funciona” se presenta como tooltip accesible desde un icono de información dentro del contenedor del formulario (esquina superior derecha), no como bloque permanente. Las acciones operativas por instancia (reinicio de servicios, acceso SSH, actualización de despliegue) se gestionan desde el drawer de detalle.
- **Botones/acciones:**
  - `Verificar` (disponibilidad, RF-04) — secundario; deshabilitar si vacío/inválido.
  - `Aprovisionar` (submit RF-02) — primario; **deshabilitar + indicador de carga** durante la operación (RNF-03).
  - `Sync/Actualizar` (refresca listado, RF-11).
  - Por instancia: `Ver detalle` (abre drawer M2, RF-05), `Eliminar` (abre confirmación, RF-06), enlace externo `open_in_new` (RF-09).
  - Modal eliminación: `Cancelar`, `Eliminar` (destructivo, con texto/ruta/comandos de referencia).
- **Elementos UI:** cabecera de sección, panel formulario, panel resumen operativo (zona/red/DocumentRoot/recarga), panel/listado de instancias activas (contador, búsqueda/filtro, tarjetas/tabla con estado `ACTIVA/CREANDO/ERROR/ELIMINADA`, IP, FQDN/enlace), área de feedback `statusMessage` (`aria-live`), overlay/loading, modal de confirmación.
- **Estados:** vacío (“no hay instancias”), cargando, éxito, error de validación (`400`), nombre duplicado, error de infraestructura (`500`/rollback comunicado), backend inaccesible.

### M2. Detalle de instancia (drawer lateral en `/index.html`)
- **Objetivo:** inspeccionar una instancia concreta y su relación VHost/DNS (RF-05/13) desde un panel lateral sin salir del listado.
- **Trazabilidad:** RF-05,09,10,12,13; RNF-01,02,05,06,08,11,12.
- **Funciones:** abrir el drawer desde la acción «Ver detalle» del listado; cargar el detalle por identificador; mostrar atributos; enlazar al FQDN; presentar referencias de VHost y registro DNS; permitir cerrar el panel y volver al contexto del listado.
- **Formulario/controles:** no hay selector independiente; el contexto lo define la instancia elegida desde M1.
- **Botones/acciones:** `Ver detalle` (abre drawer, RF-05), `Cerrar`/overlay/Escape (cierra drawer), enlace al sitio publicado (RF-09), (recomendado) `Actualizar`, `Eliminar` con confirmación (reusa patrón RF-06) y `Copiar FQDN/IP`.
- **Elementos UI:** panel lateral deslizable (drawer) con encabezado, botón de cierre, bloque de metadatos (ID/nombre, FQDN, IP, estado con badge, enlace, `creado_en`), bloque **Apache VHost (referencia/real)** (ServerName, DocumentRoot, log), bloque **Registro DNS Bind9 (referencia/real)** (registro A, TTL/IP), estados de carga/error.
- **Estados:** cerrado, cargando, encontrado, `404` no encontrado, error/inaccesible. *Evolutivo (RF-13):* reemplazar “referencia” por datos reales cuando exista endpoint de VHost/DNS por instancia.

### M3. DNS (`/dns.html`)
- **Objetivo:** visibilidad operativa de los registros DNS (RF-07).
- **Trazabilidad:** RF-07,10,11,12; RNF-01,02,05,06,11,12.
- **Funciones:** consultar y listar registros A; refrescar; mostrar la zona reportada por la API.
- **Formulario:** no requiere formulario de alta (el alta DNS la dispara el aprovisionamiento, RF-02). *Evolutivo:* filtro/búsqueda por nombre/IP.
- **Botones/acciones:** `Actualizar` (RF-11).
- **Elementos UI:** encabezado con zona (`dnsZone`) y tabla de registros (Nombre, Tipo, IP, TTL), estado vacío/error.
- **Estados:** cargando, con datos, sin registros, backend inaccesible.

### M4. Salud (`/salud.html`)
- **Objetivo:** verificar dependencias críticas y ofrecer un resumen operativo del entorno, priorizando la respuesta a “¿el sistema está listo?” (RF-08).
- **Trazabilidad:** RF-08,10,11; RNF-01,02,05,06,11.
- **Funciones:** ejecutar comprobación y representar estado general, VirtualBox y DNS con lenguaje claro; mostrar resúmenes de instancias, registros DNS y resolución del anfitrión; orientar recuperación ante fallos/prerrequisitos; exponer datos crudos para soporte como detalle progresivo (no como contenido principal).
- **Formulario:** ninguno (acción directa de comprobación).
- **Botones/acciones:** `Comprobar ahora` (primario, con estado de carga y sin doble envío).
- **Elementos UI:** indicador/encabezado de preparación general, tarjetas de estado (`h-overall`, `h-vbox`, `h-bind9`) con explicación breve, tarjetas de resumen (instancias, registros DNS, resolución del anfitrión) con contexto/acciones sugeridas, y diagnóstico técnico en detalle colapsable (`healthRaw`).
- **Estados:** pendiente/sin comprobar, comprobando, listo/ok, degradado, error, backend inaccesible. Mapear explícitamente `null/false/true` y mostrar mensajes accionables.

### M5. VHosts (`/vhosts.html`)
- **Objetivo:** visibilidad por VHost (RF-13), complementando M1/M2.
- **Trazabilidad:** RF-13,12,10,11; RNF-01,02,05,06,11,12.
- **Funciones:** listar VHosts asociados a instancias; refrescar; filtrar; abrir sitio; abrir detalle de instancia desde Instancias.
- **Formulario:** ninguno de mutación. Filtro por ServerName.
- **Botones/acciones:** `Actualizar`, `Ver detalle`, `Abrir sitio` (RF-09), filtro.
- **Elementos UI:** tabla (ServerName, DocumentRoot, FQDN/enlace, estado, acciones).
- **Elementos/IDs requeridos (contrato):** `vhostsTableBody`, `refreshVhostsBtn`, `vhostsFilter` (opcional), `data-page="vhosts"`.
- **Estados:** cargando, vacío, error. *Contrato futuro:* `GET /api/vhosts` o enriquecimiento de `/api/instancias`; mientras tanto, vista derivada del listado de instancias.

### M6. Operaciones / Eventos — logs (componente global, extensible)
- **Objetivo:** observabilidad de operaciones (RF-14) ante un flujo con rollback (arquitectura §5).
- **Trazabilidad:** RF-14,10; RNF-01,02,05,07,11.
- **Funciones:** mostrar/ocultar dock con animación, limpiar vista, redimensionar (componente aislado persistente); cargar eventos recientes y recibir eventos en tiempo real. **Evolutivo:** filtros por severidad/fuente/instancia, pausa, correlación operación↔instancia.
- **Fuente de datos (API):** `GET /api/logs?limit=` (historial reciente, `{"datos":[LogEntry],"total"}`) y `GET /api/logs/stream` (SSE `text/event-stream`, `data` con `LogEntry` JSON: `id, ts, nivel, componente, mensaje, campos`).
- **Controles/botones:** `Logs del Sistema` (toggle en footer), `Cerrar`/deslizar, `Limpiar` (limpia solo la vista cliente); indicador de conexión (conectado/conectando/error).
- **Elementos UI:** dock inferior persistente, área de salida monoespaciada (`role="log"`), indicadores de conexión/estado accesibles.
- **Contrato de componente (reutilizable):** `#logs-toggle-mount` (footer) y dock inyectado por `logs-dock.js`; API global `window.toggleLogsDock`/`openLogsDock`/`closeLogsDock`. El dock consulta `/api/logs` y se suscribe a `/api/logs/stream` (SSE/EventSource) con deduplicación por `id`.
- **Estados:** oculto, visible, conectando, conectado (`live`), reconectando/error, backend inaccesible (RNF-11).

### M8. Preparación del entorno (`setup-wizard`, página completa)
- **Objetivo:** preparar el laboratorio real la primera vez que se abre la consola (RF-15), guiando al usuario paso a paso.
- **Trazabilidad:** RF-15,10; RNF-01,02,03,05,08,11.
- **Pantallas:** bienvenida → credenciales (usuario y contraseña del invitado) → resumen → ejecución automática → listo.
- **Funciones:** detectar infraestructura faltante, recoger los datos que solo el usuario puede aportar, ejecutar el resto de forma automática y mostrar estado/progreso por paso, tiempo transcurrido y error accionable.
- **Fuente de datos:** `GET /api/setup/estado`, `POST /api/setup/configuracion`, `POST /api/setup/iniciar`, `GET /api/setup/stream` (SSE).
- **Botones/acciones:** `Comenzar`, `Siguiente`/`Atrás`, `Comenzar preparación`, `Reintentar`, `Ir a la consola`; `Configuración` en el pie reabre el resumen cuando ya está completado.
- **Elementos UI:** página completa `#setup-wizard` con `[data-screen]`, indicador de pasos, barra global `role="progressbar"`, lista `data-setup-step`, mensajes `aria-live`, error `role="alert"`.
- **Estados:** primer arranque, captura de datos, ejecución, completado, error con reintento, backend inaccesible.
- **Bloqueo:** mientras `completado` sea falso, la consola (cabecera, páginas y pie) queda oculta y no se puede cerrar la preparación. La API rechaza escrituras con `409` hasta completar.
- **Contrato de componentes:** `#setup-wizard`, `[data-screen="credenciales|resumen|ejecucion|listo"]`, `#setup-user`, `#setup-pass`, `#setup-steps`, `#setup-progress-track`, `#setup-start-btn`, `#setup-log` (con `#setup-copy-log`), `#setup-error`, `[data-setup-step]`, `#setup-open-btn` (pie), API `window.openSetupWizard()`.

---

## 6. Matriz RF/RNF → módulo

| Req | M1 | M2 | M3 | M4 | M5 | M6 | Transversal |
|---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| RF-01 | ● | ○ | | | ○ | | nav |
| RF-02 | ● | | | | | ○ | feedback/load |
| RF-03 | ● | | | | | | validación |
| RF-04 | ● | | | | | | |
| RF-05 | ○ | ● | | | ○ | | |
| RF-06 | ● | ○ | | | | ○ | modal |
| RF-07 | | | ● | | | | |
| RF-08 | | | | ● | | | |
| RF-09 | ● | ● | | | ● | | |
| RF-10 | ● | ● | ● | ● | ● | ● | mensajes/estados |
| RF-11 | ● | ○ | ● | ● | ● | | |
| RF-12 | ● | ● | ● | ○ | ● | | header/footer |
| RF-13 | | ● | | | ● | | |
| RF-14 | | | | | | ● | dock |
| RF-15 | | | | | | | M8 |
| RNF-01..12 | ● | ● | ● | ● | ● | ● | ● |

(● responsabilidad principal, ○ apoyo/navegación)

---

## 7. Contratos API usados y brechas (para conexión progresiva)

- **Existentes (usar):** `GET/POST /api/instancias`, `GET/DELETE /api/instancias/{id}`, `GET /api/dns/registros`, `GET /api/salud`, `GET/DELETE /api/logs`, `GET /api/logs/stream` (`openapi.yaml`).
- **Brechas/evolutivos (alineados a RF-13, no romper contrato):** detalle ampliado por instancia (metadatos VHost/DNS/VM), `GET /api/vhosts` (o derivado); verificación de disponibilidad explícita `HEAD/GET /api/instancias/{id}` ya sirve (RF-04) pero puede formalizarse.
- **Regla:** la UI debe funcionar con los endpoints actuales (RNF-11) y degradar con elegancia donde falte contrato futuro.

---

## 8. Contratos de componentes reutilizables e interfaces (base para tests)

Estos contratos deben cumplirse para considerar la interfaz “reconstruida” según esta especificación (TDD: los tests validan estos contratos).

### 8.1 Shell y componentes compartidos (todas las páginas)
- Cada página HTML de módulo (`index.html`, `dns.html`, `salud.html`, `vhosts.html`) incluye: `lang="es"`, landmarks/montajes principales (`id="site-header-mount"`, `id="site-footer-mount"`, `<main>`), y `body` con `data-page="<modulo>"` (`index|dns|salud|vhosts`). La navegación y el contexto esencial deben seguir siendo comprensibles si la mejora progresiva tarda o falla; el enrutamiento SPA y mejoras dinámicas son incrementales.
- **CSS local y sin runtime:** las páginas cargan `css/stitch.css` (componentes/estilos base) y `css/utilities.css` (utilidades locales equivalentes a las clases usadas). No se depende de `cdn.tailwindcss.com` ni de configuración inline de Tailwind. Las fuentes se cargan con `preconnect` desde el `<head>` y `font-display=swap` para reducir bloqueos de renderizado.
- **Carga de módulos por página:** cada página incluye solo su módulo de página (`js/pages/<modulo>.js`) y el drawer compartido (`js/pages/detalle.js`); el router carga bajo demanda los módulos de otras vistas al navegar por SPA, reduciendo el JS inicial.
- **Identidad y transiciones:** las páginas incluyen favicon (`favicon.svg`/`favicon.ico`) y `theme-color`; los cambios de módulo y las acciones principales usan transiciones breves (~150–180 ms) y respetan `prefers-reduced-motion`.
- **Títulos:** el `<title>` de cada página combina un nombre descriptivo del módulo con la marca (`Agregar nueva instancia · HTTPaaS`, `DNS de la zona local · HTTPaaS`, `Salud del entorno · HTTPaaS`, `VHosts de Apache · HTTPaaS`) y el router lo actualiza al navegar por SPA.
- **Logotipo:** el header muestra un wordmark `HTTPaaS` con el estilo del favicon (fondo teal degradado, borde sutil y punto turquesa), sin icono separado.
- `components.js` expone `window.updateActiveNav`, monta/mejora el encabezado (marca y acceso a **Configurar sistemas**) y el dock vertical de módulos (`id="side-dock"`, `aria-label="Módulos"`) con accesos a `index.html`, `dns.html`, `vhosts.html`, `salud.html`, resaltando el módulo activo. No existe entrada de navegación independiente para «Detalle» (el detalle se presenta como drawer en M1).
- `logs-dock.js` expone `window.toggleLogsDock`, `window.openLogsDock`, `window.closeLogsDock` y monta toggle/dock (cierres deslizantes, sin cambio de opacidad). El toggle expone `aria-expanded` y `aria-controls="system-logs-dock"`; controles del dock son accesibles por teclado/nombre.
- El pie es persistente y de tema oscuro; incluye un área visible con el **último log en vivo** (`id="footer-latest-log"`, actualizado en tiempo real) y el acceso/montaje de logs del sistema (`#logs-toggle-mount`).
- **Preparación/gating:** el asistente de preparación (RF-15) bloquea funcionalmente la consola mientras el entorno no esté completado, pero no debe dejar una interfaz en blanco prolongada por fallos o demoras de carga; los estados de espera/carga deben ser explícitos y el shell debe recuperarse con elegancia.

### 8.2 Contratos por módulo (IDs/elementos mínimos)
- **M1/index (`data-page="index"`):** `provisionForm`, `hostnameInput`, `zipInput`, `checkHostBtn`, `instancesList` (o tabla equivalente), `instancesCount`, `refreshInstancesBtn`, `delete-modal` (o equivalente, `role="dialog"`, `aria-modal="true"`, `aria-labelledby`, contenedor enfocable `tabindex="-1"`), `loadingOverlay` (`role="status"`, `aria-live`, sin `aria-hidden` contradictorio mientras visible), `statusMessage`, y drawer de detalle `instance-detail-drawer` con `instance-detail-close`, `instance-detail-overlay`, `instance-detail-content` y estado accesible (`aria-hidden`/`hidden`, foco gestionado al abrir/cerrar). El flujo JS gestiona foco del modal y del drawer, y estados pendientes de acciones async (`disabled`/`aria-busy` donde aplique).
- **M2/detalle (drawer, integrado en M1):** se abre desde la acción de detalle del listado; no existe página `detalle.html` ni navegación independiente. Debe mostrar metadatos, bloques VHost/DNS de referencia y permitir cierre por botón, overlay y `Escape`.
- **M3/dns:** `dnsTableBody`, `refreshDnsBtn`, `dnsZone`/etiqueta de zona.
- **M4/salud:** `checkBtn`/`checkHealthBtn`, indicadores de estado general/VBoxManage/Bind9, resumen operativo legible y diagnóstico técnico en detalle progresivo (`healthRaw` y/o contenedor equivalente).
- **M5/vhosts:** `vhostsTableBody`, `refreshVhostsBtn`.
- **M6/logs (global):** `system-logs-dock` (inyectado), `toggle-logs-btn` (inyectado, con `aria-expanded` y `aria-controls="system-logs-dock"`), `terminal-output-container` (inyectado), y `footer-latest-log` (último log en vivo en el pie). Controles con nombre accesible; resize no depende solo de puntero.
- **M8/preparación:** `setup-wizard`, `[data-screen]`, `setup-user`, `setup-pass`, `setup-iso-mode`, `setup-steps`, `setup-progress-track`, `setup-start-btn`, `setup-open-btn` (pie) y `[data-setup-step]`; la preparación se abre solo cuando `requiere_configuracion` es verdadero.

### 8.3 Reglas de reutilización
- Cero duplicación de header/footer/dock entre páginas (solo mounts + componentes).
- Utilidades compartidas (`escapeHtml`, `formatFecha`, `showMessage/showLoading`) en `common.js` / `window.HTTPaaS`.
- Inicializadores de página en `window.HTTPaaSPages.<modulo>` invocados por el router.

## 9. Recomendaciones de implementación (coherentes con RNF)

1. Mantener **componentes aislados** (`components.js`, `logs-dock.js`) y **módulos por página** (`pages/*.js`) + router; no duplicar markup/JS.
2. Centralizar validación de hostname/zip (reglas de `diseno-solucion` §4) en util compartida cliente; tratar backend como autoridad.
3. Estandarizar estados de UI: `idle | loading | success | empty | error | unavailable`.
4. Confirmar toda acción destructiva (modal) y deshabilitar reintentos concurrentes durante operaciones (RNF-03; coherente con serialización `VBoxManage`, `arquitectura` §4).
5. Registrar (cuando exista M6 backend) la correlación operación↔instancia para explicar rollbacks.
6. Al agregar M5/M7, seguir el mismo patrón: mount de header/footer + `<main>` + módulo `pages/*.js` + entrada en nav (actualizando el componente una sola vez). 