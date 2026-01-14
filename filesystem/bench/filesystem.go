package bench

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"

	wd "github.com/bornholm/calli/pkg/webdav"
	"github.com/bornholm/go-webdav"
)

type filesystemBenchmark struct {
	Name string
	Run  func(b *testing.B, fs webdav.FileSystem)
}

var filesystemBenchmarks = []filesystemBenchmark{
	{
		Name: "ConcurrentWrites_5MB",
		Run: func(b *testing.B, fs webdav.FileSystem) {
			fileSize := 1 * 1024 * 1024 // 1MB per worker
			data := make([]byte, fileSize)
			rand.Read(data)

			// Atomic counter for unique filenames across goroutines
			var counter int64

			b.SetBytes(int64(fileSize))
			b.ResetTimer()

			// b.RunParallel creates multiple goroutines (GOMAXPROCS by default)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					id := atomic.AddInt64(&counter, 1)
					filename := fmt.Sprintf("/bench_write_concurrent_par_%d.bin", id)
					ctx := context.Background()

					f, err := fs.OpenFile(ctx, filename, os.O_RDWR|os.O_CREATE, 0644)
					if err != nil {
						b.Fatalf("%+v", err)
					}

					// We use a predefined buffer to test IO speed, not memory allocation
					_, err = f.Write(data)
					if err != nil {
						b.Fatalf("%+v", err)
					}

					if err := f.Close(); err != nil {
						b.Fatalf("%+v", err)
					}
				}
			})
		},
	},
	{
		Name: "Read_1MB",
		Run: func(b *testing.B, fs webdav.FileSystem) {
			filename := "/bench_read_source.bin"
			fileSize := 1 * 1024 * 1024

			// Setup: Create file ONCE
			ctx := context.Background()
			f, _ := fs.OpenFile(ctx, filename, os.O_RDWR|os.O_CREATE, 0644)
			data := make([]byte, fileSize)
			f.Write(data)
			f.Close()

			b.SetBytes(int64(fileSize))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				f, err := fs.OpenFile(ctx, filename, os.O_RDONLY, 0644)
				if err != nil {
					b.Fatal(err)
				}

				// Discard data to measure raw throughput
				if _, err := io.Copy(io.Discard, f); err != nil {
					b.Fatal(err)
				}

				f.Close()
			}
		},
	},
	{
		Name: "Write_100MB",
		Run: func(b *testing.B, fs webdav.FileSystem) {
			fileSize := 100 * 1024 * 1024 // 100MB per worker
			data := make([]byte, fileSize)
			rand.Read(data)

			// Atomic counter for unique filenames across goroutines
			var counter int64

			b.SetBytes(int64(fileSize))
			b.ResetTimer()

			// b.RunParallel creates multiple goroutines (GOMAXPROCS by default)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					id := atomic.AddInt64(&counter, 1)
					filename := fmt.Sprintf("/bench_write_100_par_%d.bin", id)
					ctx := context.Background()

					f, err := fs.OpenFile(ctx, filename, os.O_RDWR|os.O_CREATE, 0644)
					if err != nil {
						b.Fatal(err)
					}

					// We use a predefined buffer to test IO speed, not memory allocation
					_, err = f.Write(data)
					if err != nil {
						b.Fatal(err)
					}

					if err := f.Close(); err != nil {
						b.Fatal(err)
					}
				}
			})
		},
	},
}

func RunTestSuite(b *testing.B, fs webdav.FileSystem) {
	fs = wd.WithLogger(fs, slog.Default())

	for _, bc := range filesystemBenchmarks {
		b.Run(bc.Name, func(b *testing.B) {
			bc.Run(b, fs)
		})
	}
}
