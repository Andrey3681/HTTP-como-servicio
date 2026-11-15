// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package seediso

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

const isoSectorSize = 2048

// Build genera una ISO NoCloud (volumen "cidata") con los archivos
// indicados, usando la primera herramienta disponible en la plataforma
// o el generador ISO 9660 (Rock Ridge + Joliet) nativo en Go.
func Build(ctx context.Context, executor ports.Executor, workDir, output string, files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("no hay archivos para la ISO semilla")
	}
	_ = os.Remove(output)

	// Linux / BSD: genisoimage o mkisofs con Joliet y Rock Ridge.
	for _, tool := range []string{"genisoimage", "mkisofs"} {
		if _, err := exec.LookPath(tool); err != nil {
			continue
		}
		args := append([]string{
			"-quiet", "-output", output,
			"-volid", "cidata", "-joliet", "-rock",
		}, files...)
		return runISOTool(ctx, executor, tool, args...)
	}

	// Linux / BSD / macOS: xorriso emulando mkisofs.
	if _, err := exec.LookPath("xorriso"); err == nil {
		args := append([]string{
			"-as", "mkisofs", "-quiet", "-output", output,
			"-volid", "cidata", "-joliet", "-rock",
		}, files...)
		return runISOTool(ctx, executor, "xorriso", args...)
	}

	// macOS: hdiutil trabaja sobre un directorio.
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("hdiutil"); err == nil {
			tmpDir, err := os.MkdirTemp(workDir, "seed-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(tmpDir, filepath.Base(file)), data, 0o644); err != nil {
					return err
				}
			}
			return runISOTool(ctx, executor, "hdiutil",
				"makehybrid", "-iso", "-joliet",
				"-default-volume-name", "cidata",
				"-o", output, tmpDir,
			)
		}
	}

	// Windows: oscdimg si está en el PATH; de lo contrario generador nativo en Go.
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("oscdimg"); err == nil {
			tmpDir, err := os.MkdirTemp(workDir, "seed-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(tmpDir, filepath.Base(file)), data, 0o644); err != nil {
					return err
				}
			}
			return runISOTool(ctx, executor, "oscdimg",
				"-m", "-o", "-u2", "-lCIDATA", tmpDir, output,
			)
		}
	}

	return writeNativeISO(output, "cidata", files)
}

func runISOTool(ctx context.Context, executor ports.Executor, tool string, args ...string) error {
	_, stderr, err := executor.Execute(ctx, tool, args...)
	if err != nil {
		detail := string(stderr)
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("%s falló: %s", tool, detail)
	}
	return nil
}

type isoFileEntry struct {
	name   string
	isoID  string
	data   []byte
	sector uint32
}

