package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

func main() {
	configPath := "./config.json"

	// Get user config
	masterConfig, err := loadConfig(configPath)
	if err != nil {
		log.Printf("Failed to load config.json")
		log.Fatal(err)
	}

	// Establish a registry of backends
	reg, err := makeRegistry(masterConfig.Backends)
	if err != nil {
		log.Printf("Failed to make the backend registry.")
		log.Fatal(err)
	}

	// Start the signal handler for hot swapping
	ctx, signalHandlerCancel := context.WithCancel(context.Background())
	defer signalHandlerCancel()

	startSignalHandler(ctx, reg, configPath)

	// Instantiate load balancer
	lb := &P2CLB{
		Registry: reg,
	}

	serverHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend := lb.SelectBackend()
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

	snapShot := reg.value.Load().(*Snapshot)
	numBackends := len(snapShot.PointerList)

	log.Printf("Load balancer started on :8080\n")
	log.Printf("there are %v backends", numBackends)

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
