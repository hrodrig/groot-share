package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3 talks to an S3-compatible bucket (AWS, MinIO, Contabo, …).
type S3 struct {
	api    *s3.Client
	bucket string
}

// S3Config is the subset of process config the client needs.
type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string
	PathStyle bool
}

// NewS3 builds a client using AWS_* env creds (same dialect as groot upload.s3).
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}
	region := firstNonEmpty(cfg.Region, os.Getenv("AWS_REGION"), os.Getenv("AWS_DEFAULT_REGION"), "us-east-1")
	opts := []func(*awscfg.LoadOptions) error{
		awscfg.WithRegion(region),
	}
	if id, secret, token, ok := awsCredsFromEnv(); ok {
		opts = append(opts, awscfg.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(id, secret, token),
		))
	}
	loaded, err := awscfg.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	if strings.TrimSpace(cfg.Endpoint) != "" {
		loaded.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		loaded.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	}
	api := s3.NewFromConfig(loaded, func(o *s3.Options) {
		applyEndpoint(o, cfg.Endpoint, cfg.PathStyle)
	})
	return &S3{api: api, bucket: bucket}, nil
}

func applyEndpoint(o *s3.Options, endpoint string, pathStyle bool) {
	if ep := strings.TrimSpace(endpoint); ep != "" {
		o.BaseEndpoint = aws.String(ep)
	}
	o.UsePathStyle = pathStyle
}

func awsCredsFromEnv() (id, secret, token string, ok bool) {
	id = strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	secret = strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	token = strings.TrimSpace(os.Getenv("AWS_SESSION_TOKEN"))
	ok = id != "" && secret != ""
	return id, secret, token, ok
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Put uploads r (prefer a seekable *os.File for multipart).
func (c *S3) Put(ctx context.Context, key string, r io.Reader) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String("application/gzip"),
	}
	//nolint:staticcheck // SA1019: manager.Uploader still the supported path (groot dialect).
	up := manager.NewUploader(c.api)
	//nolint:staticcheck // SA1019: see NewUploader note above.
	if _, err := up.Upload(ctx, input); err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

// Get streams the object body.
func (c *S3) Get(ctx context.Context, key string) (io.ReadCloser, Object, error) {
	out, err := c.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, Object{}, ErrNotFound
		}
		return nil, Object{}, fmt.Errorf("get object: %w", err)
	}
	obj := Object{Key: key}
	if out.ETag != nil {
		obj.ETag = strings.Trim(*out.ETag, `"`)
	}
	if out.ContentLength != nil {
		obj.Size = *out.ContentLength
	}
	if out.LastModified != nil {
		obj.LastModified = out.LastModified.UTC()
	}
	return out.Body, obj, nil
}

// List pages all keys under prefix.
func (c *S3) List(ctx context.Context, prefix string) ([]Object, error) {
	p := s3.NewListObjectsV2Paginator(c.api, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})
	out := []Object{}
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}
		for _, item := range page.Contents {
			if item.Key == nil || strings.HasSuffix(*item.Key, "/") {
				continue
			}
			obj := Object{Key: *item.Key}
			if item.ETag != nil {
				obj.ETag = strings.Trim(*item.ETag, `"`)
			}
			if item.Size != nil {
				obj.Size = *item.Size
			}
			if item.LastModified != nil {
				obj.LastModified = item.LastModified.UTC()
			}
			out = append(out, obj)
		}
	}
	return out, nil
}

// Head is HeadObject.
func (c *S3) Head(ctx context.Context, key string) (Object, error) {
	out, err := c.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return Object{}, ErrNotFound
		}
		return Object{}, fmt.Errorf("head object: %w", err)
	}
	obj := Object{Key: key, LastModified: time.Time{}}
	if out.ETag != nil {
		obj.ETag = strings.Trim(*out.ETag, `"`)
	}
	if out.ContentLength != nil {
		obj.Size = *out.ContentLength
	}
	if out.LastModified != nil {
		obj.LastModified = out.LastModified.UTC()
	}
	return obj, nil
}

// HeadBucket fails closed when the bucket is missing or creds are wrong.
func (c *S3) HeadBucket(ctx context.Context) error {
	_, err := c.api.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.bucket),
	})
	if err != nil {
		return fmt.Errorf("head bucket: %w", err)
	}
	return nil
}

// Delete is DeleteObject.
func (c *S3) Delete(ctx context.Context, key string) error {
	_, err := c.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	var nf *types.NotFound
	var nsk *types.NoSuchKey
	return errors.As(err, &nf) || errors.As(err, &nsk)
}
