// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrey y Equipo de Trabajo

// Package domain define las entidades centrales del modelo de negocio, reglas de dominio y errores centinela.
package domain

import "errors"

// Errores centinela del dominio.
var (
	// ErrInvalidHostname indica que el nombre de host no cumple las reglas de formato o longitud.
	ErrInvalidHostname = errors.New("el nombre de host no es válido")

	// ErrHostAlreadyExists indica que el nombre de host ya se encuentra registrado en el sistema o en el DNS.
	ErrHostAlreadyExists = errors.New("el nombre de host ya se encuentra registrado")

	// ErrInstanceNotFound indica que la instancia solicitada no existe en el inventario.
	ErrInstanceNotFound = errors.New("la instancia no fue encontrada")

	// ErrInvalidWebPackage indica que el archivo web proporcionado no es un paquete .zip válido o excede el límite permitido.
	ErrInvalidWebPackage = errors.New("el paquete web no es válido o está corrupto")

	// ErrZipSlipDetected indica un intento de path traversal dentro del archivo comprimido.
	ErrZipSlipDetected = errors.New("se detectó una ruta insegura (Zip Slip) en el archivo comprimido")

	// ErrOperationNotAllowed indica que la instancia no admite la operación en su estado actual.
	ErrOperationNotAllowed = errors.New("la operación no está permitida para el estado de la instancia")
)
