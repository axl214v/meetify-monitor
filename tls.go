package main

import (
	"crypto/tls"
	"log"
	"net/http"

	"golang.org/x/crypto/acme/autocert"
)

// serveHTTPS terminates TLS in-process with a Let's Encrypt certificate for
// cfg.Domain. The certificate is obtained on first request and renewed
// automatically ahead of expiry; :80 answers ACME http-01 challenges and
// redirects everything else to HTTPS. It blocks until a listener fails.
func serveHTTPS(h http.Handler, cfg Config) error {
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(cfg.Domain),
		Cache:      autocert.DirCache(cfg.CertDir),
		Email:      cfg.ACMEEmail,
	}

	go func() {
		log.Fatal(newServer(":80", m.HTTPHandler(nil)).ListenAndServe())
	}()

	srv := newServer(":443", h)
	srv.TLSConfig = &tls.Config{
		GetCertificate: m.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1", "acme-tls/1"},
		MinVersion:     tls.VersionTLS12,
	}
	log.Printf("meetify-monitor serving https://%s (certs in %s) -> polling %s every %s", cfg.Domain, cfg.CertDir, cfg.TargetURL, cfg.PollInterval)
	return srv.ListenAndServeTLS("", "")
}
