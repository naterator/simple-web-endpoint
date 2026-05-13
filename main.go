package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	defaultListenAddr = ":8080"
	shutdownTimeout   = 5 * time.Second
)

var (
	healthy int32
	counter int
	mutex   sync.Mutex
)

func main() {
	logger := log.New(os.Stdout, "simple-web-endpoint: ", log.LstdFlags)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger, listenAddr()); err != nil {
		logger.Fatal(err)
	}
}

func listenAddr() string {
	if addr := os.Getenv("LISTEN_ADDR"); addr != "" {
		return addr
	}

	if port := os.Getenv("PORT"); port != "" {
		if strings.HasPrefix(port, ":") {
			return port
		}
		return ":" + port
	}

	return defaultListenAddr
}

func run(ctx context.Context, logger *log.Logger, addr string) error {
	logger.Println("Starting...")
	server := &http.Server{
		Addr:     addr,
		Handler:  logging(logger)(router()),
		ErrorLog: logger,
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %w", addr, err)
	}

	errCh := make(chan error, 1)
	logger.Println("Ready to handle requests!")
	atomic.StoreInt32(&healthy, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()

	select {
	case err := <-errCh:
		atomic.StoreInt32(&healthy, 0)
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("could not listen on %s: %w", addr, err)
		}
	case <-ctx.Done():
		atomic.StoreInt32(&healthy, 0)
		logger.Println("Stopping...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("could not shut down: %w", err)
		}
		if err := <-errCh; err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("could not listen on %s: %w", addr, err)
		}
	}

	logger.Println("Stopped.")
	return nil
}

func router() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", index())
	mux.Handle("/healthz", healthz())
	return mux
}

func index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<h2>Hi from <em>naterator</em>!</h2><h3>Process-local visitors: ")
		mutex.Lock()
		counter++
		fmt.Fprint(w, counter)
		mutex.Unlock()
		fmt.Fprintf(w, "</h3>")
	})
}

func healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&healthy) == 1 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				logger.Println(r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}
