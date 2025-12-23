package filepicker

var (
	validFileExtensionsMap map[string]bool = map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
	}
)

// A filepicker is the method in which the application gets cloud image files from some source.
// A filepicker has one main role; wrap a source of image files and expose methods to get a random image, then remove that image from the repository.
// For convenience, other methods reveal information about the image source as well (e.g. the number of remaining images)
//
// An image source could be a local filesystem directory, a cloud based bucket, or any number of other source,
// all that is required is that the GetRandomImage method returns a valid image file.
type FilePicker interface {
	// GetRandomImage produces a random image from the source.
	// Importantly, GetRandomImage also removes that image from the source, so it cannot be selected again.
	// This means GetRandomImage is not idempotent! Repeated calls will eventually exhaust the source!
	GetRandomImage() (image []byte, err error)

	// Return the number of images remaining the source.
	ImagesRemaining() int

	listImageFiles() (imageFilenames []string)
	getImageFile(filename string) (image []byte, err error)
	removeImageFile(filename string) (err error)
}
