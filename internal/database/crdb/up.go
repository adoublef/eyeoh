package crdb

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"maragu.dev/migrate"
)

//go:embed all:migrations/*.sql
var embedFS embed.FS

// Up executes a database migration for testing.
func Up(ctx context.Context, url string) error {
	fsys, err := fs.Sub(embedFS, "migrations")
	if err != nil {
		return fmt.Errorf("crdb: failed to open migrations sub directory: %v", err)
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("crdb: failed to connect to %s: %v", url, err)
	}
	defer db.Close()
	if err := migrate.Up(ctx, db, fsys); err != nil {
		return fmt.Errorf("crdb: failed to run migrations against %s: %v", url, err)
	}
	return nil
}

// Down executes commands to reset the database.
func Down(ctx context.Context, url string) error {
	fsys, err := fs.Sub(embedFS, "migrations")
	if err != nil {
		return fmt.Errorf("crdb: failed to open migrations sub directory: %v", err)
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("crdb: failed to connect to %s: %v", url, err)
	}
	defer db.Close()
	if err := migrate.Down(ctx, db, fsys); err != nil {
		return fmt.Errorf("crdb: failed to run migrations against %s: %v", url, err)
	}
	return nil
}
