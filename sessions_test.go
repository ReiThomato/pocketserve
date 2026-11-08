package main

// Testes da revogação de sessões.
//
// Cobrem os seis casos exigidos:
//   1. login seguido de logout
//   2. reutilização do cookie antigo
//   3. acesso a endpoint protegido após logout
//   4. persistência da revogação após reinício
//   5. expiração e limpeza das entradas
//   6. logout de uma sessão sem afetar as outras

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// revokeApp monta um App pronto para exercitar sessões.
func revokeApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	a := &App{
		home:        dir,
		cfg:         Config{Root: dir, PasswordHash: "hash-fixo", SessionSecret: "segredo", SessionDays: 30},
		activity:    newActivityStore(dir),
		shares:      newShareStore(dir),
		tokens:      newTokenStore(dir),
		limiter:     newLoginLimiter(),
		spaceAlerts: newSpaceAlerter(),
		revoker:     newSessionRevoker(dir),
	}
	a.backups = newBackupManager(dir, func() string { return dir })
	return a, dir
}

// loginCookie gera um cookie de sessão como o handleLogin faria.
func loginCookie(t *testing.T, a *App) string {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	a.setSession(w, r, 0)
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			return c.Value
		}
	}
	t.Fatal("setSession não emitiu cookie")
	return ""
}

// reqWithCookie monta uma requisição autenticada por cookie.
func reqWithCookie(method, path, value string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: value})
	return r
}

// ---------- 1. login seguido de logout ----------

func TestLogoutRevokesSession(t *testing.T) {
	a, _ := revokeApp(t)
	cookie := loginCookie(t, a)

	// antes do logout: válida
	if !a.validSession(reqWithCookie("GET", "/", cookie)) {
		t.Fatal("sessão recém-criada deveria ser válida")
	}

	// faz o logout pelo handler real (com CSRF)
	req := reqWithCookie(http.MethodPost, "/api/logout", cookie)
	req.Header.Set("X-PS", "1")
	rec := httptest.NewRecorder()
	a.handleLogout(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout devolveu %d", rec.Code)
	}

	// depois do logout: inválida
	if a.validSession(reqWithCookie("GET", "/", cookie)) {
		t.Fatal("sessão continua válida após logout")
	}
}

// ---------- 2. reutilização do cookie antigo ----------

func TestReusedCookieAfterLogoutIsRejected(t *testing.T) {
	a, _ := revokeApp(t)
	cookie := loginCookie(t, a)

	req := reqWithCookie(http.MethodPost, "/api/logout", cookie)
	req.Header.Set("X-PS", "1")
	a.handleLogout(httptest.NewRecorder(), req)

	// Reutiliza o MESMO cookie, como faria um atacante que o copiou.
	for i := 0; i < 3; i++ {
		if a.validSession(reqWithCookie("GET", "/", cookie)) {
			t.Fatalf("cookie antigo aceito na tentativa %d", i+1)
		}
	}
}

// ---------- 3. endpoint protegido após logout ----------

