package main

import (
	"flag"
	"testing"
)

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

func TestFlagsReadPortEnvironment(t *testing.T) {
	t.Setenv("LANPARTY_APP_PORT", "52821")
	t.Setenv("LANPARTY_WEB_PORT", "9090")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := addCommonFlags(fs)
	if c.appPort != 52821 {
		t.Fatalf("app port = %d", c.appPort)
	}
	if c.webPort != 9090 {
		t.Fatalf("web port = %d", c.webPort)
	}
}

func TestFlagsDetectContainer(t *testing.T) {
	t.Setenv("LANPARTY_WEB_CONTAINER", "1")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := addCommonFlags(fs)
	cfg, err := c.config()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InContainer {
		t.Fatal("container flag not detected")
	}
}
