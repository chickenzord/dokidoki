package api

import (
	"net/http"
	"time"
)

// DefaultStreamingIdleTimeout is the default idle timeout between writes
// for long-running streaming operations (such as compose up/down/pull).
const DefaultStreamingIdleTimeout = 5 * time.Minute

// SlidingDeadlineMiddleware sets an initial write deadline and extends it
// whenever data is written or flushed, keeping the connection alive as long
// as there is periodic output.
func SlidingDeadlineMiddleware(idleTimeout time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rc := http.NewResponseController(w)

			// Set initial deadline for the operation to begin
			if idleTimeout > 0 {
				_ = rc.SetWriteDeadline(time.Now().Add(idleTimeout))
			}

			sw := &slidingDeadlineWriter{
				ResponseWriter: w,
				rc:             rc,
				idleTimeout:    idleTimeout,
			}

			next.ServeHTTP(sw, r)
		})
	}
}

type slidingDeadlineWriter struct {
	http.ResponseWriter
	rc          *http.ResponseController
	idleTimeout time.Duration
}

func (s *slidingDeadlineWriter) extend() {
	if s.idleTimeout > 0 && s.rc != nil {
		_ = s.rc.SetWriteDeadline(time.Now().Add(s.idleTimeout))
	}
}

func (s *slidingDeadlineWriter) Write(p []byte) (int, error) {
	s.extend()
	return s.ResponseWriter.Write(p)
}

func (s *slidingDeadlineWriter) Flush() {
	s.extend()
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter for ResponseController and middleware unwrapping.
func (s *slidingDeadlineWriter) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
