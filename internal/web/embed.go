// Package web provides embedded web UI assets and HTTP handlers for the React frontend.
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
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			//nolint:errcheck // Error writing to response writer is not critical
			_, _ = w.Write([]byte("Web UI not built. Run: cd web && npm install && npm run build"))
		})
	}

	// Read index.html once at startup
	indexHTML, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "index.html not found", http.StatusNotFound)
		})
	}

	// Create file server once
	fileServer := http.FileServer(http.FS(dist))

	return &spaHandler{
		fileServer: fileServer,
		indexHTML:  indexHTML,
		fileSystem: dist,
	}
}

// spaHandler implements http.Handler to serve a SPA with fallback to index.html
type spaHandler struct {
	fileServer http.Handler
	indexHTML  []byte
	fileSystem fs.FS
}

// ServeHTTP serves files from the embedded filesystem, falling back to index.html
// for any requests that don't match an existing file (to support client-side routing)
func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Use a custom response writer to capture 404s
	rw := &notFoundInterceptor{
		ResponseWriter: w,
		fileSystem:     h.fileSystem,
		requestPath:    r.URL.Path,
	}

	// Try to serve the file
	h.fileServer.ServeHTTP(rw, r)

	// If file was not found, serve index.html for SPA routing
	if rw.status == http.StatusNotFound {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // Error writing to response writer is not critical
		_, _ = w.Write(h.indexHTML)
	}
}

// notFoundInterceptor wraps http.ResponseWriter to intercept 404 responses
type notFoundInterceptor struct {
	http.ResponseWriter
	fileSystem  fs.FS
	requestPath string
	status      int
	written     bool
}

// WriteHeader intercepts the status code
func (w *notFoundInterceptor) WriteHeader(status int) {
	w.status = status
	if status != http.StatusNotFound {
		w.ResponseWriter.WriteHeader(status)
		w.written = true
	}
}

// Write intercepts writes to prevent writing 404 content
func (w *notFoundInterceptor) Write(b []byte) (int, error) {
	if w.status == http.StatusNotFound {
		// Don't write the 404 content, we'll serve index.html instead
		return len(b), nil
	}
	w.written = true
	return w.ResponseWriter.Write(b)
}
