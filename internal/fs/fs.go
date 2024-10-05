package fs

import (
	"context"
	"io"

	"github.com/google/uuid"
)

type Uploader interface {
	Upload(ctx context.Context, r io.Reader) (id uuid.UUID, sz int64, err error)
	Download(ctx context.Context, id uuid.UUID) (rc io.ReadCloser, mime string, err error)
}

type FS struct {
	*DB
	Uploader
}
