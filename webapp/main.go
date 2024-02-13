package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, world!"))
	})

	log.Println("Starting server on :8080")

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		log.Fatal("Starting server failure: ", err)
	}

}
