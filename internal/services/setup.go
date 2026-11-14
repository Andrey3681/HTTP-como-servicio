// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/ports"
)

// Errores del configuración inicial.
var (
	// ErrSetupEjecutando indica que la preparación ya se encuentra en ejecución.
	ErrSetupEjecutando = errors.New("la configuración inicial ya está en ejecución")

	// ErrSetupCompletado indica que el entorno ya fue configurado.
	ErrSetupCompletado = errors.New("el entorno ya está configurado")

	// ErrSetupSinConfigurar indica que falta completar el formulario de la preparación.
	ErrSetupSinConfigurar = errors.New("completa los datos de la preparación antes de iniciar")
)

// setupRecord es la representación persistida del avance de la preparación.
type setupRecord struct {
	Completado     bool                  `json:"completado"`
	PasosCompletos []domain.SetupStepID  `json:"pasos_completos,omitempty"`
	BaseDiskPath   string                `json:"disco_base,omitempty"`
	Network        *domain.NetworkConfig `json:"red,omitempty"`
	Usuario        string                `json:"usuario,omitempty"`
	ActualizadoEn  time.Time             `json:"actualizado_en"`
}

// SetupService orquesta la configuración inicial del laboratorio y publica su
// estado a los suscriptores en tiempo real.
type SetupService struct {
	cfg       domain.SetupConfig
	prov      ports.SetupProvisioner
	images    ports.ImageStore
	logger    ports.EventLogger
	stateFile string

	mu          sync.Mutex
	state       domain.SetupState
	running     bool
	input       domain.SetupInput
	network     domain.NetworkConfig
	completed   map[domain.SetupStepID]bool
	baseDisk    string
	progressLog map[domain.SetupStepID]time.Time
	subs        map[int]chan domain.SetupState
	nextSub     int
}

// NewSetupService crea el servicio de la preparación con el estado inicial pendiente.
func NewSetupService(cfg domain.SetupConfig, prov ports.SetupProvisioner, images ports.ImageStore, logger ports.EventLogger, stateFile string) *SetupService {
	s := &SetupService{
		cfg:         cfg,
		prov:        prov,
		images:      images,
		logger:      logger,
		stateFile:   stateFile,
		completed:   make(map[domain.SetupStepID]bool),
		progressLog: make(map[domain.SetupStepID]time.Time),
		subs:        make(map[int]chan domain.SetupState),
		state: domain.SetupState{
			RequiereConfiguracion: true,
			Pasos:                 setupSteps(),
			ActualizadoEn:         time.Now().UTC(),
		},
	}
	s.loadRecord()
	return s
}

func setupSteps() []domain.SetupStep {
	steps := []domain.SetupStep{
		{ID: domain.SetupStepVerify, Nombre: "Verificar entorno", Descripcion: "VirtualBox, espacio en disco y componentes existentes"},
		{ID: domain.SetupStepISO, Nombre: "Imagen base de Debian", Descripcion: "Imagen cloud de Debian 13; se reutiliza si ya está descargada"},
		{ID: domain.SetupStepNetwork, Nombre: "Red del laboratorio", Descripcion: "Interfaz host-only de VirtualBox y DHCP en su subred"},
		{ID: domain.SetupStepDNSVM, Nombre: "Máquina del servidor DNS", Descripcion: "Disco desde la imagen base y semilla cloud-init"},
		{ID: domain.SetupStepDNSInstall, Nombre: "Preparar la máquina DNS", Descripcion: "Primer arranque: paquetes, usuario y Guest Additions"},
		{ID: domain.SetupStepDNSConfig, Nombre: "Configurar Bind9", Descripcion: "Zona cloud.local. con actualizaciones dinámicas"},
		{ID: domain.SetupStepBaseVM, Nombre: "Máquina plantilla", Descripcion: "Disco desde la imagen base y semilla cloud-init"},
		{ID: domain.SetupStepBaseInstall, Nombre: "Preparar la plantilla", Descripcion: "Primer arranque: Apache, usuario y Guest Additions"},
		{ID: domain.SetupStepBaseConfig, Nombre: "Configurar Apache", Descripcion: "mod_vhost_alias, /var/www/vhosts y sitio por defecto"},
		{ID: domain.SetupStepMultiattach, Nombre: "Disco multiconexión", Descripcion: "Desconecta, marca multiattach y reconecta el disco base"},
		{ID: domain.SetupStepFinal, Nombre: "Verificación final", Descripcion: "VMs, disco base y resolución DNS"},
	}
	for i := range steps {
		steps[i].Estado = domain.StepPending
	}
	return steps
}

