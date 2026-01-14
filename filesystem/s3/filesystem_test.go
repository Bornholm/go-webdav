package s3

import (
	"context"
	"testing"

	"github.com/bornholm/go-webdav"
	"github.com/bornholm/go-webdav/filesystem/bench"
	"github.com/bornholm/go-webdav/filesystem/testsuite"
	"github.com/bornholm/go-webdav/litmus"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	testminio "github.com/testcontainers/testcontainers-go/modules/minio"
)

func TestFileSystem(t *testing.T) {
	fs, close := createFilesystem(t)
	defer close()

	testsuite.TestFileSystem(t, fs)
}

func TestLitmus(t *testing.T) {
	fs, close := createFilesystem(t)
	defer close()

	litmus.RunTestSuite(t, fs)
}

func BenchmarkFileSystem(b *testing.B) {
	fs, close := createFilesystem(b)
	defer close()

	bench.RunTestSuite(b, fs)
}

func createFilesystem(t testing.TB) (webdav.FileSystem, func()) {
	ctx := context.Background()

	const (
		minioUsername = "miniousername"
		minioPassword = "miniopassword"
	)

	minioContainer, err := testminio.Run(
		ctx, "minio/minio:RELEASE.2024-01-16T16-07-38Z",
		testminio.WithUsername(minioUsername),
		testminio.WithPassword(minioPassword),
	)

	if err != nil {
		t.Fatalf("failed to start container: %+v", errors.WithStack(err))
	}

	endpoint, err := minioContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("could not retrieve connection string: %+v", errors.WithStack(err))
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioUsername, minioPassword, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("failed to create minio client: %+v", errors.WithStack(err))
	}

	const (
		bucketName = "webdav"
	)

	if err := client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("failed to create minio bucket: %+v", errors.WithStack(err))
	}

	close := func() {
		if err := testcontainers.TerminateContainer(minioContainer); err != nil {
			t.Fatalf("failed to terminate container: %+v", errors.WithStack(err))
		}
	}

	return NewFileSystem(client, bucketName), close
}
