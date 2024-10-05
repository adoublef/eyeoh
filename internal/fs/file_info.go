package fs

import (
	"time"

	"github.com/google/uuid"
	olog "go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
)

const scopeName = "go.adoublef/eyeoh/internal/fs"

var (
	tracer = otel.Tracer(scopeName)
	_      = otel.Meter(scopeName)
	_      = olog.NewLogger(scopeName)
)

type FileInfo struct {
	ID      uuid.UUID `json:"fileId"`
	Ref     uuid.UUID `json:"-"`
	Name    Name      `json:"filename"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modifiedAt"`
	IsDir   bool      `json:"isDir"`
}

type DirEntry struct {
	Path string
	FileInfo
}

type dirEntry struct {
	//lint:ignore U1000 ignore this field for now
	id uuid.UUID
	//lint:ignore U1000 ignore this field for now
	root  *uuid.UUID // can be null
	name  Name       // using Name
	ref   *uuid.UUID
	sz    *int64 // null for directories
	modAt time.Time
	v     uint64
}
