package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Verificação de integridade sob demanda: calcula SHA-256 de um arquivo e,
// opcionalmente, compara com um valor esperado. Não há índice persistente —
// um hash de arquivo grande em /sdcard custa I/O, então é sempre explícito.

type checksumResult struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Match  *bool  `json:"match,omitempty"`
	Millis int64  `json:"ms"`
}

func (a *App) handleChecksum(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	full, err := a.resolveExisting(rel)
	if err != nil {
		jsonError(w, http.StatusNotFound, "not_found", "Arquivo não encontrado.")
		return
	}
	st, err := os.Stat(full)
	if err != nil || st.IsDir() {
		jsonError(w, http.StatusBadRequest, "not_file", "Selecione um arquivo.")
		return
	}
	start := time.Now()
	f, err := os.Open(full)
	if err != nil {
		jsonError(w, http.StatusForbidden, "permission", "Sem permissão para ler o arquivo.")
		return
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		jsonError(w, http.StatusInternalServerError, "read_failed", "Falha ao ler o arquivo.")
		return
	}
	sum := hex.EncodeToString(h.Sum(nil))

	res := checksumResult{
		Path:   a.displayPath(full),
		Size:   st.Size(),
		SHA256: sum,
		Millis: time.Since(start).Milliseconds(),
	}
	if want := r.URL.Query().Get("expect"); want != "" {
		match := equalFoldHex(want, sum)
		res.Match = &match
		a.activity.Add(Activity{
			IP: clientIP(r), Action: "checksum", OK: match,
			Detail: res.Path + " " + map[bool]string{true: "confere", false: "DIVERGE"}[match],
		})
	}
	jsonWrite(w, http.StatusOK, res)
}

func equalFoldHex(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'F' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'F' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// hashFile calcula o SHA-256. Com o aparelho dedicado, permite mais leituras
// simultâneas; o limite só evita saturar o I/O do cartão.
var hashSem = make(chan struct{}, 4)

func (a *App) hashFile(full string) (string, error) {
	hashSem <- struct{}{}
	defer func() { <-hashSem }()
	f, err := os.Open(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

var _ sync.Mutex
