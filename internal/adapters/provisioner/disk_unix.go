// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

//go:build !windows

package provisioner

import (
	"path/filepath"
	"syscall"
)

// freeDiskGB retorna el espacio libre en GB de la ruta indicada.
func freeDiskGB(path string) int64 {
	for dir := path; dir != "" && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(dir, &stat); err == nil {
			return int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024 * 1024)
		}
	}
	return 0
}
