package main

import (
	"fmt"
	"net/http"
)

func main() {
	id := "backend-3"

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, id)
	})

	http.ListenAndServe(":9003", nil)
}
