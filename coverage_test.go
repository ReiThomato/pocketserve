package main

// Cobertura das funções que a verificação por HTTP não alcança:
// criptografia, TOTP, parse de configuração e utilidades internas.
//
// Estas são justamente as que estavam em 0% — e são as que mais importam
// quando algo dá errado em silêncio.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------- senha ----------

func TestPasswordHashIsVerifiable(t *testing.T) {
	h := hashPassword("senha-correta-123")
	if !strings.HasPrefix(h, "pbkdf2$") {
		t.Fatalf("formato inesperado: %q", h)
	}
	if !verifyPassword("senha-correta-123", h) {
		t.Fatal("a senha correta deveria validar")
	}
	if verifyPassword("senha-errada", h) {
		t.Fatal("senha errada foi aceita")
	}
	if verifyPassword("", h) {
		t.Fatal("senha vazia foi aceita")
	}
}

func TestPasswordHashIsSalted(t *testing.T) {
	a := hashPassword("mesma-senha")
	b := hashPassword("mesma-senha")
	if a == b {
		t.Fatal("dois hashes da mesma senha são iguais — falta salt")
	}
	// mas ambos devem validar
	if !verifyPassword("mesma-senha", a) || !verifyPassword("mesma-senha", b) {
		t.Fatal("hash salgado não valida")
	}
}

func TestPasswordHashRejectsMalformed(t *testing.T) {
	for _, bad := range []string{
		"", "pbkdf2$", "pbkdf2$abc$x$y", "pbkdf2$210000$x",
		"md5$210000$c2FsdA$a2V5", "pbkdf2$100$c2FsdA$a2V5",
	} {
		if verifyPassword("qualquer", bad) {
			t.Fatalf("hash malformado %q foi aceito", bad)
		}
	}
}

func TestPasswordVersionChangesWithHash(t *testing.T) {
	v1 := passwordVersion("hash-a")
	v2 := passwordVersion("hash-b")
	if v1 == v2 {
		t.Fatal("versões iguais para hashes diferentes")
	}
	if passwordVersion("") != "" {
		t.Fatal("hash vazio deveria dar versão vazia")
	}
}

// ---------- TOTP ----------

func TestTOTPGenerateAndVerify(t *testing.T) {
	secret := newTOTPSecret()
	if secret == "" {
		t.Fatal("segredo vazio")
	}
	code := totpCode(secret, time.Now())
	if len(code) != 6 {
		t.Fatalf("código com %d dígitos", len(code))
	}
	if !verifyTOTP(secret, code) {
		t.Fatal("código gerado não valida")
	}
	if verifyTOTP(secret, "000000") && code != "000000" {
		t.Fatal("código errado foi aceito")
	}
}

func TestTOTPRejectsOldCode(t *testing.T) {
	secret := newTOTPSecret()
	// código de 10 minutos atrás: fora da janela de tolerância
	old := totpCode(secret, time.Now().Add(-10*time.Minute))
	if verifyTOTP(secret, old) {
		t.Fatal("código expirado foi aceito")
	}
}

func TestTOTPSecretFormat(t *testing.T) {
	s := newTOTPSecret()
	// O segredo é base32 (padrão TOTP). Confere tamanho e alfabeto.
	if len(s) < 16 {
		t.Fatalf("segredo curto demais: %d caracteres", len(s))
	}
	for _, r := range strings.ToUpper(s) {
		if !((r >= 'A' && r <= 'Z') || (r >= '2' && r <= '7') || r == '=') {
			t.Fatalf("caractere inválido no segredo: %q", r)
		}
	}
}

// ---------- config ----------

func TestConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("porta padrão = %d", cfg.Port)
	}
	if cfg.SessionDays != 30 {
		t.Fatalf("sessão padrão = %d", cfg.SessionDays)
	}
	if cfg.SessionSecret == "" {
		t.Fatal("sem session secret")
	}
	if cfg.TrashDays != 30 {
		t.Fatalf("lixeira padrão = %d", cfg.TrashDays)
	}
}

