package compose

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

type slowReader struct {
	delay time.Duration
	data  []byte
	done  bool
}

func (s *slowReader) Read(p []byte) (int, error) {
	if s.done {
		return 0, io.EOF
	}
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	n := copy(p, s.data)
	s.done = true
	return n, nil
}

func TestIdleTimeoutReader_Success(t *testing.T) {
	data := []byte("hello world")
	r := newIdleTimeoutReader(bytes.NewReader(data), 50*time.Millisecond)
	defer r.Close()

	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(buf) != string(data) {
		t.Fatalf("got %q, want %q", string(buf), string(data))
	}
}

func TestIdleTimeoutReader_TimeoutWhenStalled(t *testing.T) {
	slow := &slowReader{
		delay: 100 * time.Millisecond,
		data:  []byte("chunk"),
	}

	r := newIdleTimeoutReader(slow, 30*time.Millisecond)
	defer r.Close()

	_, err := io.ReadAll(r)
	if !errors.Is(err, ErrDownloadStalled) {
		t.Fatalf("expected ErrDownloadStalled, got %v", err)
	}
}

func TestIdleTimeoutReader_ContinuousStreamingSucceeds(t *testing.T) {
	pr, pw := io.Pipe()
	r := newIdleTimeoutReader(pr, 60*time.Millisecond)
	defer r.Close()

	go func() {
		defer pw.Close()
		for i := 0; i < 5; i++ {
			time.Sleep(20 * time.Millisecond)
			if _, err := pw.Write([]byte("chunk")); err != nil {
				return
			}
		}
	}()

	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(buf) != "chunkchunkchunkchunkchunk" {
		t.Fatalf("unexpected content: %q", string(buf))
	}
}
