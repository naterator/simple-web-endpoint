package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIndexIncrementsVisitorCounter(t *testing.T) {
	counter = 0

	handler := index()

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))

	if got, want := first.Code, http.StatusOK; got != want {
		t.Fatalf("first response status = %d, want %d", got, want)
	}
	if got, want := first.Header().Get("Content-Type"), "text/html"; got != want {
		t.Fatalf("content type = %q, want %q", got, want)
	}
	if got, want := first.Body.String(), "<h2>Hi from <em>naterator</em>!</h2><h3>Process-local visitors: 1</h3>"; got != want {
		t.Fatalf("first body = %q, want %q", got, want)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))

	if got, want := second.Body.String(), "<h2>Hi from <em>naterator</em>!</h2><h3>Process-local visitors: 2</h3>"; got != want {
		t.Fatalf("second body = %q, want %q", got, want)
	}
}

func TestListenAddr(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("LISTEN_ADDR", "")
		t.Setenv("PORT", "")

		if got, want := listenAddr(), defaultListenAddr; got != want {
			t.Fatalf("listenAddr() = %q, want %q", got, want)
		}
	})

	t.Run("listen addr", func(t *testing.T) {
		t.Setenv("LISTEN_ADDR", "127.0.0.1:9090")
		t.Setenv("PORT", "8081")

		if got, want := listenAddr(), "127.0.0.1:9090"; got != want {
			t.Fatalf("listenAddr() = %q, want %q", got, want)
		}
	})

	t.Run("port", func(t *testing.T) {
		t.Setenv("LISTEN_ADDR", "")
		t.Setenv("PORT", "9090")

		if got, want := listenAddr(), ":9090"; got != want {
			t.Fatalf("listenAddr() = %q, want %q", got, want)
		}
	})
}

func TestRunStopsGracefullyAndMarksUnhealthy(t *testing.T) {
	atomic.StoreInt32(&healthy, 0)
	t.Cleanup(func() {
		atomic.StoreInt32(&healthy, 0)
	})

	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		errCh <- run(ctx, logger, "127.0.0.1:0")
	}()

	waitForHealthy(t, 1)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for graceful shutdown")
	}

	if got, want := atomic.LoadInt32(&healthy), int32(0); got != want {
		t.Fatalf("healthy = %d, want %d", got, want)
	}
}

func TestHealthzReflectsHealthyFlag(t *testing.T) {
	handler := healthz()
	t.Cleanup(func() {
		atomic.StoreInt32(&healthy, 0)
	})

	atomic.StoreInt32(&healthy, 1)
	ok := httptest.NewRecorder()
	handler.ServeHTTP(ok, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got, want := ok.Code, http.StatusOK; got != want {
		t.Fatalf("healthy status = %d, want %d", got, want)
	}

	atomic.StoreInt32(&healthy, 0)
	unavailable := httptest.NewRecorder()
	handler.ServeHTTP(unavailable, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got, want := unavailable.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("unhealthy status = %d, want %d", got, want)
	}
}

func TestLoggingMiddlewareLogsRequestDetails(t *testing.T) {
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	handler := logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	request := httptest.NewRequest(http.MethodPost, "/example", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	request.Header.Set("User-Agent", "simple-test")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusAccepted; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	logLine := output.String()
	for _, want := range []string{http.MethodPost, "/example", "192.0.2.1:1234", "simple-test"} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log line %q does not contain %q", logLine, want)
		}
	}
}

func waitForHealthy(t *testing.T, want int32) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if got := atomic.LoadInt32(&healthy); got == want {
			return
		}

		select {
		case <-deadline:
			t.Fatalf("timed out waiting for healthy = %d", want)
		case <-ticker.C:
		}
	}
}