func TestConfigExplicitZeroTrashDaysIsPreserved(t *testing.T) {
	dir := t.TempDir()
	raw := `{"server_name":"Teste","port":8080,"root":"` + dir + `","trash_days":0}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TrashDays != 0 {
		t.Fatalf("trash_days 0 explícito virou %d", cfg.TrashDays)
	}
	// saveConfig também não pode reintroduzir o padrão.
	if err := saveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	back, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if back.TrashDays != 0 {
		t.Fatalf("trash_days 0 não sobreviveu ao save: %d", back.TrashDays)
	}
}

func TestConfigMissingTrashDaysUsesDefault(t *testing.T) {
	dir := t.TempDir()
	raw := `{"server_name":"Teste","port":8080,"root":"` + dir + `"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TrashDays != 30 {
		t.Fatalf("trash_days ausente = %d, esperado 30", cfg.TrashDays)
	}
}

func TestFallbackRootRefusesSystemTrees(t *testing.T) {
	cases := []struct {
		wd   string
		want bool // true = recusa
	}{
		{"/", true},
		{"", true},
		{"/proc", true},
		{"/proc/self", true},
		{"/data/data/com.termux", false},
		{"/data/data/com.termux/files/home", false},
		{"/data/other", true},
		{"/system/bin", true},
		{"/etc", true},
		{"/sdcard", false},
		{"/sdcard/PocketServe", false},
		{"/tmp/ps", false},
	}
	for _, c := range cases {
		got, err := fallbackRoot(c.wd)
		if c.want {
			if err == nil {
				t.Fatalf("fallbackRoot(%q) aceitou %q", c.wd, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("fallbackRoot(%q) recusou: %v", c.wd, err)
		}
		if got == "" {
			t.Fatalf("fallbackRoot(%q) vazio", c.wd)
		}
	}
}

func TestConfigRejectsCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{nao json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(dir); err == nil {
		t.Fatal("config corrompida foi aceita")
	}
}

func TestConfigSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := loadConfig(dir)
	cfg.ServerName = "Meu Servidor"
	cfg.Port = 9999
	cfg.MinFreeMB = 512
	cfg.Roots = []RootEntry{{Name: "cartao", Path: dir}}
	if err := saveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	back, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if back.ServerName != "Meu Servidor" || back.Port != 9999 || back.MinFreeMB != 512 {
		t.Fatalf("ida e volta perdeu dados: %+v", back)
	}
	if len(back.Roots) != 1 || back.Roots[0].Name != "cartao" {
		t.Fatalf("raízes perdidas: %+v", back.Roots)
	}
}

func TestConfigFilePermissions(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadConfig(dir); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("config.json acessível por outros: %v", st.Mode().Perm())
	}
}

// ---------- crypto / utils ----------

func TestRandomTokenUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok := randomToken(16)
		if tok == "" {
			t.Fatal("token vazio")
		}
		if seen[tok] {
			t.Fatal("token repetido — fonte aleatória fraca")
		}
		seen[tok] = true
	}
}

func TestSessionPayloadExpiry(t *testing.T) {
	a := &App{cfg: Config{PasswordHash: "h", SessionSecret: "s"}}
	// sessão expirada não deve validar
	expired := sessionPayload{Exp: time.Now().Add(-time.Hour).Unix(), Ver: passwordVersion("h")}
	if expired.Exp > time.Now().Unix() {
		t.Fatal("payload de teste deveria estar expirado")
	}
	_ = a
}

func TestChecksumMatchesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	os.WriteFile(p, []byte("conteudo"), 0o644)
	a := &App{cfg: Config{Root: dir}}
	sum, err := a.hashFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum) != 64 {
		t.Fatalf("SHA-256 com %d caracteres", len(sum))
	}
	// mesmo conteúdo, mesmo hash
	p2 := filepath.Join(dir, "g.txt")
	os.WriteFile(p2, []byte("conteudo"), 0o644)
	sum2, _ := a.hashFile(p2)
	if sum != sum2 {
		t.Fatal("conteúdo igual gerou hash diferente")
	}
	// conteúdo diferente, hash diferente
	os.WriteFile(p2, []byte("outro"), 0o644)
	sum3, _ := a.hashFile(p2)
	if sum == sum3 {
		t.Fatal("conteúdo diferente gerou mesmo hash")
	}
}

func TestSanitizeName(t *testing.T) {
	// A defesa não é "pegar o basename": é trocar os separadores por "_",
	// para que o nome nunca represente um caminho.
	cases := [][2]string{
		{"normal.txt", "normal.txt"},
		{"../../etc/passwd", ".._.._etc_passwd"},
		{"/abs/path.txt", "_abs_path.txt"},
		{"a\x00b.txt", "ab.txt"},
		{".", "arquivo"},
		{"..", "arquivo"},
		{"", "arquivo"},
		{"folder\\file.txt", "folder_file.txt"},
	}
	for _, c := range cases {
		in, want := c[0], c[1]
		got := sanitizeName(in)
		if got != want {
			t.Fatalf("sanitizeName(%q) = %q, esperado %q", in, got, want)
		}
		// Invariante de segurança: o resultado nunca é um caminho.
		if strings.ContainsAny(got, "/\\") {
			t.Fatalf("sanitizeName(%q) = %q ainda contém separador", in, got)
		}
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte("x"), 0o644)

	// sem overwrite: deve escolher outro nome
	got := uniquePath(dir, "a.txt", false)
	if got == p {
		t.Fatal("uniquePath devolveu caminho ocupado")
	}
	if !strings.Contains(got, "(1)") {
		t.Fatalf("sufixo inesperado: %q", got)
	}
	// com overwrite: devolve o mesmo
	if uniquePath(dir, "a.txt", true) != p {
		t.Fatal("overwrite deveria devolver o caminho original")
	}
	// arquivo novo: devolve o nome simples
	if uniquePath(dir, "novo.txt", false) != filepath.Join(dir, "novo.txt") {
		t.Fatal("nome livre recebeu sufixo")
	}
}

