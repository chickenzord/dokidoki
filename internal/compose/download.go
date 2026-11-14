package compose

import (
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	// ErrDownloadStalled is returned when a download stalls and no bytes are received within IdleTimeout.
	ErrDownloadStalled = errors.New("download stalled: no bytes received within idle timeout")
)

const (
	// DefaultDialTimeout is the timeout for establishing TCP connections.
	DefaultDialTimeout = 15 * time.Second

	// DefaultResponseHeaderTimeout is the timeout waiting for server response headers.
	DefaultResponseHeaderTimeout = 30 * time.Second

	// DefaultIdleTimeout is the maximum time allowed with zero bytes transferred during streaming.
	DefaultIdleTimeout = 30 * time.Second
)

// newDownloadHTTPClient creates an HTTP client suitable for large streaming downloads.
// It has no wall-clock total timeout so large files on slow connections can finish,
// but enforces connect and header timeouts, along with keep-alive settings.
func newDownloadHTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   DefaultDialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: DefaultResponseHeaderTimeout,
		TLSHandshakeTimeout:   15 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		// Timeout must be 0 for large streaming downloads
		Timeout: 0,
	}
}

// idleTimeoutReader wraps an io.Reader and fails with ErrDownloadStalled
// if no bytes are read within the idle timeout.
type idleTimeoutReader struct {
	reader io.Reader
	idle   time.Duration

	timer *time.Timer
	mu    sync.Mutex
	err   error
}

// newIdleTimeoutReader wraps r with an idle timeout.
func newIdleTimeoutReader(r io.Reader, idle time.Duration) *idleTimeoutReader {
	itr := &idleTimeoutReader{
		reader: r,
		idle:   idle,
	}

	itr.timer = time.AfterFunc(idle, func() {
		itr.mu.Lock()
		defer itr.mu.Unlock()
		if itr.err == nil {
			itr.err = ErrDownloadStalled
		}
	})

	return itr
}

func (r *idleTimeoutReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	if r.err != nil {
		err := r.err
		r.mu.Unlock()
		return 0, err
	}
	r.mu.Unlock()

	n, err := r.reader.Read(p)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return n, r.err
	}

	if n > 0 {
		// Reset idle timer since we received bytes
		r.timer.Reset(r.idle)
	}

	if err != nil {
		r.timer.Stop()
	}

	return n, err
}

// Close stops the timer and closes the underlying reader if it implements io.Closer.
func (r *idleTimeoutReader) Close() error {
	r.mu.Lock()
	r.timer.Stop()
	r.mu.Unlock()

	if closer, ok := r.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
