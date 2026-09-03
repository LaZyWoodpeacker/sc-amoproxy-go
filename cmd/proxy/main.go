package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"proxy-api/internal/auth"
	"proxy-api/internal/client"
	"proxy-api/internal/config"
	"proxy-api/internal/handler"
	"proxy-api/internal/limiter"
	"proxy-api/internal/logging"
)

func main() {
	fmt.Printf("TEST %s %s", os.Getenv("PORT"), os.Getenv("LOG_LEVEL"))
	// log := logging.New(os.Getenv("LOG_LEVEL"))
	log := logging.New("debug")
	log.Info().Msg("Test")
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}
	validator, err := auth.New(cfg.JWTPublicKey, cfg.JWTIssuer, cfg.JWTAudience)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid JWT configuration")
	}
	h := handler.New(cfg, validator, client.New(cfg.RestyDebug, log), limiter.New(), log)
	srv := &http.Server{Addr: "0.0.0.0:" + cfg.Port, Handler: h, ReadHeaderTimeout: cfg.Timeout, IdleTimeout: 60 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("server stopped")
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
