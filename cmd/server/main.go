// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package main es el punto de entrada y ensamblador de inyección de dependencias para el orquestador HTTPaaS.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/bind9"
	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/deployer"
	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/filestore"
	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/hostdns"
	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/localproc"
	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/provisioner"
	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/vbox"
	"github.com/Andrey3681/httpaas-orchestrator/internal/config"
	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
	"github.com/Andrey3681/httpaas-orchestrator/internal/handlers"
	"github.com/Andrey3681/httpaas-orchestrator/internal/logging"
	"github.com/Andrey3681/httpaas-orchestrator/internal/services"
)

func main() {
	port := flag.Int("port", 8080, "Puerto HTTP del panel de administración")
	domainZone := flag.String("zone", "cloud.local", "Zona de dominio DNS administrada")
	baseDisk := flag.String("base-disk", "", "Ruta al disco base multiconexión (por defecto, la detectada por la configuración inicial)")
	appDir := flag.String("app-dir", "", "Directorio de datos de HTTPaaS (por defecto /var/lib/httpaas o ~/.local/share/httpaas)")
	dataPath := flag.String("data", "", "Ruta del archivo de persistencia JSON (por defecto <app-dir>/instances.json)")
	webDir := flag.String("web", "web", "Directorio con los archivos estáticos de la interfaz web")
	rawURL := flag.String("raw-url", "https://cloud.debian.org/images/cloud/trixie/latest/debian-13-generic-amd64.raw", "URL de la imagen cloud de Debian 13")
	guestUser := flag.String("guest-user", "debian", "Usuario de aprovisionamiento en los invitados")
	guestPass := flag.String("guest-pass", "debian", "Contraseña del usuario de aprovisionamiento")
	setupState := flag.String("setup-state", "", "Ruta del estado persistido de la preparación (por defecto <app-dir>/setup-estado.json)")
	installTimeout := flag.Duration("install-timeout", 45*time.Minute, "Tiempo máximo del primer arranque de cada VM")
	flag.Parse()

	baseDir := *appDir
	if baseDir == "" {
		baseDir = config.ResolveAppDir()
	}
	layout, err := config.EnsureLayout(baseDir)
	if err != nil {
		log.Fatalf("Error crítico preparando el directorio de datos: %v", err)
	}
	if *dataPath == "" {
		*dataPath = layout.InstancesFile
	}
	if *setupState == "" {
		*setupState = layout.SetupStateFile
	}

	logBroker := logging.NewBroker(1000)
	// El registro del panel solo recibe eventos estructurados; las trazas
	// técnicas y las peticiones HTTP quedan en la salida estándar.
	log.SetOutput(os.Stderr)

	log.Println("================================================================")
	log.Println("  HTTPaaS - Servicio Administrado de Servidores HTTP (2026-2)   ")
	log.Println("================================================================")

	// 1. Inicializar adaptador de ejecución de procesos
	executor := localproc.NewProcessExecutor()

	// 2. Inicializar repositorio de persistencia
	repo, err := filestore.NewJSONRepository(*dataPath)
	if err != nil {
		log.Fatalf("Error crítico inicializando repositorio en '%s': %v", *dataPath, err)
	}

	// 3. Preparación inicial del laboratorio (primer arranque)
	setupCfg := domain.SetupConfig{
		RawDir:           layout.RawDir,
		SeedDir:          layout.SeedDir,
		SSHDir:           layout.SSHDir,
		SSHKeyPath:       filepath.Join(layout.SSHDir, "id_ed25519"),
		SeedTemplatesDir: filepath.Join("scripts", "seeds"),
		RawURL:           *rawURL,
		DNSVMName:        "ns",
		BaseVMName:       "ApacheServer",
		DNSHostname:      "ns." + *domainZone,
		BaseHostname:     "apache." + *domainZone,
		GuestUser:        *guestUser,
		GuestPassword:    *guestPass,
		MemoryMB:         1024,
		DiskMB:           8192,
		InstallTimeout:   *installTimeout,
	}
	prov := provisioner.NewProvisioner(executor)
	setupService := services.NewSetupService(setupCfg, prov, provisioner.NewISOStore(), logBroker, *setupState)
	go func() {
		_ = setupService.ActualizarDeteccion(context.Background())
	}()
	shutdown := make(chan struct{})
	setupHandler := handlers.NewSetupHandler(setupService, shutdown)

	// 4. Adaptadores de infraestructura (la red se deriva de la interfaz real)
	hyp := vbox.NewHypervisorAdapter(executor, *domainZone, setupService.Network().HostIP, logBroker)
	hyp.ConfigureInstanceSeed(setupService.GuestUser, filepath.Join(layout.SSHDir, "id_ed25519"), layout.SeedDir)
	dns := bind9.NewDnsAdapterWithNetwork(executor, *domainZone, logBroker, setupService)
	dep := deployer.NewApacheDeployer(executor, logBroker, setupService.GuestUser, filepath.Join(layout.SSHDir, "id_ed25519"))

	// El disco base se resuelve en cada solicitud: la preparación puede
	// completarse después de arrancar el servidor.
	baseDiskProvider := func() string {
		if *baseDisk != "" {
			return *baseDisk
		}
		return setupService.BaseDiskPath()
	}

	// 4. Inicializar servicios de aplicación
	orchestrator := services.NewOrchestrator(hyp, dns, dep, repo, *domainZone, baseDiskProvider, logBroker)
	instanceService := services.NewInstanceService(repo, hyp, dns, logBroker, services.InstanceOptions{
		Operator:   dep,
		Deployer:   dep,
		GuestUser:  setupService.GuestUser,
		SSHKeyPath: filepath.Join(layout.SSHDir, "id_ed25519"),
	})

	// 5. Inicializar manejadores HTTP
	instanceHandler := handlers.NewInstanceHandler(orchestrator, instanceService, logBroker)
	dnsHandler := handlers.NewDnsHandler(dns, *domainZone, logBroker)
	dnsHandler.SetHostDNS(setupService, hostdns.New(executor))
	logsHandler := handlers.NewLogsHandler(logBroker, shutdown)

	// 6. Configurar enrutador y montar endpoints
	mux := http.NewServeMux()

	// API REST
	mux.HandleFunc("/api/setup/estado", setupHandler.HandleEstado)
	mux.HandleFunc("/api/setup/configuracion", setupHandler.HandleConfigurar)
	mux.HandleFunc("/api/setup/reiniciar", setupHandler.HandleReiniciar)
	mux.HandleFunc("/api/setup/iniciar", setupHandler.HandleIniciar)
	mux.HandleFunc("/api/setup/stream", setupHandler.HandleStream)
	mux.HandleFunc("/api/logs/stream", logsHandler.HandleStream)
	mux.HandleFunc("/api/logs", logsHandler.HandleLogs)
	mux.HandleFunc("/api/instancias/", instanceHandler.HandleInstance)
	mux.HandleFunc("/api/instancias", instanceHandler.HandleInstances)
	mux.HandleFunc("/api/dns/registros", dnsHandler.HandleRecords)
	mux.HandleFunc("/api/dns/anfitrion", dnsHandler.HandleHostResolve)
	mux.HandleFunc("/api/dns/anfitrion/configurar", dnsHandler.HandleConfigureHost)
	mux.HandleFunc("/api/salud", dnsHandler.HandleHealth)

	// Servir archivos estáticos del dashboard (web/)
	fileServer := http.FileServer(http.Dir(*webDir))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Evita que el navegador conserve versiones antiguas del panel.
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	}))

	// Envolver con middlewares (CORS, Logger, Recoverer y bloqueo previo a la configuración)
	handler := handlers.Recoverer(handlers.Logger(handlers.CORS(handlers.RequireSetup(setupService)(mux))))

	serverAddr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:        serverAddr,
		Handler:     handler,
		ReadTimeout: 120 * time.Second,
		// WriteTimeout debe ser 0: los endpoints SSE mantienen la conexión
		// abierta y un límite cortaría el stream cada dos minutos.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	// Canal para captura de señales de apagado elegante
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Servidor orquestador escuchando en http://localhost:%d", *port)
		log.Printf("Directorio web: %s | Zona DNS: %s | Base VDI: %s", *webDir, *domainZone, baseDiskProvider())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error en servidor HTTP: %v", err)
		}
	}()

	<-stop
	log.Println("Señal de apagado recibida. Cerrando servidor de forma segura...")
	close(shutdown)

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Printf("Aviso: el cierre ordenado no terminó a tiempo (%v); forzando cierre", err)
		_ = srv.Close()
	}

	log.Println("Servidor HTTPaaS detenido correctamente.")
}