// Estado retorna una copia de la instantánea actual.
func (s *SetupService) Estado() domain.SetupState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cloneStateLocked()
}

// Configurar valida y aplica los datos del formulario de la preparación.
func (s *SetupService) Configurar(in domain.SetupInput) error {
	if err := in.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.input = in
	s.cfg.GuestUser = in.Usuario
	s.cfg.GuestPassword = in.Contrasena
	s.state.Configuracion = domain.SetupConfiguracion{
		Usuario: in.Usuario,
		Lista:   true,
	}
	s.persistLocked()
	s.broadcastLocked()
	return nil
}

// Network retorna la red host-only detectada o derivada.
func (s *SetupService) Network() domain.NetworkConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.network
}

// BaseDiskPath retorna la ruta persistida del disco base multiconexión.
func (s *SetupService) BaseDiskPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.baseDisk
}

// Subscribe registra un observador del estado y retorna el canal y su cancelación.
func (s *SetupService) Subscribe() (<-chan domain.SetupState, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextSub
	s.nextSub++
	ch := make(chan domain.SetupState, 16)
	s.subs[id] = ch

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if existing, ok := s.subs[id]; ok {
			delete(s.subs, id)
			close(existing)
		}
	}
	return ch, cancel
}

func (s *SetupService) cloneStateLocked() domain.SetupState {
	clone := s.state
	clone.Pasos = append([]domain.SetupStep(nil), s.state.Pasos...)
	return clone
}

func (s *SetupService) broadcastLocked() {
	s.state.ActualizadoEn = time.Now().UTC()
	snapshot := s.cloneStateLocked()
	for id, ch := range s.subs {
		select {
		case ch <- snapshot:
		default:
			// Un suscriptor lento no debe bloquear la configuración.
			_ = id
		}
	}
}

func (s *SetupService) updateStep(id domain.SetupStepID, status domain.SetupStepStatus, progress int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.state.Pasos {
		if s.state.Pasos[i].ID != id {
			continue
		}
		s.state.Pasos[i].Estado = status
		if progress >= 0 {
			s.state.Pasos[i].Progreso = progress
		}
		if message != "" {
			s.state.Pasos[i].Mensaje = message
		}
		s.broadcastLocked()
		return
	}
}

func (s *SetupService) log(level, message string, fields map[string]any) {
	if s.logger == nil {
		return
	}
	s.logger.Append(level, "setup", message, fields)
}

// ActualizarDeteccion inspecciona el host y decide si hace falta configuración.
func (s *SetupService) ActualizarDeteccion(ctx context.Context) error {
	status, err := s.prov.Inspect(ctx, s.cfg)
	if err != nil {
		s.mu.Lock()
		s.state.RequiereConfiguracion = true
		s.state.Error = fmt.Sprintf("no se pudo inspeccionar el entorno: %v", err)
		s.broadcastLocked()
		s.mu.Unlock()
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// La imagen base se busca solo en el directorio de datos de HTTPaaS; el
	// VDI reutilizable se deriva de la raw.
	s.state.Imagen = ""
	if raw, ok := s.images.Locate(ctx, s.cfg.RawDir); ok {
		base := strings.TrimSuffix(raw, ".raw") + ".vdi"
		if _, err := os.Stat(base); err == nil {
			s.state.Imagen = base
		}
	}
	if network, err := s.prov.Network(ctx); err == nil {
		s.network = network
		s.state.Red = network
	}

	s.state.Pasos = setupSteps()
	s.markIf(status.VBoxAvailable, domain.SetupStepVerify, "VirtualBox "+status.VBoxVersion)
	s.markIf(status.HostOnlyInterface != "", domain.SetupStepNetwork, "Interfaz "+status.HostOnlyInterface+" · "+s.network.CIDR)
	s.markIf(status.BaseDiskMultiattach, domain.SetupStepMultiattach, "Disco base en modo multiconexión")
	// Los pasos de creación no se heredan entre reinicios: son idempotentes y
	// garantizan que la VM y su semilla existan antes de continuar.
	if !status.Ready {
		delete(s.completed, domain.SetupStepISO)
		delete(s.completed, domain.SetupStepDNSVM)
		delete(s.completed, domain.SetupStepBaseVM)
	}
	if status.BaseDiskPath != "" {
		s.baseDisk = status.BaseDiskPath
	}

	s.state.Completado = status.Ready
	s.state.RequiereConfiguracion = !status.Ready
	if status.Ready {
		for i := range s.state.Pasos {
			s.state.Pasos[i].Estado = domain.StepCompleted
			s.state.Pasos[i].Progreso = 100
		}
		s.completed = map[domain.SetupStepID]bool{}
		for _, step := range s.state.Pasos {
			s.completed[step.ID] = true
		}
	}
	if !status.VBoxAvailable {
		s.state.Error = "VBoxManage no está disponible en el PATH"
	}
	s.broadcastLocked()
	s.persistLocked()
	return nil
}

// markIf marca un paso como completado cuando la condición se cumple.
func (s *SetupService) markIf(condition bool, id domain.SetupStepID, message string) {
	if !condition {
		return
	}
	for i := range s.state.Pasos {
		if s.state.Pasos[i].ID == id {
			s.state.Pasos[i].Estado = domain.StepCompleted
			s.state.Pasos[i].Progreso = 100
			s.state.Pasos[i].Mensaje = message
			s.completed[id] = true
			return
		}
	}
}

// Iniciar arranca la configuración inicial en segundo plano.
func (s *SetupService) Iniciar(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrSetupEjecutando
	}
	if s.state.Completado {
		s.mu.Unlock()
		return ErrSetupCompletado
	}
	if !s.state.Configuracion.Lista {
		s.mu.Unlock()
		return ErrSetupSinConfigurar
	}
	s.running = true
	s.state.Ejecutando = true
	s.state.Error = ""
	s.broadcastLocked()
	s.mu.Unlock()

	s.log("INFO", "Configuración inicial iniciada", nil)
	go s.run(ctx)
	return nil
}

