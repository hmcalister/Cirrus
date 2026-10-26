// Provides the cloud images that are attached to the daily email.
package filestore

import "context"

type FileGetter interface {
	// Returns the name of every file in the store.
	ListFiles(ctx context.Context) ([]string, error)
	// Returns the contents of the named file.
	GetFile(ctx context.Context, name string) ([]byte, error)
	// Removes the named file from the store.
	DeleteFile(ctx context.Context, name string) error
}
