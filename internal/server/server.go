package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Duliangheng2003/workflow-platform/internal/api"
	"github.com/Duliangheng2003/workflow-platform/internal/config"
	"github.com/Duliangheng2003/workflow-platform/internal/engine"
	"github.com/Duliangheng2003/workflow-platform/internal/store"
	"github.com/Duliangheng2003/workflow-platform/internal/store/sqlite"
)


func Run(cfg *config.Config) error {
	var st store.Store
	var err error

	if cfg.Database.Path != "" {
		st, err = sqlite.NewStore(cfg.Database.Path)
		if err != nil {
			return fmt.Errorf("sqlite store: %w", err)
		}
		log.Println("Using SQLite storage:", cfg.Database.Path)
	} else if cfg.Database.Host != "" {
	}

	eng := engine.New(st, cfg.LLM)
	eng.StartCronScheduler(context.Background())
	handler := api.NewHandler(st, eng, cfg.LLM)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	mux.Handle("GET /", http.FileServer(http.Dir("web/dist")))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})


	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      withCORS(mux),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
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
