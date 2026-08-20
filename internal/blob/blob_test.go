package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestHTTPKeyAndSource(t *testing.T) {
	key := HTTPKey("captures", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC))
	if key != "captures/2026/08/12/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.tar.gz" {
		t.Fatal(key)
	}
	if SourceForKey(key) != "http" {
		t.Fatal(SourceForKey(key))
	}
	if SourceForKey("captures/cluster-run.tar.gz") != "s3" {
		t.Fatal("foreign key should be s3")
	}
	sftp := SFTPKey("captures", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC))
	if sftp != "captures/sftp/2026/08/19/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.tar.gz" {
		t.Fatal(sftp)
	}
	if SourceForKey(sftp) != "sftp" {
		t.Fatal(SourceForKey(sftp))
	}
	if NormalizePrefix("") != "captures/" || NormalizePrefix("captures") != "captures/" {
		t.Fatal(NormalizePrefix("captures"))
	}
	if !UnderPrefix("captures/", "captures/a.tar.gz") {
		t.Fatal("in-prefix")
	}
	if UnderPrefix("captures/", "other/a.tar.gz") || UnderPrefix("captures/", "captures/../x") || UnderPrefix("captures/", "") {
		t.Fatal("out-of-prefix must fail")
	}
}

func TestMemoryPutGetListHead(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	if err := m.Put(ctx, "captures/a.tar.gz", strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	rc, obj, err := m.Get(ctx, "captures/a.tar.gz")
	if err != nil || obj.Size != 5 {
		t.Fatalf("%+v %v", obj, err)
	}
	b, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil || string(b) != "hello" {
		t.Fatalf("%q %v", b, err)
	}
	if _, err := m.Head(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	list, err := m.List(ctx, "captures/")
	if err != nil || len(list) != 1 {
		t.Fatalf("%+v %v", list, err)
	}
	if err := m.Delete(ctx, "captures/a.tar.gz"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Head(ctx, "captures/a.tar.gz"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	m.HeadErr = fmt.Errorf("network")
	if _, err := m.Head(ctx, "anything"); err == nil || err.Error() != "network" {
		t.Fatalf("HeadErr: %v", err)
	}
}

func TestMemoryFailPutAndHeadBucket(t *testing.T) {
	m := NewMemory()
	m.FailPuts = true
	if err := m.Put(context.Background(), "k", strings.NewReader("x")); err == nil {
		t.Fatal("expected put failure")
	}
	m.FailHeadBucket = true
	if err := m.HeadBucket(context.Background()); err == nil {
		t.Fatal("expected head bucket failure")
	}
}

func TestNewS3RequiresBucket(t *testing.T) {
	_, err := NewS3(context.Background(), S3Config{})
	if err == nil || !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("err %v", err)
	}
}

func TestApplyEndpointPathStyle(t *testing.T) {
	var o s3.Options
	applyEndpoint(&o, "https://eu2.contabo.com", true)
	if o.BaseEndpoint == nil || *o.BaseEndpoint != "https://eu2.contabo.com" {
		t.Fatalf("endpoint %+v", o.BaseEndpoint)
	}
	if !o.UsePathStyle {
		t.Fatal("path-style")
	}
	var awsDefault s3.Options
	applyEndpoint(&awsDefault, "", false)
	if awsDefault.BaseEndpoint != nil || awsDefault.UsePathStyle {
		t.Fatalf("aws default %+v", awsDefault)
	}
}
