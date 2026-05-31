package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"load-balancer/internal"
)

func main() {
	run()
}

func run() {
	initializeLogger()

	configPath := flag.String("config", "./configs/config.yml", "Path to the load balancer config file")
	flag.Parse()

	masterConfig, err := internal.LoadConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load config file", "configPath", *configPath, "error", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("starting pprof server", "addr", masterConfig.PprofAddr)

		if err := http.ListenAndServe(masterConfig.PprofAddr, nil); err != nil {
			slog.Error("pprof server failed to start or crashed", "error", err)
		}
	}()

	startMetricsServer(masterConfig.MetricsAddr)

	ctx, signalHandlerCancel := context.WithCancel(context.Background())
	defer signalHandlerCancel()

	lb := startup(ctx, *configPath, masterConfig)

	serve(newServer(masterConfig.ListenAddr, establishTransport(lb)), masterConfig.ListenAddr)
}

func initializeLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func startMetricsServer(addr string) {
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		slog.Info("Observability server started", "addr", addr+"/metrics")

		err := http.ListenAndServe(addr, mux)
		if err != nil {
			slog.Error("Metrics server failed", "error", err)
			os.Exit(1)
		}
	}()
}

func startup(ctx context.Context, configPath string, masterConfig *internal.MasterConfig) *internal.P2CLB {
	reg, err := internal.MakeRegistry(masterConfig)
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

func establishTransport(lb *internal.P2CLB) *httputil.ReverseProxy {
	errorFunc := func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("All retry attempts exhausted. Final Proxy Error", "error", err)

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

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

func serve(server *http.Server, addr string) {
	slog.Info("Load balancer started", "addr", addr)

	err := server.ListenAndServe()
	if err != nil {
		slog.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
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
