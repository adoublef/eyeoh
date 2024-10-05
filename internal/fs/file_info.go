package fs

import (
	"time"

	"github.com/google/uuid"
	olog "go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
)

const scopeName = "go.adoublef/up/internal/fs"

var (
	tracer = otel.Tracer(scopeName)
	_      = otel.Meter(scopeName)
	_      = olog.NewLogger(scopeName)
)

type FileInfo struct {
	ID      uuid.UUID
	Ref     uuid.UUID
	Name    Name
	Size    int64
	ModTime time.Time
	IsDir   bool
}

type DirEntry struct {
	Path string
	FileInfo
}

type dirEntry struct {
	id    uuid.UUID
	root  *uuid.UUID // can be null
	name  Name       // using Name
	ref   *uuid.UUID
	sz    *int64 // null for directories
	modAt time.Time
	v     uint64
}
