// Package web provides embedded web UI assets and HTTP handlers for the React frontend.
package web

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
)

//go:embed all:dist
var distFiles embed.FS

// GetHandler returns an HTTP handler that serves the embedded React SPA
func GetHandler() http.Handler {
	// Get the dist subdirectory from the embedded filesystem
	dist, err := fs.Sub(distFiles, "dist")
	if err != nil {
		// If dist doesn't exist (development), return a simple handler
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			//nolint:errcheck // Error writing to response writer is not critical
			_, _ = w.Write([]byte("Web UI not built. Run: cd web && npm install && npm run build"))
		})
	}

	return spaHandler{fileSystem: dist}
}

// spaHandler implements http.Handler to serve a SPA with fallback to index.html
type spaHandler struct {
	fileSystem fs.FS
}

// ServeHTTP serves files from the embedded filesystem, falling back to index.html
// for any requests that don't match an existing file (to support client-side routing)
func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the path to prevent directory traversal
	urlPath := path.Clean(r.URL.Path)

	// Try to open the requested file
	file, err := h.fileSystem.Open(urlPath[1:]) // Remove leading slash
	if err == nil {
		defer file.Close()

		// Check if it's a file (not a directory)
		if stat, err := file.Stat(); err == nil && !stat.IsDir() {
			// File exists, serve it
			http.FileServer(http.FS(h.fileSystem)).ServeHTTP(w, r)
			return
		}
	}

	// File doesn't exist or is a directory, try index.html
	indexFile, err := h.fileSystem.Open("index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusNotFound)
		return
	}
	defer indexFile.Close()

	// Read and serve index.html
	indexData, err := io.ReadAll(indexFile)
	if err != nil {
		http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // Error writing to response writer is not critical
	_, _ = w.Write(indexData)
}
