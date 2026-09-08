package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type mockDeadlineWriter struct {
	http.ResponseWriter
	mu        sync.Mutex
	deadlines []time.Time
	flushed   bool
}

func (m *mockDeadlineWriter) SetWriteDeadline(d time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deadlines = append(m.deadlines, d)
	return nil
}

func (m *mockDeadlineWriter) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushed = true
	if f, ok := m.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func TestSlidingDeadlineMiddleware_WithDeadlineSupport(t *testing.T) {
	rec := httptest.NewRecorder()
	mock := &mockDeadlineWriter{ResponseWriter: rec}

	timeout := 10 * time.Second
	mw := SlidingDeadlineMiddleware(timeout)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Flusher assertion
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected ResponseWriter to implement http.Flusher")
		}

		time.Sleep(10 * time.Millisecond)
		_, err := w.Write([]byte("chunk 1"))
		if err != nil {
			t.Fatalf("unexpected write error: %v", err)
		}

		time.Sleep(10 * time.Millisecond)
		flusher.Flush()
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	handler.ServeHTTP(mock, req)

	mock.mu.Lock()
	defer mock.mu.Unlock()

	// 1 initial deadline + 1 on Write + 1 on Flush = 3 deadline updates
	if len(mock.deadlines) < 3 {
		t.Fatalf("expected at least 3 deadline calls, got %d", len(mock.deadlines))
	}

	if !mock.flushed {
		t.Errorf("expected Flush() to be called on mock")
	}

	// Ensure each subsequent deadline is further in the future
	for i := 1; i < len(mock.deadlines); i++ {
		if !mock.deadlines[i].After(mock.deadlines[i-1]) {
			t.Errorf("deadline %d (%v) was not after deadline %d (%v)", i, mock.deadlines[i], i-1, mock.deadlines[i-1])
		}
	}
}

func TestSlidingDeadlineMiddleware_WithRecorder(t *testing.T) {
	// Standard httptest.ResponseRecorder does not implement SetWriteDeadline
	rec := httptest.NewRecorder()
	mw := SlidingDeadlineMiddleware(5 * time.Minute)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestSlidingDeadlineMiddleware_ChiRouteParams(t *testing.T) {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(SlidingDeadlineMiddleware(5 * time.Minute))
		r.Post("/items/{id}/stream", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			_, _ = w.Write([]byte("item: " + id))
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/items/123/stream", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "item: 123" {
		t.Errorf("expected body 'item: 123', got %q", rec.Body.String())
	}
}
