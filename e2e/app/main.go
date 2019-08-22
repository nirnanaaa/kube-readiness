package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

var (
	termDelay                 *int
	terminated                bool
	signalReceived            time.Time
	lastRequest               time.Duration
	requestsBeforeTermination uint64
	requestsAfterTermination  uint64
	mu                        sync.Mutex
)

func handler(w http.ResponseWriter, r *http.Request) {

	if !terminated {
		atomic.AddUint64(&requestsBeforeTermination, 1)
	} else {
		atomic.AddUint64(&requestsAfterTermination, 1)
		since := time.Since(signalReceived)
		if since > lastRequest {
			mu.Lock()
			lastRequest = since
			mu.Unlock()
		}
	}

	query := r.URL.Query()
	name := query.Get("name")
	if name == "" {
		name = "Guest"
	}
	// log.Printf("Received request for %s (%d/%d)\n", name, requestsBeforeTermination, requestsAfterTermination)
	w.Write([]byte(fmt.Sprintf("Hello, %s\n ", name)))

}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	if terminated {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func main() {

	termDelay = flag.Int("term-delay", 15, "The amount of delay before shutdown.")
	flag.Parse()

	// Create Server and Route Handlers
	r := mux.NewRouter()

	r.HandleFunc("/", handler)
	r.HandleFunc("/health", healthHandler)
	r.HandleFunc("/readiness", readinessHandler)

	srv := &http.Server{
		Handler:      r,
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start Server
	go func() {
		log.Println("Starting Server")
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	// Graceful Shutdown
	waitForShutdown(srv)
}

func waitForShutdown(srv *http.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive our signal.
	<-interruptChan

	log.Printf("Caught TERM signal, initiating shutdown...")
	signalReceived = time.Now()

	terminated = true

	log.Printf("Waiting %d seconds for remaining connections...", *termDelay)
	time.Sleep(time.Duration(*termDelay) * time.Second)

	log.Println("Gracefully shutting down the webserver...")
	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	srv.Shutdown(ctx)

	before := atomic.LoadUint64(&requestsBeforeTermination)
	after := atomic.LoadUint64(&requestsAfterTermination)
	log.Printf("requests before signal: %d, after signal: %d, last request time since term: %s", before, after, lastRequest.String())

	log.Println("All done, bye!")
	os.Exit(0)
}
