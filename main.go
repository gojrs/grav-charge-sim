package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

//go:embed client
var clientFiles embed.FS

func main() {
	sub, err := fs.Sub(clientFiles, "client")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(sub)))
	log.Println("Serving at http://localhost:8088")
	log.Fatal(http.ListenAndServe(":8088", nil))
}