func TestProtectedEndpointAfterLogout(t *testing.T) {
	a, _ := revokeApp(t)
	cookie := loginCookie(t, a)
	h := a.routes()

	rotas := []struct{ method, path string }{
		{http.MethodGet, "/api/files"},
		{http.MethodGet, "/api/settings"},
		{http.MethodGet, "/api/tokens"},
		{http.MethodGet, "/api/duplicates"},
	}

	// antes: todas passam da autenticação
	for _, rt := range rotas {
		req := reqWithCookie(rt.method, rt.path, cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized {
			t.Fatalf("%s deveria estar autenticado antes do logout", rt.path)
		}
	}

	// logout
	req := reqWithCookie(http.MethodPost, "/api/logout", cookie)
	req.Header.Set("X-PS", "1")
	h.ServeHTTP(httptest.NewRecorder(), req)

	// depois: todas dão 401
	for _, rt := range rotas {
		req := reqWithCookie(rt.method, rt.path, cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s devolveu %d após logout, esperado 401", rt.path, rec.Code)
		}
	}
}

// ---------- 4. persistência após reinício ----------

func TestRevocationSurvivesRestart(t *testing.T) {
	a, dir := revokeApp(t)
	cookie := loginCookie(t, a)

	req := reqWithCookie(http.MethodPost, "/api/logout", cookie)
	req.Header.Set("X-PS", "1")
	a.handleLogout(httptest.NewRecorder(), req)

	// o arquivo precisa existir
	path := filepath.Join(dir, revokedSessionsFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("arquivo de revogação ausente: %v", err)
	}
	// e não pode conter o ID em claro
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), extractSessionID(t, cookie)) {
		t.Fatal("o arquivo guarda o ID da sessão em claro")
	}

	// simula reinício: novo App, mesma pasta
	b := &App{
		home:        dir,
		cfg:         a.cfg,
		activity:    newActivityStore(dir),
		shares:      newShareStore(dir),
		tokens:      newTokenStore(dir),
		limiter:     newLoginLimiter(),
		spaceAlerts: newSpaceAlerter(),
		revoker:     newSessionRevoker(dir),
	}
	if b.validSession(reqWithCookie("GET", "/", cookie)) {
		t.Fatal("após reiniciar, o cookie revogado voltou a valer")
	}
}

// extractSessionID lê o ID do payload (só para o teste de sigilo acima).
func extractSessionID(t *testing.T, cookie string) string {
	t.Helper()
	payload, _, ok := strings.Cut(cookie, ".")
	if !ok {
		t.Fatal("cookie malformado")
	}
	raw, err := decodeBase64URL(payload)
	if err != nil {
		t.Fatal(err)
	}
	var p sessionPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	return p.ID
}

// ---------- 5. expiração e limpeza ----------

func TestRevocationExpiresAndIsPruned(t *testing.T) {
	a, _ := revokeApp(t)
	const id = "sessao-qualquer"

	// revogação já vencida não entra
	a.revoker.revoke(id, time.Now().Add(-time.Minute))
	if a.revoker.count() != 0 {
		t.Fatal("revogação já expirada foi registrada")
	}

	// revogação válida entra
	a.revoker.revoke(id, time.Now().Add(time.Hour))
	if a.revoker.count() != 1 {
		t.Fatal("revogação válida não foi registrada")
	}
	if !a.revoker.isRevoked(id) {
		t.Fatal("sessão revogada deveria constar")
	}

	// força expiração e roda a limpeza
	hash := sessionIDHash(id)
	a.revoker.mu.Lock()
	a.revoker.items[hash] = time.Now().Add(-time.Second)
	a.revoker.mu.Unlock()

	if n := a.revoker.prune(); n != 1 {
		t.Fatalf("prune removeu %d, esperado 1", n)
	}
	if a.revoker.count() != 0 {
		t.Fatal("entrada expirada permaneceu após prune")
	}
	if a.revoker.isRevoked(id) {
		t.Fatal("sessão vencida ainda consta como revogada")
	}
}

func TestRevocationFileDropsExpiredOnLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, revokedSessionsFile)

	// arquivo com uma válida e uma vencida
	list := []revokedSession{
		{IDHash: "valida", Expires: time.Now().Add(time.Hour)},
		{IDHash: "vencida", Expires: time.Now().Add(-time.Hour)},
	}
	b, _ := json.MarshalIndent(list, "", "  ")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}

	r := newSessionRevoker(dir)
	if r.count() != 1 {
		t.Fatalf("esperava 1 entrada após carga, veio %d", r.count())
	}
	// o arquivo deve ter sido reescrito sem a vencida
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "vencida") {
		t.Fatal("entrada vencida continuou no arquivo")
	}
}

