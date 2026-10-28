package config

import (
	"errors"
	"fmt"
	"time"
)

// DailyEmailTimeConfig is the time of day, in UTC, that the daily email job starts.
type DailyEmailTimeConfig struct {
	Hour   int
	Minute int
}

// Next returns the next occurrence of the configured time of day, in UTC.
func (c DailyEmailTimeConfig) Next(now time.Time) time.Time {
	now = now.UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), c.Hour, c.Minute, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// DailyEmailConfig is when, and how, the daily email job runs.
type DailyEmailConfig struct {
	DailyEmailTimeConfig
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

// Read the time of day the daily email job starts. The server reads it to show
// users when their next clouds arrive.
func loadDailyEmailTime() (DailyEmailTimeConfig, error) {
	var l loader

	hour, minute := l.clock(envDailyEmailTime)

	return DailyEmailTimeConfig{Hour: hour, Minute: minute}, l.err()
}

func loadDailyEmail() (DailyEmailConfig, error) {
	when, whenErr := loadDailyEmailTime()

	var l loader

	cfg := DailyEmailConfig{
		DailyEmailTimeConfig: when,
		BCCBatchSize:         l.int(envDailyEmailBCCBatchSize),
	}
	if cfg.BCCBatchSize < 1 {
		l.errs = append(l.errs, fmt.Errorf("environment variable %s must be at least 1", envDailyEmailBCCBatchSize))
	}

	return cfg, errors.Join(whenErr, l.err())
}
