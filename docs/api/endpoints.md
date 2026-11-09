# Inventario riguroso de endpoints — HTTPaaS

Este documento identifica de forma exhaustiva los endpoints HTTP expuestos por el orquestador HTTPaaS, su contrato, propósito y estado de implementación. Es la referencia de diseño para la API; el contrato formal OpenAPI 3.0 se mantiene en [`docs/openapi.yaml`](../openapi.yaml).

## Convenciones

- **Prefijo**: todos los endpoints de negocio viven bajo `/api/`.
- **Formato**: JSON (`application/json; charset=utf-8`) salvo creación de instancias, que usa `multipart/form-data`.
- **Errores**: formato estándar `{"error":"<mensaje>","codigo":<HTTP>}` (ver `internal/handlers/respuesta.go`).
- **Nombres de contrato**: rutas y campos JSON en español (p. ej. `instancias`, `registros`, `datos`, `nombre`); identificadores Go en inglés con etiquetas `json`.
- **Estado "existe (stub)"**: el endpoint está registrado y responde con la forma del contrato, pero puede devolver datos simulados/vacíos hasta que la integración real esté completa.

## Inventario

| # | Método | Ruta | Propósito | Cuerpo/params | Respuesta exitosa | Códigos | Estado |
|---|--------|------|-----------|---------------|-------------------|---------|--------|
| 1 | `GET` | `/api/salud` | Comprobar salud del entorno (VBoxManage, Bind9). | — | `200` `{estado, vboxmanage, bind9}` | `200`, `405` | Existe |
| 2 | `GET` | `/api/instancias` | Listar instancias registradas. | — | `200` `{datos:[Instancia], total}` | `200`, `405`, `500` | Existe |
| 3 | `POST` | `/api/instancias` | Aprovisionar una instancia HTTP (VM + DNS + despliegue web). | `multipart/form-data`: `nombre` (subdominio), `archivo` (`.zip`, máx. 55 MB en handler / 50 MB validación de dominio) | `201` `{mensaje, datos:Instancia}` | `201`, `400`, `405`, `500` | Existe |
| 4 | `GET` | `/api/instancias/{id}` | Obtener detalle de una instancia por identificador/nombre. | path `id` | `200` `{datos:Instancia}` | `200`, `400`, `404`, `405`, `500` | Existe |
| 5 | `DELETE` | `/api/instancias/{id}` | Eliminar instancia (VM, DNS, inventario). | path `id` | `200` `{mensaje}` | `200`, `400`, `404`, `405`, `500` | Existe |
| 6 | `GET` | `/api/dns/registros` | Consultar registros A activos en la zona DNS administrada. | — | `200` `{zona, registros:[{nombre,tipo,ip,ttl}]}` | `200`, `405` | Existe (stub permitido) |
| 7 | `GET` | `/api/logs` | Obtener eventos de log recientes (anillo en memoria, filtrable). | query `limit` (1–1000, def. 200), `nivel`, `componente`, `since` (id, exclusivo) | `200` `{datos:[LogEntry], total}` | `200`, `400`, `405` | Existe |
| 8 | `GET` | `/api/logs/stream` | Stream de eventos de log en tiempo real (SSE), con reanudación. | query `since` o header `Last-Event-ID` | `200` `text/event-stream` (`id:` + `data: <LogEntry JSON>`) | `200`, `405`, `500` | Existe |
| 9 | `DELETE` | `/api/logs` | Limpiar el anillo de logs del servidor. | — | `200` `{mensaje}` | `200`, `405` | Existe |

## Esquemas clave (resumen)

- **Instancia**: `id`, `nombre`, `fqdn`, `ip`, `enlace` (URL), `estado` (`CREANDO|ACTIVA|ERROR|ELIMINADA`), `creado_en` (date-time).
- **Registro DNS**: `nombre` (FQDN), `tipo` (`A`), `ip`, `ttl`.
- **LogEntry**: `id`, `ts` (RFC3339), `nivel` (`DEBUG|INFO|WARN|ERROR`), `componente`, `mensaje`, `campos` (objeto opcional).
- **ErrorRespuesta**: `error`, `codigo`.

## Notas de rigor

- `GET /api/dns/registros` debe existir aunque la integración real de listado Bind9 no esté implementada; en ese caso responde con la estructura del contrato y `registros` vacío.
- Los handlers validan método HTTP y responden `405` cuando no aplica; los middlewares globales aplican CORS, logging y recuperación de pánicos.
- Cualquier ampliación del contrato se realiza primero en `docs/openapi.yaml` y en este inventario, luego en pruebas y finalmente en implementación (flujo **docs → tests → implementación**).
