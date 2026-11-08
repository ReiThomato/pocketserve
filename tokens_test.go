package main

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// newTestRequest cria uma requisição mínima para exercitar os helpers de auth.
func newTestRequest() *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/api/files", nil)
	return r
}

func nowForTest() time.Time { return time.Now() }

func newTestTokenApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	a := &App{
		home:     dir,
		cfg:      Config{Root: dir, PasswordHash: "x", SessionSecret: "s"},
		activity: newActivityStore(dir),
		tokens:   newTokenStore(dir),
	}
	return a
}

func TestTokenCreateAndLookup(t *testing.T) {
	a := newTestTokenApp(t)

	value := newTokenValue()
	if !strings.HasPrefix(value, "ps_") {
		t.Fatalf("token sem prefixo esperado: %q", value)
	}
	// 32 bytes em base64url = 43 caracteres + "ps_"
	if len(value) != len("ps_")+43 {
		t.Fatalf("tamanho inesperado do token: %d", len(value))
	}

	tok := &APIToken{
		ID: "id1", Name: "agente", Hash: tokenHash(value),
		Prefix: displayPrefix(value), Scopes: []string{scopeFilesRead},
		CreatedAt: time.Now(),
	}
	a.tokens.add(tok)

	got := a.tokens.lookup(value)
	if got == nil {
		t.Fatal("lookup não encontrou um token recém-criado")
	}
	if got.ID != "id1" {
		t.Fatalf("id = %q", got.ID)
	}

	// O hash nunca deve ser a chave em claro.
	if got.Hash == value {
		t.Fatal("hash não pode ser igual ao valor do token")
	}
	if strings.Contains(got.Hash, value) {
		t.Fatal("hash contém o token em claro")
	}
}

func TestTokenWrongValueRejected(t *testing.T) {
	a := newTestTokenApp(t)
	value := newTokenValue()
	a.tokens.add(&APIToken{
		ID: "id1", Hash: tokenHash(value), Scopes: []string{scopeFilesRead},
		CreatedAt: time.Now(),
	})

	if got := a.tokens.lookup("ps_errado"); got != nil {
		t.Fatal("valor errado foi aceito")
	}
	// Um prefixo válido mas sufixo trocado também precisa falhar.
	tampered := value[:len(value)-1] + "X"
	if got := a.tokens.lookup(tampered); got != nil {
		t.Fatal("token adulterado foi aceito")
	}
}

func TestTokenScopes(t *testing.T) {
	read := &APIToken{Scopes: []string{scopeFilesRead}}
	if !read.allows(scopeFilesRead) {
		t.Fatal("deveria permitir files:read")
	}
	if read.allows(scopeFilesWrite) {
		t.Fatal("não deveria permitir files:write")
	}
	if read.allows(scopeAdmin) {
		t.Fatal("não deveria permitir admin")
	}

	admin := &APIToken{Scopes: []string{scopeAdmin}}
	for _, s := range allScopes {
		if !admin.allows(s) {
			t.Fatalf("admin deveria permitir %q", s)
		}
	}
}

func TestTokenExpiry(t *testing.T) {
	a := newTestTokenApp(t)
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	expired := newTokenValue()
	a.tokens.add(&APIToken{
		ID: "exp", Hash: tokenHash(expired), Scopes: []string{scopeFilesRead},
		CreatedAt: time.Now(), ExpiresAt: &past,
	})
	if got := a.tokens.lookup(expired); got != nil {
		t.Fatal("token expirado foi aceito")
	}

	valid := newTokenValue()
	a.tokens.add(&APIToken{
		ID: "ok", Hash: tokenHash(valid), Scopes: []string{scopeFilesRead},
		CreatedAt: time.Now(), ExpiresAt: &future,
	})
	if got := a.tokens.lookup(valid); got == nil {
		t.Fatal("token dentro da validade foi recusado")
	}
}

func TestTokenRevokeIsImmediate(t *testing.T) {
	a := newTestTokenApp(t)
	value := newTokenValue()
	a.tokens.add(&APIToken{
		ID: "id1", Hash: tokenHash(value), Scopes: []string{scopeFilesRead},
		CreatedAt: time.Now(),
	})
	if a.tokens.lookup(value) == nil {
		t.Fatal("token deveria funcionar antes de revogar")
	}
	if !a.tokens.revoke("id1") {
		t.Fatal("revoke devolveu falso")
	}
	if got := a.tokens.lookup(value); got != nil {
		t.Fatal("token revogado continua válido — revogação precisa ser imediata")
	}
	// Revogar duas vezes não deve ser reportado como sucesso.
	if a.tokens.revoke("id1") {
		t.Fatal("revogar duas vezes deveria devolver falso")
	}
}

func TestTokenNotShownInList(t *testing.T) {
	a := newTestTokenApp(t)
	value := newTokenValue()
	a.tokens.add(&APIToken{
		ID: "id1", Name: "x", Hash: tokenHash(value), Prefix: displayPrefix(value),
		Scopes: []string{scopeFilesRead}, CreatedAt: time.Now(),
	})
	pub := a.tokens.list()[0].public()
	if _, ok := pub["hash"]; ok {
		t.Fatal("a visão pública não pode conter o hash")
	}
	if _, ok := pub["token"]; ok {
		t.Fatal("a visão pública não pode conter o valor do token")
	}
	if pub["prefix"] != value[:12] {
		t.Fatalf("prefixo = %v", pub["prefix"])
	}
}

func TestBearerParsing(t *testing.T) {
	cases := map[string]string{
		"Bearer ps_abc":  "ps_abc",
		"bearer ps_abc":  "ps_abc",
		"BEARER  ps_abc": "ps_abc",
		"":               "",
		"Basic abc":      "",
		"Bearer":         "",
	}
	for header, want := range cases {
		r := newTestRequest()
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		if got := bearerToken(r); got != want {
			t.Fatalf("bearerToken(%q) = %q, esperado %q", header, got, want)
		}
	}
}

func TestTokenStorePersists(t *testing.T) {
	dir := t.TempDir()
	s1 := newTokenStore(dir)
	value := newTokenValue()
	s1.add(&APIToken{
		ID: "id1", Name: "n", Hash: tokenHash(value), Prefix: displayPrefix(value),
		Scopes: []string{scopeFilesRead, scopeFilesWrite}, CreatedAt: time.Now(),
	})

	// Recarrega do disco: o token precisa continuar válido após reiniciar.
	s2 := newTokenStore(dir)
	got := s2.lookup(value)
	if got == nil {
		t.Fatal("token não sobreviveu ao restart")
	}
	if len(got.Scopes) != 2 {
		t.Fatalf("escopos perdidos: %v", got.Scopes)
	}
}

func TestTokenRejectsUnknownScope(t *testing.T) {
	for _, s := range allScopes {
		if !validScope(s) {
			t.Fatalf("escopo declarado é inválido: %q", s)
		}
	}
	for _, bad := range []string{"", "admin:all", "files:*", "root"} {
		if validScope(bad) {
			t.Fatalf("escopo %q não deveria ser aceito", bad)
		}
	}
}
