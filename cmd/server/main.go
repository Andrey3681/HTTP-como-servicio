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
	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/localproc"
	"github.com/Andrey3681/httpaas-orchestrator/internal/adapters/vbox"
	"github.com/Andrey3681/httpaas-orchestrator/internal/handlers"
	"github.com/Andrey3681/httpaas-orchestrator/internal/services"
)

func main() {
	port := flag.Int("port", 8080, "Puerto HTTP del panel de administración")
	domainZone := flag.String("zone", "cloud.local", "Zona de dominio DNS administrada")
	dnsIP := flag.String("dns-ip", "192.168.10.10", "Dirección IP del servidor DNS Bind9 (VM ns)")
	baseDisk := flag.String("base-disk", "ApacheServer.vdi", "Ruta al disco base multiconexión de Apache")
	dataPath := flag.String("data", filepath.Join("data", "instances.json"), "Ruta del archivo de persistencia JSON")
	webDir := flag.String("web", "web", "Directorio con los archivos estáticos de la interfaz web")
	flag.Parse()

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

	// 3. Inicializar adaptadores de infraestructura
	hyp := vbox.NewHypervisorAdapter(executor, *domainZone)
	dns := bind9.NewDnsAdapter(executor, *dnsIP, *domainZone)
	dep := deployer.NewApacheDeployer(executor)

	// 4. Inicializar servicios de aplicación
	orchestrator := services.NewOrchestrator(hyp, dns, dep, repo, *domainZone, *baseDisk)
	instanceService := services.NewInstanceService(repo, hyp, dns)

	// 5. Inicializar manejadores HTTP
	instanceHandler := handlers.NewInstanceHandler(orchestrator, instanceService)
	dnsHandler := handlers.NewDnsHandler(dns, *domainZone)

	// 6. Configurar enrutador y montar endpoints
	mux := http.NewServeMux()

	// API REST
	mux.HandleFunc("/api/instancias/", instanceHandler.HandleInstance)
	mux.HandleFunc("/api/instancias", instanceHandler.HandleInstances)
	mux.HandleFunc("/api/dns/registros", dnsHandler.HandleRecords)
	mux.HandleFunc("/api/salud", dnsHandler.HandleHealth)

	// Servir archivos estáticos del dashboard (web/)
	fileServer := http.FileServer(http.Dir(*webDir))
	mux.Handle("/", fileServer)

	// Envolver con middlewares (CORS, Logger, Recoverer)
	handler := handlers.Recoverer(handlers.Logger(handlers.CORS(mux)))

	serverAddr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      handler,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Canal para captura de señales de apagado elegante
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Servidor orquestador escuchando en http://localhost:%d", *port)
		log.Printf("Directorio web: %s | Zona DNS: %s | Base VDI: %s", *webDir, *domainZone, *baseDisk)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error en servidor HTTP: %v", err)
		}
	}()

	<-stop
	log.Println("Señal de apagado recibida. Cerrando servidor de forma segura...")

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Error durante el cierre forzado del servidor: %v", err)
	}

	log.Println("Servidor HTTPaaS detenido correctamente.")
}
