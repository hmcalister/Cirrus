// Implemented FileGetter for S3 buckets
package s3

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/hmcalister/cirrus/internal/config"
	"github.com/hmcalister/cirrus/internal/filestore"
)

var _ filestore.FileStore = (*Store)(nil)

type Store struct {
	client *awss3.Client
	bucket string
}

// Connects to the bucket described by cfg.
//
// Credentials come from the AWS default chain, so the access key and secret can be given as AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY.
func New(ctx context.Context, cfg config.S3Config) (*Store, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("loading aws config: %w", err)
	}

	client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		if cfg.Endpoint != "" {
			// S3-compatible services such as DigitalOcean Spaces have their own endpoint and are reached with path-style URLs.
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})

	return &Store{client: client, bucket: cfg.Bucket}, nil
}

func (s *Store) ListFiles(ctx context.Context) ([]string, error) {
	var names []string

	paginator := awss3.NewListObjectsV2Paginator(s.client, &awss3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing files in bucket %s: %w", s.bucket, err)
		}
		for _, object := range page.Contents {
			name := aws.ToString(object.Key)
			// Keys ending in a slash are folder markers, not files.
			if strings.HasSuffix(name, "/") {
				continue
			}
			names = append(names, name)
		}
	}

	return names, nil
}

func (s *Store) GetFile(ctx context.Context, name string) ([]byte, error) {
	object, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		return nil, fmt.Errorf("getting %s from bucket %s: %w", name, s.bucket, err)
	}
	defer func() {
		_ = object.Body.Close()
	}()

	data, err := io.ReadAll(object.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s from bucket %s: %w", name, s.bucket, err)
	}

	return data, nil
}

func (s *Store) DeleteFile(ctx context.Context, name string) error {
	_, err := s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		return fmt.Errorf("deleting %s from bucket %s: %w", name, s.bucket, err)
	}

	return nil
}