func TestRevocationFilePermissions(t *testing.T) {
	dir := t.TempDir()
	r := newSessionRevoker(dir)
	r.revoke("x", time.Now().Add(time.Hour))
	st, err := os.Stat(filepath.Join(dir, revokedSessionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("arquivo de revogação acessível por outros: %v", st.Mode().Perm())
	}
}

// ---------- 6. logout isolado por sessão ----------

func TestLogoutDoesNotAffectOtherSessions(t *testing.T) {
	a, _ := revokeApp(t)

	celular := loginCookie(t, a)
	notebook := loginCookie(t, a)
	if celular == notebook {
		t.Fatal("duas sessões geraram cookie idêntico — falta ID único")
	}

	// encerra só o notebook
	req := reqWithCookie(http.MethodPost, "/api/logout", notebook)
	req.Header.Set("X-PS", "1")
	a.handleLogout(httptest.NewRecorder(), req)

	if a.validSession(reqWithCookie("GET", "/", notebook)) {
		t.Fatal("sessão encerrada continua válida")
	}
	if !a.validSession(reqWithCookie("GET", "/", celular)) {
		t.Fatal("logout de uma sessão derrubou a outra")
	}
}

// Cada login precisa de um ID diferente, mesmo no mesmo segundo.
func TestEachLoginHasUniqueID(t *testing.T) {
	a, _ := revokeApp(t)
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		id := extractSessionID(t, loginCookie(t, a))
		if id == "" {
			t.Fatal("sessão sem ID")
		}
		if seen[id] {
			t.Fatal("ID de sessão repetido")
		}
		seen[id] = true
	}
}

// ---------- compatibilidade ----------

// Cookie emitido antes desta versão (sem campo ID) continua aceito até expirar.
func TestLegacyCookieWithoutIDStillWorks(t *testing.T) {
	a, _ := revokeApp(t)
	p := sessionPayload{
		Exp: time.Now().Add(time.Hour).Unix(),
		Ver: passwordVersion(a.cfg.PasswordHash),
		// ID vazio, como nos cookies da 1.10
	}
	raw, _ := json.Marshal(p)
	cookie := signSessionPayload(t, raw, a.cfg.SessionSecret)

	if !a.validSession(reqWithCookie("GET", "/", cookie)) {
		t.Fatal("cookie legado deveria continuar válido")
	}
}

// Trocar a senha continua invalidando todas as sessões.
func TestPasswordChangeStillInvalidatesAll(t *testing.T) {
	a, _ := revokeApp(t)
	c1 := loginCookie(t, a)
	c2 := loginCookie(t, a)

	a.cfg.PasswordHash = "hash-novo"

	if a.validSession(reqWithCookie("GET", "/", c1)) {
		t.Fatal("sessão sobreviveu à troca de senha")
	}
	if a.validSession(reqWithCookie("GET", "/", c2)) {
		t.Fatal("segunda sessão sobreviveu à troca de senha")
	}
}

// A revogação não pode afetar sessões novas do mesmo usuário.
func TestNewLoginAfterLogoutWorks(t *testing.T) {
	a, _ := revokeApp(t)
	antigo := loginCookie(t, a)

	req := reqWithCookie(http.MethodPost, "/api/logout", antigo)
	req.Header.Set("X-PS", "1")
	a.handleLogout(httptest.NewRecorder(), req)

	novo := loginCookie(t, a)
	if !a.validSession(reqWithCookie("GET", "/", novo)) {
		t.Fatal("login após logout deveria funcionar")
	}
	if a.validSession(reqWithCookie("GET", "/", antigo)) {
		t.Fatal("cookie antigo voltou a valer com o login novo")
	}
}

// ---------- limite de crescimento ----------

func TestRevocationListIsBounded(t *testing.T) {
	a, _ := revokeApp(t)

	// 500 revogações com validade curta
	for i := 0; i < 500; i++ {
		a.revoker.revoke("s"+itoa(i), time.Now().Add(time.Hour))
	}
	if n := a.revoker.count(); n != 500 {
		t.Fatalf("esperava 500 entradas, veio %d", n)
	}

	// todas vencem; a limpeza deve zerar
	a.revoker.mu.Lock()
	for k := range a.revoker.items {
		a.revoker.items[k] = time.Now().Add(-time.Second)
	}
	a.revoker.mu.Unlock()

	a.revoker.prune()
	if n := a.revoker.count(); n != 0 {
		t.Fatalf("lista não foi limpa: %d entradas", n)
	}
}
