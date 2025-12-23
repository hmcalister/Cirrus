package filepicker

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// File picker using a local directory as a source.
type LocalDirectoryFilePicker struct {
	mu            sync.RWMutex
	directoryPath string
}

// Creates a new LocalDirectoryFilePicker for the given directory path.
// An error is returned if the directory doesn't exist or cannot be read.
//
// The directory is scanned for valid files whenever a new file is requested
// meaning new files may be added asynchronously and safely.
func NewLocalDirectoryFilePicker(directoryPath string) (*LocalDirectoryFilePicker, error) {
	absPath, err := filepath.Abs(directoryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absPath)
	}

	picker := &LocalDirectoryFilePicker{
		directoryPath: absPath,
	}

	return picker, nil
}

// GetRandomImage selects a random image from the directory, returns its contents,
// and removes it from the filesystem.
func (p *LocalDirectoryFilePicker) GetRandomImage() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	imageFiles := p.listImageFiles()
	if len(imageFiles) == 0 {
		return nil, fmt.Errorf("no images available in directory")
	}

	// Select random image
	randomIndex := rand.IntN(len(imageFiles))
	selectedFile := imageFiles[randomIndex]

	// Read the image
	image, err := p.getImageFile(selectedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	// Remove the image
	err = p.removeImageFile(selectedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to remove image: %w", err)
	}

	return image, nil
}

// ImagesRemaining returns the count of valid image files in the directory.
func (p *LocalDirectoryFilePicker) ImagesRemaining() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.listImageFiles())
}

// listImageFiles scans the directory and returns filenames of valid image files.
func (p *LocalDirectoryFilePicker) listImageFiles() []string {
	entries, err := os.ReadDir(p.directoryPath)
	if err != nil {
		return []string{}
	}

	var imageFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if validFileExtensionsMap[ext] {
			imageFiles = append(imageFiles, entry.Name())
		}
	}

	return imageFiles
}

// getImageFile reads the entire contents of the specified image file.
func (p *LocalDirectoryFilePicker) getImageFile(filename string) ([]byte, error) {
	fullPath := filepath.Join(p.directoryPath, filename)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	return data, nil
}

// removeImageFile deletes the specified image file from the directory.
func (p *LocalDirectoryFilePicker) removeImageFile(filename string) error {
	fullPath := filepath.Join(p.directoryPath, filename)
	err := os.Remove(fullPath)
	if err != nil {
		return fmt.Errorf("failed to remove file %s: %w", filename, err)
	}
	return nil
}
