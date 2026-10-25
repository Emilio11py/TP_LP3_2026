package services

import (
	"context"
	"database/sql"
	"fmt"
)

func AsegurarEsquemaGestionUsuarios(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS USUARIOS_PENDIENTES (
			ID_USUARIO_PENDIENTE INTEGER PRIMARY KEY,
			NRO_CEDULA INTEGER NOT NULL,
			NOMBRES VARCHAR(120) NOT NULL,
			APELLIDOS VARCHAR(120) NOT NULL,
			CODIGO_MATRICULA VARCHAR(10) NOT NULL,
			EMAIL VARCHAR(120) NOT NULL,
			TELEFONO_CELULAR VARCHAR(18) NOT NULL,
			CONTRASENHA VARCHAR(100) NOT NULL,
			CARRERA VARCHAR(120) NOT NULL,
			ESTADO VARCHAR(12) NOT NULL CHECK (ESTADO IN ('PENDIENTE', 'APROBADO', 'RECHAZADO')),
			FECHA_SOLICITUD DATETIME NOT NULL,
			FECHA_REVISION DATETIME,
			ID_USUARIO_REVISOR INTEGER
		)
	`)
	if err != nil {
		return fmt.Errorf("crear tabla USUARIOS_PENDIENTES: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS IDX_USUARIOS_PENDIENTES_DOCUMENTO_PENDIENTE
		ON USUARIOS_PENDIENTES (NRO_CEDULA) WHERE ESTADO = 'PENDIENTE'
	`); err != nil {
		return fmt.Errorf("crear índice de solicitudes pendientes: %w", err)
	}
	indexes := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS IDX_USUARIOS_PENDIENTES_MATRICULA_PENDIENTE ON USUARIOS_PENDIENTES (LOWER(CODIGO_MATRICULA)) WHERE ESTADO = 'PENDIENTE'",
		"CREATE UNIQUE INDEX IF NOT EXISTS IDX_USUARIOS_PENDIENTES_EMAIL_PENDIENTE ON USUARIOS_PENDIENTES (LOWER(EMAIL)) WHERE ESTADO = 'PENDIENTE'",
	}
	for _, statement := range indexes {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("crear índice de solicitudes pendientes: %w", err)
		}
	}
	return nil
}
