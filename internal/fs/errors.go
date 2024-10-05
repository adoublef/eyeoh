package fs

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	dberrors "go.adoublef/eyeoh/internal/database/errors"
	"go.adoublef/eyeoh/internal/runtime/debug"
)

func Error(err error) error {
	if err == nil {
		return nil
	}
	if err == pgx.ErrNoRows {
		return fmt.Errorf("fs: no entry for file: %w", dberrors.ErrNotExist)
	}
	switch {
	case errors.As(err, new(*pgconn.PgError)):
		switch pe := err.(*pgconn.PgError); pe.Code {
		case "23505": // unique constraint (i.e. fs_expr_name_key)
			return fmt.Errorf("file name taken: %w", dberrors.ErrExist)
		}
	}
	debug.Printf(`fs: %T, %v := err`, err, err)
	return err
}