// Reiniciar elimina las VMs base, limpia el progreso y vuelve a detectar el entorno.
func (s *SetupService) Reiniciar(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrSetupEjecutando
	}
	cfg := s.cfg
	s.mu.Unlock()

	if err := s.prov.ResetEnvironment(ctx, cfg); err != nil {
		return err
	}

	s.mu.Lock()
	s.completed = make(map[domain.SetupStepID]bool)
	s.baseDisk = ""
	s.state = domain.SetupState{
		RequiereConfiguracion: true,
		Configuracion:         s.state.Configuracion,
		Red:                   s.network,
		Imagen:                s.state.Imagen,
		Pasos:                 setupSteps(),
		ActualizadoEn:         time.Now().UTC(),
	}
	s.persistLocked()
	s.broadcastLocked()
	s.mu.Unlock()

	s.log("INFO", "Preparación reiniciada", nil)
	return s.ActualizarDeteccion(ctx)
}

func (s *SetupService) run(parent context.Context) {
	defer func() {
		s.mu.Lock()
		s.running = false
		s.state.Ejecutando = false
		s.broadcastLocked()
		s.mu.Unlock()
	}()

	cfg := s.cfg
	timeout := cfg.InstallTimeout
	if timeout <= 0 {
		timeout = 45 * time.Minute
	}
	// La configuración continúa aunque el cliente HTTP cierre la petición que la
	// inició: el contexto de la petición muere al escribir la respuesta 202.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout*3)
	defer cancel()

	var baseVDI string
	var network domain.NetworkConfig

	// Fase secuencial: entorno, imagen y red.
	for _, id := range []domain.SetupStepID{domain.SetupStepVerify, domain.SetupStepISO, domain.SetupStepNetwork} {
		if err := s.runSteps(ctx, []domain.SetupStepID{id}, &baseVDI, &network); err != nil {
			return
		}
	}
	// Resolver una vez los valores compartidos antes de las fases en paralelo.
	if _, err := s.resolveBaseVDI(ctx, &baseVDI); err != nil {
		s.fail(domain.SetupStepISO, err)
		return
	}
	if _, err := s.resolveNet(ctx, &network); err != nil {
		s.fail(domain.SetupStepNetwork, err)
		return
	}

	// Fases paralelas: DNS y plantilla avanzan a la vez.
	phases := [][]domain.SetupStepID{
		{domain.SetupStepDNSVM, domain.SetupStepBaseVM},
		{domain.SetupStepDNSInstall, domain.SetupStepBaseInstall},
		{domain.SetupStepDNSConfig, domain.SetupStepBaseConfig},
		{domain.SetupStepMultiattach},
		{domain.SetupStepFinal},
	}
	for _, phase := range phases {
		if err := s.runSteps(ctx, phase, &baseVDI, &network); err != nil {
			return
		}
	}

	if err := s.ActualizarDeteccion(ctx); err != nil {
		s.fail(domain.SetupStepFinal, err)
		return
	}
	s.log("INFO", "Configuración inicial completada", nil)
}

