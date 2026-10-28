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

		envHTTPPort:              "8080",
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

		envAuthRateLimitEmailBurst:    "3",
		envAuthRateLimitEmailInterval: "15m",
		envAuthRateLimitIPBurst:       "20",
		envAuthRateLimitIPInterval:    "45s",
		envAuthRateLimitRevealDenials: "true",

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

	if cfg.HTTP.Port != 8080 {
		t.Errorf("HTTP.Port = %d", cfg.HTTP.Port)
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
	if cfg.DailyEmailTime.Hour != 8 || cfg.DailyEmailTime.Minute != 0 {
		t.Errorf("DailyEmailTime = %+v, want 08:00", cfg.DailyEmailTime)
	}
	if cfg.AuthRateLimit.EmailBurst != 3 || cfg.AuthRateLimit.EmailInterval != 15*time.Minute {
		t.Errorf("AuthRateLimit email = (%d, %v)", cfg.AuthRateLimit.EmailBurst, cfg.AuthRateLimit.EmailInterval)
	}
	if cfg.AuthRateLimit.IPBurst != 20 || cfg.AuthRateLimit.IPInterval != 45*time.Second {
		t.Errorf("AuthRateLimit ip = (%d, %v)", cfg.AuthRateLimit.IPBurst, cfg.AuthRateLimit.IPInterval)
	}
	if !cfg.AuthRateLimit.RevealDenials {
		t.Error("AuthRateLimit.RevealDenials = false, want true")
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

	t.Setenv(envHTTPPort, "")
	t.Setenv(envPostgresPassword, "")
	t.Setenv(envAuthSessionExpiry, "not-a-duration")

	_, err := LoadServer()
	if err == nil {
		t.Fatal("LoadServer() err = nil, want error")
	}

	for _, key := range []string{envHTTPPort, envPostgresPassword, envAuthSessionExpiry} {
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

// The server shows when the next email is due but does not send it, so it needs
// the time but not the BCC batch size.
func TestLoadServerIgnoresDailyEmailBatchSize(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envDailyEmailBCCBatchSize, "")

	if _, err := LoadServer(); err != nil {
		t.Fatalf("LoadServer() err = %v, want nil", err)
	}
}

func TestLoadServerMalformedDailyEmailTime(t *testing.T) {
	setValidEnv(t)
	t.Setenv(envDailyEmailTime, "half past eight")

	_, err := LoadServer()
	if err == nil || !strings.Contains(err.Error(), envDailyEmailTime) {
		t.Errorf("LoadServer() err = %v, want error naming %s", err, envDailyEmailTime)
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

func TestDailyEmailTimeNext(t *testing.T) {
	at := DailyEmailTimeConfig{Hour: 8, Minute: 0}
	sameDay := time.Date(2026, time.October, 26, 8, 0, 0, 0, time.UTC)
	nextDay := sameDay.AddDate(0, 0, 1)

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"before the send time", sameDay.Add(-30 * time.Minute), sameDay},
		{"at the send time", sameDay, nextDay},
		{"after the send time", sameDay.Add(time.Hour), nextDay},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := at.Next(tc.now); !got.Equal(tc.want) {
				t.Errorf("Next(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

func TestDailyEmailTimeNextConvertsToUTC(t *testing.T) {
	at := DailyEmailTimeConfig{Hour: 8, Minute: 0}
	now := time.Date(2026, time.October, 26, 7, 30, 0, 0, time.UTC)
	// A location east of UTC where it is already the next calendar day.
	east := time.FixedZone("UTC+10", 10*60*60)

	if got, want := at.Next(now.In(east)), now.Add(30*time.Minute); !got.Equal(want) {
		t.Errorf("Next() = %v, want %v", got, want)
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
	t.Setenv(envHTTPPort, "")
	t.Setenv(envAuthBaseURL, "")
	t.Setenv(envEmailRateLimitAddressBurst, "")
	t.Setenv(envAuthRateLimitEmailBurst, "")

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
