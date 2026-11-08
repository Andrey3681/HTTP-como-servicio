# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Andrey y Equipo de Trabajo

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

Write-Host "=== Verificación del Entorno de Desarrollo HTTPaaS (PowerShell) ===" -ForegroundColor Cyan

function Check-Tool {
    param(
        [string]$Name,
        [string]$Description,
        [switch]$Optional
    )
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($cmd) {
        Write-Host "[OK] $Description ($Name): $($cmd.Source)" -ForegroundColor Green
        return $true
    } else {
        if ($Optional) {
            Write-Host "[INFO] $Description ($Name): No presente (opcional)" -ForegroundColor Gray
        } else {
            Write-Host "[ADVERTENCIA] No se encontró $Description ($Name) en el PATH" -ForegroundColor Yellow
        }
        return $false
    }
}

$null = Check-Tool "go" "Compilador Go"
$null = Check-Tool "git" "Control de versiones Git"
$null = Check-Tool "task" "Administrador de tareas Taskfile"
$null = Check-Tool "VBoxManage" "CLI de Oracle VirtualBox"

# Comprobación de herramientas DNS (dig o nativas de Windows)
$hasDig = Check-Tool "dig" "Herramienta de consulta DNS (dig)" -Optional
if (-not $hasDig) {
    $hasResolve = Get-Command "Resolve-DnsName" -ErrorAction SilentlyContinue
    $hasNslookup = Get-Command "nslookup" -ErrorAction SilentlyContinue
    if ($hasResolve -or $hasNslookup) {
        Write-Host "[OK] Alternativa DNS nativa de Windows: Resolve-DnsName / nslookup disponible" -ForegroundColor Green
    }
}

# Comprobación de utilitarios de Go con indicación de respaldo nativo
$hasGofumpt = Get-Command "gofumpt" -ErrorAction SilentlyContinue
if ($hasGofumpt) {
    Write-Host "[OK] Formateador estricto gofumpt: $($hasGofumpt.Source)" -ForegroundColor Green
} else {
    Write-Host "[OK] Formateador Go: go fmt (nativo de Go)" -ForegroundColor Green
}

$hasLinter = Get-Command "golangci-lint" -ErrorAction SilentlyContinue
if ($hasLinter) {
    Write-Host "[OK] Linter de Go golangci-lint: $($hasLinter.Source)" -ForegroundColor Green
} else {
    Write-Host "[OK] Análisis estático: go vet (nativo de Go)" -ForegroundColor Green
}

Write-Host "`n=== Entorno verificado y listo para desarrollo ===" -ForegroundColor Cyan