// runSteps ejecuta los pasos pendientes indicados, en paralelo cuando hay más
// de uno. Los valores compartidos ya deben estar resueltos.
func (s *SetupService) runSteps(ctx context.Context, ids []domain.SetupStepID, baseVDI *string, network *domain.NetworkConfig) error {
	type result struct {
		id  domain.SetupStepID
		err error
	}

	var pending []domain.SetupStepID
	for _, id := range ids {
		if !s.isCompleted(id) {
			pending = append(pending, id)
		}
	}
	if len(pending) == 0 {
		return nil
	}

	results := make(chan result, len(pending))
	for _, id := range pending {
		go func(stepID domain.SetupStepID) {
			s.track(stepID, domain.StepRunning, 0, "En curso")
			results <- result{id: stepID, err: s.runStep(ctx, stepID, baseVDI, network)}
		}(id)
	}

	var firstErr error
	for range pending {
		res := <-results
		if res.err != nil {
			s.fail(res.id, res.err)
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		s.track(res.id, domain.StepCompleted, 100, "")
	}
	return firstErr
}

func (s *SetupService) isCompleted(id domain.SetupStepID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.completed[id]
}

func (s *SetupService) track(id domain.SetupStepID, status domain.SetupStepStatus, progress int, message string) {
	s.updateStep(id, status, progress, message)
	if status == domain.StepRunning {
		s.log("INFO", "Iniciando: "+s.stepName(id), map[string]any{"paso": string(id)})
	}
	if status == domain.StepCompleted {
		s.log("INFO", "Completado: "+s.stepName(id), map[string]any{"paso": string(id)})
		s.mu.Lock()
		s.completed[id] = true
		s.persistLocked()
		s.mu.Unlock()
	}
}

// stepName retorna el nombre visible de un paso.
func (s *SetupService) stepName(id domain.SetupStepID) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, step := range s.state.Pasos {
		if step.ID == id {
			return step.Nombre
		}
	}
	return string(id)
}

func (s *SetupService) fail(id domain.SetupStepID, err error) {
	s.updateStep(id, domain.StepError, -1, err.Error())
	s.mu.Lock()
	s.state.Error = err.Error()
	s.broadcastLocked()
	s.mu.Unlock()
	s.log("ERROR", "Fallo en la configuración inicial", map[string]any{"paso": string(id), "error": err.Error()})
}

func (s *SetupService) progressFor(id domain.SetupStepID) ports.ProgressFunc {
	return func(percent int, message string) {
		if percent < 0 {
			percent = -1
		}
		if percent > 100 {
			percent = 100
		}
		s.updateStep(id, domain.StepRunning, percent, message)

		// Refleja el avance en el registro en vivo, sin inundarlo: como máximo
		// una línea cada 15 segundos por paso.
		s.mu.Lock()
		last := s.progressLog[id]
		if time.Since(last) >= 15*time.Second {
			s.progressLog[id] = time.Now()
			s.mu.Unlock()
			s.log("INFO", message, map[string]any{"paso": string(id), "progreso": percent})
			return
		}
		s.mu.Unlock()
	}
}

