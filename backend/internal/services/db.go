package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

func AbrirBaseDeDatos(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("abrir SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("conectar con SQLite: %w", err)
	}
	if _, err := db.ExecContext(pingCtx, "PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("activar claves foráneas SQLite: %w", err)
	}
	return db, nil
}

func ValidarEsquema(ctx context.Context, db *sql.DB) error {
	queries := []struct {
		table string
		query string
	}{
		{"USUARIOS", "SELECT ID_USUARIO, EMAIL, CONTRASENHA, ROLE_ACCESO, ACTIVO FROM USUARIOS LIMIT 0"},
		{"ACTIVIDADES", "SELECT ID_ACTIVIDAD, NOMBRE_ACTIVIDAD, ACTIVO FROM ACTIVIDADES LIMIT 0"},
		{"SOLICITUDES_CAES", "SELECT ID_SOLICITUD_CAE, ID_USUARIO_SOLICITA, ESTADO_SOLICITUD FROM SOLICITUDES_CAES LIMIT 0"},
		{"DETALLE_SOLICITUDES_CAES", "SELECT ID_SOLICITUD_CAE, FECHA_ACTIVIDAD, ACTIVIDAD, CARGA_HORARIA_EN_MINUTOS FROM DETALLE_SOLICITUDES_CAES LIMIT 0"},
		{"DETALLE_DOCUMENT_SOLICITUD_CAES", "SELECT ID_DETALLE_DOC_SOLICITUD_CAE, ID_SOLICITUD_CAE, DOCUMENTO FROM DETALLE_DOCUMENT_SOLICITUD_CAES LIMIT 0"},
	}
	for _, item := range queries {
		rows, err := db.QueryContext(ctx, item.query)
		if err != nil {
			return fmt.Errorf("validar tabla %s: %w", item.table, err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("cerrar validación de tabla %s: %w", item.table, err)
		}
	}
	return nil
}
