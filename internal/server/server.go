package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Duliangheng2003/workflow-platform/internal/api"
	"github.com/Duliangheng2003/workflow-platform/internal/engine"
	"github.com/Duliangheng2003/workflow-platform/internal/store"
)

//go:embed static/*.html
var staticFiles embed.FS

type Config struct {
	Port    int
	Timeout time.Duration
}

func DefaultConfig() Config {
	return Config{
		Port:    8080,
		Timeout: 60 * time.Second,
	}
}

func Run(cfg Config) error {
	st := store.NewMemoryStore()
	eng := engine.New(st)
	handler := api.NewHandler(st, eng)

	mux := http.NewServeMux()

	// API routes
	handler.RegisterRoutes(mux)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Serve frontend
	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("static sub: %w", err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(staticSub)))

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      withCORS(mux),
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Workflow Platform starting on http://localhost%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}