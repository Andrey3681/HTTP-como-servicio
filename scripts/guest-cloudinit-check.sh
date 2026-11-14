#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Andrey y Equipo de Trabajo
#
# Confirma que cloud-init terminó y publica su estado y registro.
# Se ejecuta como root dentro del invitado cuando Guest Additions ya responde.

set -uo pipefail

cloud-init status --wait >/dev/null 2>&1 || true
cloud-init status --long 2>&1 || true

echo "----- /var/log/cloud-init-output.log (últimas 40 líneas) -----"
tail -n 40 /var/log/cloud-init-output.log 2>/dev/null || true

if cloud-init status 2>/dev/null | grep -q "status: done"; then
    echo "HTTPAAS_GUEST_OK cloud-init"
    exit 0
fi

exit 1
