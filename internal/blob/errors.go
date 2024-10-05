package blob

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	ErrNotExist = errors.New("does not exist")
)

func Error(err error) error {
	// https://aws.github.io/aws-sdk-go-v2/docs/handling-errors/
	if err == nil {
		return nil
	}
	// handling aws errors is so stupidly annoying
	switch {
	case errors.As(err, new(*types.NoSuchKey)):
		return ErrNotExist
	}
	return err
}
