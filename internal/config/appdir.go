// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package config resuelve el directorio de datos del sistema para HTTPaaS.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvAppDir permite sobrescribir el directorio de datos por entorno.
const EnvAppDir = "HTTPAAS_DIR"

// SystemDir es la ubicación estándar del estado variable del servicio.
const SystemDir = "/var/lib/httpaas"

// Layout agrupa las rutas de datos que usa el orquestador.
type Layout struct {
	Base           string
	ISODir         string
	RawDir         string
	SeedDir        string
	InstancesFile  string
	SetupStateFile string
}

// ResolveAppDir determina el directorio de datos: variable de entorno, ruta de
// sistema escribible o, en su defecto, el directorio de datos del usuario.
func ResolveAppDir() string {
	if dir := strings.TrimSpace(os.Getenv(EnvAppDir)); dir != "" {
		return dir
	}
	if writableDir(SystemDir) {
		return SystemDir
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		return filepath.Join(xdg, "httpaas")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "httpaas")
	}
	return filepath.Join(os.TempDir(), "httpaas")
}

// EnsureLayout crea el directorio base y el de imágenes, y deriva las rutas.
func EnsureLayout(base string) (Layout, error) {
	if strings.TrimSpace(base) == "" {
		return Layout{}, fmt.Errorf("el directorio de datos no puede estar vacío")
	}
	layout := Layout{
		Base:           base,
		ISODir:         filepath.Join(base, "iso"),
		RawDir:         filepath.Join(base, "raw"),
		SeedDir:        filepath.Join(base, "seeds"),
		InstancesFile:  filepath.Join(base, "instances.json"),
		SetupStateFile: filepath.Join(base, "setup-estado.json"),
	}
	for _, dir := range []string{layout.Base, layout.ISODir, layout.RawDir, layout.SeedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Layout{}, fmt.Errorf("no se pudo crear %s: %w", dir, err)
		}
	}
	return layout, nil
}

func writableDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	probe, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return true
}
