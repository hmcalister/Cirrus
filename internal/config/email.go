package config

import "time"

type EmailConfig struct {
	Host        string
	Port        string
	Username    string
	Password    string
	FromAddress string
	FromName    string
	Timeout     time.Duration
}

// Read the SMTP configuration shared by every executable.
func loadEmail() (EmailConfig, error) {
	var l loader

	cfg := EmailConfig{
		Host:        l.string(envEmailSMTPHost),
		Port:        l.string(envEmailSMTPPort),
		Username:    l.string(envEmailSMTPUsername),
		Password:    l.string(envEmailSMTPPassword),
		FromAddress: l.string(envEmailFromAddress),
		FromName:    l.string(envEmailFromName),
		Timeout:     l.duration(envEmailSMTPTimeout),
	}

	return cfg, l.err()
}

type EmailRateLimitConfig struct {
	AddressBurst         int
	AddressInterval      time.Duration
	GlobalBurst          int
	GlobalInterval       time.Duration
	SweepCadenceDuration time.Duration
}

// Read the rate limits applied to magic link emails, which only the server sends.
func loadEmailRateLimit() (EmailRateLimitConfig, error) {
	var l loader

	cfg := EmailRateLimitConfig{
		AddressBurst:         l.int(envEmailRateLimitAddressBurst),
		AddressInterval:      l.duration(envEmailRateLimitAddressInterval),
		GlobalBurst:          l.int(envEmailRateLimitGlobalBurst),
		GlobalInterval:       l.duration(envEmailRateLimitGlobalInterval),
		SweepCadenceDuration: l.duration(envEmailRateLimitSweepCadenceDuration),
	}

	return cfg, l.err()
}
