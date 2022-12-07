package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

var (
	healthy int32
	counter int
	mutex   sync.Mutex
)

func main() {
	logger := log.New(os.Stdout, "simple-web-endpoint: ", log.LstdFlags)
	logger.Println("Starting...")
	router := http.NewServeMux()
	router.Handle("/", index())
	router.Handle("/healthz", healthz())
	server := &http.Server{
		Addr:     ":8080",
		Handler:  logging(logger)(router),
		ErrorLog: logger,
	}

	logger.Println("Ready to handle requests!")
	atomic.StoreInt32(&healthy, 1)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on 0.0.0.0:8080!")
	}
	logger.Println("Stopping...")
}

func index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<h2>Hi from <em>naterator</em>!</h2><h3>Visitors: ")
		mutex.Lock()
		counter++
		fmt.Fprintf(w, strconv.Itoa(counter))
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
