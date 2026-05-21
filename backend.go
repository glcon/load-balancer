package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

type Backend struct {
	URL               *url.URL
	ReverseProxy      *httputil.ReverseProxy
	ActiveConnections int64
	Alive             atomic.Bool
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func NewBackend(rawURL string) (*Backend, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		clientIP, _, _ := net.SplitHostPort(req.RemoteAddr)

		req.Header.Set("X-Forwarded-For", clientIP)
		req.Header.Set("X-Proxy-Source", "balancer")
	}

	proxy.ErrorHandler = customErrorHandler

	b := &Backend{
		URL:          target,
		ReverseProxy: proxy,
	}

	// .Store: special method for atomic bool
	b.Alive.Store(true)

	return b, nil
}

func (b *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&b.ActiveConnections, 1)
	defer atomic.AddInt64(&b.ActiveConnections, -1)

	// learning: this serveHTTP is unrelated to the one just made
	b.ReverseProxy.ServeHTTP(w, r)
}

func customErrorHandler(w http.ResponseWriter, r *http.Request, e error) {
	log.Printf("Proxy error: %v", e)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)

	givenError := ErrorResponse{
		Error:   "Bad Gateway",
		Message: "Backend server is unreachable",
	}

	_ = json.NewEncoder(w).Encode(givenError)
}
