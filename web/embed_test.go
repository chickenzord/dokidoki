package web

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	handler := Handler()

	t.Run("serves root index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		body, err := io.ReadAll(rec.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		if !strings.Contains(string(body), "Dokidoki") {
			t.Errorf("expected body to contain 'Dokidoki', got: %s", string(body))
		}
	})

	t.Run("serves static assets", func(t *testing.T) {
		assets, err := fs.ReadDir(distFS, "dist/assets")
		if err != nil || len(assets) == 0 {
			t.Fatalf("no assets found in dist/assets: %v", err)
		}

		assetName := assets[0].Name()
		req := httptest.NewRequest(http.MethodGet, "/assets/"+assetName, nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 for asset %s, got %d", assetName, rec.Code)
		}
	})

	t.Run("falls back to index.html on missing path for SPA routing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/stacks/my-stack", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		body, err := io.ReadAll(rec.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		if !strings.Contains(string(body), "Dokidoki") {
			t.Errorf("expected fallback body to contain 'Dokidoki', got: %s", string(body))
		}
	})
}
