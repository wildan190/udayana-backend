package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

var (
	ErrObjectNotFound = errors.New("gambar produk tidak ditemukan")
	objectKeyPattern  = regexp.MustCompile(`^products-[0-9a-f]{32}\.(jpg|png|webp)$`)
)

type ProductImages interface {
	Upload(context.Context, string, string, io.Reader, int64) error
	Open(context.Context, string) (io.ReadCloser, string, error)
}

type S3ProductImages struct {
	client *s3.Client
	bucket string
}

func NewS3ProductImages(
	endpoint, bucket, region, accessKey, secretKey string,
	usePathStyle bool,
) (*S3ProductImages, error) {
	if endpoint == "" || bucket == "" || region == "" || accessKey == "" || secretKey == "" {
		return nil, errors.New("konfigurasi S3 belum lengkap")
	}
	client := s3.NewFromConfig(aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = usePathStyle
	})
	return &S3ProductImages{client: client, bucket: bucket}, nil
}

func (s *S3ProductImages) Upload(
	ctx context.Context,
	key string,
	contentType string,
	body io.Reader,
	size int64,
) error {
	if !objectKeyPattern.MatchString(key) {
		return errors.New("kunci gambar produk tidak valid")
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("upload gambar produk ke S3: %w", err)
	}
	return nil
}

func (s *S3ProductImages) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if !objectKeyPattern.MatchString(key) {
		return nil, "", ErrObjectNotFound
	}
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) &&
			(apiError.ErrorCode() == "NoSuchKey" || apiError.ErrorCode() == "NotFound") {
			return nil, "", ErrObjectNotFound
		}
		return nil, "", fmt.Errorf("ambil gambar produk dari S3: %w", err)
	}
	return output.Body, aws.ToString(output.ContentType), nil
}

var _ ProductImages = (*S3ProductImages)(nil)
