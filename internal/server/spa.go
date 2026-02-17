package server

import (
	"io/fs"
	"net/http"
	"strings"
)

// spaFileServer serves static files from an fs.FS, falling back to index.html
// for any path that doesn't match a real file. This supports SvelteKit's SPA
// mode where client-side routing handles /login, /dashboard, etc.
func spaFileServer(assets fs.FS) http.Handler {
	fileServer := http.FileServerFS(assets)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path — strip leading slash for fs.FS lookup.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check if the file exists in the embedded filesystem.
		if _, err := fs.Stat(assets, path); err != nil {
			// File doesn't exist — serve index.html for SPA routing.
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}
