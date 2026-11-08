---
name: golang-engineering
description: Gobernanza de estilo, buenas prácticas oficiales de Go y arquitectura hexagonal para el proyecto HTTPaaS.
---

# Habilidad: Ingeniería en Golang y Arquitectura Hexagonal

## Propósito
Esta habilidad rige el diseño, la estructura de código y los estándares de implementación en Golang para el sistema HTTPaaS, asegurando mantenibilidad, legibilidad y desacoplamiento estricto.

## Reglas Obligatorias de Nombres e Idiomas
1. **Identificadores de Go en Inglés**: Los nombres de paquetes, tipos, interfaces, funciones, métodos, variables, campos de estructuras y constantes deben redactarse siempre en **inglés técnico**.
2. **Documentación y Mensajes en Español**: 
   - Todos los comentarios de código y la documentación de tipos y funciones exportadas deben redactarse en **español formal**.
   - Toda función o tipo exportado debe iniciar su comentario con su propio nombre (ej: `// Instance representa una máquina virtual...`).
   - Los mensajes de error orientados al usuario y los registros de log deben redactarse en **español**.
3. **Contrato de API y Rutas en Español**:
   - Las rutas HTTP de la API REST se definen en español: `/api/instancias`, `/api/dns/registros`, `/api/salud`.
   - Las etiquetas JSON se definen en español: `json:"nombre"`, `json:"estado"`, `json:"creado_en"`.

## Encabezado de Licencia Obligatorio
Cada archivo de código Go (`*.go`) debe comenzar con el encabezado de la licencia Apache 2.0:
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

```
Debe existir exactamente una línea en blanco entre el encabezado de licencia y el comentario del paquete.

## Reglas de Arquitectura Hexagonal
1. **El núcleo (`internal/core`) no tiene dependencias externas**: Ni de librerías de terceros, ni de paquetes de infraestructura (`os/exec`, adaptadores, etc.).
2. **Inversión de Dependencias**: Los servicios de aplicación (`internal/services`) solo dependen de interfaces declaradas en `internal/core/ports`.
3. **Inyección de Dependencias**: Se centraliza exclusivamente en `cmd/server/main.go`.

## Calidad y Formato
- Formateo estricto con `gofumpt` mediante `task fmt`.
- Validación estática con `golangci-lint` mediante `task lint`.
- Errores envueltos de manera idiomática con `fmt.Errorf("...: %w", err)`.

