package main

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

// "Lembrar deste aparelho": depois de um login com 2FA correto, o navegador
// recebe um token assinado. Enquanto ele for válido, aquele aparelho não
// precisa digitar o código de novo. É o que evita pegar o celular toda hora.
//
// O token não substitui a senha: só dispensa a segunda etapa. Revogar todos
// é uma ação explícita em Ajustes, e trocar a senha ou desligar o 2FA limpa
// a lista automaticamente.

const (
	trustCookie   = "ps_trust"
	trustDuration = 30 * 24 * time.Hour
)

type trustedDevice struct {
	Hash    string    `json:"hash"`
	AddedAt time.Time `json:"added_at"`
	Expires time.Time `json:"expires"`
	Label   string    `json:"label,omitempty"`
}

func (a *App) trustedActive() []trustedDevice {
	cfg := a.config()
	now := time.Now()
	out := make([]trustedDevice, 0, len(cfg.TrustedDevices))
	for _, d := range cfg.TrustedDevices {
		if d.Expires.After(now) {
			out = append(out, d)
		}
	}
	return out
}

func (a *App) trustedValid(r *http.Request) bool {
	c, err := r.Cookie(trustCookie)
	if err != nil || c.Value == "" {
		return false
	}
	cfg := a.config()
	if cfg.TOTPSecret == "" {
		return false
	}
	sum := sha256Hex(c.Value)
	now := time.Now()
	for _, d := range cfg.TrustedDevices {
		if d.Expires.After(now) && subtleEqual(d.Hash, sum) {
			return true
		}
	}
	return false
}

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// trustDevice registra o aparelho e devolve o token em claro (só agora).
func (a *App) trustDevice(w http.ResponseWriter, r *http.Request) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	dev := trustedDevice{
		Hash:    sha256Hex(token),
		AddedAt: time.Now(),
		Expires: time.Now().Add(trustDuration),
		Label:   deviceLabel(r),
	}
	// Mantém no máximo 20 aparelhos confiáveis.
	err := a.updateConfig(func(c *Config) {
		list := append([]trustedDevice{}, c.TrustedDevices...)
		list = append(list, dev)
		if len(list) > 20 {
			list = list[len(list)-20:]
		}
		c.TrustedDevices = list
	})
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     trustCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(trustDuration.Seconds()),
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func deviceLabel(r *http.Request) string {
	ua := r.Header.Get("User-Agent")
	switch {
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"):
		return "iOS"
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "Macintosh"):
		return "macOS"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	}
	return "navegador"
}

func (a *App) clearTrustCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: trustCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
	})
}

// Revoga todos os aparelhos confiáveis.
func (a *App) handleTrustRevoke(w http.ResponseWriter, r *http.Request) {
	if err := a.updateConfig(func(c *Config) { c.TrustedDevices = nil }); err != nil {
		jsonError(w, http.StatusInternalServerError, "save_failed", "Não foi possível salvar.")
		return
	}
	a.clearTrustCookie(w)
	a.activity.Add(Activity{IP: clientIP(r), Action: "trust_revoke", Detail: "", OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleTrustList(w http.ResponseWriter, r *http.Request) {
	list := a.trustedActive()
	out := make([]map[string]any, 0, len(list))
	for _, d := range list {
		out = append(out, map[string]any{
			"label": d.Label, "added_at": d.AddedAt, "expires": d.Expires,
		})
	}
	jsonWrite(w, http.StatusOK, map[string]any{
		"devices": out, "current": a.trustedValid(r),
	})
}

// pruneTrusted remove entradas vencidas (chamado no boot).
func (a *App) pruneTrusted() {
	cfg := a.config()
	now := time.Now()
	kept := make([]trustedDevice, 0, len(cfg.TrustedDevices))
	for _, d := range cfg.TrustedDevices {
		if d.Expires.After(now) {
			kept = append(kept, d)
		}
	}
	if len(kept) != len(cfg.TrustedDevices) {
		_ = a.updateConfig(func(c *Config) { c.TrustedDevices = kept })
	}
}
