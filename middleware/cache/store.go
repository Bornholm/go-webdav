package cache

import (
	"context"
	"os"
)

type Store interface {
	Get(ctx context.Context, path string) (os.FileInfo, bool, error)
	Put(ctx context.Context, path string, info os.FileInfo) error
	Invalidate(ctx context.Context, path string) error
	GetChildren(ctx context.Context, path string) ([]os.FileInfo, bool, error)
	PutChildren(ctx context.Context, path string, children []os.FileInfo) error
	InvalidateChildren(ctx context.Context, path string) error
}
