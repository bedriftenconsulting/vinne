package storage

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type spacesStorage struct {
	client       *s3.Client
	bucket       string
	endpoint     string
	cdnEndpoint  string
	maxRetries   int
	retryBackoff time.Duration
}

func newSpacesStorage(cfg Config) (Storage, error) {
	if cfg.Endpoint == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.Bucket == "" {
		return nil, ErrInvalidConfig
	}

	// Create custom resolver for DigitalOcean Spaces
	//nolint:staticcheck // Using deprecated EndpointResolver for compatibility with DigitalOcean Spaces
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		//nolint:staticcheck // Using deprecated Endpoint struct for compatibility
		return aws.Endpoint{
			URL:               cfg.Endpoint,
			SigningRegion:     cfg.Region,
			HostnameImmutable: true,
		}, nil
	})

	// Configure AWS SDK
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		//nolint:staticcheck // Using deprecated WithEndpointResolverWithOptions for compatibility
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Create S3 client with custom configuration
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.ForcePathStyle
	})

	return &spacesStorage{
		client:       client,
		bucket:       cfg.Bucket,
		endpoint:     cfg.Endpoint,
		cdnEndpoint:  cfg.CDNEndpoint,
		maxRetries:   cfg.MaxRetries,
		retryBackoff: cfg.RetryBackoff,
	}, nil
}

// withRetry executes an operation with retry logic
func (s *spacesStorage) withRetry(ctx context.Context, operation string, fn func() error) error {
	_, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "storage."+operation)
	defer span.End()

	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		// Check context cancellation before each attempt
		if err := ctx.Err(); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "context timeout")
			return ErrOperationTimeout
		}

		if attempt > 0 {
			span.SetAttributes(attribute.Int("retry.attempt", attempt))
			// Use context-aware sleep
			timer := time.NewTimer(s.retryBackoff * time.Duration(attempt))
			select {
			case <-ctx.Done():
				timer.Stop()
				span.RecordError(ctx.Err())
				span.SetStatus(codes.Error, "context timeout")
				return ErrOperationTimeout
			case <-timer.C:
			}
		}

		if err := fn(); err != nil {
			lastErr = err
			// Check if error is retryable
			if strings.Contains(err.Error(), "RequestTimeout") ||
				strings.Contains(err.Error(), "SlowDown") ||
				strings.Contains(err.Error(), "InternalError") {
				continue
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err // Non-retryable error
		}
		return nil // Success
	}

	span.RecordError(lastErr)
	span.SetStatus(codes.Error, fmt.Sprintf("failed after %d retries", s.maxRetries))
	return fmt.Errorf("operation failed after %d retries: %w", s.maxRetries, lastErr)
}

func (s *spacesStorage) Upload(ctx context.Context, info UploadInfo) (*ObjectInfo, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "storage.upload")
	defer span.End()

	span.SetAttributes(
		attribute.String("storage.provider", "spaces"),
		attribute.String("game.id", info.GameID),
		attribute.String("file.name", info.FileName),
		attribute.String("file.content_type", info.ContentType),
		attribute.Int64("file.size", info.Size),
	)

	// Validate image
	if err := ValidateImage(info.ContentType, info.Size); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "validation failed")
		return nil, err
	}

	key := buildKey(info.GameID, info.FileName)
	var result *ObjectInfo

	err := s.withRetry(ctx, "upload", func() error {
		// Create a fresh reader for each retry attempt
		// This ensures retries don't read from an exhausted reader
		reader := bytes.NewReader(info.Data)

		input := &s3.PutObjectInput{
			Bucket:        aws.String(s.bucket),
			Key:           aws.String(key),
			Body:          reader,
			ContentLength: aws.Int64(info.Size),
			ContentType:   aws.String(info.ContentType),
		}

		// Set ACL based on Permission
		if info.Permission == "public" {
			input.ACL = types.ObjectCannedACLPublicRead
		} else {
			input.ACL = types.ObjectCannedACLPrivate
		}

		output, err := s.client.PutObject(ctx, input)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUploadFailed, err)
		}

		url := s.getObjectURL(key)
		cdnURL := s.getCDNURL(key)

		// Strip quotes from ETag (S3/Spaces returns ETags wrapped in quotes)
		etag := aws.ToString(output.ETag)
		etag = strings.Trim(etag, "\"")

		result = &ObjectInfo{
			Key:          key,
			Size:         info.Size,
			LastModified: time.Now(),
			ContentType:  info.ContentType,
			ETag:         etag,
			URL:          url,
			CDNURL:       cdnURL,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	span.SetAttributes(
		attribute.String("object.key", result.Key),
		attribute.String("object.url", result.URL),
		attribute.String("object.cdn_url", result.CDNURL),
	)

	return result, nil
}

func (s *spacesStorage) Delete(ctx context.Context, gameID string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "storage.delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("storage.provider", "spaces"),
		attribute.String("game.id", gameID),
	)

	// List all objects with the game prefix
	prefix := "games/" + gameID + "/"

	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	}

	listOutput, err := s.client.ListObjectsV2(ctx, listInput)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list objects")
		return fmt.Errorf("failed to list objects: %w", err)
	}

	if len(listOutput.Contents) == 0 {
		// No files to delete
		return nil
	}

	// Delete all objects
	for _, obj := range listOutput.Contents {
		key := aws.ToString(obj.Key)
		err := s.withRetry(ctx, "delete", func() error {
			input := &s3.DeleteObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    aws.String(key),
			}

			_, err := s.client.DeleteObject(ctx, input)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrDeleteFailed, err)
			}

			return nil
		})

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to delete object")
			return err
		}

		span.AddEvent("deleted object", trace.WithAttributes(attribute.String("object.key", key)))
	}

	return nil
}

func (s *spacesStorage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "storage.get_url")
	defer span.End()

	span.SetAttributes(
		attribute.String("storage.provider", "spaces"),
		attribute.String("object.key", key),
		attribute.String("url.expires", expires.String()),
	)

	var url string
	err := s.withRetry(ctx, "get_url", func() error {
		presignClient := s3.NewPresignClient(s.client)
		request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		}, s3.WithPresignExpires(expires))
		if err != nil {
			return fmt.Errorf("failed to generate presigned URL: %w", err)
		}

		url = request.URL
		return nil
	})

	if err != nil {
		return "", err
	}

	span.SetAttributes(attribute.String("presigned.url", url))
	return url, nil
}

func (s *spacesStorage) getObjectURL(key string) string {
	// For path-style (MinIO/local), use endpoint/bucket/key
	// For virtual-hosted style (DigitalOcean Spaces), use bucket.endpoint/key
	if strings.Contains(s.endpoint, "localhost") || strings.Contains(s.endpoint, "minio") || strings.Contains(s.endpoint, "127.0.0.1") {
		return fmt.Sprintf("%s/%s/%s", strings.TrimRight(s.endpoint, "/"), s.bucket, key)
	}
	return fmt.Sprintf("https://%s.%s/%s", s.bucket, strings.TrimPrefix(s.endpoint, "https://"), key)
}

func (s *spacesStorage) getCDNURL(key string) string {
	if s.cdnEndpoint != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(s.cdnEndpoint, "/"), key)
	}
	return s.getObjectURL(key)
}

func (s *spacesStorage) Close() error {
	// No cleanup needed for S3 client
	return nil
}
