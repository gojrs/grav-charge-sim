package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed client
var clientFiles embed.FS

func main() {
	sub, err := fs.Sub(clientFiles, "client")
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	http.Handle("/", http.FileServer(http.FS(sub)))
	log.Printf("Serving at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
