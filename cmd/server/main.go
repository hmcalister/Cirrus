package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/hmcalister/cirrus/internal/api"
	"github.com/hmcalister/cirrus/internal/config"
	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/cirrus/internal/logging"
	"github.com/hmcalister/cirrus/internal/postgres"
	skysatchel "github.com/hmcalister/skysatchel/pkg/email"
	"github.com/hmcalister/skysatchel/pkg/email/skysender"
	"github.com/hmcalister/skysatchel/pkg/email/skysender/ratelimit"
	"github.com/hmcalister/stratus/pkg/auth"
	"github.com/hmcalister/stratus/pkg/sessionstore"
)

const magicLinkEmailBody = `Sign in to Cirrus by visiting the link below:

%s

This link expires shortly and can only be used once. If you did not request it, you can ignore this email.`

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadServer()
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

	emailSender, err := skysender.NewSMTPSender(skysender.SMTPConfig{
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

	// A Secure session cookie is required outside debug mode.
	cookieSecure := !cfg.Debug || strings.HasPrefix(cfg.AuthHTTP.BaseURL, "https://")
	sessionCookieName := api.SessionCookieName(cookieSecure)

	// Stratus owns authentication.
	authService, err := auth.New(auth.AuthServiceConfig{
		Sender:     ratelimitedEmailSender,
		Sessions:   sessionstore.NewMemorySessionStore(),
		BaseURL:    cfg.AuthHTTP.BaseURL,
		LinkTTL:    cfg.Auth.LinkExpiryDuration,
		SessionTTL: cfg.Auth.SessionExpiryDuration,
		SuccessURL: "/account",
		Cookie: auth.CookieConfig{
			Name:   sessionCookieName,
			Secure: cookieSecure,
		},
		LinkEmail: func(link string) skysatchel.Message {
			return skysatchel.Message{
				Subject: "Sign in to Cirrus",
				Body:    fmt.Sprintf(magicLinkEmailBody, link),
			}
		},
	})
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	humaAPI := humago.New(mux, api.NewHumaConfig())
	authService.RegisterHumaHandlers(humaAPI)

	apiServer, err := api.NewServer(mux, humaAPI, api.ServerConfig{
		AuthService:       authService,
		Users:             database.New(pool),
		SessionCookieName: sessionCookieName,
	})
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           apiServer.Routes(),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("ready to serve", "addr", cfg.HTTP.Addr)
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
