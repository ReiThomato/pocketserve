package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// TOTP (RFC 6238) com SHA-1, 6 dígitos, janela de 30s — compatível com
// Google Authenticator, Aegis, 1Password, Bitwarden e afins.
//
// O segredo fica no config.json (já protegido por 0600 e pelo sandbox do
// Android). Códigos de recuperação são guardados como hash.

const (
	totpPeriod = 30
	totpDigits = 6
	totpWindow = 1 // aceita o código anterior e o seguinte (relógio dessincronizado)
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() string {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return b32.EncodeToString(b)
}

// totpCode gera o código para um instante específico.
func totpCode(secret string, t time.Time) string {
	key, err := b32.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(key) == 0 {
		return ""
	}
	counter := uint64(t.Unix() / totpPeriod)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[off])&0x7f)<<24 |
		uint32(sum[off+1])<<16 |
		uint32(sum[off+2])<<8 |
		uint32(sum[off+3])
	return fmt.Sprintf("%0*d", totpDigits, code%1000000)
}

// verifyTOTP aceita o código atual e os vizinhos da janela.
func verifyTOTP(secret, code string) bool {
	code = strings.TrimSpace(strings.ReplaceAll(code, " ", ""))
	if len(code) != totpDigits {
		return false
	}
	now := time.Now()
	for d := -totpWindow; d <= totpWindow; d++ {
		want := totpCode(secret, now.Add(time.Duration(d)*totpPeriod*time.Second))
		if want != "" && subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func totpURL(secret, account, issuer string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriod))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

func newRecoveryCodes() ([]string, []string) {
	plain := make([]string, 10)
	hashed := make([]string, 10)
	for i := range plain {
		b := make([]byte, 5)
		_, _ = rand.Read(b)
		code := strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
		if len(code) > 8 {
			code = code[:4] + "-" + code[4:8]
		}
		plain[i] = code
		sum := sha256Hex(code)
		hashed[i] = sum
	}
	return plain, hashed
}

func consumeRecoveryCode(code string, stored []string) (string, bool) {
	norm := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), " ", ""))
	sum := sha256Hex(norm)
	for i, h := range stored {
		if subtle.ConstantTimeCompare([]byte(h), []byte(sum)) == 1 {
			rest := append(append([]string{}, stored[:i]...), stored[i+1:]...)
			return strings.Join(rest, ","), true
		}
	}
	return strings.Join(stored, ","), false
}

// ---------- handlers ----------

func (a *App) handleTOTPStatus(w http.ResponseWriter, r *http.Request) {
	cfg := a.config()
	n := 0
	if cfg.RecoveryCodes != "" {
		n = len(strings.Split(cfg.RecoveryCodes, ","))
	}
	jsonWrite(w, http.StatusOK, map[string]any{
		"enabled":        cfg.TOTPSecret != "",
		"recovery_codes": n,
	})
}

// handleTOTPSetup gera um segredo novo, mas só ativa depois da confirmação.
func (a *App) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	cfg := a.config()
	if cfg.TOTPSecret != "" {
		jsonError(w, http.StatusConflict, "already_enabled", "A verificação em duas etapas já está ativa.")
		return
	}
	secret := newTOTPSecret()
	if secret == "" {
		jsonError(w, http.StatusInternalServerError, "rand_failed", "Não foi possível gerar o segredo.")
		return
	}
	// Guarda como pendente: não protege o login até ser confirmado.
	if err := a.updateConfig(func(c *Config) { c.TOTPPending = secret }); err != nil {
		jsonError(w, http.StatusInternalServerError, "save_failed", "Não foi possível salvar.")
		return
	}
	account := cfg.ServerName
	jsonWrite(w, http.StatusOK, map[string]any{
		"secret": secret,
		"uri":    totpURL(secret, account, "PocketServe"),
	})
}

func (a *App) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Code string `json:"code"`
	}
	if !decodeBody(w, r, &b) {
		return
	}
	cfg := a.config()
	if cfg.TOTPSecret != "" {
		jsonError(w, http.StatusConflict, "already_enabled", "Já está ativo.")
		return
	}
	if cfg.TOTPPending == "" {
		jsonError(w, http.StatusBadRequest, "no_pending", "Inicie a configuração primeiro.")
		return
	}
	if !verifyTOTP(cfg.TOTPPending, b.Code) {
		jsonError(w, http.StatusUnauthorized, "bad_code", "Código incorreto. Verifique o relógio do aparelho e tente novamente.")
		return
	}
	plain, hashed := newRecoveryCodes()
	secret := cfg.TOTPPending
	if err := a.updateConfig(func(c *Config) {
		c.TOTPSecret = secret
		c.TOTPPending = ""
		c.RecoveryCodes = strings.Join(hashed, ",")
	}); err != nil {
		jsonError(w, http.StatusInternalServerError, "save_failed", "Não foi possível salvar.")
		return
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "totp_enable", OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "recovery_codes": plain})
}

func (a *App) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decodeBody(w, r, &b) {
		return
	}
	cfg := a.config()
	// Desativar exige senha E código atual: impede que uma sessão roubada
	// simplesmente desligue o segundo fator.
	if !verifyPassword(b.Password, cfg.PasswordHash) {
		jsonError(w, http.StatusUnauthorized, "invalid_password", "Senha incorreta.")
		return
	}
	if cfg.TOTPSecret != "" && !verifyTOTP(cfg.TOTPSecret, b.Code) {
		jsonError(w, http.StatusUnauthorized, "bad_code", "Código incorreto.")
		return
	}
	if err := a.updateConfig(func(c *Config) {
		c.TOTPSecret = ""
		c.TOTPPending = ""
		c.RecoveryCodes = ""
	}); err != nil {
		jsonError(w, http.StatusInternalServerError, "save_failed", "Não foi possível salvar.")
		return
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "totp_disable", OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

// checkSecondFactor valida TOTP ou código de recuperação.
func (a *App) checkSecondFactor(code string) bool {
	cfg := a.config()
	if verifyTOTP(cfg.TOTPSecret, code) {
		return true
	}
	if cfg.RecoveryCodes != "" {
		rest, ok := consumeRecoveryCode(code, strings.Split(cfg.RecoveryCodes, ","))
		if ok {
			_ = a.updateConfig(func(c *Config) { c.RecoveryCodes = rest })
			return true
		}
	}
	return false
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

// QR em PNG usando a mesma biblioteca já usada pelo painel.
func (a *App) handleTOTPQR(w http.ResponseWriter, r *http.Request) {
	cfg := a.config()
	secret := cfg.TOTPPending
	if secret == "" {
		secret = strings.TrimSpace(r.URL.Query().Get("secret"))
	}
	if secret == "" {
		http.Error(w, "sem segredo", http.StatusBadRequest)
		return
	}
	uri := totpURL(secret, cfg.ServerName, "PocketServe")
	qr, err := qrcode.New(uri, qrcode.Medium)
	if err != nil {
		http.Error(w, "falha ao gerar QR", http.StatusInternalServerError)
		return
	}
	qr.DisableBorder = false
	png, err := qr.PNG(420)
	if err != nil {
		http.Error(w, "falha ao gerar QR", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}
