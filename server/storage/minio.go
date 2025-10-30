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
	publicEndpoint    string // Public URL to replace in presigned URLs
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
	// Check if object exists before generating URL
	_, err := s.client.StatObject(ctx, bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("object not found: %w", err)
	}
	
	// Generate presigned URL with internal endpoint
	url, err := s.client.PresignedGetObject(ctx, bucket, objectName, 3600*time.Second, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	
	urlStr := url.String()
	
	// Replace internal endpoint with public endpoint if configured
	if s.publicEndpoint != "" {
		// URL format: http://minio:9000/bucket/object?params
		// Need to replace scheme + host part while keeping path and query
		// Parse to get path and query
		if idx := strings.Index(urlStr, "/"+bucket); idx > 0 {
			// Extract path with query: /bucket/object?params
			pathWithQuery := urlStr[idx:]
			// Combine public endpoint with path
			urlStr = s.publicEndpoint + pathWithQuery
		}
	}
	
	return urlStr, nil
}
