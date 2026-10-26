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

		envAuthLinkExpiry:    "15m",
		envAuthSessionExpiry: "720h",
		envAuthBaseURL:       "https://cloud.example.com",

		envDailyEmailTime:         "08:00",
		envDailyEmailBCCBatchSize: "50",

		envS3Bucket:   "cirrus-clouds",
		envS3Region:   "nyc3",
		envS3Endpoint: "https://nyc3.digitaloceanspaces.com",
	} {
		t.Setenv(key, value)
	}
}

func TestLoadServer(t *testing.T) {
	setValidEnv(t)

	cfg, err := LoadServer()
	if err != nil {
		t.Fatalf("LoadServer() err = %v, want nil", err)
	}

	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("HTTP.Addr = %q", cfg.HTTP.Addr)
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
	if cfg.Auth.LinkExpiryDuration != 15*time.Minute {
		t.Errorf("Auth.LinkExpiryDuration = %v", cfg.Auth.LinkExpiryDuration)
	}
	if cfg.Auth.SessionExpiryDuration != 720*time.Hour {
		t.Errorf("Auth.SessionExpiryDuration = %v", cfg.Auth.SessionExpiryDuration)
	}
	if cfg.AuthHTTP.BaseURL != "https://cloud.example.com" {
		t.Errorf("AuthHTTP.BaseURL = %q", cfg.AuthHTTP.BaseURL)
	}
	if cfg.S3.Bucket != "cirrus-clouds" || cfg.S3.Region != "nyc3" {
		t.Errorf("S3 = %+v", cfg.S3)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
}

func TestLoadServerAccumulatesErrors(t *testing.T) {
	setValidEnv(t)

	t.Setenv(envHTTPAddr, "")
	t.Setenv(envPostgresPassword, "")
	t.Setenv(envAuthSessionExpiry, "not-a-duration")

	_, err := LoadServer()
	if err == nil {
		t.Fatal("LoadServer() err = nil, want error")
	}

	for _, key := range []string{envHTTPAddr, envPostgresPassword, envAuthSessionExpiry} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %s: %v", key, err)
		}
	}
}

func TestLoadServerMissingVariable(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envEmailSMTPHost, "")

	_, err := LoadServer()
	if err == nil || !strings.Contains(err.Error(), envEmailSMTPHost) {
		t.Errorf("LoadServer() err = %v, want error naming %s", err, envEmailSMTPHost)
	}
}

func TestLoadServerMalformedVariable(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envDBMaxConns, "many")

	_, err := LoadServer()
	if err == nil || !strings.Contains(err.Error(), envDBMaxConns) {
		t.Errorf("LoadServer() err = %v, want error naming %s", err, envDBMaxConns)
	}
}

// The server does not schedule the daily email, so it must not need its settings.
func TestLoadServerIgnoresDailyEmailVariables(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envDailyEmailTime, "")
	t.Setenv(envDailyEmailBCCBatchSize, "")

	if _, err := LoadServer(); err != nil {
		t.Fatalf("LoadServer() err = %v, want nil", err)
	}
}

func TestLoadServerOptionalDebug(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envDebug, "")

	cfg, err := LoadServer()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Debug {
		t.Error("Debug = true, want false when unset")
	}
}

func TestLoadMailer(t *testing.T) {
	setValidEnv(t)

	cfg, err := LoadMailer()
	if err != nil {
		t.Fatalf("LoadMailer() err = %v, want nil", err)
	}

	if want := "postgres://cirrus:secret@database:5432/cirrus"; cfg.Database.DatabaseURL != want {
		t.Errorf("Database.DatabaseURL = %q, want %q", cfg.Database.DatabaseURL, want)
	}
	if cfg.Email.Timeout != 30*time.Second {
		t.Errorf("Email.Timeout = %v", cfg.Email.Timeout)
	}
	if cfg.DailyEmail.Hour != 8 || cfg.DailyEmail.Minute != 0 {
		t.Errorf("DailyEmail = %+v", cfg.DailyEmail)
	}
	if cfg.DailyEmail.BCCBatchSize != 50 {
		t.Errorf("DailyEmail.BCCBatchSize = %d, want 50", cfg.DailyEmail.BCCBatchSize)
	}
	if cfg.S3.Bucket != "cirrus-clouds" || cfg.S3.Region != "nyc3" {
		t.Errorf("S3 = %+v", cfg.S3)
	}
	if cfg.S3.Endpoint != "https://nyc3.digitaloceanspaces.com" {
		t.Errorf("S3.Endpoint = %q", cfg.S3.Endpoint)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
}

func TestLoadMailerMalformedDailyEmailTime(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envDailyEmailTime, "half past eight")

	_, err := LoadMailer()
	if err == nil || !strings.Contains(err.Error(), envDailyEmailTime) {
		t.Errorf("LoadMailer() err = %v, want error naming %s", err, envDailyEmailTime)
	}
}

func TestLoadMailerInvalidBCCBatchSize(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envDailyEmailBCCBatchSize, "0")

	_, err := LoadMailer()
	if err == nil || !strings.Contains(err.Error(), envDailyEmailBCCBatchSize) {
		t.Errorf("LoadMailer() err = %v, want error naming %s", err, envDailyEmailBCCBatchSize)
	}
}

func TestLoadMailerMissingS3Bucket(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envS3Bucket, "")

	_, err := LoadMailer()
	if err == nil || !strings.Contains(err.Error(), envS3Bucket) {
		t.Errorf("LoadMailer() err = %v, want error naming %s", err, envS3Bucket)
	}
}

// The mailer serves no HTTP and handles no sign in, so it must not need the
// server's settings.
func TestLoadMailerIgnoresServerVariables(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envHTTPAddr, "")
	t.Setenv(envAuthBaseURL, "")
	t.Setenv(envEmailRateLimitAddressBurst, "")

	if _, err := LoadMailer(); err != nil {
		t.Fatalf("LoadMailer() err = %v, want nil", err)
	}
}

func TestLoadMailerOptionalDebugAndEndpoint(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envDebug, "")
	t.Setenv(envS3Endpoint, "")

	cfg, err := LoadMailer()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Debug {
		t.Error("Debug = true, want false when unset")
	}
	if cfg.S3.Endpoint != "" {
		t.Errorf("S3.Endpoint = %q, want empty", cfg.S3.Endpoint)
	}
}
