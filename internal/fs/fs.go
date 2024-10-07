package fs

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"

	"github.com/google/uuid"
	"go.adoublef/eyeoh/internal/runtime/debug"
)

var (
	ErrOpenFile = errors.New("cannot open file")
)

type Uploader interface {
	Upload(ctx context.Context, r io.Reader) (id uuid.UUID, sz int64, err error)
}
type Downloader interface {
	Download(ctx context.Context, id uuid.UUID) (rc io.ReadCloser, mime string, err error)
}
type FS struct {
	*DB
	Uploader
	Downloader
}

func (fsys *FS) Create(ctx context.Context, filename Name, r io.Reader, parent uuid.UUID) (file uuid.UUID, err error) {
	file, err = fsys.Touch(ctx, filename, parent)
	if err != nil {
		return uuid.Nil, err
	}
	// what if the s3 goes down shortly
	// we must rollback the request that created the header
	// but it too could go down, so need something like temporal
	// to figure it out for us. add this next
	h := sha256.New()
	tr := io.TeeReader(r, h)
	// seek to find out the content type may not work with encryption?
	ref, sz, err := fsys.Upload(ctx, tr)
	if err != nil {
		return uuid.Nil, err
	}
	debug.Printf(`ref, %d, err := fsys.Upload(ctx, tr)`, sz)
	if err := fsys.Cat(ctx, ref, sz, h.Sum(nil), file, 0); err != nil {
		return uuid.Nil, err
	}
	return file, nil
}

func (fsys *FS) Open(ctx context.Context, file uuid.UUID) (f *File, mime string, etag Etag, err error) {
	fi, _, sha, err := fsys.Stat(ctx, file)
	if err != nil {
		return nil, "", nil, err
	}
	// does the stdlib allow opening a directory or can only read?
	// any id seems to allow this to be pass, which is wrong.
	// for folders I can just check with [FS.Stat] but
	// I should check that an error can be returned at this level
	// we need to cause a block
	if fi.IsDir {
		// return a file that has no body
		// let the caller determine what they want to do with this.
		return &File{ReadCloser: io.NopCloser(nil), Info: fi}, "inode/directory", nil, nil
	}
	rc, mime, err := fsys.Download(ctx, fi.Ref)
	if err != nil {
		return nil, "", nil, err
	}
	return &File{ReadCloser: rc, Info: fi}, mime, sha, nil
}

type Cursor struct {
	Next uuid.UUID
}

func (c Cursor) String() string {
	return "next:" + c.Next.String()
}

func (c Cursor) Base64() string {
	// serialize as a string
	// [uuid],[...]
	s := c.Next.String()
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func Parse(s string) (Cursor, error) {
	p, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, err
	}
	// [uuid],[...]
	// for now its just [uuid]
	uid, err := uuid.FromBytes(p)
	if err != nil {
		return Cursor{}, err
	}
	return Cursor{Next: uid}, nil
}
