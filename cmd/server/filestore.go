package main

import (
	"context"

	"github.com/hmcalister/cirrus/internal/config"
	"github.com/hmcalister/cirrus/internal/filestore"
	"github.com/hmcalister/cirrus/internal/filestore/s3"
)

func getFileStore(ctx context.Context, cfg config.ServerConfig) (filestore.FileStore, error) {
	if cfg.Debug {
		return filestore.NoOpStore{}, nil
	}

	return s3.New(ctx, cfg.S3)
}
