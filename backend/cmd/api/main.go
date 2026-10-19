package main

import (
	"fmt"
	"log"
	"net/http"

	"tp-backend/internal/handlers"
	"tp-backend/internal/services"
)

func main() {
	// 1. Conectar a la base de datos oficial
	services.InicializarDB("data/sistema_cae.db")

	// 2. Endpoints
	http.HandleFunc("/api/auth/login", handlers.LoginUsuario)
	http.HandleFunc("/api/solicitudes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.ListarSolicitudes(w, r)
		case http.MethodPost:
			handlers.CrearSolicitud(w, r)
		default:
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		}
	})

	fmt.Println("Servidor Go corriendo en http://localhost:8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))

}
