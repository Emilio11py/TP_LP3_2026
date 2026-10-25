package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"tp-backend/internal/services"
)

func main() {
	document := flag.String("documento", "", "Número de documento del usuario")
	role := flag.String("rol", "", "Rol administrativo: dir_academica o dir_carrera")
	flag.Parse()

	normalizedRole := strings.ToLower(strings.TrimSpace(*role))
	if flag.NArg() != 0 || strings.TrimSpace(*document) == "" ||
		(normalizedRole != "dir_academica" && normalizedRole != "dir_carrera") {
		log.Fatal("Uso: go run .\\cmd\\set-role -documento NUMERO -rol dir_academica|dir_carrera")
	}

	db, err := services.AbrirBaseDeDatos(context.Background(), obtenerValorDeEntornoOPredeterminado("DB_PATH", "data/sistema_cae.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := services.ValidarEsquema(context.Background(), db); err != nil {
		log.Fatal(err)
	}

	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM USUARIOS WHERE CAST(NRO_CEDULA AS TEXT) = ? AND ACTIVO = 1",
		strings.TrimSpace(*document),
	).Scan(&count); err != nil {
		log.Fatal(err)
	}
	if count != 1 {
		log.Fatal("El documento no corresponde a un único usuario activo")
	}
	if _, err := db.Exec(
		"UPDATE USUARIOS SET ROLE_ACCESO = ? WHERE CAST(NRO_CEDULA AS TEXT) = ? AND ACTIVO = 1",
		normalizedRole, strings.TrimSpace(*document),
	); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Rol administrativo asignado al usuario seleccionado.")
}

func obtenerValorDeEntornoOPredeterminado(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
