package fs

import (
	"encoding/hex"
	"io"
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

type File struct {
	io.ReadCloser
	Info FileInfo
}

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

type Etag []byte

func (e Etag) String() string {
	return hex.EncodeToString(e)
}

type dirEntry struct {
	//lint:ignore U1000 ignore this field for now
	id uuid.UUID
	//lint:ignore U1000 ignore this field for now
	root  *uuid.UUID // can be null
	name  Name       // using Name
	modAt time.Time
	v     uint64
}

type blobData struct {
	id *uuid.UUID // should this be a reference?
	// v     uint64
	sz *int64 // null for directories
	//lint:ignore U1000 ignore this field for now
	modAt time.Time
	sha   []byte // for files
}