// writeNativeISO construye una imagen ISO 9660 con extensiones Rock Ridge (SUSP/RRIP)
// y Joliet (UCS-2) directamente en Go, sin depender de procesos externos ni COM.
func writeNativeISO(output, volumeID string, files []string) error {
	entries := make([]isoFileEntry, 0, len(files))
	for i, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("no se pudo leer %s: %w", path, err)
		}
		base := filepath.Base(path)
		entries = append(entries, isoFileEntry{
			name:  base,
			isoID: toISO9660Identifier(base, i),
			data:  data,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	// Mapa de sectores:
	// 0..15: Área del sistema
	// 16: Primary Volume Descriptor (PVD)
	// 17: Supplementary Volume Descriptor (Joliet)
	// 18: Volume Descriptor Set Terminator
	// 19: Path Table L (ISO)
	// 20: Path Table M (ISO)
	// 21: Path Table L (Joliet)
	// 22: Path Table M (Joliet)
	// 23: Directorio raíz ISO + Rock Ridge
	// 24: Directorio raíz Joliet
	// 25..: Datos de los archivos
	const (
		isoLPathSector    = 19
		isoMPathSector    = 20
		jolietLPathSector = 21
		jolietMPathSector = 22
		isoRootSector     = 23
		jolietRootSector  = 24
		firstDataSector   = 25
	)

	currentSector := uint32(firstDataSector)
	for i := range entries {
		entries[i].sector = currentSector
		sectors := uint32((len(entries[i].data) + isoSectorSize - 1) / isoSectorSize)
		if sectors == 0 {
			sectors = 1
		}
		currentSector += sectors
	}

	totalSectors := currentSector + 150
	if totalSectors < 180 {
		totalSectors = 180
	}

	img := make([]byte, int(totalSectors)*isoSectorSize)

	// Sector 16: Primary Volume Descriptor.
	pvd := img[16*isoSectorSize : 17*isoSectorSize]
	pvd[0] = 0x01
	copy(pvd[1:6], "CD001")
	pvd[6] = 0x01
	putPaddedASCII(pvd[8:40], "LINUX")
	putPaddedASCII(pvd[40:72], volumeID)
	putBothEndian32(pvd[80:88], totalSectors)
	putBothEndian16(pvd[120:124], 1)
	putBothEndian16(pvd[124:128], 1)
	putBothEndian16(pvd[128:132], isoSectorSize)
	putBothEndian32(pvd[132:140], 10)
	binary.LittleEndian.PutUint32(pvd[140:144], isoLPathSector)
	binary.BigEndian.PutUint32(pvd[148:152], isoMPathSector)
	rootRec := buildDirRecord(isoRootSector, isoSectorSize, 0x02, []byte{0x00}, nil)
	copy(pvd[156:156+len(rootRec)], rootRec)
	putPaddedASCII(pvd[190:318], "")
	putPaddedASCII(pvd[318:446], "")
	putPaddedASCII(pvd[446:574], "HTTPAAS")
	putPaddedASCII(pvd[574:702], "HTTPAAS SEED GENERATOR")
	putPaddedASCII(pvd[702:813], "")
	putVolumeDates(pvd[813:881])
	pvd[881] = 0x01

	// Sector 17: Supplementary Volume Descriptor (Joliet UCS-2 Level 3).
	svd := img[17*isoSectorSize : 18*isoSectorSize]
	svd[0] = 0x02
	copy(svd[1:6], "CD001")
	svd[6] = 0x01
	putPaddedUTF16BE(svd[8:40], "LINUX")
	putPaddedUTF16BE(svd[40:72], volumeID)
	putBothEndian32(svd[80:88], totalSectors)
	copy(svd[88:91], []byte{0x25, 0x2F, 0x45}) // %/E
	putBothEndian16(svd[120:124], 1)
	putBothEndian16(svd[124:128], 1)
	putBothEndian16(svd[128:132], isoSectorSize)
	putBothEndian32(svd[132:140], 10)
	binary.LittleEndian.PutUint32(svd[140:144], jolietLPathSector)
	binary.BigEndian.PutUint32(svd[148:152], jolietMPathSector)
	jolietRootRec := buildDirRecord(jolietRootSector, isoSectorSize, 0x02, []byte{0x00}, nil)
	copy(svd[156:156+len(jolietRootRec)], jolietRootRec)
	putPaddedUTF16BE(svd[190:318], "")
	putPaddedUTF16BE(svd[318:446], "")
	putPaddedUTF16BE(svd[446:574], "HTTPAAS")
	putPaddedUTF16BE(svd[574:702], "HTTPAAS")
	putVolumeDates(svd[813:881])
	svd[881] = 0x01

	// Sector 18: Volume Descriptor Set Terminator.
	term := img[18*isoSectorSize : 19*isoSectorSize]
	term[0] = 0xFF
	copy(term[1:6], "CD001")
	term[6] = 0x01

	// Sectores 19..22: Tablas de rutas.
	writePathTable(img[isoLPathSector*isoSectorSize:], isoRootSector, false)
	writePathTable(img[isoMPathSector*isoSectorSize:], isoRootSector, true)
	writePathTable(img[jolietLPathSector*isoSectorSize:], jolietRootSector, false)
	writePathTable(img[jolietMPathSector*isoSectorSize:], jolietRootSector, true)

	// Sector 23: Directorio raíz ISO 9660 con Rock Ridge (SUSP + RRIP).
	isoRoot := img[isoRootSector*isoSectorSize : (isoRootSector+1)*isoSectorSize]
	offset := 0
	dotSU := append(buildRockRidgeSP(), buildRockRidgePX(0o040755, 2)...)
	dotSU = append(dotSU, buildRockRidgeER()...)
	rec := buildDirRecord(isoRootSector, isoSectorSize, 0x02, []byte{0x00}, dotSU)
	offset += copy(isoRoot[offset:], rec)

	dotDotSU := buildRockRidgePX(0o040755, 2)
	rec = buildDirRecord(isoRootSector, isoSectorSize, 0x02, []byte{0x01}, dotDotSU)
	offset += copy(isoRoot[offset:], rec)

	for _, entry := range entries {
		fileSU := append(buildRockRidgeRR(0x09), buildRockRidgePX(0o100644, 1)...)
		fileSU = append(fileSU, buildRockRidgeNM(entry.name)...)
		rec = buildDirRecord(entry.sector, uint32(len(entry.data)), 0x00, []byte(entry.isoID), fileSU)
		offset += copy(isoRoot[offset:], rec)
	}

	// Sector 24: Directorio raíz Joliet (UCS-2 Big Endian).
	jolietRoot := img[jolietRootSector*isoSectorSize : (jolietRootSector+1)*isoSectorSize]
	jOffset := 0
	jOffset += copy(jolietRoot[jOffset:], buildDirRecord(jolietRootSector, isoSectorSize, 0x02, []byte{0x00}, nil))
	jOffset += copy(jolietRoot[jOffset:], buildDirRecord(jolietRootSector, isoSectorSize, 0x02, []byte{0x01}, nil))
	for _, entry := range entries {
		ucs2Name := encodeUTF16BE(entry.name + ";1")
		rec = buildDirRecord(entry.sector, uint32(len(entry.data)), 0x00, ucs2Name, nil)
		jOffset += copy(jolietRoot[jOffset:], rec)
	}

	// Sectores 25..: Contenido de los archivos.
	for _, entry := range entries {
		start := int(entry.sector) * isoSectorSize
		copy(img[start:start+len(entry.data)], entry.data)
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(output, img, 0o644)
}

func toISO9660Identifier(name string, index int) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		s = fmt.Sprintf("FILE%d", index)
	}
	if len(s) > 8 {
		s = s[:8]
	}
	return s + ".;1"
}

