package http

import (
	"cmp"
	"fmt"
	"io"
	"net/http"
	"strconv"

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
		ctx, span := tracer.Start(r.Context(), "http.file_upload")
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
		file, err := fsys.Create(ctx, filename, part, parent)
		if err != nil {
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

func handleFileDownload(fsys *fs.FS) http.HandlerFunc {
	var badPathValue = statusHandler{http.StatusBadRequest, `file id in path has invalid format`}
	var forbiddenFile = statusHandler{http.StatusForbidden, "file is a directory"}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "http.file_download")
		defer span.End()

		// query for attachment (default) or inline
		// "type;filename=somefile.ext"

		file, err := uuid.Parse(r.PathValue("file"))
		if err != nil {
			badPathValue.ServeHTTP(w, r)
			return
		}

		f, mime, etag, err := fsys.Open(ctx, file)
		if err != nil {
			Error(w, r, err)
			return
		}
		defer f.Close() // be sure this wont panic for directories
		if f.Info.IsDir {
			forbiddenFile.ServeHTTP(w, r)
			return
		}
		debug.Printf(`rc, %q, %v := fsys.Download(ctx, %q)`, err, mime, file)
		// if len(etag) > 0 { // directory won't have an etag
		w.Header().Set("ETag", strconv.Quote(etag.String()))
		// }
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
			io.CopyN(w, f, f.Info.Size)
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
		ctx, span := tracer.Start(r.Context(), "http.create_folder")
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
		ETag        string `json:"etag,omitempty"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "http.file_info")
		defer span.End()

		file, err := uuid.Parse(r.PathValue("file"))
		if err != nil {
			badPathValue.ServeHTTP(w, r)
			return
		}
		info, v, etag, err := fsys.Stat(ctx, file)
		if err != nil {
			Error(w, r, err)
			return
		}

		st := stat{
			FileInfo: info,
			Version:  v,
			ETag:     etag.String(),
		}
		respond(w, r, st)
	}
}

func handleFileRename(fsys *fs.FS) http.HandlerFunc {
	type rename struct {
		Name    fs.Name `json:"name"`
		Version uint64  `json:"revision"`
	}
	parse := func(w http.ResponseWriter, r *http.Request) (uuid.UUID, uint64, fs.Name, error) {
		// get path name
		file, err := uuid.Parse(r.PathValue("file"))
		if err != nil {
			return uuid.Nil, 0, "", err
		} // proper status error
		c, err := Decode[rename](w, r, 0, 0)
		return file, c.Version, c.Name, err
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "http.file_rename")
		defer span.End()

		file, v, name, err := parse(w, r)
		if err != nil {
			Error(w, r, err) // todo: fix errors
			return
		}

		err = fsys.Mv(ctx, name, file, v)
		if err != nil {
			Error(w, r, err) // todo: fix errors
			return
		}

		// todo: json payload may be good?
		w.WriteHeader(http.StatusNoContent)
	}
}
