package web

import (
	"embed"
	"errors"
	"io/fs"
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
