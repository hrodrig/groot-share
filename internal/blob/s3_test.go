package blob

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestFirstNonEmptyAndAwsCredsFromEnv(t *testing.T) {
	if firstNonEmpty("", "  ", "ok", "later") != "ok" {
		t.Fatal("firstNonEmpty")
	}
	if firstNonEmpty("", "") != "" {
		t.Fatal("all empty")
	}
	t.Setenv("AWS_ACCESS_KEY_ID", " ak ")
	t.Setenv("AWS_SECRET_ACCESS_KEY", " sk ")
	t.Setenv("AWS_SESSION_TOKEN", " tok ")
	id, secret, token, ok := awsCredsFromEnv()
	if !ok || id != "ak" || secret != "sk" || token != "tok" {
		t.Fatalf("creds %q %q %q %v", id, secret, token, ok)
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	if _, _, _, ok := awsCredsFromEnv(); ok {
		t.Fatal("missing creds")
	}
}

func TestNewS3LoadsConfigWithCreds(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "ak")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "sk")
	t.Setenv("AWS_REGION", "eu-west-1")
	c, err := NewS3(context.Background(), S3Config{Bucket: "lab", Endpoint: "https://example.com", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.bucket != "lab" {
		t.Fatalf("client %+v", c)
	}
}

func TestIsNotFound(t *testing.T) {
	if !isNotFound(&types.NotFound{}) {
		t.Fatal("NotFound")
	}
	if !isNotFound(&types.NoSuchKey{}) {
		t.Fatal("NoSuchKey")
	}
	if isNotFound(errors.New("other")) {
		t.Fatal("generic err")
	}
}

func TestNewS3DefaultRegionFromEnv(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "ak")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "sk")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "ap-south-1")
	c, err := NewS3(context.Background(), S3Config{Bucket: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("nil client")
	}
	_ = os.Unsetenv("AWS_DEFAULT_REGION")
}
