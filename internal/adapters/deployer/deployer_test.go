// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package deployer_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/netip"
	"regexp"
	"strings"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/deployer"
)

type recordingExecutor struct {
	calls []string
}

func (e *recordingExecutor) Execute(_ context.Context, command string, args ...string) ([]byte, []byte, error) {
	e.calls = append(e.calls, strings.Join(append([]string{command}, args...), " "))
	return []byte("OK"), nil, nil
}

func (e *recordingExecutor) find(prefix string) string {
	for _, call := range e.calls {
		if strings.HasPrefix(call, prefix) {
			return call
		}
	}
	return ""
}

var scriptPattern = regexp.MustCompile(`echo '([A-Za-z0-9+/=]+)' \| base64 -d`)

func TestApacheDeployer_DeployWebZip(t *testing.T) {
	exec := &recordingExecutor{}
	dep := deployer.NewApacheDeployer(exec, nil, func() string { return "jorge" }, "/keys/id_ed25519")
	data := []byte("PK\x05\x06" + string(make([]byte, 18))) // ZIP vacío válido

	err := dep.DeployWebZip(context.Background(), "paisajes", netip.MustParseAddr("192.168.57.103"), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("unexpected error deploying web zip: %v", err)
	}

	scp := exec.find("scp ")
	if !strings.Contains(scp, "jorge@192.168.57.103:/tmp/httpaas-paisajes.zip") {
		t.Errorf("scp sin destino esperado: %s", scp)
	}
	if !strings.Contains(scp, "/keys/id_ed25519") {
		t.Errorf("scp sin la clave del laboratorio: %s", scp)
	}

	ssh := ""
	for _, call := range exec.calls {
		if strings.HasPrefix(call, "ssh ") && strings.Contains(call, "base64 -d") {
			ssh = call
		}
	}
	match := scriptPattern.FindStringSubmatch(ssh)
	if len(match) < 2 {
		t.Fatalf("no se encontró el script remoto en: %s", ssh)
	}
	script, err := base64.StdEncoding.DecodeString(match[1])
	if err != nil {
		t.Fatalf("script no es base64 válido: %v", err)
	}
	for _, want := range []string{
		"ServerName paisajes.cloud.local",
		"/var/www/vhosts/paisajes/public",
		`a2ensite "$HOST"`,
		"systemctl reload apache2",
	} {
		if !strings.Contains(string(script), want) {
			t.Errorf("el script no contiene %q:\n%s", want, script)
		}
	}
}

func TestApacheDeployer_RejectsInvalidHostname(t *testing.T) {
	dep := deployer.NewApacheDeployer(&recordingExecutor{}, nil, nil, "/keys/id_ed25519")
	err := dep.DeployWebZip(context.Background(), "PAISAJES/x", netip.MustParseAddr("192.168.57.103"), bytes.NewReader(nil), 0)
	if err == nil {
		t.Fatal("expected an error for an invalid hostname")
	}
}

func TestApacheDeployer_RequiresKey(t *testing.T) {
	dep := deployer.NewApacheDeployer(&recordingExecutor{}, nil, nil, "")
	err := dep.DeployWebZip(context.Background(), "paisajes", netip.MustParseAddr("192.168.57.103"), bytes.NewReader(nil), 0)
	if err == nil {
		t.Fatal("expected an error without an SSH key")
	}
}

func TestApacheDeployer_RestartWebService(t *testing.T) {
	exec := &recordingExecutor{}
	dep := deployer.NewApacheDeployer(exec, nil, func() string { return "jorge" }, "/keys/id_ed25519")

	err := dep.RestartWebService(context.Background(), "mario", netip.MustParseAddr("192.168.57.105"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, call := range exec.calls {
		if strings.HasPrefix(call, "ssh ") && strings.Contains(call, "systemctl restart apache2") {
			found = true
			if !strings.Contains(call, "systemctl is-active apache2") {
				t.Errorf("no se verifica Apache activo: %s", call)
			}
			if !strings.Contains(call, "jorge@192.168.57.105") {
				t.Errorf("usuario o host incorrectos: %s", call)
			}
		}
	}
	if !found {
		t.Errorf("no se ejecutó el reinicio por SSH: %v", exec.calls)
	}
}
