// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package provisioner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// ISOStore localiza y descarga imágenes ISO de Debian.
type ISOStore struct {
	client *http.Client
}

// NewISOStore crea el almacén de imágenes con un cliente HTTP con tiempo límite global.
func NewISOStore() *ISOStore {
	return &ISOStore{
		client: &http.Client{Timeout: 6 * time.Hour},
	}
}

// Locate busca una imagen cloud de Debian 13 en el directorio indicado.
func (s *ISOStore) Locate(_ context.Context, dir string) (string, bool) {
	matches, err := filepath.Glob(filepath.Join(dir, "debian-13*generic*.raw"))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	return matches[0], true
}

// Download descarga la ISO indicada y reporta el avance en porcentaje.
func (s *ISOStore) Download(ctx context.Context, url, dir string, progress ports.ProgressFunc) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("no se pudo crear el directorio de imágenes: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("no se pudo descargar la imagen: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("la descarga respondió HTTP %d", resp.StatusCode)
	}

	name := filepath.Base(resp.Request.URL.Path)
	if name == "" || name == "." || name == "/" {
		name = "debian.iso"
	}
	dest := filepath.Join(dir, name)
	tmp := dest + ".part"

	out, err := os.Create(tmp)
	if err != nil {
		return "", err
	}

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 1<<20)
	lastReport := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				_ = out.Close()
				_ = os.Remove(tmp)
				return "", err
			}
			written += int64(n)
			if progress != nil && (total <= 0 || time.Since(lastReport) > 500*time.Millisecond) {
				percent := -1
				if total > 0 {
					percent = int(written * 100 / total)
				}
				progress(percent, fmt.Sprintf("%s / %s", humanBytes(written), humanBytes(total)))
				lastReport = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = out.Close()
			_ = os.Remove(tmp)
			return "", readErr
		}
	}

	if err := out.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	if progress != nil {
		progress(100, "Imagen descargada")
	}
	return dest, nil
}

func humanBytes(n int64) string {
	if n <= 0 {
		return "—"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
