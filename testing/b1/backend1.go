package main

import (
	"fmt"
	"net/http"
)

func main() {
	id := "backend-1"

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, id)
	})

	http.ListenAndServe(":9001", nil)
}
