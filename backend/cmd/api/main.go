package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tp-backend/internal/handlers"
	"tp-backend/internal/services"
)

func main() {
	databasePath := obtenerValorDeEntornoOPredeterminado("DB_PATH", "data/sistema_cae.db")
	db, err := services.AbrirBaseDeDatos(context.Background(), databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := services.ValidarEsquema(context.Background(), db); err != nil {
		log.Fatal(err)
	}
	if err := services.AsegurarEsquemaGestionUsuarios(context.Background(), db); err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              ":" + strings.TrimPrefix(obtenerValorDeEntornoOPredeterminado("PORT", "8080"), ":"),
		Handler:           handlers.CrearEnrutador(db, obtenerValorDeEntornoOPredeterminado("FRONTEND_ORIGIN", "http://localhost:4200")),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("API disponible en http://localhost%s", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Error del servidor: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error al cerrar el servidor: %v", err)
		}
	}
}

func obtenerValorDeEntornoOPredeterminado(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
