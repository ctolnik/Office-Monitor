package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client            *minio.Client
	screenshotsBucket string
	usbCopiesBucket   string
	publicEndpoint    string // External endpoint for presigned URLs
}

func New(endpoint, accessKey, secretKey string, useSSL bool, screenshotsBucket, usbCopiesBucket, publicEndpoint string) (*Storage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Use provided bucket names or defaults
	if screenshotsBucket == "" {
		screenshotsBucket = "screenshots"
	}
	if usbCopiesBucket == "" {
		usbCopiesBucket = "usb-copies"
	}

	s := &Storage{
		client:            client,
		screenshotsBucket: screenshotsBucket,
		usbCopiesBucket:   usbCopiesBucket,
		publicEndpoint:    publicEndpoint,
	}

	ctx := context.Background()

	buckets := []string{s.screenshotsBucket, s.usbCopiesBucket}
	for _, bucket := range buckets {
		exists, err := client.BucketExists(ctx, bucket)
		if err != nil {
			return nil, fmt.Errorf("failed to check bucket %s: %w", bucket, err)
		}
		if !exists {
			if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return nil, fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
	}

	return s, nil
}

func (s *Storage) UploadScreenshot(ctx context.Context, screenshotID string, data []byte) (string, error) {
	// Object is stored in bucket root with name: COMPUTER_USERNAME_TIMESTAMP.jpg
	objectName := fmt.Sprintf("%s.jpg", screenshotID)

	_, err := s.client.PutObject(
		ctx,
		s.screenshotsBucket,
		objectName,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{ContentType: "image/jpeg"},
	)
	if err != nil {
		return "", fmt.Errorf("failed to upload screenshot: %w", err)
	}

	return objectName, nil
}

func (s *Storage) UploadUSBFile(ctx context.Context, computerName, relativePath string, data io.Reader, size int64) (string, error) {
	objectName := fmt.Sprintf("%s/%s", computerName, relativePath)

	_, err := s.client.PutObject(
		ctx,
		s.usbCopiesBucket,
		objectName,
		data,
		size,
		minio.PutObjectOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to upload USB file: %w", err)
	}

	return objectName, nil
}

func (s *Storage) GetPresignedURL(ctx context.Context, bucket, objectName string) (string, error) {
	// Use time.Duration for expires parameter (1 hour)
	url, err := s.client.PresignedGetObject(ctx, bucket, objectName, 3600*time.Second, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	
	// Replace internal endpoint with public endpoint if configured
	if s.publicEndpoint != "" {
		urlStr := url.String()
		// URL format: http://minio:9000/bucket/object?params
		// Replace internal endpoint with public endpoint
		urlStr = strings.Replace(urlStr, s.client.EndpointURL().String(), s.publicEndpoint, 1)
		return urlStr, nil
	}
	
	return url.String(), nil
}
