package main

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Histórico por arquivo.
//
// O backup já é content-addressed: cada snapshot é um manifesto
// (caminho → sha256) e o conteúdo vive em packs/<sha>.pack, compartilhados
// entre snapshots. Isso significa que a versão antiga de um arquivo
// sobrescrito JÁ ESTÁ no disco — só não havia como pedir por ela.
//
// Esta camada só lê os manifestos e cruza por caminho. Não escreve nada, não
// cria índice, não duplica dados.

type fileVersion struct {
	Snapshot string `json:"snapshot"`
	At       string `json:"at"`
	SHA      string `json:"sha"`
	Size     int64  `json:"size"`
	// Current marca a versão que está no disco agora (o snapshot mais
	// recente que contém o caminho).
	Current bool `json:"current"`
	// Recoverable indica se o pack existe e o download vai funcionar.
	Recoverable bool `json:"recoverable"`
}

type fileHistory struct {
	Path     string        `json:"path"`
	Versions []fileVersion `json:"versions"`
	// Distinct conta hashes diferentes: se for 1, o arquivo nunca mudou.
	Distinct int    `json:"distinct"`
	Note     string `json:"note,omitempty"`
}

// buildFileHistory cruza todos os snapshots e devolve as versões de um path.
func (a *App) buildFileHistory(rel string) fileHistory {
	h := fileHistory{Path: rel, Versions: []fileVersion{}}
	rel = strings.TrimPrefix(strings.TrimSpace(rel), "/")
	if rel == "" {
		h.Note = "Informe o caminho do arquivo."
		return h
	}

	snaps, err := a.backup().snapshotFiles()
	if err != nil || len(snaps) == 0 {
		h.Note = "Nenhum snapshot de backup ainda. Rode um backup em Ajustes → Backups."
		return h
	}

	b := a.backup()
	seen := map[string]bool{} // (snapshot|sha) para não repetir
	distinct := map[string]bool{}

	// snaps já vem do mais recente para o mais antigo.
	for _, s := range snaps {
		for _, f := range s.Files {
			if !strings.EqualFold(f.Path, rel) && f.Path != rel {
				continue
			}
			key := s.ID + "|" + f.SHA
			if seen[key] {
				continue
			}
			seen[key] = true
			distinct[f.SHA] = true
			h.Versions = append(h.Versions, fileVersion{
				Snapshot:    s.ID,
				At:          s.CreatedAt.Format("2006-01-02 15:04"),
				SHA:         f.SHA,
				Size:        f.Size,
				Recoverable: b.packExists(f.SHA),
			})
		}
	}

	if len(h.Versions) == 0 {
		h.Note = "Este caminho não aparece em nenhum snapshot. Pode ter sido criado depois do último backup."
		return h
	}

	// O primeiro (mais recente) é o que está no disco.
	h.Versions[0].Current = true
	h.Distinct = len(distinct)

	// Ordena por data decrescente (já está, mas garante).
	sort.SliceStable(h.Versions, func(i, j int) bool {
		return h.Versions[i].Snapshot > h.Versions[j].Snapshot
	})
	return h
}

// versionSummary descreve o histórico em uma linha (para a listagem de arquivos).
func (h fileHistory) summary() string {
	if len(h.Versions) == 0 {
		return ""
	}
	if h.Distinct <= 1 {
		return "sem alterações desde o primeiro backup"
	}
	return itoa(h.Distinct) + " versões diferentes"
}

func (a *App) handleFileHistory(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	h := a.buildFileHistory(rel)
	jsonWrite(w, http.StatusOK, h)
}

