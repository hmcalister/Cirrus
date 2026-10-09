package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hmcalister/Cirrus/internal/api"
	"github.com/hmcalister/Cirrus/internal/config"
	"github.com/hmcalister/Cirrus/internal/database"
	"github.com/hmcalister/Cirrus/internal/email"
	"github.com/hmcalister/Cirrus/internal/logging"
	"github.com/hmcalister/Cirrus/internal/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logging.Setup(cfg.Debug)

	// Cancelled on SIGINT/SIGTERM
	// i.e. how Podman compose stops the container.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	emailSender, err := email.NewSMTPSender(email.SMTPConfig{
		Host:        cfg.Email.Host,
		Port:        cfg.Email.Port,
		Username:    cfg.Email.Username,
		Password:    cfg.Email.Password,
		FromAddress: cfg.Email.FromAddress,
		FromName:    cfg.Email.FromName,
		Timeout:     cfg.Email.Timeout,
	})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           api.NewServer(database.New(pool), emailSender).Routes(),
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("ready to serve", "addr", cfg.Server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
