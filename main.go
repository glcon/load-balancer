package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	// define some backends
	backendURLs := []string{
		"http://localhost:8081",
		"http://localhost:8082",
		"http://localhost:8083",
	}

	var backends []*Backend
	for _, url := range backendURLs {
		b, err := NewBackend(url)
		if err != nil {
			log.Fatalf("Failed to make Backend struct for %v", url)
		}

		backends = append(backends, b)
	}

	// choose either rr or p2c
	var lb LoadBalancer = NewP2CLB(backends)

	serverHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend := lb.Next()
		if backend == nil {
			log.Printf("No backends available")
			return
		}

		backend.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:         ":8080",
		Handler:      serverHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Load balancer started on :8080\n")
	log.Printf("there are %v backends", len(backends))

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
