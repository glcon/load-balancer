package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

    "load-balancer/internal"
)

func main() {
	configPath := flag.String("config", "./config.yml", "Path to the load balancer config file")
	flag.Parse()

	// start prometheus
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Println("Observability server started on :9090/metrics")

		err := http.ListenAndServe(":9090", mux)
		if err != nil {
			log.Fatalf("Metrics server failed: %v", err)
		}
	}()

	ctx, signalHandlerCancel := context.WithCancel(context.Background())
	defer signalHandlerCancel()

	lb := Startup(ctx, *configPath)

	globalProxy := EstablishTransport(lb)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      globalProxy,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Load balancer started on :8080\n")

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func Startup(ctx context.Context, configPath string) *internal.P2CLB {
	// Get user config
	masterConfig, err := internal.LoadConfig(configPath)
	if err != nil {
		log.Printf("Failed to load config file")
		log.Fatal(err)
	}

	// Establish a registry of backends
	reg, err := internal.MakeRegistry(masterConfig.Backends)
	if err != nil {
		log.Printf("Failed to make the backend registry.")
		log.Fatal(err)
	}

	// Start the signal handler for hot swapping
	internal.StartSignalHandler(ctx, reg, configPath)

	lb := &internal.P2CLB{
		Registry: reg,
	}

	return lb
}

func EstablishTransport(lb *internal.P2CLB) *httputil.ReverseProxy {
	// error handler
	errorFunc := func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("All retry attempts exhausted. Final Proxy Error: %v", err)

		// Send json to user for error
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error": "Bad Gateway", "message": "All backends failed to respond."}`))
	}

	globalProxy := &httputil.ReverseProxy{
		Transport: &internal.Transport{LB: lb},
		Director: func(req *http.Request) {
			req.Header.Set("X-Proxy-Source", "balancer")
		},
		ErrorHandler: errorFunc,
		BufferPool:   bytesPool{},
	}

	return globalProxy
}

// byte pools that the proxy can reuse
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
