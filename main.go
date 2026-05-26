package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"
)

func main() {
	ctx, signalHandlerCancel := context.WithCancel(context.Background())
	defer signalHandlerCancel()

	lb := Startup(ctx)

	globalProxy := EstablishTransport(lb)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      globalProxy,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Load balancer started on :8080\n")

	// TAKE OUT LATER
	numBackends := len(lb.Registry.value.Load().(*Snapshot).PointerList)
	log.Printf("there are %v backends", numBackends)

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func Startup(ctx context.Context) *P2CLB {
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
	startSignalHandler(ctx, reg, configPath)

	lb := &P2CLB{
		Registry: reg,
	}

	return lb
}

func EstablishTransport(lb *P2CLB) *httputil.ReverseProxy {
	// error handler
	errorFunc := func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("All retry attempts exhausted. Final Proxy Error: %v", err)

		// Send json to user for error
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error": "Bad Gateway", "message": "All backends failed to respond."}`))
	}

	globalProxy := &httputil.ReverseProxy{
		Transport: &Transport{LB: lb},
		Director: func(req *http.Request) {
			req.Header.Set("X-Proxy-Source", "balancer")
		},
		ErrorHandler: errorFunc,
		BufferPool:   bytesPool{},
	}

	return globalProxy
}

var proxyBufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 32*1024)
	},
}

type bytesPool struct{}

func (bp bytesPool) Get() []byte {
	return proxyBufferPool.Get().([]byte)
}

func (bp bytesPool) Put(b []byte) {
	proxyBufferPool.Put(b)
}
