package main

import "testing"

func TestDefaultHealthURL(t *testing.T) {
	if got := defaultHealthURL("", ""); got != "http://127.0.0.1:8080/healthz" {
		t.Fatalf("plain URL = %q", got)
	}
	if got := defaultHealthURL("/tls/cert.pem", "/tls/key.pem"); got != "https://127.0.0.1:8080/healthz" {
		t.Fatalf("TLS URL = %q", got)
	}
	if got := defaultHealthURL("/tls/cert.pem", ""); got != "http://127.0.0.1:8080/healthz" {
		t.Fatalf("half-configured TLS URL = %q", got)
	}
}
