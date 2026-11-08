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

