package main

import (
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("client"))
	http.Handle("/", fs)
	log.Println("Serving at http://localhost:8088")
	log.Fatal(http.ListenAndServe(":8088", nil))
}
