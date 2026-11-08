package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// sessionCookieForTest fabrica um cookie de sessão válido para o App dado,
// usando o mesmo formato de setSession. Serve para testar rotas protegidas
// sem passar pelo login.
func sessionCookieForTest(t *testing.T, a *App) *http.Cookie {
	t.Helper()
	cfg := a.config()
	p := sessionPayload{
		Exp: time.Now().Add(time.Hour).Unix(),
		Ver: passwordVersion(cfg.PasswordHash),
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(cfg.SessionSecret))
	mac.Write([]byte(payload))
	value := payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return &http.Cookie{Name: sessionCookie, Value: value}
}

// postJSON dispara um POST autenticado por sessão e devolve o recorder.
func postJSON(t *testing.T, a *App, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-PS", "1")
	req.AddCookie(sessionCookieForTest(t, a))
	rec := httptest.NewRecorder()
	a.routes().ServeHTTP(rec, req)
	return rec
}

// newRecorder evita repetir httptest.NewRecorder nos testes.
func newRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

// timeSleepShort separa snapshots criados no mesmo segundo. O ID usa
// nanossegundos, então não há colisão; isto só evita ordenação instável.
func timeSleepShort() { time.Sleep(time.Millisecond) }

// helpers de arquivo usados pelos testes de investigação.
func writeFileForTest(root, rel, content string) error {
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

func statForTest(root, rel string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(root, rel))
}

func removeForTest(root, rel string) error {
	return os.RemoveAll(filepath.Join(root, rel))
}

func mkdirForTest(root, rel string) error {
	return os.MkdirAll(filepath.Join(root, rel), 0o755)
}

func readFileForTest(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

// signSessionPayload assina um payload cru como setSession faz.
// Usado para montar cookies "legados" (sem ID) nos testes.
func signSessionPayload(t *testing.T, raw []byte, secret string) string {
	t.Helper()
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// decodeBase64URL decodifica o payload de um cookie.
func decodeBase64URL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
