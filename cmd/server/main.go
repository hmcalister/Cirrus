package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hmcalister/cirrus/internal/api"
	"github.com/hmcalister/cirrus/internal/config"
	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/cirrus/internal/logging"
	"github.com/hmcalister/cirrus/internal/postgres"
	templates "github.com/hmcalister/cirrus/internal/template"
	skysatchel "github.com/hmcalister/skysatchel/pkg/email"
	"github.com/hmcalister/skysatchel/pkg/email/skysender"
	"github.com/hmcalister/skysatchel/pkg/email/skysender/ratelimit"
	"github.com/hmcalister/stratus/pkg/auth"
	"github.com/hmcalister/stratus/pkg/sessionstore"
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

	// Templates are embedded in the binary (internal/template/embed.go), so a
	// content change is a code change: rebuild and redeploy. Parsing happens
	// here, once, at startup.
	//
	// New parses every *.tmpl in the embedded FS (catching malformed templates)
	// and fails if any name in the package's required lists is absent. The lists
	// are code, not configuration; they live in the template package beside the
	// constants.
	templateLoader, err := templates.New()
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
			body, err := templateLoader.RenderEmail(templates.TemplateMagicLink, struct{ Link string }{Link: link})
			if err != nil {
				slog.Error("error rendering sign-in email", "err", err)
				body = "Sign in to Cirrus: " + link
			}
			return skysatchel.Message{
				Subject: "Sign in to Cirrus",
				Body:    body,
			}
		},
	})
	if err != nil {
		return err
	}

	apiServer, err := api.NewServer(api.ServerConfig{
		AuthService:       authService,
		Users:             database.New(pool),
		AuthHandlers:      authService.AuthHandlers(),
		PagesRenderer:     templateLoader,
		CookieSecure:      cookieSecure,
		SessionCookieName: sessionCookieName,
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
