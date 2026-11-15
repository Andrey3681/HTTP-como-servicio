#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Andrey y Equipo de Trabajo
#
# Configura systemd-resolved del anfitrión para que el navegador resuelva la
# zona cloud.local contra el DNS del laboratorio. Requiere administrador.
#
# Uso: sudo ./scripts/configurar-dns-anfitrion.sh

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
    echo "Ejecuta este script con sudo: sudo $0" >&2
    exit 1
fi

API="${HTTPAAS_URL:-http://localhost:8080}"
estado="$(curl -s --max-time 5 "$API/api/setup/estado")"

iface="$(printf '%s' "$estado" | python3 -c 'import json,sys; print(json.load(sys.stdin)["red"]["interfaz"])')"
dns_ip="$(printf '%s' "$estado" | python3 -c 'import json,sys; print(json.load(sys.stdin)["red"]["ip_dns"])')"

if [ -z "$iface" ] || [ -z "$dns_ip" ]; then
    echo "No se pudo obtener la interfaz o la IP del DNS desde $API" >&2
    exit 1
fi

resolvectl dns "$iface" "$dns_ip"
resolvectl domain "$iface" "~cloud.local"
resolvectl flush-caches

echo "DNS del laboratorio configurado en $iface -> $dns_ip para ~cloud.local"
resolvectl query "ns.cloud.local" || true
