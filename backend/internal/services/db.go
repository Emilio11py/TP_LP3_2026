package services

import (
	"database/sql"
	"log"

	_ "github.com/glebarez/go-sqlite"
)

var DB *sql.DB

// InicializarDB abre la conexión con el archivo SQLite sin requerir CGO
func InicializarDB(rutaDB string) {
	var err error
	DB, err = sql.Open("sqlite", rutaDB)
	if err != nil {
		log.Fatal("Error al abrir la base de datos:", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatal("No se pudo conectar con SQLite:", err)
	}
}
