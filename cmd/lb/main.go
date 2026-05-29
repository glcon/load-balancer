package main

import (
	"context"
	"flag"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"load-balancer/internal"
	"log/slog"
	"os"
)

func main() {
	// initialize global structured logger
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	configPath := flag.String("config", "./config.yml", "Path to the load balancer config file")
	flag.Parse()

	// start prometheus
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		slog.Info("Observability server started", "addr", ":9090/metrics")

		err := http.ListenAndServe(":9090", mux)
		if err != nil {
			slog.Error("Metrics server failed", "error", err)
			os.Exit(1)
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

	slog.Info("Load balancer started", "addr", ":8080")

	err := server.ListenAndServe()
	if err != nil {
		slog.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}

func Startup(ctx context.Context, configPath string) *internal.P2CLB {
	// Get user config
	masterConfig, err := internal.LoadConfig(configPath)
	if err != nil {
		slog.Error("Failed to load config file", "configPath", configPath, "error", err)
		os.Exit(1)
	}

	// Establish a registry of backends
	reg, err := internal.MakeRegistry(masterConfig.Backends)
	if err != nil {
		slog.Error("Failed to make the backend registry", "error", err)
		os.Exit(1)
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
		slog.Error("All retry attempts exhausted. Final Proxy Error", "error", err)

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
