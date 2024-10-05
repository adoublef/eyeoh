package blob

import (
	"context"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"go.adoublef/eyeoh/internal/runtime/debug"
)

type Uploader struct {
	bucket string
	// using the manager util for now
	m *manager.Uploader
}

func (u Uploader) Upload(ctx context.Context, r io.Reader) (id uuid.UUID, sz int64, err error) {
	id, err = uuid.NewV7()
	if err != nil {
		return uuid.Nil, 0, err
	}
	// https://stackoverflow.com/questions/44852649/evenly-spread-files-in-directories-using-uuid-splits
	// given a uuid, create a 2-level directory
	// uuid does not _need_ to be sortable
	// 01/23/456789...
	s := strings.Replace(id.String(), "-", "", 4)
	uri := path.Join("_blob", s[:2], s[2:4], s[4:])
	cr := &countReader{r: r}
	in := &s3.PutObjectInput{
		Key:    &uri,
		Bucket: &u.bucket,
		Body:   cr,
	}
	out, err := u.m.Upload(ctx, in)
	if err != nil {
		return uuid.Nil, 0, err
	}
	debug.Printf("%v := out.ETag", out.ETag)
	return id, cr.n.Load(), nil
}

func NewUploader(bucket string, c manager.UploadAPIClient) *Uploader {
	o := func(u *manager.Uploader) { u.PartSize = 1 << 24 }
	u := manager.NewUploader(c, o)
	return &Uploader{bucket, u}
}
