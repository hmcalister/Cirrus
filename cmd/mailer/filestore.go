package main

import (
	"context"

	"github.com/hmcalister/cirrus/internal/config"
	"github.com/hmcalister/cirrus/internal/filestore"
	"github.com/hmcalister/cirrus/internal/filestore/s3"
)

// Returns the cloud image store. Debug mode uses a store that serves nothing, so a local
// run neither attaches a cloud nor deletes one from the bucket.
func getFileStore(ctx context.Context, cfg config.MailerConfig) (filestore.FileStore, error) {
	if cfg.Debug {
		return filestore.NoOpStore{}, nil
	}

	return s3.New(ctx, cfg.S3)
}
