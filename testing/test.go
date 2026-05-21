package main

import (
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

func startBackend(port string, mode *atomic.Int32) {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case 0:
			w.WriteHeader(200)
			w.Write([]byte("ok"))
		case 1:
			w.WriteHeader(500)
		case 2:
			time.Sleep(3 * time.Second)
			w.WriteHeader(200)
		}
	})

	go func() {
		log.Printf("backend %s started", port)
		_ = http.ListenAndServe(":"+port, mux)
	}()
}

func runScenario(b2, b3 *atomic.Int32) {
	time.Sleep(3 * time.Second)
	log.Println("b2 fails")
	b2.Store(1)

	time.Sleep(5 * time.Second)
	log.Println("b2 recovers")
	b2.Store(0)

	time.Sleep(3 * time.Second)
	log.Println("b3 times out")
	b3.Store(2)

	time.Sleep(5 * time.Second)
	log.Println("b3 recovers")
	b3.Store(0)
}

func main() {
	var b1, b2, b3 atomic.Int32

	startBackend("8081", &b1)
	startBackend("8082", &b2)
	startBackend("8083", &b3)

	time.Sleep(time.Second)

	go runScenario(&b2, &b3)

	select {}
}
