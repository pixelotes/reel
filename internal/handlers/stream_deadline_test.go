package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A handler that clears the write deadline must survive the server-wide
// WriteTimeout; without clearing, the stream is cut (control case).
func TestClearWriteDeadline_LongStreamSurvivesWriteTimeout(t *testing.T) {
	slow := func(clear bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if clear {
				clearWriteDeadline(w, nil)
			}
			fl := w.(http.Flusher)
			for i := 0; i < 6; i++ {
				w.Write([]byte("chunk"))
				fl.Flush()
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	run := func(clear bool) (int, error) {
		srv := httptest.NewUnstartedServer(slow(clear))
		srv.Config.WriteTimeout = 250 * time.Millisecond
		srv.Start()
		defer srv.Close()
		resp, err := http.Get(srv.URL)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		return len(b), err
	}

	if n, err := run(true); err != nil || n != 30 {
		t.Errorf("with cleared deadline: got %d bytes, err=%v; want 30, nil", n, err)
	}
	if n, err := run(false); err == nil && n == 30 {
		t.Errorf("control: expected WriteTimeout to cut the stream without clearing")
	}
}
