// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// InstanceStatus define los posibles estados de ciclo de vida de una instancia.
type InstanceStatus string

const (
	// StatusCreating indica que la máquina virtual se encuentra en fase de aprovisionamiento.
	StatusCreating InstanceStatus = "CREANDO"

	// StatusRunning indica que la instancia está activa, con DNS registrado y Apache sirviendo contenido.
	StatusRunning InstanceStatus = "ACTIVA"

	// StatusError indica que ocurrió una falla durante el aprovisionamiento o ejecución.
	StatusError InstanceStatus = "ERROR"

	// StatusStopped indica que la instancia está apagada y puede reactivarse.
	StatusStopped InstanceStatus = "DETENIDA"

	// StatusTerminated indica que la instancia ha sido apagada y desmantelada.
	StatusTerminated InstanceStatus = "ELIMINADA"
)

// Instance representa una máquina virtual que aloja una instancia del servidor web Apache.
type Instance struct {
	ID        string         `json:"id"`
	Name      string         `json:"nombre"`
	FQDN      string         `json:"fqdn"`
	IP        string         `json:"ip"`
	URL       string         `json:"enlace"`
	Status    InstanceStatus `json:"estado"`
	CreatedAt time.Time      `json:"creado_en"`
}

var (
	hostnameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`)
	reservedHosts = map[string]bool{
		"ns":        true,
		"dns":       true,
		"router":    true,
		"gateway":   true,
		"localhost": true,
		"mail":      true,
		"cloud":     true,
	}
)

// ValidateHostname valida que el nombre de host cumpla las reglas de formato, longitud y exclusión de nombres reservados.
func ValidateHostname(name string) error {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if len(trimmed) < 3 || len(trimmed) > 32 {
		return fmt.Errorf("%w: la longitud debe estar entre 3 y 32 caracteres", ErrInvalidHostname)
	}

	if !hostnameRegex.MatchString(trimmed) {
		return fmt.Errorf("%w: solo se permiten letras minúsculas, números y guiones intermedios", ErrInvalidHostname)
	}

	if reservedHosts[trimmed] {
		return fmt.Errorf("%w: el nombre '%s' es una palabra reservada del sistema", ErrInvalidHostname, trimmed)
	}

	return nil
}

// NewInstance crea una nueva entidad de instancia inicializada en estado de creación.
func NewInstance(name string, domainZone string) (*Instance, error) {
	if err := ValidateHostname(name); err != nil {
		return nil, err
	}

	trimmed := strings.ToLower(strings.TrimSpace(name))
	cleanZone := strings.TrimSuffix(domainZone, ".")
	fqdn := fmt.Sprintf("%s.%s", trimmed, cleanZone)
	url := fmt.Sprintf("http://%s", fqdn)

	return &Instance{
		ID:        trimmed,
		Name:      trimmed,
		FQDN:      fqdn,
		URL:       url,
		Status:    StatusCreating,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// InstanceAccess describe cómo acceder por SSH a una instancia aprovisionada.
type InstanceAccess struct {
	Usuario             string `json:"usuario"`
	Host                string `json:"host"`
	IP                  string `json:"ip"`
	FQDN                string `json:"fqdn"`
	Puerto              int    `json:"puerto"`
	Comando             string `json:"comando"`
	UsaClaveLaboratorio bool   `json:"usa_clave_laboratorio"`
	NotaClave           string `json:"nota_clave"`
}
