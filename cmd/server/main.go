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
	"github.com/hmcalister/Cirrus/internal/authentication"
	"github.com/hmcalister/Cirrus/internal/config"
	"github.com/hmcalister/Cirrus/internal/database"
	"github.com/hmcalister/Cirrus/internal/email/mailer"
	"github.com/hmcalister/Cirrus/internal/email/ratelimit"
	"github.com/hmcalister/Cirrus/internal/email/sender"
	"github.com/hmcalister/Cirrus/internal/logging"
	"github.com/hmcalister/Cirrus/internal/postgres"
	templates "github.com/hmcalister/Cirrus/internal/template"
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

	templateLoader, err := templates.NewLoader(
		os.DirFS(cfg.Templates.Directory),
		"email",
		"web",
		nil,
		nil,
	)
	if err != nil {
		return err
	}

	// Cancelled on SIGINT/SIGTERM
	// i.e. how Podman compose stops the container.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	mailerServerAuth, err := mailer.NewServerAuth(
		cfg.Email.Host,
		cfg.Email.Port,
		cfg.Email.Username,
		cfg.Email.Password,
	)
	if err != nil {
		return err
	}

	emailSender, err := sender.NewSMTPSender(sender.SMTPConfig{
		ServerAuth:  mailerServerAuth,
		FromAddress: cfg.Email.FromAddress,
		FromName:    cfg.Email.FromName,
		Timeout:     cfg.Email.Timeout,
	})
	if err != nil {
		return err
	}

	ratelimitedEmailSender, err := ratelimit.NewRateLimitedSender(ratelimit.RateLimitedSenderConfig{
		Next:                 emailSender,
		AddressBurst:         cfg.EmailRateLimit.AddressBurst,
		AddressInterval:      cfg.EmailRateLimit.AddressInterval,
		GlobalBurst:          cfg.EmailRateLimit.GlobalBurst,
		GlobalInterval:       cfg.EmailRateLimit.GlobalInterval,
		SweepCadenceDuration: cfg.EmailRateLimit.SweepCadenceDuration,
	})
	if err != nil {
		return err
	}

	linkBuilder, err := api.NewLinkBuilder(cfg.AuthHTTP.BaseURL)
	if err != nil {
		return err
	}

	authService, err := authentication.NewAuthenticationService(authentication.AuthenticationConfig{
		VerificationExpiryDuration: cfg.Auth.VerificationExpiryDuration,
		ResetExpiryDuration:        cfg.Auth.ResetExpiryDuration,
		SessionExpiryDuration:      cfg.Auth.SessionExpiryDuration,
		Links:                      linkBuilder,
		Querier:                    database.New(pool),
		EmailSender:                ratelimitedEmailSender,
		EmailRenderer:              templateLoader,
	})
	if err != nil {
		return err
	}

	apiServer, err := api.NewServer(api.ServerConfig{
		Querier:        database.New(pool),
		Sender:         emailSender,
		TokenLinks:     linkBuilder,
		AuthService:    authService,
		PagesTemplates: templateLoader,
	})
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           apiServer.Routes(),
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
