package main

import (
	"context"
	_ "embed"
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
	"github.com/hmcalister/stratus/pkg/auth"
	"github.com/hmcalister/stratus/pkg/sessionstore"
)

//go:embed magic_link_template.txt
var magicLinkEmailBody string

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
		slog.Error("could not reach database", "err", err)
		return err
	}
	defer pool.Close()

	files, err := getFileStore(ctx, cfg)
	if err != nil {
		slog.Error("error while constructing filestore", "err", err)
		return err
	}

	emailSender, err := constructSender(cfg)
	if err != nil {
		slog.Error("error while creating email sender", "err", err)
		return err
	}

	// A Secure session cookie is required outside debug mode.
	cookieSecure := !cfg.Debug || strings.HasPrefix(cfg.AuthHTTP.BaseURL, "https://")
	sessionCookieName := api.SessionCookieName(cookieSecure)

	// Stratus owns authentication.
	authService, err := auth.New(auth.AuthServiceConfig{
		Sender:     *emailSender,
		Sessions:   sessionstore.NewMemorySessionStore(),
		BaseURL:    cfg.AuthHTTP.BaseURL,
		LinkTTL:    cfg.Auth.LinkExpiryDuration,
		SessionTTL: cfg.Auth.SessionExpiryDuration,
		SuccessURL: "/account",
		Cookie: auth.CookieConfig{
			Name:   sessionCookieName,
			Secure: cookieSecure,
		},
		RateLimit: auth.RateLimitConfig{
			EmailBurst:    cfg.AuthRateLimit.EmailBurst,
			EmailInterval: cfg.AuthRateLimit.EmailInterval,
			IPBurst:       cfg.AuthRateLimit.IPBurst,
			IPInterval:    cfg.AuthRateLimit.IPInterval,
			RevealDenials: cfg.AuthRateLimit.RevealDenials,
		},
		LinkEmail: func(link string) skysatchel.Message {
			return skysatchel.Message{
				Subject: "Sign in to Cirrus",
				Body:    fmt.Sprintf(magicLinkEmailBody, link),
			}
		},
		ClientIP: getClientIPMapperFunc(cfg.Debug),
	})
	if err != nil {
		slog.Error("error while creating new auth service", "err", err)
		return err
	}

	mux := http.NewServeMux()
	humaAPI := humago.New(mux, api.NewHumaConfig())
	authService.RegisterHumaHandlers(humaAPI)

	apiServer, err := api.NewServer(mux, humaAPI, api.ServerConfig{
		AuthService:       authService,
		Users:             database.New(pool),
		Files:             files,
		SessionCookieName: sessionCookieName,
		DailyEmailTime:    cfg.DailyEmailTime,
	})
	if err != nil {
		slog.Error("error while creating new api server", "err", err)
		return err
	}

	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           apiServer.Routes(),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("ready to serve", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		slog.Error("http server failed", "err", err)
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("error during shutdown", "err", err)
		return err
	}

	slog.Info("server stopped")
	return nil
}