// runStep ejecuta un paso concreto de la preparación.
func (s *SetupService) runStep(ctx context.Context, id domain.SetupStepID, baseVDI *string, network *domain.NetworkConfig) error {
	cfg := s.cfg

	switch id {
	case domain.SetupStepVerify:
		status, err := s.prov.Inspect(ctx, cfg)
		if err != nil {
			return err
		}
		if !status.VBoxAvailable {
			return errors.New("VBoxManage no está disponible en el PATH")
		}
		return nil

	case domain.SetupStepISO:
		resolved, err := s.resolveBaseVDI(ctx, baseVDI, s.progressFor(id))
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.state.Imagen = resolved
		s.persistLocked()
		s.broadcastLocked()
		s.mu.Unlock()
		return nil

	case domain.SetupStepNetwork:
		derived, err := s.prov.EnsureNetwork(ctx, cfg)
		if err != nil {
			return err
		}
		*network = derived
		s.setNetwork(derived)
		return nil

	case domain.SetupStepDNSVM:
		net, err := s.resolveNet(ctx, network)
		if err != nil {
			return err
		}
		base, err := s.resolveBaseVDI(ctx, baseVDI)
		if err != nil {
			return err
		}
		*baseVDI = base
		if _, err := s.prov.CreateVM(ctx, cfg, cfg.DNSVMName, net.Interface, *baseVDI); err != nil {
			return err
		}
		return s.createSeed(ctx, cfg.DNSVMName)

	case domain.SetupStepDNSInstall:
		if err := s.prov.ReserveDHCPAddress(ctx, cfg, cfg.DNSVMName, network.DNSIP); err != nil {
			return err
		}
		seed, err := s.ensureSeed(ctx, cfg.DNSVMName)
		if err != nil {
			return err
		}
		if err := s.prov.StartSeedBoot(ctx, cfg, cfg.DNSVMName, seed); err != nil {
			return err
		}
		if err := s.prov.WaitGuestReady(ctx, cfg.DNSVMName, cfg.InstallTimeout, s.progressFor(id)); err != nil {
			return err
		}
		return s.verifyCloudInit(ctx, id, cfg, cfg.DNSVMName)

	case domain.SetupStepDNSConfig:
		out, err := s.prov.RunGuestScript(ctx, cfg, cfg.DNSVMName, filepath.Join("scripts", "guest-bind9.sh"), network.DNSIP, network.CIDR)
		s.logGuestOutput(out)
		return err

	case domain.SetupStepBaseVM:
		net, err := s.resolveNet(ctx, network)
		if err != nil {
			return err
		}
		base, err := s.resolveBaseVDI(ctx, baseVDI)
		if err != nil {
			return err
		}
		*baseVDI = base
		if _, err := s.prov.CreateVM(ctx, cfg, cfg.BaseVMName, net.Interface, *baseVDI); err != nil {
			return err
		}
		return s.createSeed(ctx, cfg.BaseVMName)

	case domain.SetupStepBaseInstall:
		seed, err := s.ensureSeed(ctx, cfg.BaseVMName)
		if err != nil {
			return err
		}
		if err := s.prov.StartSeedBoot(ctx, cfg, cfg.BaseVMName, seed); err != nil {
			return err
		}
		if err := s.prov.WaitGuestReady(ctx, cfg.BaseVMName, cfg.InstallTimeout, s.progressFor(id)); err != nil {
			return err
		}
		return s.verifyCloudInit(ctx, id, cfg, cfg.BaseVMName)

	case domain.SetupStepBaseConfig:
		out, err := s.prov.RunGuestScript(ctx, cfg, cfg.BaseVMName, filepath.Join("scripts", "guest-apache.sh"))
		s.logGuestOutput(out)
		return err

	case domain.SetupStepMultiattach:
		disk, err := s.prov.MarkBaseMultiattach(ctx, cfg, cfg.BaseVMName)
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.baseDisk = disk
		s.persistLocked()
		s.mu.Unlock()
		return nil

	case domain.SetupStepFinal:
		status, err := s.prov.Inspect(ctx, cfg)
		if err != nil {
			return err
		}
		if !status.Ready {
			return errors.New("la verificación final no encontró el entorno completo")
		}
		return nil
	}
	return fmt.Errorf("paso desconocido: %s", id)
}

// createSeed renderiza la semilla cloud-init y genera la ISO NoCloud.
func (s *SetupService) createSeed(ctx context.Context, vmName string) error {
	userData, metaData, err := s.renderSeed(vmName)
	if err != nil {
		return err
	}
	_, err = s.prov.CreateSeedISO(ctx, s.cfg, vmName, userData, metaData)
	return err
}

// ensureSeed reconstruye la semilla si falta la ISO o sus archivos internos
// (por ejemplo, al reanudar tras un fallo o un cambio de formato).
func (s *SetupService) ensureSeed(ctx context.Context, vmName string) (string, error) {
	path := s.seedPath(vmName)
	staging := filepath.Join(s.cfg.SeedDir, vmName)
	if _, err := os.Stat(path); err == nil {
		if _, errUser := os.Stat(filepath.Join(staging, "user-data")); errUser == nil {
			if _, errMeta := os.Stat(filepath.Join(staging, "meta-data")); errMeta == nil {
				return path, nil
			}
		}
	}
	if err := s.createSeed(ctx, vmName); err != nil {
		return "", err
	}
	return path, nil
}

