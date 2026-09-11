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
func Handler() http.Handler {
	distSub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}

	// Warn if dist appears empty (frontend not built). `go build` succeeds
	// even when only the .gitkeep placeholder is present; check for the
	// SPA entry point (index.html) to detect a missed `bun run build`.
	if _, statErr := fs.Stat(distSub, "index.html"); statErr != nil {
		log.Println("warning: embedded web/dist has no index.html — run `make build-ui` (or `bun run build` in web/) before `go build` to bundle the frontend")
	}

	fileServer := http.FileServer(http.FS(distSub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if the requested file exists in embedded filesystem
		f, err := distSub.Open(path)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA client-side routing
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}
