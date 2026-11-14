// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

package handlers

import (
	"net/http"
	"strings"

	"github.com/Andrey3681/httpaas-orchestrator/internal/core/domain"
)

// SetupStateProvider expone el estado del configuración inicial.
type SetupStateProvider interface {
	Estado() domain.SetupState
}

// RequireSetup bloquea las operaciones que modifican el laboratorio mientras la
// configuración inicial no haya finalizado. Las lecturas y los endpoints del
// propia preparación permanecen disponibles.
func RequireSetup(setup SetupStateProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isReadOnly(r.Method) || isSetupRoute(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if !setup.Estado().Completado {
				Error(w, http.StatusConflict, "El entorno aún no está configurado. Completa la configuración inicial antes de aprovisionar instancias.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isReadOnly(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func isSetupRoute(path string) bool {
	return strings.HasPrefix(path, "/api/setup/")
}
