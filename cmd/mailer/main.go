// Package main runs the daily email job.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/hmcalister/cirrus/internal/config"
	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/cirrus/internal/filestore"
	"github.com/hmcalister/cirrus/internal/logging"
	"github.com/hmcalister/cirrus/internal/postgres"
	skysatchel "github.com/hmcalister/skysatchel/pkg/email"
	"github.com/hmcalister/skysatchel/pkg/email/skysender"
)

const (
	dailyEmailSubject = "Cloud Mail"
	dailyEmailBody    = "Enjoy the clouds today!"

	lowImageThreshold = 7
	lowImageSubject   = "Cloud images running low"
	lowImageBody      = "Only %d cloud images remain."
)

// Lists the addresses that receive the daily email, and the admins who are
// alerted when images run low.
type userLister interface {
	ListActiveEmails(ctx context.Context) ([]string, error)
	ListAdminEmails(ctx context.Context) ([]string, error)
}

// The cloud image shared by every email in one run.
type dailyCloud struct {
	name       string
	attachment skysatchel.Attachment
}

// The configured dependencies of one daily send.
type dailyEmailer struct {
	users       userLister
	files       filestore.FileStore
	sender      skysender.Sender
	fromAddress string
	batchSize   int
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadMailer()
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

	sender, err := constructSender(cfg)
	if err != nil {
		slog.Error("error while creating email sender", "err", err)
		return err
	}

	files, err := getFileStore(ctx, cfg)
	if err != nil {
		slog.Error("could not reach cloud bucket", "err", err)
		return fmt.Errorf("connecting to cloud image bucket: %w", err)
	}

	daily := dailyEmailer{
		users:       database.New(pool),
		files:       files,
		sender:      sender,
		fromAddress: cfg.Email.FromAddress,
		batchSize:   cfg.DailyEmail.BCCBatchSize,
	}

	for {
		next := cfg.DailyEmail.Next(time.Now())
		slog.Info("waiting for the next daily email", "at", next)
		if !waitUntil(ctx, next) {
			slog.Info("shutdown signal received")
			return nil
		}

		if err := daily.send(ctx); err != nil {
			slog.Error("daily email run failed", "err", err)
		}
	}
}

// Waits until the given time, returning false if the job is stopped first.
func waitUntil(ctx context.Context, at time.Time) bool {
	timer := time.NewTimer(time.Until(at))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// Sends the day's cloud to every active user in BCC batches. A failed batch is logged and skipped so that it cannot stop the run.
func (d dailyEmailer) send(ctx context.Context) error {
	emails, err := d.users.ListActiveEmails(ctx)
	if err != nil {
		return fmt.Errorf("listing active users: %w", err)
	}
	if len(emails) == 0 {
		slog.Info("no active users, skipping daily email")
		return nil
	}

	cloud, err := d.pickCloud(ctx)
	if err != nil {
		return err
	}

	sent := 0
	for batch := range slices.Chunk(emails, d.batchSize) {
		if err := d.sendBatch(ctx, batch, cloud.attachment); err != nil {
			slog.Warn("failed to send daily email batch", "recipients", len(batch), "err", err)
			continue
		}
		sent += len(batch)
	}

	slog.Info("daily email run complete", "recipients", len(emails), "sent", sent)

	if sent > 0 {
		if err := d.files.DeleteFile(ctx, cloud.name); err != nil {
			slog.Warn("failed to delete sent cloud image", "name", cloud.name, "err", err)
		}

		if err := d.alertWhenImagesLow(ctx); err != nil {
			slog.Warn("failed to alert admins about low images", "err", err)
		}
	}

	return nil
}

// Emails every admin when lowImageThreshold files or fewer remain in the store.
func (d dailyEmailer) alertWhenImagesLow(ctx context.Context) error {
	names, err := d.files.ListFiles(ctx)
	if err != nil {
		return fmt.Errorf("listing cloud images: %w", err)
	}
	if len(names) > lowImageThreshold {
		return nil
	}

	admins, err := d.users.ListAdminEmails(ctx)
	if err != nil {
		return fmt.Errorf("listing admins: %w", err)
	}
	if len(admins) == 0 {
		slog.Warn("cloud images are low but there are no admins to alert", "remaining", len(names))
		return nil
	}

	if err := d.sendAdminAlert(ctx, admins, len(names)); err != nil {
		return fmt.Errorf("sending low image alert: %w", err)
	}

	slog.Info("alerted admins about low cloud images", "remaining", len(names), "admins", len(admins))
	return nil
}

// Sends the low image alert to every admin in one BCC'd message.
func (d dailyEmailer) sendAdminAlert(ctx context.Context, admins []string, remaining int) error {
	return d.sender.Send(ctx, skysender.SendParams{
		Subject:       lowImageSubject,
		Body:          fmt.Sprintf(lowImageBody, remaining),
		ToRecipients:  []string{d.fromAddress},
		BCCRecipients: admins,
	})
}

// Picks the one cloud image used by every email of this run.
func (d dailyEmailer) pickCloud(ctx context.Context) (dailyCloud, error) {
	names, err := d.files.ListFiles(ctx)
	if err != nil {
		return dailyCloud{}, fmt.Errorf("listing cloud images: %w", err)
	}
	if len(names) == 0 {
		return dailyCloud{}, errors.New("no clouds remaining")
	}

	name := names[rand.IntN(len(names))]
	data, err := d.files.GetFile(ctx, name)
	if err != nil {
		return dailyCloud{}, fmt.Errorf("getting cloud image: %w", err)
	}

	return dailyCloud{
		name: name,
		attachment: skysatchel.Attachment{
			Filename: filepath.Base(name),
			Data:     data,
		},
	}, nil
}

// Sends one email to a batch of recipients.
//
// Every recipient is BCC so that no recipient sees another's address. The sender's address is the To because SMTP requires at least one.
func (d dailyEmailer) sendBatch(ctx context.Context, emails []string, attachment skysatchel.Attachment) error {
	return d.sender.Send(ctx, skysender.SendParams{
		Subject:       dailyEmailSubject,
		Body:          dailyEmailBody,
		Attachments:   []skysatchel.Attachment{attachment},
		ToRecipients:  []string{d.fromAddress},
		BCCRecipients: emails,
	})
}
