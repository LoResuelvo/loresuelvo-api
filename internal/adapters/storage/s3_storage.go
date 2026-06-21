package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type S3Storage struct {
	client           *s3.Client
	presignClient    *s3.PresignClient
	publicBaseURL    string
	presignExpiresIn func(*s3.PresignOptions)
}

func NewS3Storage(config Config) *S3Storage {
	awsConfig := aws.Config{
		Region:      config.Region,
		Credentials: credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, ""),
	}

	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		if strings.TrimSpace(config.Endpoint) != "" {
			options.BaseEndpoint = aws.String(config.Endpoint)
			options.UsePathStyle = true
		}
	})

	return &S3Storage{
		client:           client,
		presignClient:    s3.NewPresignClient(client),
		publicBaseURL:    strings.TrimRight(config.PublicBaseURL, "/"),
		presignExpiresIn: func(options *s3.PresignOptions) { options.Expires = config.PresignExpiration },
	}
}

func (storage *S3Storage) GenerateUploadURL(ctx context.Context, object filedomain.ObjectToUpload) (*filedomain.UploadTarget, error) {
	contentLength := int64(object.SizeBytes)
	result, err := storage.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(object.Bucket),
		Key:           aws.String(object.Key),
		ContentType:   aws.String(object.MimeType),
		ContentLength: aws.Int64(contentLength),
	}, storage.presignExpiresIn)
	if err != nil {
		return nil, fmt.Errorf("presigning put object: %w", err)
	}

	headers := map[string]string{"Content-Type": object.MimeType}
	for key, values := range result.SignedHeader {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return &filedomain.UploadTarget{URL: result.URL, Headers: headers}, nil
}

func (storage *S3Storage) GenerateDownloadURL(ctx context.Context, object filedomain.ObjectToDownload) (string, error) {
	result, err := storage.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(object.Bucket),
		Key:    aws.String(object.Key),
	}, storage.presignExpiresIn)
	if err != nil {
		return "", fmt.Errorf("presigning get object: %w", err)
	}
	return result.URL, nil
}

func (storage *S3Storage) ReadObjectMetadata(ctx context.Context, bucket, key string) (*filedomain.ObjectMetadata, error) {
	result, err := storage.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("reading object metadata: %w", err)
	}
	return &filedomain.ObjectMetadata{
		MimeType:  aws.ToString(result.ContentType),
		SizeBytes: int(aws.ToInt64(result.ContentLength)),
	}, nil
}

func (storage *S3Storage) PublicURL(bucket, key string) string {
	if storage.publicBaseURL != "" {
		return storage.publicBaseURL + "/" + strings.TrimLeft(key, "/")
	}
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, strings.TrimLeft(key, "/"))
}
