package fs

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	dberrors "go.adoublef/up/internal/database/errors"
	"go.adoublef/up/internal/runtime/debug"
)

func Error(err error) error {
	if err == nil {
		return nil
	}
	if err == pgx.ErrNoRows {
		return fmt.Errorf("fs: no entry for file: %w", dberrors.ErrNotExist)
	}
	debug.Printf(`fs: %T, %v := err`, err, err)
	return err
}
