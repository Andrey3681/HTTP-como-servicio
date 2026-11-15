// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

//go:build windows

package provisioner

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// freeDiskGB retorna el espacio libre en GB de la ruta indicada.
func freeDiskGB(path string) int64 {
	for dir := path; dir != ""; dir = filepath.Dir(dir) {
		if ptr, err := syscall.UTF16PtrFromString(dir); err == nil {
			var free, total, totalFree uint64
			r1, _, _ := getDiskFreeSpaceEx.Call(
				uintptr(unsafe.Pointer(ptr)),
				uintptr(unsafe.Pointer(&free)),
				uintptr(unsafe.Pointer(&total)),
				uintptr(unsafe.Pointer(&totalFree)),
			)
			if r1 != 0 {
				return int64(free / (1024 * 1024 * 1024))
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return 0
}
