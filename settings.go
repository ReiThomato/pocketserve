package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (a *App) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	cfg := a.config()
	jsonWrite(w, http.StatusOK, map[string]any{
		"server_name":    cfg.ServerName,
		"port":           cfg.Port,
		"root":           cfg.Root,
		"session_days":   cfg.SessionDays,
		"tailscale_name": cfg.TailscaleName,
		"max_upload_mb":  cfg.MaxUploadMB,
		"min_free_mb":    cfg.MinFreeMB,
		"trash_days":     cfg.TrashDays,
		"backup_daily":   cfg.BackupDaily,
		"version":        Version,
		"home":           a.home,
		"roots":          a.rootsView(),
		"volumes":        a.detectVolumes(),
		"space":          a.spaceReport(),
	})
}

func (a *App) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ServerName    string      `json:"server_name"`
		Port          int         `json:"port"`
		Root          string      `json:"root"`
		SessionDays   int         `json:"session_days"`
		TailscaleName string      `json:"tailscale_name"`
		MaxUploadMB   int64       `json:"max_upload_mb"`
		MinFreeMB     *int64      `json:"min_free_mb"`
		TrashDays     int         `json:"trash_days"`
		BackupDaily   bool        `json:"backup_daily"`
		Roots         []RootEntry `json:"roots"`
	}
	if !decodeBody(w, r, &b) {
		return
	}
	name := strings.TrimSpace(b.ServerName)
	if name == "" || len([]rune(name)) > 40 {
		jsonError(w, http.StatusBadRequest, "bad_name", "Nome do servidor inválido (1 a 40 caracteres).")
		return
	}
	if b.Port < 1024 || b.Port > 65535 {
		jsonError(w, http.StatusBadRequest, "bad_port", "A porta deve ficar entre 1024 e 65535.")
		return
	}
	root := normalizeRoot(b.Root)
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		jsonError(w, http.StatusBadRequest, "bad_root", "A pasta raiz não existe ou não é acessível.")
		return
	}
	if b.SessionDays < 1 || b.SessionDays > 365 {
		jsonError(w, http.StatusBadRequest, "bad_session", "A duração da sessão deve ficar entre 1 e 365 dias.")
		return
	}
	if b.MaxUploadMB < 0 {
		b.MaxUploadMB = 0
	}
	minFree := int64(defaultMinFreeMB)
	if b.MinFreeMB != nil {
		minFree = *b.MinFreeMB
	}
	if minFree < 0 {
		jsonError(w, http.StatusBadRequest, "bad_reserve", "A reserva de disco não pode ser negativa.")
		return
	}
	if minFree > 1024*1024 {
		jsonError(w, http.StatusBadRequest, "bad_reserve", "A reserva de disco é grande demais.")
		return
	}
	if b.TrashDays < 0 {
		b.TrashDays = 0
	}
	if b.TrashDays > 3650 {
		jsonError(w, http.StatusBadRequest, "bad_trash", "A retenção da lixeira deve ficar entre 0 e 3650 dias.")
		return
	}
	ts := strings.TrimSpace(strings.ToLower(b.TailscaleName))
	ts = strings.TrimPrefix(ts, "http://")
	ts = strings.TrimPrefix(ts, "https://")
	ts = strings.TrimSuffix(ts, "/")
	for _, r := range ts {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '.' && r != '-' {
			jsonError(w, http.StatusBadRequest, "bad_ts", "Nome do Tailscale inválido.")
			return
		}
	}
	if len(ts) > 120 {
		jsonError(w, http.StatusBadRequest, "bad_ts", "Nome do Tailscale inválido.")
		return
	}

	restart := false
	old := a.config()
	err = a.updateConfig(func(c *Config) {
		if c.Port != b.Port || c.Root != root {
			restart = true
		}
		c.ServerName = name
		c.Port = b.Port
		c.Root = root
		c.SessionDays = b.SessionDays
		c.TailscaleName = ts
		c.MaxUploadMB = b.MaxUploadMB
		c.MinFreeMB = minFree
		c.TrashDays = b.TrashDays
		c.BackupDaily = b.BackupDaily
		// Filtra e normaliza as raízes extras: precisam existir e não podem
		// duplicar a principal.
		var roots []RootEntry
		seen := map[string]bool{}
		primary := normalizeRoot(c.Root)
		seen[primary] = true
		for _, r := range b.Roots {
			p := normalizeRoot(r.Path)
			if p == "" || seen[p] {
				continue
			}
			if st, err := os.Stat(p); err != nil || !st.IsDir() {
				continue
			}
			name := sanitizeAlias(r.Name)
			if name == "" {
				name = sanitizeAlias(filepath.Base(p))
			}
			if name == "" {
				name = "pasta"
			}
			roots = append(roots, RootEntry{Name: name, Path: p})
			seen[p] = true
			if len(roots) >= 8 {
				break
			}
		}
		c.Roots = roots
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "save_failed", "Não foi possível salvar as configurações.")
		return
	}
	detail := "configurações atualizadas"
	if old.Port != b.Port {
		detail = "porta " + strconv.Itoa(old.Port) + " → " + strconv.Itoa(b.Port)
	} else if old.Root != root {
		detail = "raiz → " + root
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "settings", Detail: detail, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "restart_required": restart})
}

func (a *App) handleRestart(w http.ResponseWriter, r *http.Request) {
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
	a.activity.Add(Activity{IP: clientIP(r), Action: "restart", Detail: "", OK: true})
	go func() {
		time.Sleep(450 * time.Millisecond)
		exe, err := os.Executable()
		if err != nil {
			return
		}
		exe, _ = filepath.EvalSymlinks(exe)
		_ = syscall.Exec(exe, os.Args, os.Environ())
	}()
}
