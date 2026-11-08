package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientIPIgnoresProxyHeadersFromNetwork(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.0.10:4321"
	r.Header.Set("CF-Connecting-IP", "203.0.113.7")
	r.Header.Set("X-Forwarded-For", "198.51.100.2")
	r.Header.Set("X-Real-IP", "198.51.100.3")
	if got := clientIP(r); got != "192.168.0.10" {
		t.Fatalf("clientIP = %q, esperado 192.168.0.10", got)
	}
}

func TestClientIPUsesCloudflareHeaderFromLoopback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:4321"
	r.Header.Set("CF-Connecting-IP", "203.0.113.7")
	r.Header.Set("X-Forwarded-For", "198.51.100.2")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Fatalf("clientIP = %q, esperado 203.0.113.7", got)
	}
}

func TestClientIPFallsBackToForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "[::1]:4321"
	r.Header.Set("X-Forwarded-For", "198.51.100.2, 10.0.0.1")
	if got := clientIP(r); got != "198.51.100.2" {
		t.Fatalf("clientIP = %q, esperado 198.51.100.2", got)
	}
}

func TestClientIPIgnoresMalformedCloudflareHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:4321"
	r.Header.Set("CF-Connecting-IP", "nao-e-ip")
	r.Header.Set("X-Real-IP", "198.51.100.3")
	if got := clientIP(r); got != "198.51.100.3" {
		t.Fatalf("clientIP = %q, esperado 198.51.100.3", got)
	}
}

func TestSharePublicDoesNotExposePasswordHash(t *testing.T) {
	it := &Share{Token: "abc", Path: "docs", Name: "docs", PasswordHash: "pbkdf2$..."}
	body, err := json.Marshal(it.public())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "password_hash") || strings.Contains(string(body), "pbkdf2") {
		t.Fatalf("resposta pública expõe o hash: %s", body)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["has_password"] != true {
		t.Fatalf("has_password = %v, esperado true", got["has_password"])
	}
}

func TestShareListDoesNotExposePasswordHash(t *testing.T) {
	dir := t.TempDir()
	store := newShareStore(dir)
	store.add(&Share{Token: "tok", Name: "x", Path: "x", PasswordHash: "segredo"})
	a := &App{home: dir, shares: store}

	rec := httptest.NewRecorder()
	a.handleShareList(rec, httptest.NewRequest("GET", "/api/shares", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "password_hash") || strings.Contains(rec.Body.String(), "segredo") {
		t.Fatalf("listagem expõe o hash: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"has_password":true`) {
		t.Fatalf("listagem sem has_password: %s", rec.Body.String())
	}
}
