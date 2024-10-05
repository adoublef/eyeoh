package fs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.adoublef/up/internal/database/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type DB struct {
	RWC *pgxpool.Pool
}

func (d *DB) Touch(ctx context.Context, name Name, root uuid.UUID) (file uuid.UUID, err error) {
	file, err = uuid.NewV7()
	if err != nil {
		return uuid.Nil, Error(err)
	}
	const query = `insert into up.fs (id, name, root) values ($1, $2, $3)`

	attr := trace.WithAttributes(
		attribute.String("sql.query", query),
		attribute.String("file.id", file.String()),
		attribute.String("file.root", root.String()),
	)
	ctx, span := tracer.Start(ctx, "DB.Touch", attr)
	defer span.End()

	_, err = d.RWC.Exec(ctx, query, file, name, ptr(root))
	if err != nil {
		return uuid.Nil, Error(err)
	}
	return file, nil
}

func (d *DB) Cat(ctx context.Context, ref uuid.UUID, sz int64, file uuid.UUID, v uint64) error {
	const query = `update up.fs
set ref = $1, sz = $2, mod_at = now(), v = v + 1
where id = $3 and v = $4`

	attr := trace.WithAttributes(
		attribute.String("sql.query", query),
		attribute.String("file.id", file.String()),
		attribute.Int("file.v", int(v)),
		attribute.String("file.ref", ref.String()),
		attribute.Int("file.sz", int(sz)),
	)
	ctx, span := tracer.Start(ctx, "DB.Cat", attr)
	defer span.End()

	cmd, err := d.RWC.Exec(ctx, query, ref, sz, file, v)
	if err != nil {
		return Error(err)
	}
	return mustRowsAffected(cmd)
}

func (d *DB) Stat(ctx context.Context, file uuid.UUID) (info FileInfo, v uint64, err error) {
	const query = `select f.name
, f.ref
, f.sz
, f.mod_at
, f.v
from up.fs f where id = $1`
	attr := trace.WithAttributes(
		attribute.String("sql.query", query),
		attribute.String("file.id", file.String()),
	)
	ctx, span := tracer.Start(ctx, "DB.Stat", attr)
	defer span.End()

	var found dirEntry
	if err = d.RWC.QueryRow(ctx, query, file).Scan(
		&found.name,
		&found.ref,
		&found.sz,
		&found.modAt,
		&found.v,
	); err != nil {
		return FileInfo{}, 0, Error(err)
	}
	fi := FileInfo{
		ID:      file,
		Ref:     value(found.ref),
		Name:    found.name,
		Size:    value(found.sz),
		ModTime: found.modAt,
		IsDir:   false, // todo: fix
	}
	return fi, found.v, nil
}

func (d *DB) Mkdir(ctx context.Context, name Name, root uuid.UUID) (file uuid.UUID, err error) {
	file, err = uuid.NewV7()
	if err != nil {
		return uuid.Nil, Error(err)
	}
	const query = `insert into up.fs (id, name, root, sz) values ($1, $2, $3, null)`
	attr := trace.WithAttributes(
		attribute.String("sql.query", query),
		attribute.String("file.id", file.String()),
		attribute.String("file.root", root.String()),
	)
	ctx, span := tracer.Start(ctx, "DB.Mkdir", attr)
	defer span.End()

	_, err = d.RWC.Exec(ctx, query, file, name, ptr(root))
	if err != nil {
		return uuid.Nil, Error(err)
	}
	return file, nil
}

func ptr[V comparable](v V) *V {
	if z := *new(V); v == z {
		return nil
	}
	return &v
}

func value[V comparable](v *V) V {
	if v == nil {
		return *new(V)
	}
	return *v
}

func mustRowsAffected(cmd pgconn.CommandTag) error {
	// if update and no affect then assume not found
	if cmd.Update() && cmd.RowsAffected() < 1 {
		return errors.ErrNotExist
	}
	// todo: if delete and no affect then assume not found
	return nil
}
