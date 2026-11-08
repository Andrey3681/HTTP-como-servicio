// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package domain_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"net/netip"
	"testing"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		name      string
		hostname  string
		expectErr bool
	}{
		{name: "Valid lowercase", hostname: "paisajes", expectErr: false},
		{name: "Valid with hyphen", hostname: "mi-web", expectErr: false},
		{name: "Valid alphanumeric", hostname: "sitio123", expectErr: false},
		{name: "Too short", hostname: "ab", expectErr: true},
		{name: "Too long", hostname: "este-es-un-nombre-demasiado-largo-para-un-host-local", expectErr: true},
		{name: "Starts with hyphen", hostname: "-paisajes", expectErr: true},
		{name: "Ends with hyphen", hostname: "paisajes-", expectErr: true},
		{name: "Invalid characters", hostname: "paisajes!", expectErr: true},
		{name: "Reserved name ns", hostname: "ns", expectErr: true},
		{name: "Reserved name dns", hostname: "dns", expectErr: true},
		{name: "Reserved name localhost", hostname: "localhost", expectErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateHostname(tc.hostname)
			if tc.expectErr && err == nil {
				t.Fatalf("expected error for hostname '%s', got nil", tc.hostname)
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("unexpected error for hostname '%s': %v", tc.hostname, err)
			}
		})
	}
}

func TestNewInstance(t *testing.T) {
	inst, err := domain.NewInstance("paisajes", "cloud.local")
	if err != nil {
		t.Fatalf("unexpected error creating instance: %v", err)
	}

	if inst.Name != "paisajes" {
		t.Errorf("expected name 'paisajes', got '%s'", inst.Name)
	}
	if inst.FQDN != "paisajes.cloud.local" {
		t.Errorf("expected FQDN 'paisajes.cloud.local', got '%s'", inst.FQDN)
	}
	if inst.URL != "http://paisajes.cloud.local" {
		t.Errorf("expected URL 'http://paisajes.cloud.local', got '%s'", inst.URL)
	}
	if inst.Status != domain.StatusCreating {
		t.Errorf("expected status 'CREANDO', got '%s'", inst.Status)
	}
}

func TestNewARecord(t *testing.T) {
	ip := netip.MustParseAddr("192.168.10.30")
	rec, err := domain.NewARecord("paisajes", "cloud.local", ip, 300)
	if err != nil {
		t.Fatalf("unexpected error creating DNS record: %v", err)
	}

	if rec.Name != "paisajes.cloud.local." {
		t.Errorf("expected Name 'paisajes.cloud.local.', got '%s'", rec.Name)
	}
	if rec.IP != "192.168.10.30" {
		t.Errorf("expected IP '192.168.10.30', got '%s'", rec.IP)
	}
	if rec.Type != "A" {
		t.Errorf("expected Type 'A', got '%s'", rec.Type)
	}
}

func TestValidateZipPackage_ZipSlip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	maliciousFile, err := zw.Create("../../../etc/passwd")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	_, _ = maliciousFile.Write([]byte("root:x:0:0:..."))
	_ = zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	_, err = domain.ValidateZipPackage(reader, int64(buf.Len()))
	if err == nil {
		t.Fatalf("expected error detecting zip slip, got nil")
	}
	if !errors.Is(err, domain.ErrZipSlipDetected) {
		t.Fatalf("expected ErrZipSlipDetected, got: %v", err)
	}
}

func TestValidateZipPackage_Valid(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	indexFile, err := zw.Create("index.html")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	_, _ = indexFile.Write([]byte("<h1>Hola Mundo</h1>"))
	_ = zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	pkg, err := domain.ValidateZipPackage(reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("unexpected error validating valid zip: %v", err)
	}

	if !pkg.HasIndexHTML {
		t.Errorf("expected pkg.HasIndexHTML to be true")
	}
	if pkg.FileCount != 1 {
		t.Errorf("expected file count 1, got %d", pkg.FileCount)
	}
}
