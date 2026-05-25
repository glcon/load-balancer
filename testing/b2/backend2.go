package main

import (
	"fmt"
	"net/http"
)

func main() {
	id := "backend-2"

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, id)
	})

	http.ListenAndServe(":9002", nil)
}
