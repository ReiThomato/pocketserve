package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const Version = "1.0.0"

// Config is persisted as JSON in <home>/config.json.
type Config struct {
	ServerName     string          `json:"server_name"`
	Port           int             `json:"port"`
	Root           string          `json:"root"`
	PasswordHash   string          `json:"password_hash"`
	SessionSecret  string          `json:"session_secret"`
	SessionDays    int             `json:"session_days"`
	TailscaleName  string          `json:"tailscale_name"`
	MaxUploadMB    int64           `json:"max_upload_mb"`
	MinFreeMB      int64           `json:"min_free_mb"`
	TrashDays      int             `json:"trash_days"`
	BackupDaily    bool            `json:"backup_daily"`
	TOTPSecret     string          `json:"totp_secret,omitempty"`
	TOTPPending    string          `json:"totp_pending,omitempty"`
	RecoveryCodes  string          `json:"recovery_codes,omitempty"`
	TrustedDevices []trustedDevice `json:"trusted_devices,omitempty"`
	TLSCert        string          `json:"tls_cert,omitempty"`
	TLSKey         string          `json:"tls_key,omitempty"`
	TLSPort        int             `json:"tls_port,omitempty"`
	PublicHost     string          `json:"public_host,omitempty"`
	Roots          []RootEntry     `json:"roots,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

func defaultHome() string {
	if h := strings.TrimSpace(os.Getenv("POCKETSERVE_HOME")); h != "" {
		return h
	}
	if h := strings.TrimSpace(os.Getenv("HOME")); h != "" {
		return filepath.Join(h, ".pocketserve")
	}
	return ".pocketserve"
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is fatal for security-sensitive values.
		panic("cannot read random bytes: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func defaultRoot() string {
	if st, err := os.Stat("/sdcard"); err == nil && st.IsDir() {
		return "/sdcard"
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// fallbackRoot valida o diretório usado quando POCKETSERVE_ALLOW_MISSING_ROOT
// está ativo. A regra é simples: nunca expor "/" nem uma árvore de sistema.
//
// O diretório do Termux (/data/data/com.termux/...) é permitido de propósito:
// é o cwd normal do app e o único lugar onde o escape hatch faz sentido. Como
// ele contém ~/.pocketserve, o log avisa em destaque.
func fallbackRoot(wd string) (string, error) {
	if strings.TrimSpace(wd) == "" {
		return "", errors.New("diretório atual desconhecido")
	}
	abs, err := filepath.Abs(wd)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(abs)
	if clean == string(filepath.Separator) {
		return "", errors.New("o diretório atual é a raiz do sistema")
	}
	// Árvores de sistema: expor qualquer uma delas seria pior do que não subir.
	// A exceção é o diretório do Termux, que é o cwd normal do app no Android.
	risky := []string{"/system", "/proc", "/sys", "/dev", "/vendor", "/etc", "/root", "/data"}
	const termuxPrefix = "/data/data/com.termux/"
	if strings.HasPrefix(clean, termuxPrefix) || clean == "/data/data/com.termux" {
		return clean, nil
	}
	for _, p := range risky {
		if clean == p || strings.HasPrefix(clean, p+"/") {
			return "", fmt.Errorf("o diretório atual (%s) está dentro de uma árvore de sistema", clean)
		}
	}
	return clean, nil
}

func normalizeRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = defaultRoot()
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	return filepath.Clean(root)
}

func (c *Config) fillDefaults() {
	if c.ServerName == "" {
		c.ServerName = "PocketServe"
	}
	if c.Port == 0 {
		c.Port = 8080
	}
	if c.Root == "" {
		c.Root = defaultRoot()
	}
	c.Root = normalizeRoot(c.Root)
	if c.SessionDays <= 0 {
		c.SessionDays = 30
	}
	if c.SessionDays > 365 {
		c.SessionDays = 365
	}
	if c.SessionSecret == "" {
		c.SessionSecret = randomToken(32)
	}
	if c.MaxUploadMB < 0 {
		c.MaxUploadMB = 0
	}
	if c.MinFreeMB < 0 {
		c.MinFreeMB = 0
	}
	if c.MinFreeMB > 1024*1024 {
		c.MinFreeMB = 1024 * 1024
	}
	if c.TLSPort == 0 {
		c.TLSPort = 8443
	}
	if c.TrashDays < 0 {
		c.TrashDays = 0
	}
	if c.TrashDays > 3650 {
		c.TrashDays = 3650
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
}

func loadConfig(home string) (*Config, error) {
	path := filepath.Join(home, "config.json")
	cfg := &Config{}
	b, err := os.ReadFile(path)
	hasMinFree := false
	hasTrashDays := false
	if err == nil {
		// Config antiga não tem "min_free_mb": nesse caso aplica-se o padrão.
		// Zero explícito significa "reserva desligada" e é respeitado.
		// O mesmo vale para "trash_days": ausente = 30 dias, 0 = lixeira desligada.
		var probe map[string]json.RawMessage
		if json.Unmarshal(b, &probe) == nil {
			_, hasMinFree = probe["min_free_mb"]
			_, hasTrashDays = probe["trash_days"]
		}
		if err := json.Unmarshal(b, cfg); err != nil {
			return nil, fmt.Errorf("config.json inválido: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	cfg.fillDefaults()
	if !hasMinFree {
		cfg.MinFreeMB = defaultMinFreeMB
	}
	if !hasTrashDays {
		cfg.TrashDays = 30
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := saveConfig(home, cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func saveConfig(home string, cfg *Config) error {
	cfg.fillDefaults()
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(home, "config.json.tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(home, "config.json"))
}

func (c Config) clone() Config { return c }
