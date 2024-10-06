// TODO: mv (rename and multiple files under a new directory location), cp (single file or many under a new directory location), rm (single file or many)
package fs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.adoublef/eyeoh/internal/database/errors"
	"go.adoublef/eyeoh/internal/runtime/debug"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type DB struct {
	RWC *pgxpool.Pool
}

// Touch attempts to create a new [DirEntry] for a file. If root is set, the file is nested.
func (d *DB) Touch(ctx context.Context, name Name, root uuid.UUID) (file uuid.UUID, err error) {
	file, err = uuid.NewV7()
	if err != nil {
		return uuid.Nil, Error(err)
	}
	const query = `insert into fs.dir_entry (id, name, root) values ($1, $2, $3)`

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

// Cat updates [FileInfo.Ref] and [FileInfo.Size]. The version of the file enables safe mutli-user modifications.
func (d *DB) Cat(ctx context.Context, ref uuid.UUID, sz int64, sha []byte, file uuid.UUID, v uint64) error {
	const query = `
with dir_entry as (
	update fs.dir_entry
	set v = v + 1, mod_at = now()
	where id = $1 and v = $2
	returning id, mod_at, v)
insert into fs.blob_data (id, dir_entry, sz, sha, mod_at, v)
values ($3
	, (select id from dir_entry)
	, $4
	, $5
	, (select mod_at from dir_entry)
	, (select v from dir_entry))
`

	attr := trace.WithAttributes(
		attribute.String("sql.query", query),
		attribute.String("file.id", file.String()),
		attribute.Int("file.v", int(v)),
		attribute.String("file.ref", ref.String()),
		attribute.Int("file.sz", int(sz)),
	)
	ctx, span := tracer.Start(ctx, "DB.Cat", attr)
	defer span.End()

	cmd, err := d.RWC.Exec(ctx, query, file, v, ref, sz, sha)
	if err != nil {
		return Error(err)
	}
	return mustRowsAffected(cmd)
}

// Stat return [FileInfo] if successful, else returns an error.
func (d *DB) Stat(ctx context.Context, file uuid.UUID) (info FileInfo, v uint64, etag Etag, err error) {
	const query = `select 
	f.name
	, b.id
	, b.sz
	, f.mod_at
	, f.v
	, b.sha
from fs.dir_entry f left join fs.blob_data b
on f.id = b.dir_entry where f.id = $1
limit 1` // what if there are many blobs

	attr := trace.WithAttributes(
		attribute.String("sql.query", query),
		attribute.String("file.id", file.String()),
	)
	ctx, span := tracer.Start(ctx, "DB.Stat", attr)
	defer span.End()

	var de dirEntry
	var bd blobData // make this a pointer instead?
	if err = d.RWC.QueryRow(ctx, query, file).Scan(
		&de.name,
		&bd.id,
		&bd.sz,
		&de.modAt,
		&de.v,
		&bd.sha,
	); err != nil {
		return FileInfo{}, 0, nil, Error(err)
	}
	// using b instead?
	debug.Printf(`%d = len(bd.sha)`, len(bd.sha))
	debug.Printf(`%v = bd.sha`, (bd.sha))
	fi := FileInfo{
		ID:      file,
		Ref:     value(bd.id),
		Name:    de.name,
		Size:    value(bd.sz),
		ModTime: de.modAt,
		IsDir:   bd.sz == nil, // todo: maybe check if blobdata exists instead
	}
	// URLEncoding version?
	return fi, de.v, bd.sha, nil
}

// Mkdir attempts to create a new [DirEntry] for a directory. If root is not nil, the directory is nested.
func (d *DB) Mkdir(ctx context.Context, name Name, root uuid.UUID) (file uuid.UUID, err error) {
	file, err = uuid.NewV7()
	if err != nil {
		return uuid.Nil, Error(err)
	}
	const query = `insert into fs.dir_entry (id, name, root) values ($1, $2, $3)`

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
