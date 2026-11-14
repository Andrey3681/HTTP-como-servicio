#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Andrey y Equipo de Trabajo
#
# Prepara la VM plantilla ApacheServer: Apache2 con mod_vhost_alias, SSH y
# el directorio /var/www/vhosts. Se ejecuta como root dentro del invitado.

set -euo pipefail

VHOSTS_ROOT="/var/www/vhosts"

echo "[httpAAS] Preparando plantilla Apache"

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq --no-install-recommends apache2

a2enmod vhost_alias rewrite headers >/dev/null

mkdir -p "${VHOSTS_ROOT}"
chmod 755 "${VHOSTS_ROOT}"
if [ ! -f "${VHOSTS_ROOT}/index.html" ]; then
    cat > "${VHOSTS_ROOT}/index.html" <<'EOF'
<!doctype html><html lang="es"><head><meta charset="utf-8"><title>HTTPaaS</title></head>
<body><h1>Plantilla ApacheServer</h1><p>Este directorio es la raíz de los contenidos por instancia.</p></body></html>
EOF
fi

cat > /etc/apache2/sites-available/000-httpaas.conf <<'EOF'
# Cada FQDN <host>.cloud.local se sirve desde /var/www/vhosts/<host>/public
<VirtualHost *:80>
    ServerName apache.cloud.local
    ServerAlias *.cloud.local
    VirtualDocumentRoot /var/www/vhosts/%1/public

    <Directory "/var/www/vhosts/*/public">
        Options Indexes FollowSymLinks
        AllowOverride All
        Require all granted
    </Directory>

    ErrorLog ${APACHE_LOG_DIR}/httpaas-error.log
    CustomLog ${APACHE_LOG_DIR}/httpaas-access.log combined
</VirtualHost>
EOF

a2dissite 000-default >/dev/null 2>&1 || true
a2ensite 000-httpaas >/dev/null
apache2ctl configtest

systemctl enable apache2
systemctl restart apache2

if ! systemctl is-active --quiet apache2; then
    echo "[httpAAS] ERROR: Apache2 no está activo" >&2
    exit 1
fi

if ! command -v VBoxControl >/dev/null 2>&1; then
    echo "[httpAAS] ADVERTENCIA: Guest Additions no detectadas; el orquestador no podrá leer la IP." >&2
fi

echo "HTTPAAS_GUEST_OK apache"
