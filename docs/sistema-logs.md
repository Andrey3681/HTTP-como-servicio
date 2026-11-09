# Sistema de logs — HTTPaaS

Este documento define el diseño del sistema de logs del orquestador HTTPaaS. Es la referencia previa a la implementación (flujo **docs → tests → implementación**). El contrato HTTP de los endpoints de logs se refleja también en [`docs/api/endpoints.md`](api/endpoints.md) y [`docs/openapi.yaml`](../openapi.yaml) cuando se implementen.

## 1. Objetivos

- **Observabilidad unificada**: capturar eventos relevantes del orquestador (HTTP, aprovisionamiento, VirtualBox, Bind9, despliegue, errores/pánicos) en un formato consistente.
- **Consulta reciente**: exponer los últimos eventos vía API para diagnóstico.
- **Tiempo real**: entregar eventos nuevos al panel web (dock «Logs del Sistema») mediante streaming.
- **Bajo acoplamiento**: mantener la regla hexagonal (el núcleo no depende de infraestructura concreta); la captura/almacenamiento/salida de logs se resuelve mediante puertos/adaptadores o componentes de infraestructura inyectados.
- **Sin dependencias externas nuevas**: implementación con biblioteca estándar (`log/slog` o envoltorio propio mínimo) salvo justificación explícita en `docs/arquitectura.md`.

## 2. Modelo de evento

Cada entrada de log es un objeto inmutable con la forma canónica:

| Campo | Tipo | Descripción |
|---|---|---|
| `id` | entero/string monotónico | Identificador secuencial para cursor/orden. |
| `ts` | RFC3339 UTC | Marca de tiempo del evento. |
| `nivel` | `DEBUG` \| `INFO` \| `WARN` \| `ERROR` | Severidad. |
| `componente` | string | Origen lógico (`http`, `orchestrator`, `vbox`, `bind9`, `deployer`, `storage`, `system`, etc.). |
| `mensaje` | string | Texto legible en español (mensaje al usuario/operador). |
| `campos` | objeto (opcional) | Metadatos estructurados (`instancia`, `metodo`, `ruta`, `estado_http`, `duracion_ms`, `error`, etc.). |

Representación JSON (contrato):

```json
{
  "id": 42,
  "ts": "2026-04-22T17:02:03Z",
  "nivel": "INFO",
  "componente": "orchestrator",
  "mensaje": "Aprovisionamiento completado",
  "campos": {"instancia": "paisajes", "ip": "192.168.10.30/24"}
}
```

## 3. Arquitectura

### 3.1 Componentes

- **Núcleo de logging (infraestructura de aplicación)**: define el tipo de evento y un *broker* en memoria con:
  - **Ring buffer** acotado (p. ej. 1000–5000 entradas) para historial reciente.
  - **Suscripción** para observadores en tiempo real (canales/notificación no bloqueante con descarte controlado ante consumidores lentos).
- **Integración con `log` estándar**: puente para capturar salidas existentes (`log.Printf/Println`) durante la migración, etiquetándolas con componente/nivel inferido cuando sea posible.
- **Emisor estructurado**: API interna (p. ej. `logging.Info(componente, mensaje, campos)`) para nuevos eventos; reemplaza gradualmente `log.Printf` en puntos críticos.
- **Middleware HTTP**: registra solicitudes/respuestas (método, ruta, código, duración, remoto) como eventos `http`.

### 3.2 Reglas de diseño

- La lógica de aprovisionamiento/dominio no depende de detalles de transporte SSE/HTTP del log.
- El *broker* se inyecta desde `cmd/server/main.go` (composición), igual que otros adaptadores.
- No se persisten logs en disco en esta fase salvo decisión explícita posterior (el anillo en memoria cubre el dock/diagnóstico reciente).

## 4. Contrato de API

Todos los endpoints bajo `/api/logs`, JSON salvo SSE:

| Método | Ruta | Propósito | Respuesta |
|---|---|---|---|
| `GET` | `/api/logs` | Obtener eventos recientes del anillo (con filtros). | `200` `{"datos":[LogEntry],"total":N}`; query `limit` (1–1000, def. 200), `nivel` (`DEBUG\|INFO\|WARN\|ERROR`), `componente`, `since` (id exclusivo, cursor). |
| `GET` | `/api/logs/stream` | Stream en tiempo real (Server-Sent Events). | `200` `text/event-stream`, `id:` + `data: <LogEntry JSON>`; reanudación vía header `Last-Event-ID` o query `since`. |
| `DELETE` | `/api/logs` | Limpiar el anillo de logs del servidor. | `200` `{"mensaje":"..."}`; `405`. |

Notas:
- `GET /api/logs/stream` mantiene la conexión abierta; el cliente (dock, rama UI) la consume para el indicador «live».
- `DELETE /api/logs` limpia el historial en memoria del servidor (complementa la limpieza de vista del cliente).
- Errores siguen `{"error","codigo"}` cuando aplique (p. ej. `400`, `405`).

## 5. Niveles y uso

- `DEBUG`: detalle diagnóstico (deshabilitable por configuración futura).
- `INFO`: flujo normal (inicio, aprovisionamiento OK, IP/DNS/despliegue).
- `WARN`: condición recuperable o degradada (nsupdate no disponible, advertencias de limpieza).
- `ERROR`: fallo de operación, pánico recuperado, error HTTP 5xx relevante.

## 6. Plan de implementación (incremental)

1. **Docs/contrato** (este documento) + `docs/openapi.yaml` y `docs/api/endpoints.md`.
2. **Tests**: modelo/evento, ring buffer (capacidad/orden/limpieza/cursor), filtros, broker pub/sub, handlers `GET/DELETE /api/logs` y SSE (resume).
3. **Implementación**: `internal/logging`/broker, puerto `EventLogger` inyectado en servicios/adaptadores para eventos estructurados, handlers/rutas, sanitización de mensajes `500` (detalle a logs, mensaje público estable).
4. **Integración UI**: conectar `logs-dock.js` (rama UI) al stream/consulta — trabajo coordinado con la rama de interfaz.

## 7. Criterios de aceptación (fase inicial)

- Los endpoints de logs existen y cumplen el contrato (aunque el volumen inicial sea limitado).
- Los eventos tienen forma JSON estable (`id,ts,nivel,componente,mensaje,campos`).
- `task fmt`, `task test`, `go vet ./...` y validación OpenAPI pasan antes de commit.
