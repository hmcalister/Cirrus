package config

import (
	"strings"
	"testing"
	"time"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		envDebug: "true",

		envHTTPAddr:              ":8080",
		envHTTPReadHeaderTimeout: "5s",
		envHTTPReadTimeout:       "30s",
		envHTTPWriteTimeout:      "30s",
		envHTTPIdleTimeout:       "2m",
		envHTTPShutdownTimeout:   "10s",

		envPostgresUser:     "cirrus",
		envPostgresPassword: "secret",
		envPostgresHost:     "database",
		envPostgresPort:     "5432",
		envPostgresDB:       "cirrus",

		envDBMaxConns:          "10",
		envDBMinConns:          "1",
		envDBMaxConnLifetime:   "1h",
		envDBMaxConnIdleTime:   "30m",
		envDBHealthCheckPeriod: "1m",

		envEmailSMTPHost:     "smtp.example.com",
		envEmailSMTPPort:     "465",
		envEmailSMTPUsername: "user",
		envEmailSMTPPassword: "secret",
		envEmailFromAddress:  "noreply@example.com",
		envEmailFromName:     "Cirrus",
		envEmailSMTPTimeout:  "30s",

		envEmailRateLimitAddressBurst:         "5",
		envEmailRateLimitAddressInterval:      "1m",
		envEmailRateLimitGlobalBurst:          "100",
		envEmailRateLimitGlobalInterval:       "1s",
		envEmailRateLimitSweepCadenceDuration: "1m",

		envAuthVerificationExpiry: "24h",
		envAuthResetExpiry:        "1h",
		envAuthSessionExpiry:      "720h",
		envAuthBaseURL:            "https://cloud.example.com",
	} {
		t.Setenv(key, value)
	}
}

func TestLoadValid(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v, want nil", err)
	}

	if cfg.Server.Addr != ":8080" {
		t.Errorf("Server.Addr = %q", cfg.Server.Addr)
	}
	if cfg.Database.MaxConns != 10 || cfg.Database.MinConns != 1 {
		t.Errorf("Database conns = (%d, %d)", cfg.Database.MaxConns, cfg.Database.MinConns)
	}
	if want := "postgres://cirrus:secret@database:5432/cirrus"; cfg.Database.DatabaseURL != want {
		t.Errorf("Database.DatabaseURL = %q, want %q", cfg.Database.DatabaseURL, want)
	}
	if cfg.Email.Timeout != 30*time.Second {
		t.Errorf("Email.Timeout = %v", cfg.Email.Timeout)
	}
	if cfg.EmailRateLimit.AddressBurst != 5 || cfg.EmailRateLimit.GlobalBurst != 100 {
		t.Errorf("EmailRateLimit = %+v", cfg.EmailRateLimit)
	}
	if cfg.Auth.SessionExpiryDuration != 720*time.Hour {
		t.Errorf("Auth.SessionExpiryDuration = %v", cfg.Auth.SessionExpiryDuration)
	}
	if cfg.AuthHTTP.BaseURL != "https://cloud.example.com" {
		t.Errorf("AuthHTTP.BaseURL = %q", cfg.AuthHTTP.BaseURL)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
}

func TestLoadAccumulatesErrors(t *testing.T) {
	setValidEnv(t)

	t.Setenv(envHTTPAddr, "")
	t.Setenv(envPostgresPassword, "")
	t.Setenv(envAuthSessionExpiry, "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() err = nil, want error")
	}

	for _, key := range []string{envHTTPAddr, envPostgresPassword, envAuthSessionExpiry} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %s: %v", key, err)
		}
	}
}

func TestLoadMissingVariable(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envEmailSMTPHost, "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), envEmailSMTPHost) {
		t.Errorf("Load() err = %v, want error naming %s", err, envEmailSMTPHost)
	}
}

func TestLoadMalformedVariable(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envDBMaxConns, "many")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), envDBMaxConns) {
		t.Errorf("Load() err = %v, want error naming %s", err, envDBMaxConns)
	}
}

func TestLoadOptionalVariables(t *testing.T) {
	setValidEnv(t)
	// Debug is optional; unset it.
	t.Setenv(envDebug, "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Debug {
		t.Error("Debug = true, want false when unset")
	}
}