// renderSeed sustituye las credenciales en la plantilla de la VM indicada.
func (s *SetupService) renderSeed(vmName string) (string, string, error) {
	templateName := "apache-user-data.yaml"
	host := "apache-server"
	if vmName == s.cfg.DNSVMName {
		templateName = "ns-user-data.yaml"
		host = "ns-server"
	}

	raw, err := os.ReadFile(filepath.Join(s.cfg.SeedTemplatesDir, templateName))
	if err != nil {
		return "", "", fmt.Errorf("no se pudo leer la semilla %s: %w", templateName, err)
	}
	userData := strings.ReplaceAll(string(raw), "@@USUARIO@@", s.cfg.GuestUser)
	password := strings.ReplaceAll(s.cfg.GuestPassword, "'", "''")
	userData = strings.ReplaceAll(userData, "@@CONTRASENA@@", password)
	metaData := fmt.Sprintf("instance-id: iid-%s-01\nlocal-hostname: %s\n", host, host)
	return userData, metaData, nil
}

// seedPath retorna la ruta de la ISO semilla de una VM.
func (s *SetupService) seedPath(vmName string) string {
	return filepath.Join(s.cfg.SeedDir, vmName+"-seed.iso")
}

// resolveBaseVDI asegura la imagen raw y su VDI base. La conversión se hace una
// sola vez; el VDI se conserva entre reinicios.
func (s *SetupService) resolveBaseVDI(ctx context.Context, base *string, progress ...ports.ProgressFunc) (string, error) {
	if *base != "" {
		return *base, nil
	}
	emit := ports.ProgressFunc(nil)
	if len(progress) > 0 {
		emit = progress[0]
	}

	raw, ok := s.images.Locate(ctx, s.cfg.RawDir)
	if !ok {
		path, err := s.images.Download(ctx, s.cfg.RawURL, s.cfg.RawDir, emit)
		if err != nil {
			return "", fmt.Errorf("no se pudo descargar la imagen de Debian: %w", err)
		}
		raw = path
	}
	path := strings.TrimSuffix(raw, ".raw") + ".vdi"
	if err := s.prov.ConvertToBaseVDI(ctx, s.cfg, raw, path, emit); err != nil {
		return "", err
	}
	*base = path
	return path, nil
}

// verifyCloudInit espera a que cloud-init termine y publica su registro.
func (s *SetupService) verifyCloudInit(ctx context.Context, id domain.SetupStepID, cfg domain.SetupConfig, vmName string) error {
	out, err := s.prov.RunGuestScript(ctx, cfg, vmName, filepath.Join("scripts", "guest-cloudinit-check.sh"))
	s.logGuestOutput(out)
	if err != nil {
		return fmt.Errorf("cloud-init no terminó correctamente en %s: %w", vmName, err)
	}
	return nil
}

// logGuestOutput publica en el registro las últimas líneas de un script.
func (s *SetupService) logGuestOutput(output string) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			s.log("INFO", line, nil)
		}
	}
}

// resolveNet recupera o crea la red host-only de forma idempotente.
func (s *SetupService) resolveNet(ctx context.Context, network *domain.NetworkConfig) (domain.NetworkConfig, error) {
	if network.Interface != "" {
		return *network, nil
	}
	derived, err := s.prov.EnsureNetwork(ctx, s.cfg)
	if err != nil {
		return domain.NetworkConfig{}, err
	}
	*network = derived
	s.setNetwork(derived)
	return derived, nil
}

// setNetwork publica la red detectada en el estado del servicio.
func (s *SetupService) setNetwork(network domain.NetworkConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.network = network
	s.state.Red = network
	s.persistLocked()
	s.broadcastLocked()
}

func (s *SetupService) loadRecord() {
	raw, err := os.ReadFile(s.stateFile)
	if err != nil {
		return
	}
	var record setupRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return
	}
	s.completed = make(map[domain.SetupStepID]bool, len(record.PasosCompletos))
	for _, id := range record.PasosCompletos {
		s.completed[id] = true
	}
	s.baseDisk = record.BaseDiskPath
	if record.Network != nil {
		s.network = *record.Network
		s.state.Red = *record.Network
	}
	s.state.Configuracion = domain.SetupConfiguracion{
		Usuario: record.Usuario,
	}
	if record.Completado {
		s.state.Completado = true
		s.state.RequiereConfiguracion = false
	}
}

func (s *SetupService) persistLocked() {
	record := setupRecord{
		Completado:    s.state.Completado,
		BaseDiskPath:  s.baseDisk,
		Network:       &s.network,
		Usuario:       s.state.Configuracion.Usuario,
		ActualizadoEn: time.Now().UTC(),
	}
	for id := range s.completed {
		record.PasosCompletos = append(record.PasosCompletos, id)
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.stateFile), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(s.stateFile, raw, 0o644)
}
