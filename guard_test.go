package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newGuardApp monta um App com raiz temporária e reserva configurada.
func newGuardApp(t *testing.T, minFreeMB int64) *App {
	t.Helper()
	dir := t.TempDir()
	a := &App{
		home:        dir,
		cfg:         Config{Root: dir, PasswordHash: "x", SessionSecret: "s", MinFreeMB: minFreeMB},
		activity:    newActivityStore(dir),
		shares:      newShareStore(dir),
		tokens:      newTokenStore(dir),
		limiter:     newLoginLimiter(),
		spaceAlerts: newSpaceAlerter(),
	}
	a.backups = newBackupManager(dir, func() string { return dir })
	return a
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{2 * 1024 * 1024 * 1024, "2.0 GB"},
		{202 * 1024 * 1024 * 1024, "202 GB"},
		{-5, "0 B"},
	}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Fatalf("humanBytes(%d) = %q, esperado %q", c.in, got, c.want)
		}
	}
}

// A reserva precisa bloquear a escrita antes de encher o disco, não depois.
func TestCheckSpaceBlocksBelowReserve(t *testing.T) {
	dir := t.TempDir()

	// Reserva absurda (1 TB) num volume que não tem isso livre: qualquer
	// escrita precisa ser recusada.
	a := &App{cfg: Config{Root: dir, MinFreeMB: 1024 * 1024}}
	if _, _, err := a.checkSpace(dir, 10); err == nil {
		t.Fatal("reserva maior que o volume deveria recusar a escrita")
	}

	// Sem reserva, espaço pequeno passa.
	b := &App{cfg: Config{Root: dir, MinFreeMB: 0}}
	if _, _, err := b.checkSpace(dir, 1024); err != nil {
		t.Fatalf("sem reserva deveria aceitar: %v", err)
	}
}

// freeSpace precisa funcionar mesmo quando a pasta de destino ainda não existe.
func TestFreeSpaceOnMissingDir(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "ainda", "nao", "existe")
	free, err := freeSpace(missing)
	if err != nil {
		t.Fatalf("freeSpace em caminho inexistente falhou: %v", err)
	}
	if free <= 0 {
		t.Fatalf("espaço livre inválido: %d", free)
	}
}

func TestSpaceMessageMentionsReserve(t *testing.T) {
	a := &App{cfg: Config{MinFreeMB: 2048}}
	msg := a.spaceMessage(1<<30, 0)
	if !strings.Contains(msg, "reserva") {
		t.Fatalf("mensagem deveria citar a reserva: %q", msg)
	}

	b := &App{cfg: Config{MinFreeMB: 0}}
	msg2 := b.spaceMessage(1<<30, 0)
	if strings.Contains(msg2, "reserva") {
		t.Fatalf("sem reserva não deveria falar de reserva: %q", msg2)
	}
}

// O upload precisa devolver 507 quando o volume está abaixo da reserva,
// antes de receber o corpo.
func TestUploadRejectedWhenReserveExceeded(t *testing.T) {
	a := newGuardApp(t, 1024*1024) // reserva de 1 TB
	h := a.routes()

	body := &bytes.Buffer{}
	fmt.Fprintf(body, "--X\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\n\r\n")
	body.WriteString("conteudo")
	fmt.Fprintf(body, "\r\n--X--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/api/upload?path=", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=X")
	req.Header.Set("X-PS", "1")
	req.AddCookie(sessionCookieForTest(t, a))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("esperava 507, veio %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "no_space") {
		t.Fatalf("código de erro esperado no_space: %s", rec.Body.String())
	}
}

// Sem reserva, o mesmo upload precisa funcionar.
func TestUploadWorksWithoutReserve(t *testing.T) {
	a := newGuardApp(t, 0)
	h := a.routes()

	body := &bytes.Buffer{}
	fmt.Fprintf(body, "--X\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\n\r\n")
	body.WriteString("conteudo")
	fmt.Fprintf(body, "\r\n--X--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/api/upload?path=", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=X")
	req.Header.Set("X-PS", "1")
	req.AddCookie(sessionCookieForTest(t, a))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, veio %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(a.cfg.Root, "a.txt")); err != nil {
		t.Fatalf("arquivo não foi gravado: %v", err)
	}
}

// Config antiga (sem o campo) recebe o padrão; zero explícito é respeitado.
func TestMinFreeDefaultOnOldConfig(t *testing.T) {
	dir := t.TempDir()
	old := map[string]any{"server_name": "Antigo", "root": dir, "port": 8080}
	b, _ := json.Marshal(old)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinFreeMB != defaultMinFreeMB {
		t.Fatalf("config antiga deveria receber o padrão %d, veio %d", defaultMinFreeMB, cfg.MinFreeMB)
	}

	// Com o campo presente e zero, deve permanecer zero (desligado).
	withZero := map[string]any{"server_name": "Novo", "root": dir, "port": 8080, "min_free_mb": 0}
	b2, _ := json.Marshal(withZero)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), b2, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg2, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.MinFreeMB != 0 {
		t.Fatalf("zero explícito deveria desligar a reserva, veio %d", cfg2.MinFreeMB)
	}
}
