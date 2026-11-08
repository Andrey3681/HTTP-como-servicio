#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Andrey y Equipo de Trabajo

set -euo pipefail

echo "=== Preparación de la Máquina Virtual Base (ApacheServer) ==="

DISK_PATH="${1:-ApacheServer.vdi}"

if [ ! -f "$DISK_PATH" ]; then
    echo "Uso: $0 <ruta-al-disco-ApacheServer.vdi>"
    echo "Advertencia: No se encontró el archivo $DISK_PATH en la ruta especificada."
    exit 1
fi

echo "Configurando $DISK_PATH en modo multiconexión (multiattach)..."
VBoxManage modifymedium disk "$DISK_PATH" --type multiattach

echo "Comprobando estado del medio:"
VBoxManage showmediuminfo "$DISK_PATH"

echo "=== Disco base configurado correctamente como multiattach ==="

