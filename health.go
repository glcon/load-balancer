package main

import (
	"context"
	"log"
	"math/rand/v2"
	"net/http"
	"time"
)

func (b *Backend) Ping() {
	// create a context, we want the request to timeout eventually
	context, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	healthURL := b.URL.String() + "/health"

	// request with context
	request, err := http.NewRequestWithContext(context, http.MethodGet, healthURL, nil)
	if err != nil {
		b.updateStatus(false)
		return
	}

	// create a local client -> doesn't compete with the balancer
	localClient := &http.Client{}
	response, err := localClient.Do(request)
	if err != nil {
		b.updateStatus(false)
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		b.updateStatus(false)
		return
	}

	b.updateStatus(true)
}

func (b *Backend) updateStatus(isAlive bool) {
	// CompareAndSwap sets var equal to param 2 ONLY if it currently equals param 1
	// method of atomic bool -> prevent duplicate logs and locks
	if b.Alive.CompareAndSwap(!isAlive, isAlive) {
		log.Printf("Backend %v status changed to %v", b.URL.String(), isAlive)
	}
}

func startHealthCheck(backends []*Backend, interval time.Duration) {
	// ticker sends a "go ahead"  every INTERVAL seconds
	for _, backend := range backends {
		go healthLoop(backend, interval)
	}
}

func healthLoop(backend *Backend, interval time.Duration) {
	// avoid synced startup storms, delay each check by random 1-5 seconds
	delay := time.Duration(rand.IntN(5000)) * time.Millisecond
	time.Sleep(delay)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		backend.Ping()
	}
}
