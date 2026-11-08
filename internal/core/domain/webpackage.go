// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package domain

import (
	"archive/zip"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const (
	// MaxZipSizeBytes define el límite máximo admitido para un archivo .zip (50 MB).
	MaxZipSizeBytes = 50 * 1024 * 1024
)

// WebPackage encapsula la información y validaciones de un archivo comprimido web.
type WebPackage struct {
	Filename     string
	SizeBytes    int64
	FileCount    int
	HasIndexHTML bool
}

// ValidateZipPackage inspecciona un lector zip buscando anomalías de seguridad (Zip Slip) y comprobando contenido.
func ValidateZipPackage(reader io.ReaderAt, size int64) (*WebPackage, error) {
	if size <= 0 || size > MaxZipSizeBytes {
		return nil, fmt.Errorf("%w: el tamaño (%d bytes) excede el máximo permitido (%d bytes)",
			ErrInvalidWebPackage, size, MaxZipSizeBytes)
	}

	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, fmt.Errorf("%w: fallo al leer estructura zip: %w", ErrInvalidWebPackage, err)
	}

	pkg := &WebPackage{
		SizeBytes: size,
		FileCount: len(zipReader.File),
	}

	for _, file := range zipReader.File {
		cleanPath := filepath.Clean(file.Name)

		// Verificación de Zip Slip (path traversal)
		if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) || strings.Contains(file.Name, `../`) || strings.Contains(file.Name, `..\`) {
			return nil, fmt.Errorf("%w: ruta peligrosa detectada '%s'", ErrZipSlipDetected, file.Name)
		}

		baseName := strings.ToLower(filepath.Base(cleanPath))
		if baseName == "index.html" || baseName == "index.htm" || baseName == "index.php" {
			pkg.HasIndexHTML = true
		}
	}

	return pkg, nil
}
