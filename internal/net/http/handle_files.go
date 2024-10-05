package http

import (
	"cmp"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"go.adoublef/eyeoh/internal/fs"
	"go.adoublef/eyeoh/internal/runtime/debug"
)

func handleFileUpload(fsys *fs.FS) http.HandlerFunc {
	var unsupportedMediaType = statusHandler{http.StatusUnsupportedMediaType, `request is not a mulitpart/form`}
	var badParentID = statusHandler{http.StatusUnsupportedMediaType, `parent id has invalid format`}
	var unprocessableEntity = func(format string, v ...any) statusHandler {
		return statusHandler{http.StatusUnprocessableEntity, fmt.Sprintf(format, v...)}
	}

	type upload struct {
		ID string `json:"fileId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "handleFileUpload")
		defer span.End()

		// parse root directory if present in the query param
		// could use form data input but being lazy
		parent, err := uuid.Parse(cmp.Or(r.URL.Query().Get("parent"), uuid.Nil.String()))
		if err != nil {
			badParentID.ServeHTTP(w, r)
			return
		}

		mr, err := r.MultipartReader()
		if err != nil {
			unsupportedMediaType.ServeHTTP(w, r)
			return
		}
		part, err := mr.NextPart()
		if err != nil {
			unprocessableEntity("failed to decode part: %v", err).ServeHTTP(w, r)
			return
		}
		defer part.Close()
		// validate filename/formname
		// if not then use nil and set
		filename, err := fs.ParseName(part.FileName())
		if err != nil {
			Error(w, r, err)
			return
		}
		file, err := fsys.Touch(ctx, filename, parent)
		if err != nil {
			Error(w, r, err)
			return
		}
		// what if the s3 goes down shortly
		// we must rollback the request that created the header
		// but it too could go down, so need something like temporal
		// to figure it out for us. add this next
		ref, sz, err := fsys.Upload(ctx, part)
		if err != nil {
			Error(w, r, err)
			return
		}
		if err := fsys.Cat(ctx, ref, sz, file, 0); err != nil {
			Error(w, r, err)
			return
		}
		// add size
		// todo: render function
		c := upload{
			ID: file.String(),
		}
		respond(w, r, c)
	}
}

func handleDownloadFile(fsys *fs.FS) http.HandlerFunc {
	var badPathValue = statusHandler{http.StatusBadRequest, `file id in path has invalid format`}
	var forbiddenFile = statusHandler{http.StatusForbidden, "file is a directory"}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "handleDownloadFile")
		defer span.End()

		// query for attachment (default) or inline
		// "type;filename=somefile.ext"

		file, err := uuid.Parse(r.PathValue("file"))
		if err != nil {
			badPathValue.ServeHTTP(w, r)
			return
		}

		fi, _, err := fsys.Stat(ctx, file)
		if err != nil {
			Error(w, r, err)
			return
		}
		// any id seems to allow this to be pass, which is wrong.
		// for folders I can just check with [*fs.FS.Stat] but
		// I should check that an error can be returned at this level
		// we need to cause a block
		if fi.IsDir {
			forbiddenFile.ServeHTTP(w, r)
			return
		}
		rc, mime, err := fsys.Download(ctx, fi.Ref)
		if err != nil {
			Error(w, r, err)
			return
		}
		defer rc.Close()
		debug.Printf(`rc, %q, %v := fsys.Download(ctx, %q)`, err, mime, fi.ID)
		// return this to the user as attatchment or inline?
		// serveContent Headers
		// 1. last-modified
		// 1. pre-conditions
		// 1. content-type
		// 1. content-range (ordered?)
		// 1. accept-ranges
		// 1. content-encoding
		// 1. content-length - w.Header().Set("Content-Length", strconv.FormatInt(sendSize, 10))
		// check "HEAD"
		// io.CopyN(w, sendContent, sendSize)
		// if I serve a range should omit 'disposition'
		// see: https://stackoverflow.com/a/1401619/4239443
		// normal encoding: Content-Disposition: attachment; filename="filename.jpg"
		// special encoding (RFC 5987): Content-Disposition: attachment; filename*="filename.jpg"
		if r.Method != http.MethodHead {
			io.CopyN(w, rc, fi.Size)
		}
	}
}

func handleCreateFolder(fsys *fs.FS) http.HandlerFunc {
	type create struct {
		Root uuid.UUID `json:"parentId"`
		Name fs.Name   `json:"name"`
	}
	parse := func(w http.ResponseWriter, r *http.Request) (uuid.UUID, fs.Name, error) {
		c, err := Decode[create](w, r, 0, 0)
		return c.Root, c.Name, err
	}

	type folder struct {
		ID string `json:"folderId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "handleCreateFolder")
		defer span.End()

		root, name, err := parse(w, r)
		if err != nil {
			Error(w, r, err)
			return
		}

		file, err := fsys.Mkdir(ctx, name, root)
		if err != nil {
			Error(w, r, err)
			return
		}

		f := folder{
			ID: file.String(),
		}
		respond(w, r, f)
	}
}

func handleFileInfo(fsys *fs.FS) http.HandlerFunc {
	var badPathValue = statusHandler{
		code: http.StatusBadRequest,
		s:    `file id in path has invalid format`,
	}

	type stat struct {
		fs.FileInfo        // inline
		Version     uint64 `json:"version"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "handleFileInfo")
		defer span.End()

		file, err := uuid.Parse(r.PathValue("file"))
		if err != nil {
			badPathValue.ServeHTTP(w, r)
			return
		}
		info, v, err := fsys.Stat(ctx, file)
		if err != nil {
			Error(w, r, err)
			return
		}

		st := stat{
			FileInfo: info,
			Version:  v,
		}
		respond(w, r, st)
	}
}
