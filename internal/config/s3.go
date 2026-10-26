package config

import "os"

// S3Config is the bucket that daily cloud images are stored in.
type S3Config struct {
	Bucket string
	Region string
	// Custom endpoint for S3-compatible services such as DigitalOcean Spaces. Empty uses the AWS endpoint for Region.
	Endpoint string
}

// Read the bucket configuration, which only the daily email job uses.
func loadS3() (S3Config, error) {
	var l loader

	cfg := S3Config{
		Bucket:   l.string(envS3Bucket),
		Region:   l.string(envS3Region),
		Endpoint: os.Getenv(envS3Endpoint),
	}

	return cfg, l.err()
}