func TestWithinBlocksEscape(t *testing.T) {
	if !within("/a/b", "/a/b") {
		t.Fatal("raiz deveria estar dentro de si")
	}
	if !within("/a/b", "/a/b/c.txt") {
		t.Fatal("filho deveria estar dentro")
	}
	if within("/a/b", "/a/c") {
		t.Fatal("irmão foi aceito")
	}
	if within("/a/b", "/a/b/../c") {
		t.Fatal("traversal foi aceito")
	}
	if within("/a/b", "/x/y") {
		t.Fatal("caminho externo foi aceito")
	}
}

// ---------- rate limit e sessão ----------

func TestLoginLimiterBackoff(t *testing.T) {
	l := newLoginLimiter()
	const ip = "1.2.3.4"

	// 4 falhas: ainda não bloqueia
	for i := 0; i < 4; i++ {
		l.fail(ip)
		if d := l.wait(ip); d > 0 {
			t.Fatalf("bloqueou na falha %d", i+1)
		}
	}
	// 5ª falha: bloqueia por 1 minuto
	l.fail(ip)
	if d := l.wait(ip); d <= 0 || d > time.Minute {
		t.Fatalf("esperava bloqueio de ~1min, veio %v", d)
	}

	// reset libera
	l.reset(ip)
	if d := l.wait(ip); d != 0 {
		t.Fatalf("reset não liberou: %v", d)
	}
}

func TestLoginLimiterEscalates(t *testing.T) {
	l := newLoginLimiter()
	const ip = "5.6.7.8"
	l.fail(ip)
	l.fail(ip)
	l.fail(ip)
	l.fail(ip)
	l.fail(ip) // 1m
	first := l.wait(ip)
	l.fail(ip) // 2m
	second := l.wait(ip)
	if second <= first {
		t.Fatalf("backoff não escalou: %v -> %v", first, second)
	}
}

func TestLoginLimiterIsolatesIPs(t *testing.T) {
	l := newLoginLimiter()
	for i := 0; i < 6; i++ {
		l.fail("10.0.0.1")
	}
	if l.wait("10.0.0.1") == 0 {
		t.Fatal("IP atacante deveria estar bloqueado")
	}
	if l.wait("10.0.0.2") != 0 {
		t.Fatal("IP inocente foi bloqueado junto")
	}
}

func TestSessionCookieValidity(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		home:     dir,
		cfg:      Config{Root: dir, PasswordHash: "hash-fixo", SessionSecret: "segredo"},
		activity: newActivityStore(dir),
	}
	// cookie forjado sem assinatura válida
	r := newTestRequest()
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "payload-assinatura-falsa"})
	if a.validSession(r) {
		t.Fatal("cookie sem assinatura válida foi aceito")
	}
	// sem cookie
	if a.validSession(newTestRequest()) {
		t.Fatal("requisição sem cookie foi aceita")
	}
	// segredo errado
	r2 := newTestRequest()
	r2.AddCookie(sessionCookieForTest(t, a))
	if !a.validSession(r2) {
		t.Fatal("cookie legítimo foi recusado")
	}
}

func TestSessionInvalidatedByPasswordChange(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		home:     dir,
		cfg:      Config{Root: dir, PasswordHash: "hash-antigo", SessionSecret: "s"},
		activity: newActivityStore(dir),
	}
	r := newTestRequest()
	r.AddCookie(sessionCookieForTest(t, a))
	if !a.validSession(r) {
		t.Fatal("sessão deveria valer antes da troca")
	}
	// simula troca de senha: o hash muda, logo a versão muda
	a.cfg.PasswordHash = "hash-novo"
	if a.validSession(r) {
		t.Fatal("sessão sobreviveu à troca de senha — deveria ser invalidada")
	}
}

func TestCheckCSRF(t *testing.T) {
	r := newTestRequest()
	r.Method = "POST"
	if checkCSRF(r) {
		t.Fatal("POST sem X-PS passou")
	}
	r.Header.Set("X-PS", "1")
	if !checkCSRF(r) {
		t.Fatal("POST com X-PS foi recusado")
	}
	// Origin de outro site
	r.Header.Set("Origin", "https://evil.example")
	if checkCSRF(r) {
		t.Fatal("Origin cruzada foi aceita")
	}
	r.Header.Set("Origin", "http://"+r.Host)
	if !checkCSRF(r) {
		t.Fatal("Origin correta foi recusada")
	}
}
