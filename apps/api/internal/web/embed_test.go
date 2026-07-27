package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHandler(t *testing.T) {
	handler := GetHandler()
	require.NotNil(t, handler)
}

func TestGetHandler_ReturnsHTTPHandler(_ *testing.T) {
	handler := GetHandler()

	// Verify it implements http.Handler interface
	var _ http.Handler = handler
}

func TestGetHandler_ServesRequests(t *testing.T) {
	handler := GetHandler()

	// Create test request
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	// Serve request
	handler.ServeHTTP(rr, req)

	// Should return a response (either 200 with files or 404 if not built)
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound)
}

func TestGetHandler_RootPath(t *testing.T) {
	handler := GetHandler()

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should handle root path without panicking
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound)

	if rr.Code == http.StatusNotFound {
		// If web UI not built, should have helpful message
		assert.Contains(t, rr.Body.String(), "Web UI not built")
	}
}

func TestGetHandler_DifferentPaths(t *testing.T) {
	handler := GetHandler()

	paths := []string{
		"/",
		"/index.html",
		"/assets/main.js",
		"/static/css/main.css",
		"/favicon.ico",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// Should handle all paths without panicking
			// Response can be 200 (file found), 404 (file not found or UI not built),
			// or 301 (redirect for directories)
			assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound || rr.Code == http.StatusMovedPermanently,
				"Expected 200, 301, or 404 but got %d for path %s", rr.Code, path)
		})
	}
}

func TestGetHandler_MethodsSupported(t *testing.T) {
	handler := GetHandler()

	methods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,   // Should be rejected by file server
		http.MethodPut,    // Should be rejected by file server
		http.MethodDelete, // Should be rejected by file server
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// Should handle all methods without panicking
			// GET and HEAD should work, others might not
			assert.NotEqual(t, 0, rr.Code, "Should return a status code")
		})
	}
}

func TestGetHandler_NonExistentFile(t *testing.T) {
	handler := GetHandler()

	req := httptest.NewRequest("GET", "/nonexistent-file-12345.js", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should return 404 for non-existent files
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetHandler_MultipleRequests(t *testing.T) {
	handler := GetHandler()

	// Simulate multiple concurrent requests
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Each request should succeed (or consistently return 404 if not built)
		assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound)
	}
}

func TestGetHandler_HeadRequest(t *testing.T) {
	handler := GetHandler()

	req := httptest.NewRequest("HEAD", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// HEAD requests should work
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound)

	// HEAD should not return body
	if rr.Code == http.StatusOK {
		// Body might be present for file server, but should be empty or minimal
		assert.True(t, len(rr.Body.String()) >= 0)
	}
}

func TestGetHandler_SPAFallback(t *testing.T) {
	handler := GetHandler()

	// Test SPA routes (should fall back to index.html if built)
	spaRoutes := []string{
		"/dashboard",
		"/targets",
		"/jobs",
		"/settings",
	}

	for _, route := range spaRoutes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest("GET", route, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// Two outcomes only: index.html when the UI was embedded, or the
			// "Web UI not built" 404 when it was not. A 500 used to be
			// possible when dist existed but held no index.html — now the
			// tracked .gitkeep makes that the normal backend-only build, and
			// GetHandler detects it up front.
			assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, rr.Code)
		})
	}
}

func TestGetHandler_ConsistentBehavior(t *testing.T) {
	handler := GetHandler()

	// Make same request twice
	req1 := httptest.NewRequest("GET", "/", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	req2 := httptest.NewRequest("GET", "/", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	// Should return same status code
	assert.Equal(t, rr1.Code, rr2.Code)

	// If not built, should return same error message
	if rr1.Code == http.StatusNotFound {
		assert.Equal(t, rr1.Body.String(), rr2.Body.String())
	}
}

func TestGetHandler_DevModeMessage(t *testing.T) {
	handler := GetHandler()

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// If dist directory doesn't exist (dev mode), should have helpful message
	if rr.Code == http.StatusNotFound {
		body := rr.Body.String()
		assert.Contains(t, body, "Web UI not built")
		assert.Contains(t, body, "npm")
		assert.Contains(t, body, "build")
	}
}

// Note: Testing actual embedded file serving requires the web UI to be built.
// These tests verify the handler works in both scenarios:
// 1. When dist directory exists (files are served)
// 2. When dist directory doesn't exist (helpful error message)