// handleFileVersion serve o conteúdo de uma versão específica (snapshot +
// path). Reaproveita a extração do pack que já existia no backup.
func (a *App) handleFileVersion(w http.ResponseWriter, r *http.Request) {
	snapID := r.URL.Query().Get("snapshot")
	rel := r.URL.Query().Get("path")
	if snapID == "" || rel == "" {
		jsonError(w, http.StatusBadRequest, "bad_request", "Snapshot e caminho são obrigatórios.")
		return
	}

	snaps, err := a.backup().snapshotFiles()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "read_failed", "Falha ao ler snapshots.")
		return
	}
	var target *backupSnapshot
	for i := range snaps {
		if snaps[i].ID == snapID {
			target = &snaps[i]
			break
		}
	}
	if target == nil {
		jsonError(w, http.StatusNotFound, "not_found", "Snapshot não encontrado.")
		return
	}

	var file *backupFile
	for i := range target.Files {
		if target.Files[i].Path == rel {
			file = &target.Files[i]
			break
		}
	}
	if file == nil {
		jsonError(w, http.StatusNotFound, "not_found", "Arquivo não está neste snapshot.")
		return
	}

	tmpDir := filepath.Join(a.backup().jobDir(), "tmp")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		jsonError(w, http.StatusInternalServerError, "tmp_failed", "Falha ao preparar a restauração.")
		return
	}
	tmpFile, err := os.CreateTemp(tmpDir, "version-*")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "tmp_failed", "Falha ao preparar a restauração.")
		return
	}
	tmpName := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpName)

	if err := a.backup().restoreFile(file.SHA, tmpName); err != nil {
		jsonError(w, http.StatusInternalServerError, "restore_failed", err.Error())
		return
	}
	f, err := os.Open(tmpName)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "open_failed", "Falha ao abrir a versão.")
		return
	}
	defer f.Close()
	st, _ := os.Stat(tmpName)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", contentDisposition(filepath.Base(file.Path), false))
	w.Header().Set("Cache-Control", "no-store")
	a.activity.Add(Activity{
		IP: clientIP(r), Action: "version_download", OK: true,
		Detail: snapID + " " + rel,
	})
	http.ServeContent(w, r, filepath.Base(file.Path), st.ModTime(), f)
}

// handleFileVersionRestore coloca a versão escolhida de volta no caminho
// original, movendo a atual para a lixeira antes.
func (a *App) handleFileVersionRestore(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Snapshot string `json:"snapshot"`
		Path     string `json:"path"`
	}
	if !decodeBody(w, r, &b) {
		return
	}
	if b.Snapshot == "" || b.Path == "" {
		jsonError(w, http.StatusBadRequest, "bad_request", "Snapshot e caminho são obrigatórios.")
		return
	}

	snaps, err := a.backup().snapshotFiles()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "read_failed", "Falha ao ler snapshots.")
		return
	}
	var target *backupSnapshot
	for i := range snaps {
		if snaps[i].ID == b.Snapshot {
			target = &snaps[i]
			break
		}
	}
	if target == nil {
		jsonError(w, http.StatusNotFound, "not_found", "Snapshot não encontrado.")
		return
	}
	var file *backupFile
	for i := range target.Files {
		if target.Files[i].Path == b.Path {
			file = &target.Files[i]
			break
		}
	}
	if file == nil {
		jsonError(w, http.StatusNotFound, "not_found", "Arquivo não está neste snapshot.")
		return
	}

	dest, err := a.resolveCreate(b.Path)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "bad_path", "Caminho inválido.")
		return
	}

	// Preserva o que está lá agora: a versão atual vai para a lixeira, não
	// some. Restaurar nunca deve ser uma perda irreversível.
	if st, serr := os.Stat(dest); serr == nil && !st.IsDir() {
		if a.config().TrashDays > 0 {
			if terr := a.moveToTrash(dest, b.Path, clientIP(r)); terr != nil {
				jsonError(w, http.StatusInternalServerError, "trash_failed",
					"Não foi possível guardar a versão atual na lixeira: "+terr.Error())
				return
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, "mkdir_failed", "Não foi possível recriar a pasta.")
		return
	}
	// Extrai para um temporário no mesmo diretório e renomeia: assim o arquivo
	// final nunca fica truncado se houver falha no meio.
	tmpDir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(tmpDir, ".ps-restore-*")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "write_failed", "Não foi possível gravar.")
		return
	}
	tmpName := tmp.Name()
	_ = tmp.Close()
	if err := a.backup().restoreFile(file.SHA, tmpName); err != nil {
		_ = os.Remove(tmpName)
		jsonError(w, http.StatusInternalServerError, "restore_failed", err.Error())
		return
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		_ = os.Remove(tmpName)
		jsonError(w, http.StatusInternalServerError, "restore_failed", "Não foi possível colocar a versão no lugar.")
		return
	}
	a.activity.Add(Activity{
		IP: clientIP(r), Action: "version_restore", OK: true,
		Detail: b.Snapshot + " " + b.Path,
	})
	jsonWrite(w, http.StatusOK, map[string]any{
		"ok": true, "path": a.displayPath(dest), "snapshot": b.Snapshot,
	})
}
