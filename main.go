package main

import (
	"crypto/tls"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"golang.org/x/crypto/acme/autocert"
)

//go:embed client
var clientFiles embed.FS

func main() {
	sub, err := fs.Sub(clientFiles, "client")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(sub)))

	// Local dev: set PORT to run plain HTTP without touching certs.
	if port := os.Getenv("PORT"); port != "" {
		log.Printf("Serving at http://localhost:%s", port)
		log.Fatal(http.ListenAndServe(":"+port, nil))
	}

	// Production: autocert provisions and renews a Let's Encrypt cert.
	// DOMAIN and CERT_DIR can be overridden via environment for self-hosters.
	domain := os.Getenv("DOMAIN")
	if domain == "" {
		domain = "gsim.vdisknow.com"
	}
	certDir := os.Getenv("CERT_DIR")
	if certDir == "" {
		certDir = "/opt/grav-charge-sim/certs"
	}

	m := &autocert.Manager{
		Cache:      autocert.DirCache(certDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain),
	}

	// :80 handles ACME HTTP-01 challenges; everything else redirects to HTTPS.
	go func() {
		log.Fatal(http.ListenAndServe(":80", m.HTTPHandler(http.HandlerFunc(redirectHTTPS))))
	}()

	// :443 serves the simulation over TLS with auto-provisioned certs.
	srv := &http.Server{
		Addr:      ":443",
		TLSConfig: &tls.Config{GetCertificate: m.GetCertificate},
	}
	log.Printf("Serving at https://%s", domain)
	log.Fatal(srv.ListenAndServeTLS("", ""))
}

func redirectHTTPS(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://"+r.Host+r.URL.RequestURI(), http.StatusMovedPermanently)
}
