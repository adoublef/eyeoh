package blob

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"go.adoublef/eyeoh/internal/runtime/debug"
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
	case errors.As(err, new(*smithy.OperationError)):
		// APIError
		oe := err.(*smithy.OperationError)
		debug.Printf(`%#v = oe.Unwrap()`, oe.Unwrap())
	}
	return err
}

// Helper function to check AWS error types
func isAwsError(err error, errorCode string) bool {
	var oe *smithy.OperationError
	if errors.As(err, &oe) {
		var ae smithy.APIError
		if errors.As(oe.Err, &ae) {
			return ae.ErrorCode() == errorCode
		}
	}
	return false
}
