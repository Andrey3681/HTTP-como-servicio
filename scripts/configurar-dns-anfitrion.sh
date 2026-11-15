#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Andrey y Equipo de Trabajo
#
# Configura systemd-resolved del anfitrión para que la zona cloud.local se
# resuelva con el Bind9 del laboratorio. Instala una configuración persistente
# (sobrevive reinicios) y la activa de inmediato.
#
# Normalmente lo invoca la consola mediante pkexec. También puede ejecutarse a
# mano: sudo ./scripts/configurar-dns-anfitrion.sh

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
    echo "Este configurador requiere administrador (pkexec o root)." >&2
    exit 1
fi

API="${HTTPAAS_URL:-http://localhost:8080}"
iface="${1:-}"
dns_ip="${2:-}"

if [ -z "$iface" ] || [ -z "$dns_ip" ]; then
    estado="$(curl -s --max-time 5 "$API/api/setup/estado")"
    iface="$(printf '%s' "$estado" | python3 -c 'import json,sys; print(json.load(sys.stdin)["red"]["interfaz"])')"
    dns_ip="$(printf '%s' "$estado" | python3 -c 'import json,sys; print(json.load(sys.stdin)["red"]["ip_dns"])')"
fi

if [ -z "$iface" ] || [ -z "$dns_ip" ]; then
    echo "No se pudo determinar la interfaz o la IP del DNS del laboratorio." >&2
    exit 1
fi

install -d -m 755 /etc/systemd/resolved.conf.d
cat > /etc/systemd/resolved.conf.d/httpaas.conf <<EOF
# Generado por HTTPaaS: resuelve la zona del laboratorio con su Bind9.
[Resolve]
DNS=${dns_ip}
Domains=~cloud.local
EOF
chmod 644 /etc/systemd/resolved.conf.d/httpaas.conf

systemctl restart systemd-resolved
resolvectl dns "$iface" "$dns_ip" || true
resolvectl domain "$iface" "~cloud.local" || true
resolvectl flush-caches || true

echo "DNS del laboratorio configurado: ${iface} -> ${dns_ip} (zona cloud.local)"
resolvectl query ns.cloud.local || true
