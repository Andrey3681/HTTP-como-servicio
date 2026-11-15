#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Andrey y Equipo de Trabajo
#
# Configura Bind9 en la VM ns del laboratorio HTTPaaS.
# Se ejecuta como root dentro del invitado (ver docs/setup-inicial.md).

set -euo pipefail

ZONE="cloud.local"
NS_IP="${1:-192.168.10.10}"
ZONE_NET="${2:-192.168.10.0/24}"

echo "[httpAAS] Configurando Bind9 para ${ZONE} en ${NS_IP}"

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq --no-install-recommends bind9 bind9utils dnsutils

cat > /etc/bind/named.conf.options <<EOF
options {
    directory "/var/cache/bind";
    recursion yes;
    allow-query { any; };
    listen-on { 127.0.0.1; ${NS_IP}; };
    listen-on-v6 { none; };
    forwarders { 1.1.1.1; 8.8.8.8; };
    dnssec-validation no;
};
EOF

cat > /etc/bind/named.conf.local <<EOF
zone "cloud.local" {
    type master;
    file "/var/lib/bind/db.cloud.local";
    allow-update { ${ZONE_NET}; };
    allow-transfer { ${ZONE_NET}; };
};
EOF

cat > /var/lib/bind/db.cloud.local <<EOF
\$TTL    300
@       IN      SOA     ns.cloud.local. admin.cloud.local. (
                              1         ; Serial
                         604800         ; Refresh
                          86400         ; Retry
                        2419200         ; Expire
                            300 )       ; Negative Cache TTL
;
@       IN      NS      ns.cloud.local.
ns      IN      A       ${NS_IP}
EOF

chown -R bind:bind /var/lib/bind
chmod 775 /var/lib/bind
chmod 664 /var/lib/bind/db.cloud.local

named-checkconf
named-checkzone "${ZONE}" /var/lib/bind/db.cloud.local

systemctl enable named
systemctl restart named
sleep 1

if ! dig @127.0.0.1 "ns.${ZONE}" +short | grep -q "${NS_IP}"; then
    echo "[httpAAS] ERROR: Bind9 no resuelve ns.${ZONE}" >&2
    exit 1
fi

echo "HTTPAAS_GUEST_OK bind9"
