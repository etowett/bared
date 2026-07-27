// Package web provides embedded web UI assets and HTTP handlers for the React frontend.
package web

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

// dist is tracked with a .gitkeep so this pattern always matches — go:embed
// fails the build outright when it matches nothing, which used to make
// `go build ./...` fail on a fresh clone. The real assets are produced by the
// frontend build and stay gitignored.
//
//go:embed all:dist
var distFiles embed.FS

// GetHandler returns an HTTP handler that serves the embedded React SPA
func GetHandler() http.Handler {
	// Get the dist subdirectory from the embedded filesystem
	dist, err := fs.Sub(distFiles, "dist")
	if err != nil {
		return webUINotBuiltHandler()
	}

	// The directory now always exists (see above), so its emptiness is what
	// distinguishes a backend-only build from one with the UI embedded.
	// Without this check every dashboard route would answer 500.
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		return webUINotBuiltHandler()
	}

	// Create file server once
	fileServer := http.FileServer(http.FS(dist))

	return &spaHandler{
		fileServer: fileServer,
		fileSystem: dist,
	}
}

// webUINotBuiltHandler answers dashboard routes when the binary was built
// without the frontend assets.
func webUINotBuiltHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		//nolint:errcheck // Error writing to response writer is not critical
		_, _ = w.Write([]byte("Web UI not built. Run: make build-with-web (or cd apps/web && bun run build)"))
	})
}

// spaHandler implements http.Handler to serve a SPA with fallback to index.html
type spaHandler struct {
	fileServer http.Handler
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

	// If file was not found, only serve index.html for routes without file extensions
	// (to support client-side routing). Actual file requests should return 404.
	if rw.status == http.StatusNotFound {
		// Check if the request path has a file extension
		ext := filepath.Ext(r.URL.Path)
		hasExtension := ext != "" && strings.Contains(ext, ".")

		// Only serve index.html for routes without extensions (SPA routes)
		if !hasExtension {
			// Open and stream index.html
			indexFile, err := h.fileSystem.Open("index.html")
			if err != nil {
				http.Error(w, "Failed to open index.html", http.StatusInternalServerError)
				return
			}
			//nolint:errcheck // Closing read-only file in defer is not critical
			defer indexFile.Close()

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			//nolint:errcheck // Error writing to response writer is not critical
			_, _ = io.Copy(w, indexFile)
		} else {
			// For file requests (with extensions), return the 404
			w.WriteHeader(http.StatusNotFound)
			//nolint:errcheck // Error writing to response writer is not critical
			_, _ = w.Write([]byte("404 page not found"))
		}
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
		// Don't write the 404 content, we'll serve index.html instead.
		// Return len(b) to indicate "success" and prevent errors from bubbling up.
		return len(b), nil
	}
	w.written = true
	return w.ResponseWriter.Write(b)
}
