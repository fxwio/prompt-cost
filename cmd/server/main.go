package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/fxwio/prompt-cost/internal/api"
	"github.com/fxwio/prompt-cost/internal/config"
	_ "github.com/fxwio/prompt-cost/internal/metrics" // register Prometheus metrics
	"github.com/fxwio/prompt-cost/internal/model"
	"github.com/fxwio/prompt-cost/internal/pricing"
	"github.com/fxwio/prompt-cost/pkg/pgstore"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	cfgPath := os.Getenv("PROMPT_COST_CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := pgstore.Init(ctx, cfg.Postgres.DSN, cfg.Postgres.MaxConns, cfg.Postgres.MinConns); err != nil {
		logger.Fatal("init postgres", zap.Error(err))
	}
	defer pgstore.Close()
	logger.Info("postgres ready")

	// Apply pricing overrides from config
	for _, o := range cfg.Pricing.Overrides {
		pricing.Default.Override(model.ModelPricing{
			Model:       o.Model,
			InputPer1M:  o.InputPer1M,
			OutputPer1M: o.OutputPer1M,
		})
	}

	handler := api.New(pricing.Default, logger)
	router := api.NewRouter(handler, logger)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info("server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
