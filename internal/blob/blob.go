package blob

import (
	"io"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
)

type Client struct {
	*Uploader
	*Downloader
}

type s3Client interface {
	manager.UploadAPIClient
	manager.DownloadAPIClient
}

// New returns a new [Client]
func New(bucket string, c s3Client) *Client {
	return &Client{
		Uploader:   NewUploader(bucket, c),
		Downloader: NewDownloader(bucket, c),
	}
}

type countReader struct {
	n atomic.Int64
	r io.Reader
}

func (c *countReader) Read(p []byte) (int, error) {
	nr, err := c.r.Read(p)
	if nr > 0 {
		c.n.Add(int64(nr))
	}
	return nr, err
}

type writerAt struct {
	io.Writer
}

func (w *writerAt) WriteAt(p []byte, _ int64) (n int, err error) {
	// ignore 'offset' because we forced sequential downloads
	return w.Writer.Write(p)
}