func buildDirRecord(extent, dataLen uint32, flags byte, name, systemUse []byte) []byte {
	suOffset := 33 + len(name)
	if len(name)%2 == 0 {
		suOffset++
	}
	totalLen := suOffset + len(systemUse)
	if totalLen%2 != 0 {
		totalLen++
	}
	rec := make([]byte, totalLen)
	rec[0] = byte(totalLen)
	rec[1] = 0
	putBothEndian32(rec[2:10], extent)
	putBothEndian32(rec[10:18], dataLen)
	// Fecha: 2026-01-01 00:00:00 UTC
	rec[18] = 126
	rec[19] = 1
	rec[20] = 1
	rec[25] = flags
	putBothEndian16(rec[28:32], 1)
	rec[32] = byte(len(name))
	copy(rec[33:33+len(name)], name)
	if len(systemUse) > 0 {
		copy(rec[suOffset:suOffset+len(systemUse)], systemUse)
	}
	return rec
}

func buildRockRidgeSP() []byte {
	return []byte{'S', 'P', 7, 1, 0xBE, 0xEF, 0}
}

func buildRockRidgeRR(flags byte) []byte {
	return []byte{'R', 'R', 5, 1, flags}
}

func buildRockRidgePX(mode, nlink uint32) []byte {
	px := make([]byte, 36)
	px[0] = 'P'
	px[1] = 'X'
	px[2] = 36
	px[3] = 1
	putBothEndian32(px[4:12], mode)
	putBothEndian32(px[12:20], nlink)
	putBothEndian32(px[20:28], 0)
	putBothEndian32(px[28:36], 0)
	return px
}

func buildRockRidgeNM(name string) []byte {
	nb := []byte(name)
	nm := make([]byte, 5+len(nb))
	nm[0] = 'N'
	nm[1] = 'M'
	nm[2] = byte(5 + len(nb))
	nm[3] = 1
	nm[4] = 0
	copy(nm[5:], nb)
	return nm
}

func buildRockRidgeER() []byte {
	const id = "RRIP_1991A"
	er := make([]byte, 8+len(id))
	er[0] = 'E'
	er[1] = 'R'
	er[2] = byte(len(er))
	er[3] = 1
	er[4] = byte(len(id))
	er[5] = 0
	er[6] = 0
	er[7] = 1
	copy(er[8:], id)
	return er
}

func writePathTable(dst []byte, rootSector uint32, bigEndian bool) {
	dst[0] = 1
	dst[1] = 0
	if bigEndian {
		binary.BigEndian.PutUint32(dst[2:6], rootSector)
		binary.BigEndian.PutUint16(dst[6:8], 1)
	} else {
		binary.LittleEndian.PutUint32(dst[2:6], rootSector)
		binary.LittleEndian.PutUint16(dst[6:8], 1)
	}
	dst[8] = 0
	dst[9] = 0
}

func putBothEndian16(dst []byte, v uint16) {
	binary.LittleEndian.PutUint16(dst[0:2], v)
	binary.BigEndian.PutUint16(dst[2:4], v)
}

func putBothEndian32(dst []byte, v uint32) {
	binary.LittleEndian.PutUint32(dst[0:4], v)
	binary.BigEndian.PutUint32(dst[4:8], v)
}

func putPaddedASCII(dst []byte, s string) {
	for i := range dst {
		dst[i] = ' '
	}
	copy(dst, s)
}

func encodeUTF16BE(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	out := make([]byte, len(u16)*2)
	for i, v := range u16 {
		binary.BigEndian.PutUint16(out[i*2:], v)
	}
	return out
}

func putPaddedUTF16BE(dst []byte, s string) {
	for i := 0; i+1 < len(dst); i += 2 {
		dst[i] = 0x00
		dst[i+1] = 0x20
	}
	encoded := encodeUTF16BE(s)
	if len(encoded) > len(dst) {
		encoded = encoded[:len(dst)]
	}
	copy(dst, encoded)
}

func putVolumeDates(dst []byte) {
	const dateSet = "2026010100000000\x00"
	const dateZero = "0000000000000000\x00"
	copy(dst[0:17], dateSet)
	copy(dst[17:34], dateSet)
	copy(dst[34:51], dateZero)
	copy(dst[51:68], dateSet)
}
