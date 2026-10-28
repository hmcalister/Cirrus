// Provides the cloud images that are attached to the daily email.
package filestore

import "context"

var _ FileStore = NoOpStore{}

type FileStore interface {
	// Returns the name of every file in the store.
	ListFiles(ctx context.Context) ([]string, error)
	// Returns the contents of the named file.
	GetFile(ctx context.Context, name string) ([]byte, error)
	// Removes the named file from the store.
	DeleteFile(ctx context.Context, name string) error
}

// Reports an empty store and discards every read and delete.
type NoOpStore struct{}

func (NoOpStore) ListFiles(context.Context) ([]string, error) {
	return nil, nil
}

func (NoOpStore) GetFile(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (NoOpStore) DeleteFile(context.Context, string) error {
	return nil
}
