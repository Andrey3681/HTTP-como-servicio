# Inventario riguroso de endpoints — HTTPaaS

Este documento identifica de forma exhaustiva los endpoints HTTP expuestos por el orquestador HTTPaaS, su contrato, propósito y estado de implementación. Es la referencia de diseño para la API; el contrato formal OpenAPI 3.0 se mantiene en [`docs/openapi.yaml`](../openapi.yaml).

## Convenciones

- **Prefijo**: todos los endpoints de negocio viven bajo `/api/`.
- **Formato**: JSON (`application/json; charset=utf-8`) salvo creación de instancias, que usa `multipart/form-data`.
- **Errores**: formato estándar `{"error":"<mensaje>","codigo":<HTTP>}` (ver `internal/handlers/respuesta.go`).
- **Nombres de contrato**: rutas y campos JSON en español (p. ej. `instancias`, `registros`, `datos`, `nombre`); identificadores Go en inglés con etiquetas `json`.

## Inventario

| # | Método | Ruta | Propósito | Cuerpo/params | Respuesta exitosa | Códigos | Estado |
|---|--------|------|-----------|---------------|-------------------|---------|--------|
| 1 | `GET` | `/api/salud` | Comprobar salud del entorno (VBoxManage, Bind9). | — | `200` `{estado, vboxmanage, bind9}` | `200`, `405` | Existe |
| 2 | `GET` | `/api/instancias` | Listar instancias registradas. | — | `200` `{datos:[Instancia], total}` | `200`, `405`, `500` | Existe |
| 3 | `POST` | `/api/instancias` | Aprovisionar una instancia HTTP (VM + DNS + despliegue web). | `multipart/form-data`: `nombre` (subdominio), `archivo` (`.zip`, máx. 55 MB en handler / 50 MB validación de dominio) | `201` `{mensaje, datos:Instancia}` | `201`, `400`, `405`, `500` | Existe |
| 4 | `GET` | `/api/instancias/{id}` | Obtener detalle de una instancia por identificador/nombre. | path `id` | `200` `{datos:Instancia}` | `200`, `400`, `404`, `405`, `500` | Existe |
| 5 | `DELETE` | `/api/instancias/{id}` | Eliminar instancia (VM, DNS, inventario). | path `id` | `200` `{mensaje}` | `200`, `400`, `404`, `405`, `500` | Existe |
| 6 | `GET` | `/api/dns/registros` | Consultar registros A activos en la zona DNS administrada. | — | `200` `{zona, registros:[{nombre,tipo,ip,ttl}]}` | `200`, `405` | Existe |
| 6b | `GET` | `/api/dns/anfitrion` | Comprobar si el resolver del anfitrión resuelve un nombre de la zona. | query `nombre` (def. `ns.<zona>`) | `200` `{resuelve, ip, mensaje}` | `200`, `405` | Existe |
| 7 | `GET` | `/api/logs` | Obtener eventos de log recientes (anillo en memoria, filtrable). | query `limit` (1–1000, def. 200), `nivel`, `componente`, `since` (id, exclusivo) | `200` `{datos:[LogEntry], total}` | `200`, `400`, `405` | Existe |
| 8 | `GET` | `/api/logs/stream` | Stream de eventos de log en tiempo real (SSE), con reanudación. | query `since` o header `Last-Event-ID` | `200` `text/event-stream` (`id:` + `data: <LogEntry JSON>`) | `200`, `405`, `500` | Existe |
| 9 | `DELETE` | `/api/logs` | Limpiar el anillo de logs del servidor. | — | `200` `{mensaje}` | `200`, `405` | Existe |
| 10 | `GET` | `/api/setup/estado` | Obtener el estado del configuración inicial. | — | `200` `SetupEstado` | `200`, `405` | Existe |
| 11 | `POST` | `/api/setup/configuracion` | Guardar los datos del formulario (usuario y contraseña). | JSON `SetupInput` | `200` `{mensaje}` | `200`, `400`, `405` | Existe |
| 12 | `POST` | `/api/setup/reiniciar` | Eliminar las VMs base y reiniciar la preparación (conserva imagen y red). | — | `200` `{mensaje}` | `200`, `405`, `409`, `500` | Existe |
| 13 | `POST` | `/api/setup/iniciar` | Arrancar la preparación automática (ISO, red, VMs, multiattach). | — | `202` `{mensaje}` | `202`, `405`, `409`, `500` | Existe |
| 14 | `GET` | `/api/setup/stream` | Estado de la preparación en tiempo real (SSE). | — | `200` `text/event-stream` (`data: <SetupEstado JSON>`) | `200`, `405`, `500` | Existe |

## Esquemas clave (resumen)

- **Instancia**: `id`, `nombre`, `fqdn`, `ip`, `enlace` (URL), `estado` (`CREANDO|ACTIVA|ERROR|ELIMINADA`), `creado_en` (date-time).
- **Registro DNS**: `nombre` (FQDN), `tipo` (`A`), `ip`, `ttl`.
- **LogEntry**: `id`, `ts` (RFC3339), `nivel` (`DEBUG|INFO|WARN|ERROR`), `componente`, `mensaje`, `campos` (objeto opcional).
- **SetupInput**: `usuario` y `contrasena`.
- **SetupEstado**: `requiere_configuracion`, `ejecutando`, `completado`, `error`, `configuracion` (`usuario`, `lista`), `imagen`, `red` (`interfaz`, `ip_anfitrion`, `mascara`, `cidr`, `ip_dns`, `ip_plantilla`, `dhcp_inicio`, `dhcp_fin`), `actualizado_en` y `pasos[]` con `id`, `nombre`, `descripcion`, `estado` (`PENDIENTE|EN_CURSO|COMPLETADO|ERROR|OMITIDO`), `progreso` (0–100; `-1` indeterminado) y `mensaje`.
- **ErrorRespuesta**: `error`, `codigo`.

## Notas de rigor

- `GET /api/dns/anfitrion` consulta con el resolver del propio sistema (`net.LookupHost`) un nombre de la zona; si el anfitrión no usa el DNS del laboratorio, `resuelve` es `false` y `mensaje` explica cómo configurarlo.
- `GET /api/dns/registros` consulta la zona mediante transferencia AXFR (`dig`) a través de `DnsProvider.ListRecords`; si la consulta no está disponible responde `200` con `registros` vacío para preservar el contrato (degradación explícita, no error).
- Los handlers validan método HTTP y responden `405` cuando no aplica; los middlewares globales aplican CORS, logging, recuperación de pánicos y **bloqueo de escrituras** (`409`) mientras la configuración inicial no esté completa.
- Cualquier ampliación del contrato se realiza primero en `docs/openapi.yaml` y en este inventario, luego en pruebas y finalmente en implementación (flujo **docs → tests → implementación**).
