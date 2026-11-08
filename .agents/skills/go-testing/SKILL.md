---
name: go-testing
description: Asegurar la disciplina TDD, pruebas por tablas y dobles de prueba para puertos en Go.
---

# Habilidad: Pruebas y TDD en Go

## Propósito
Esta habilidad establece el flujo de desarrollo guiado por pruebas (Test-Driven Development) para garantizar la máxima cobertura y robustez del orquestador HTTPaaS sin requerir infraestructura externa activa durante las pruebas unitarias.

## Ciclo de Desarrollo Mandatorio
1. **Documentación (`docs/`)**: Escribir o actualizar la especificación antes de programar.
2. **Pruebas (`*_test.go`)**: Escribir casos de prueba unitarios que fallen inicialmente.
3. **Implementación (`*.go`)**: Codificar la lógica mínima necesaria para hacer pasar las pruebas.
4. **Refactorización y Verificación**: Ejecutar `task test` y `task check`.

## Convenciones de Pruebas
1. **Table-Driven Tests**: Usar estructuras anónimas con casos nombrados y ejecutar cada uno mediante `t.Run(tc.name, func(t *testing.T) { ... })`.
2. **Aserciones Estándar**: Utilizar la biblioteca estándar (`testing`) con `t.Fatalf` para fallos irrecuperables y `t.Errorf` para discrepancias.
3. **Nombres en Inglés para Pruebas**: Los identificadores de prueba, variables y nombres de casos se redactan en inglés (`TestOrchestrator_Provision_Success`).

## Dobles de Prueba (Fakes / Mocks)
Dado que las pruebas unitarias deben ejecutarse en cualquier entorno sin depender del binario de VirtualBox o del servidor Bind9:
- Crear implementaciones en memoria (`fakeHypervisor`, `fakeDnsProvider`, `fakeDeployer`, `fakeRepository`, `fakeExecutor`).
- Simular escenarios de éxito y escenarios de fallo (ej. error en creación de VM, error de DNS, timeout de IP) para verificar la rutina de rollback transaccional.

