package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// fakeFS builds an in-memory fs.FS mimicking a built SPA so handler tests
// stay independent of web/dist contents.
func fakeFS(t *testing.T) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte("<!doctype html><title>TestApp</title>")},
		"assets/app.js":  &fstest.MapFile{Data: []byte("console.log('app')")},
		"assets/app.css": &fstest.MapFile{Data: []byte("body{margin:0}")},
	}
}

func TestHandlerFromFS(t *testing.T) {
	handler := HandlerFromFS(fakeFS(t))

	t.Run("serves root", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		body, _ := io.ReadAll(rec.Body)
		if len(body) == 0 {
			t.Fatal("expected non-empty body for root")
		}
	})

	t.Run("serves nested asset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("falls back to root for SPA route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/stacks/my-stack", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		body, _ := io.ReadAll(rec.Body)
		if len(body) == 0 {
			t.Fatal("expected non-empty fallback body")
		}
	})
}

func TestHandlerFromFS_EmptyFS(t *testing.T) {
	// Verifies the handler does not panic or 500 when given an empty fs.FS —
	// simulates the post-cleanup, pre-build state during local dev.
	handler := HandlerFromFS(fstest.MapFS{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}
