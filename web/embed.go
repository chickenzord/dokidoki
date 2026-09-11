package web

import (
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded frontend files
// from the dist directory, with fallback to index.html for client-side routing.
//
// If the embedded dist is missing the SPA entry point (index.html) — typically
// because someone ran `go build` without first running `bun run build` — a
// warning is logged at startup.
func Handler() http.Handler {
	distSub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}

	if _, statErr := fs.Stat(distSub, "index.html"); statErr != nil {
		log.Println("warning: embedded web/dist has no index.html — run `make build-ui` (or `bun run build` in web/) before `go build` to bundle the frontend")
	}

	return HandlerFromFS(distSub)
}

// HandlerFromFS returns an http.Handler that serves files from fsys with
// fallback to the root file for SPA client-side routing. Useful for tests
// that want to inject an in-memory fs.FS instead of relying on the embedded
// dist (which may not exist when `go build` is run without `bun run build`).
func HandlerFromFS(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if the requested file exists in the filesystem
		f, err := fsys.Open(path)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to root file for SPA client-side routing
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}
