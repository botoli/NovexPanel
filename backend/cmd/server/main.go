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

	"novexpanel/backend/internal/app"
	"novexpanel/backend/internal/config"
	"novexpanel/backend/internal/storage"

	"github.com/gin-gonic/gin"
)

func main() {
	ginMode := strings.TrimSpace(os.Getenv("GIN_MODE"))
	if ginMode == "" {
		ginMode = gin.ReleaseMode
	}
	gin.SetMode(ginMode)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := storage.Open(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	application := app.New(cfg, db)
	router := application.Router()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	application.StartBackgroundJobs(ctx)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("backend listening on %s", cfg.HTTPAddr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}
