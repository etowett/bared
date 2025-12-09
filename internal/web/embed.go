package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFiles embed.FS

// GetHandler returns an HTTP handler that serves the embedded React SPA
func GetHandler() http.Handler {
	// Get the dist subdirectory from the embedded filesystem
	dist, err := fs.Sub(distFiles, "dist")
	if err != nil {
		// If dist doesn't exist (development), return a simple handler
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			//nolint:errcheck // Error writing to response writer is not critical
			_, _ = w.Write([]byte("Web UI not built. Run: cd web && npm install && npm run build"))
		})
	}

	return http.FileServer(http.FS(dist))
}
