#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Andrey y Equipo de Trabajo

set -euo pipefail

echo "=== Verificación del Entorno de Desarrollo HTTPaaS ==="

check_cmd() {
    local cmd="$1"
    local desc="$2"
    if command -v "$cmd" >/dev/null 2>&1; then
        echo "[OK] $desc: $(command -v "$cmd")"
    else
        echo "[ADVERTENCIA] No se encontró $desc ($cmd) en el PATH"
    fi
}

check_cmd "go" "Compilador Go"
check_cmd "git" "Control de versiones Git"
check_cmd "task" "Administrador de tareas Taskfile"
check_cmd "VBoxManage" "CLI de Oracle VirtualBox"
check_cmd "dig" "Herramienta de consulta DNS (bind-utils / dnsutils)"
check_cmd "gofumpt" "Formateador estricto gofumpt"
check_cmd "golangci-lint" "Linter de Go"

echo "=== Fin de la verificación ==="

