package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRoutesRegister protege contra um erro que só aparece em runtime:
// padrões duplicados no ServeMux derrubam o processo no boot. Aconteceu de
// verdade ao migrar /api/checksum para authAny e deixar o registro antigo.
func TestRoutesRegister(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		home:     dir,
		cfg:      Config{Root: dir, PasswordHash: "x", SessionSecret: "s"},
		activity: newActivityStore(dir),
		shares:   newShareStore(dir),
		tokens:   newTokenStore(dir),
		limiter:  newLoginLimiter(),
	}
	a.backups = newBackupManager(dir, func() string { return dir })

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("routes() entrou em pânico: %v", r)
		}
	}()
	if h := a.routes(); h == nil {
		t.Fatal("routes() devolveu nil")
	}
}

// TestTokenCannotEscalate garante que um token só de leitura não consegue
// escrever, mesmo sabendo o caminho.
func TestTokenCannotEscalate(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		home:     dir,
		cfg:      Config{Root: dir, PasswordHash: "x", SessionSecret: "s"},
		activity: newActivityStore(dir),
		shares:   newShareStore(dir),
		tokens:   newTokenStore(dir),
		limiter:  newLoginLimiter(),
	}
	a.backups = newBackupManager(dir, func() string { return dir })
	h := a.routes()

	value := newTokenValue()
	a.tokens.add(&APIToken{
		ID: "ro", Name: "leitor", Hash: tokenHash(value),
		Prefix: displayPrefix(value), Scopes: []string{scopeFilesRead},
		CreatedAt: nowForTest(),
	})

	// Leitura: deve passar.
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=", nil)
	req.Header.Set("Authorization", "Bearer "+value)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("leitura com token read deveria funcionar, veio %d: %s", rec.Code, rec.Body.String())
	}

	// Escrita: deve ser barrada por escopo, não deixada passar.
	req2 := httptest.NewRequest(http.MethodPost, "/api/mkdir", nil)
	req2.Header.Set("Authorization", "Bearer "+value)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("escrita com token read deveria dar 403, veio %d: %s", rec2.Code, rec2.Body.String())
	}
}

// TestNoCredentialRejected confirma que nada passa sem credencial.
func TestNoCredentialRejected(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		home:     dir,
		cfg:      Config{Root: dir, PasswordHash: "x", SessionSecret: "s"},
		activity: newActivityStore(dir),
		shares:   newShareStore(dir),
		tokens:   newTokenStore(dir),
		limiter:  newLoginLimiter(),
	}
	a.backups = newBackupManager(dir, func() string { return dir })
	h := a.routes()

	for _, path := range []string{"/api/files", "/api/settings", "/api/tokens"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s sem credencial deveria dar 401, veio %d", path, rec.Code)
		}
	}
}
