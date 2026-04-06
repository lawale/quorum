//go:build console

package console

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed frontend/dist/*
var assets embed.FS

// Handler returns an http.Handler that serves the embedded SPA.
// It serves static files from the dist directory and falls back
// to index.html for client-side routing.
func Handler() http.Handler {
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		panic("console: failed to sub embed.FS: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the exact file first
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check if the file exists in the embedded FS
		if f, err := dist.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
