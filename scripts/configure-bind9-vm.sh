#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Andrey y Equipo de Trabajo

set -euo pipefail

echo "=== Configuración de Servidor Bind9 en VM ns (Debian 13.0) ==="

# 1. Actualizar e instalar Bind9
apt-get update
apt-get install -y bind9 bind9utils bind9-doc dnsutils

# 2. Configurar opciones globales (/etc/bind/named.conf.options)
cat << 'EOF' > /etc/bind/named.conf.options
options {
    directory "/var/cache/bind";
    recursion yes;
    allow-query { any; };
    listen-on { 127.0.0.1; 192.168.10.10; };
    listen-on-v6 { none; };
    forwarders {
        8.8.8.8;
        1.1.1.1;
    };
    dnssec-validation no;
};
EOF

# 3. Declarar la zona local cloud.local (/etc/bind/named.conf.local)
cat << 'EOF' > /etc/bind/named.conf.local
zone "cloud.local" {
    type master;
    file "/var/lib/bind/db.cloud.local";
    allow-update { any; }; // Permitir actualización dinámica local
};
EOF

# 4. Crear archivo inicial de zona (/var/lib/bind/db.cloud.local)
cat << 'EOF' > /var/lib/bind/db.cloud.local
$TTL    86400
@       IN      SOA     ns.cloud.local. admin.cloud.local. (
                              1         ; Serial
                         604800         ; Refresh
                          86400         ; Retry
                        2419200         ; Expire
                          86400 )       ; Negative Cache TTL
;
@       IN      NS      ns.cloud.local.
ns      IN      A       192.168.10.10
EOF

# 5. Ajustar permisos para Bind
chown -R bind:bind /var/lib/bind
chmod 775 /var/lib/bind

# 6. Reiniciar y habilitar servicio
systemctl restart named
systemctl enable named

echo "=== Bind9 configurado exitosamente en 192.168.10.10 para zona cloud.local ==="

