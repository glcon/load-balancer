package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type Backend struct {
	URL          *url.URL
	ReverseProxy *httputil.ReverseProxy
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func main() {
	rawURL := "http://localhost:8081"
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

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		log.Printf("Proxy error: %v", e)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)

		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "Bad Gateway",
			Message: "Backend server is unreachable",
		})
	}

	server := &http.Server{
		Addr:         ":8080",
		Handler:      proxy,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("lab Proxy stated on :8080 -> Forwarding to %s\n", rawURL)

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}
