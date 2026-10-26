package config

import (
	"errors"
	"fmt"
)

// DailyEmailConfig is when, and how, the daily email job runs.
type DailyEmailConfig struct {
	// Time of day, in UTC, that the job starts.
	Hour   int
	Minute int
	// How many recipients share one BCC'd email.
	BCCBatchSize int
}

// MailerConfig is the configuration used by cmd/mailer.
type MailerConfig struct {
	Database   DatabaseConfig
	Email      EmailConfig
	DailyEmail DailyEmailConfig
	S3         S3Config
	Debug      bool
}

// Read the configuration needed by cmd/mailer from the environment.
func LoadMailer() (MailerConfig, error) {
	database, databaseErr := loadDatabase()
	email, emailErr := loadEmail()
	dailyEmail, dailyEmailErr := loadDailyEmail()
	s3Cfg, s3Err := loadS3()

	cfg := MailerConfig{
		Database:   database,
		Email:      email,
		DailyEmail: dailyEmail,
		S3:         s3Cfg,
		Debug:      debugEnabled(),
	}

	return cfg, errors.Join(databaseErr, emailErr, dailyEmailErr, s3Err)
}

func loadDailyEmail() (DailyEmailConfig, error) {
	var l loader

	hour, minute := l.clock(envDailyEmailTime)

	cfg := DailyEmailConfig{
		Hour:         hour,
		Minute:       minute,
		BCCBatchSize: l.int(envDailyEmailBCCBatchSize),
	}
	if cfg.BCCBatchSize < 1 {
		l.errs = append(l.errs, fmt.Errorf("environment variable %s must be at least 1", envDailyEmailBCCBatchSize))
	}

	return cfg, l.err()
}
