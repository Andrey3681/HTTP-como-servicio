// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package deployer implementa ports.Deployer para transferir y descomprimir
// paquetes web en instancias Apache a través de SSH.
package deployer

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/netip"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// ApacheDeployer transfiere y extrae paquetes web en las máquinas virtuales Apache.
type ApacheDeployer struct {
	executor  ports.Executor
	logger    ports.EventLogger
	guestUser func() string
	keyPath   string
}

// NewApacheDeployer crea el despliegue SSH inyectando el usuario del invitado y
// la ruta de la clave privada del laboratorio.
func NewApacheDeployer(executor ports.Executor, logger ports.EventLogger, guestUser func() string, keyPath string) *ApacheDeployer {
	return &ApacheDeployer{
		executor:  executor,
		logger:    logger,
		guestUser: guestUser,
		keyPath:   keyPath,
	}
}

func (d *ApacheDeployer) log(level, message string, fields map[string]any) {
	if d.logger != nil {
		d.logger.Append(level, "deployer", message, fields)
	}
}

// DeployWebZip copia el paquete a la instancia, lo descomprime en su directorio
// público y publica un virtual host con el FQDN de la instancia.
func (d *ApacheDeployer) DeployWebZip(ctx context.Context, hostname string, targetIP netip.Addr, zipContent io.Reader, size int64) error {
	if !safeHostname(hostname) {
		return fmt.Errorf("hostname inválido para despliegue: %q", hostname)
	}
	if d.keyPath == "" {
		return fmt.Errorf("falta la clave SSH del laboratorio")
	}
	user := "debian"
	if d.guestUser != nil {
		if value := d.guestUser(); value != "" {
			user = value
		}
	}

	log.Printf("[Deployer] Desplegando contenido web en %s (%d bytes)", targetIP.String(), size)
	d.log("INFO", "Iniciando despliegue web", map[string]any{"instancia": hostname, "ip": targetIP.String(), "bytes": size})

	tmpFile, err := os.CreateTemp("", "web-upload-*.zip")
	if err != nil {
		return fmt.Errorf("fallo al crear archivo temporal para carga: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	written, err := io.Copy(tmpFile, zipContent)
	if err != nil {
		return fmt.Errorf("fallo al escribir el paquete web: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("fallo al cerrar el paquete web: %w", err)
	}
	d.log("INFO", "Paquete web recibido", map[string]any{"instancia": hostname, "bytes": written})

	if err := d.waitSSH(ctx, user, targetIP.String(), 90*time.Second); err != nil {
		return err
	}

	remoteZip := "/tmp/httpaas-" + hostname + ".zip"
	if _, stderr, err := d.executor.Execute(ctx, "scp", d.scpArgs(tmpPath, user, targetIP.String(), remoteZip)...); err != nil {
		return fmt.Errorf("fallo al copiar el paquete a %s: %w (%s)", targetIP, err, strings.TrimSpace(string(stderr)))
	}

	script := d.deployScript(hostname, remoteZip)
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	remote := "echo '" + encoded + "' | base64 -d | sudo -n bash -s"
	if _, stderr, err := d.executor.Execute(ctx, "ssh", d.sshArgs(user, targetIP.String(), remote)...); err != nil {
		return fmt.Errorf("fallo al publicar el sitio en %s: %w (%s)", targetIP, err, strings.TrimSpace(string(stderr)))
	}

	d.log("INFO", "Despliegue web completado", map[string]any{"instancia": hostname, "ip": targetIP.String()})
	return nil
}

// waitSSH espera a que el invitado acepte conexiones SSH antes de desplegar.
func (d *ApacheDeployer) waitSSH(ctx context.Context, user, ip string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		if _, stderr, err := d.executor.Execute(ctx, "ssh", d.sshArgs(user, ip, "true")...); err == nil {
			return nil
		} else {
			lastErr = fmt.Errorf("%v (%s)", err, strings.TrimSpace(string(stderr)))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("SSH no disponible en %s: %w", ip, lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// sshArgs arma las opciones comunes de SSH para el invitado.
func (d *ApacheDeployer) sshArgs(user, ip, command string) []string {
	return []string{
		"-i", d.keyPath,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=15",
		user + "@" + ip,
		command,
	}
}

// scpArgs arma las opciones de copia del paquete web.
func (d *ApacheDeployer) scpArgs(localPath, user, ip, remotePath string) []string {
	return []string{
		"-i", d.keyPath,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		localPath,
		user + "@" + ip + ":" + remotePath,
	}
}

// deployScript construye el script que se ejecuta como root en la instancia.
func (d *ApacheDeployer) deployScript(hostname, remoteZip string) string {
	siteRoot := path.Join("/var/www/vhosts", hostname, "public")
	return fmt.Sprintf(`set -euo pipefail
HOST=%q
ROOT=%q
mkdir -p "$ROOT"
unzip -o -q %q -d "$ROOT"
rm -f %q

# Si el paquete trae una única carpeta raíz, su contenido sube un nivel.
shopt -s nullglob dotglob
entries=("$ROOT"/*)
if [ ${#entries[@]} -eq 1 ] && [ -d "${entries[0]}" ]; then
    mv "${entries[0]}"/* "$ROOT"/
    rmdir "${entries[0]}"
fi
chmod -R a+rX "$ROOT"

cat > "/etc/apache2/sites-available/$HOST.conf" <<'APACHE'
<VirtualHost *:80>
    ServerName %s
    DocumentRoot %s
    <Directory %s>
        Options Indexes FollowSymLinks
        AllowOverride All
        Require all granted
    </Directory>
    ErrorLog ${APACHE_LOG_DIR}/%s-error.log
    CustomLog ${APACHE_LOG_DIR}/%s-access.log combined
</VirtualHost>
APACHE

a2ensite "$HOST" >/dev/null
apache2ctl configtest
systemctl reload apache2
`, hostname, siteRoot, remoteZip, remoteZip, hostname+".cloud.local", siteRoot, siteRoot, hostname, hostname)
}

// safeHostname acepta solo nombres ya validados por el dominio.
func safeHostname(hostname string) bool {
	if hostname == "" || len(hostname) > 63 {
		return false
	}
	for _, r := range hostname {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}
