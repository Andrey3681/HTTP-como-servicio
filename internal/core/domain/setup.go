// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var setupUserPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,31}$`)

// SetupStepID identifica un paso del configuración inicial.
type SetupStepID string

// Identificadores de los pasos de la preparación, en el orden en que se ejecutan.
const (
	SetupStepVerify      SetupStepID = "verificar"
	SetupStepISO         SetupStepID = "iso"
	SetupStepNetwork     SetupStepID = "red"
	SetupStepDNSVM       SetupStepID = "vm-ns"
	SetupStepDNSInstall  SetupStepID = "instalar-ns"
	SetupStepDNSConfig   SetupStepID = "config-ns"
	SetupStepBaseVM      SetupStepID = "vm-base"
	SetupStepBaseInstall SetupStepID = "instalar-base"
	SetupStepBaseConfig  SetupStepID = "config-base"
	SetupStepMultiattach SetupStepID = "multiatach"
	SetupStepFinal       SetupStepID = "final"
)

// SetupStepStatus describe el estado de un paso de la preparación.
type SetupStepStatus string

// Estados posibles de un paso.
const (
	StepPending   SetupStepStatus = "PENDIENTE"
	StepRunning   SetupStepStatus = "EN_CURSO"
	StepCompleted SetupStepStatus = "COMPLETADO"
	StepError     SetupStepStatus = "ERROR"
	StepSkipped   SetupStepStatus = "OMITIDO"
)

// SetupStep representa un paso visible del configuración inicial.
type SetupStep struct {
	ID          SetupStepID     `json:"id"`
	Nombre      string          `json:"nombre"`
	Descripcion string          `json:"descripcion"`
	Estado      SetupStepStatus `json:"estado"`
	Progreso    int             `json:"progreso"`
	Mensaje     string          `json:"mensaje,omitempty"`
}

// SetupInput es la entrada del formulario de la preparación.
type SetupInput struct {
	Usuario    string `json:"usuario"`
	Contrasena string `json:"contrasena"`
}

// Validate comprueba las reglas del formulario de la preparación.
func (in SetupInput) Validate() error {
	if !setupUserPattern.MatchString(in.Usuario) {
		return fmt.Errorf("el usuario debe tener entre 3 y 32 caracteres, iniciar con letra y usar solo minúsculas, números, guion o guion bajo")
	}
	if strings.TrimSpace(in.Contrasena) == "" {
		return fmt.Errorf("la contraseña es obligatoria")
	}
	return nil
}

// SetupConfiguracion es la parte pública (sin contraseña) del formulario.
type SetupConfiguracion struct {
	Usuario string `json:"usuario"`
	Lista   bool   `json:"lista"`
}

// SetupState es la instantánea completa de la preparación, apta para SSE y JSON.
type SetupState struct {
	RequiereConfiguracion bool               `json:"requiere_configuracion"`
	Ejecutando            bool               `json:"ejecutando"`
	Completado            bool               `json:"completado"`
	Error                 string             `json:"error,omitempty"`
	Configuracion         SetupConfiguracion `json:"configuracion"`
	Red                   NetworkConfig      `json:"red"`
	Imagen                string             `json:"imagen,omitempty"`
	Pasos                 []SetupStep        `json:"pasos"`
	ActualizadoEn         time.Time          `json:"actualizado_en"`
}

// Step devuelve el paso con el identificador indicado y un indicador de existencia.
func (s SetupState) Step(id SetupStepID) (SetupStep, bool) {
	for _, step := range s.Pasos {
		if step.ID == id {
			return step, true
		}
	}
	return SetupStep{}, false
}

// SetupConfig agrupa los parámetros del entorno real que prepara la preparación.
type SetupConfig struct {
	RawDir           string
	SeedDir          string
	SSHDir           string
	SSHKeyPath       string
	SeedTemplatesDir string
	RawURL           string

	DNSVMName    string
	BaseVMName   string
	DNSHostname  string
	BaseHostname string

	GuestUser      string
	GuestPassword  string
	MemoryMB       int
	DiskMB         int
	InstallTimeout time.Duration
}

// EnvironmentStatus resume lo que ya existe en el host antes de configurar.
type EnvironmentStatus struct {
	VBoxAvailable       bool     `json:"vbox_disponible"`
	VBoxVersion         string   `json:"vbox_version,omitempty"`
	ExistingVMs         []string `json:"vms_existentes,omitempty"`
	HostOnlyInterface   string   `json:"interfaz_hostonly,omitempty"`
	HostOnlyIP          string   `json:"interfaz_ip,omitempty"`
	HasDNSVM            bool     `json:"vm_ns"`
	HasBaseVM           bool     `json:"vm_base"`
	BaseDiskPath        string   `json:"disco_base,omitempty"`
	BaseDiskMultiattach bool     `json:"disco_multiatach"`
	FreeDiskGB          int64    `json:"disco_libre_gb"`
	Ready               bool     `json:"listo"`
}
