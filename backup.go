package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Backup por snapshot incremental, sem dependências externas.
//
// Ideia: cada snapshot é um manifesto (caminho → sha256) guardado no próprio
// servidor. Os dados vão para um .tar.gz com nome <hash>.pack, compartilhado
// entre snapshots: um arquivo cujo conteúdo já existe em algum pack não é
// gravado de novo. O resultado é histórico (nada é sobrescrito) com custo de
// armazenamento próximo ao de um espelho.
//
// Layout em <home>/backups/<job>/:
//   snapshots/<id>.json          manifesto + metadados
//   packs/<sha256>.pack          bloco gzip com os arquivos daquele hash
//   index.json                   estado do último snapshot (delta rápido)

const (
	backupDirName    = ".pocketserve-backups"
	backupJobName    = "arquivos"
	backupRetention  = 14
	backupMaxFileMB  = 512
	backupPackSuffix = ".pack"
)

type backupFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
	Mode uint32 `json:"mode"`
}

type backupSnapshot struct {
	ID        string            `json:"id"`
	CreatedAt time.Time         `json:"created_at"`
	Files     []backupFile      `json:"files"`
	TotalSize int64             `json:"total_size"`
	Note      string            `json:"note,omitempty"`
	Missing   []string          `json:"missing,omitempty"`
	BySHA     map[string]string `json:"-"`
}

type backupManager struct {
	mu     sync.Mutex
	home   string
	rootFn func() string
}

func newBackupManager(home string, rootFn func() string) *backupManager {
	return &backupManager{home: home, rootFn: rootFn}
}

// Os snapshots ficam na mesma unidade dos arquivos (o HD), e não no
// armazenamento interno do app: é lá que sobra espaço. A pasta é oculta e
// excluída da listagem e do próprio backup. A raiz é lida a cada uso para
// acompanhar uma mudança feita em Ajustes.
func (b *backupManager) jobDir() string {
	return filepath.Join(b.rootFn(), backupDirName, backupJobName)
}
func (b *backupManager) snapsDir() string { return filepath.Join(b.jobDir(), "snapshots") }
func (b *backupManager) packsDir() string { return filepath.Join(b.jobDir(), "packs") }

func (a *App) backup() *backupManager { return a.backups }

// ---------- coleta ----------

// collectForBackup percorre as raízes ativas e devolve os arquivos elegíveis.
// Pula lixeira, temporários e arquivos acima do limite.
//
// Com mais de uma raiz, cada arquivo recebe o caminho exibível (com apelido),
// para que o manifesto continue plano e restaurável de qualquer volume.
func (a *App) collectForBackup() ([]backupFile, []string, error) {
	maxBytes := int64(backupMaxFileMB) << 20
	var files []backupFile
	var skipped []string
	seen := map[string]bool{}

	for _, r := range a.allRoots() {
		root := r.Path
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if hiddenFromListing(name) {
					return filepath.SkipDir
				}
				// Não faz backup do próprio backup.
				if p != root && (name == "backups" && filepath.Dir(p) == a.home) {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			// Caminho exibível: relativo à raiz principal ou prefixado com o
			// apelido do volume, nunca um caminho relativo com "..".
			rel := a.displayPath(p)
			if rel == "" || seen[rel] {
				return nil
			}
			if info.Size() > maxBytes {
				skipped = append(skipped, rel)
				return nil
			}
			var sha string
			mtimeNs := info.ModTime().UnixNano()
			ctimeNs := ctimeNsOf(info)
			if cached, ok := a.manifestCacheGet(rel, info.Size(), mtimeNs, ctimeNs); ok {
				sha = cached
			} else {
				var herr error
				sha, herr = a.hashFile(p)
				if herr != nil {
					return nil
				}
				a.manifestCachePut(rel, info.Size(), mtimeNs, ctimeNs, sha)
			}
			seen[rel] = true
			files = append(files, backupFile{
				Path: rel, Size: info.Size(), SHA: sha, Mode: uint32(info.Mode().Perm()),
			})
			return nil
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, skipped, nil
}

// cache de hash por (caminho, tamanho, mtime, ctime) para não reler tudo a
// cada snapshot.
//
// Precisão importa: usar mtime em segundos fazia duas edições no mesmo segundo
// com o mesmo tamanho reaproveitarem o hash antigo — o backup gravava conteúdo
// velho sem avisar. Além de nanossegundos, arquivos com mtime muito recente
// nunca vêm do cache: no exFAT do cartão a granularidade do timestamp é grossa
// (2 s), então o timestamp sozinho não é prova de que o conteúdo não mudou.
type hashCacheEntry struct {
	Size  int64
	Mtime int64
	Ctime int64
	SHA   string
}

// cacheSettleNs é a janela em que o cache não é confiável.
const cacheSettleNs int64 = 10 * int64(time.Second)

var (
	manifestMu    sync.Mutex
	manifestCache = map[string]hashCacheEntry{}
)

func (a *App) manifestCacheGet(p string, size, mtime, ctime int64) (string, bool) {
	if mtime == 0 || time.Since(time.Unix(0, mtime)) < time.Duration(cacheSettleNs) {
		return "", false
	}
	manifestMu.Lock()
	defer manifestMu.Unlock()
	e, ok := manifestCache[p]
	if !ok || e.Size != size || e.Mtime != mtime || e.Ctime != ctime || e.SHA == "" {
		return "", false
	}
	return e.SHA, true
}

func (a *App) manifestCachePut(p string, size, mtime, ctime int64, sha string) {
	manifestMu.Lock()
	defer manifestMu.Unlock()
	manifestCache[p] = hashCacheEntry{Size: size, Mtime: mtime, Ctime: ctime, SHA: sha}
}

// ---------- snapshot ----------

func (b *backupManager) lastSnapshot() (*backupSnapshot, error) {
	des, err := os.ReadDir(b.snapsDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, d := range des {
		if strings.HasSuffix(d.Name(), ".json") {
			names = append(names, d.Name())
		}
	}
	if len(names) == 0 {
		return nil, nil
	}
	sort.Strings(names)
	last := names[len(names)-1]
	data, err := os.ReadFile(filepath.Join(b.snapsDir(), last))
	if err != nil {
		return nil, err
	}
	var s backupSnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (b *backupManager) snapshotFiles() ([]backupSnapshot, error) {
	des, err := os.ReadDir(b.snapsDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []backupSnapshot
	for _, d := range des {
		if !strings.HasSuffix(d.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(b.snapsDir(), d.Name()))
		if err != nil {
			continue
		}
		var s backupSnapshot
		if json.Unmarshal(data, &s) == nil {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// packExists diz se um blob já foi gravado em algum snapshot anterior.
func (b *backupManager) packExists(sha string) bool {
	if sha == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(b.packsDir(), sha+backupPackSuffix))
	return err == nil
}

// writePack grava um arquivo em um pack novo, nomeado pelo hash do conteúdo.
func (b *backupManager) writePack(full, sha string, mode uint32) error {
	if err := os.MkdirAll(b.packsDir(), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(b.packsDir(), ".pack-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	gz := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gz)
	f, err := os.Open(full)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	hdr := &tar.Header{
		Name:    filepath.Base(full),
		Mode:    int64(mode),
		Size:    st.Size(),
		ModTime: st.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := io.Copy(tw, f); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(b.packsDir(), sha+backupPackSuffix))
}

// Run executa um snapshot. Devolve o snapshot criado e quantos arquivos novos.
func (a *App) runBackup(note string) (*backupSnapshot, int, error) {
	b := a.backup()
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := os.MkdirAll(b.snapsDir(), 0o700); err != nil {
		return nil, 0, err
	}
	if err := os.MkdirAll(b.packsDir(), 0o700); err != nil {
		return nil, 0, err
	}

	files, skipped, err := a.collectForBackup()
	if err != nil {
		return nil, 0, err
	}

	// O manifesto guarda caminhos com apelido ("cartao/..."). resolve() sabe
	// voltar ao caminho real em qualquer volume; usar rootPath() direto
	// apontaria todos os arquivos do cartão para o interno.
	added := 0
	var failed []string
	for _, f := range files {
		if b.packExists(f.SHA) {
			continue
		}
		full, rerr := a.resolve(f.Path)
		if rerr != nil {
			failed = append(failed, f.Path)
			continue
		}
		if err := b.writePack(full, f.SHA, f.Mode); err != nil {
			failed = append(failed, f.Path)
			continue
		}
		added++
	}

	var total int64
	for _, f := range files {
		total += f.Size
	}
	// IDs com nanossegundos: dois snapshots no mesmo segundo não podem
	// colidir e sobrescrever o anterior.
	id := time.Now().Format("20060102-150405.000000000")
	snap := &backupSnapshot{
		ID: id, CreatedAt: time.Now(), Files: files,
		TotalSize: total, Note: note, Missing: skipped,
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	if err := os.WriteFile(filepath.Join(b.snapsDir(), id+".json"), data, 0o600); err != nil {
		return nil, 0, err
	}
	if err := b.prune(); err != nil {
		// poda falhar não invalida o snapshot
		_ = err
	}
	return snap, added, nil
}

// prune mantém os N snapshots mais recentes e remove packs órfãos.
func (b *backupManager) prune() error {
	snaps, err := b.snapshotFiles()
	if err != nil {
		return err
	}
	if len(snaps) <= backupRetention {
		return nil
	}
	keep := snaps[:backupRetention]
	drop := snaps[backupRetention:]
	for _, s := range drop {
		_ = os.Remove(filepath.Join(b.snapsDir(), s.ID+".json"))
	}
	// Recomputa quais hashes ainda são referenciados.
	needed := map[string]bool{}
	for _, s := range keep {
		for _, f := range s.Files {
			needed[f.SHA] = true
		}
	}
	packs, err := os.ReadDir(b.packsDir())
	if err != nil {
		return err
	}
	for _, p := range packs {
		name := p.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		sha := strings.TrimSuffix(name, backupPackSuffix)
		if !needed[sha] {
			_ = os.Remove(filepath.Join(b.packsDir(), name))
		}
	}
	return nil
}

// restoreFile extrai um arquivo de um pack para o destino.
func (b *backupManager) restoreFile(sha, dest string) error {
	packPath := filepath.Join(b.packsDir(), sha+backupPackSuffix)
	f, err := os.Open(packPath)
	if err != nil {
		return errors.New("bloco de backup não encontrado")
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	if _, err := tr.Next(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, h), tr); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != sha {
		_ = os.Remove(dest)
		return errors.New("bloco corrompido: hash não confere")
	}
	return nil
}

// stats devolve números para o painel.
func (b *backupManager) stats() map[string]any {
	var packsSize int64
	var packs int
	if des, err := os.ReadDir(b.packsDir()); err == nil {
		for _, d := range des {
			if i, err := d.Info(); err == nil {
				packsSize += i.Size()
				packs++
			}
		}
	}
	snaps, _ := b.snapshotFiles()
	var last *backupSnapshot
	if len(snaps) > 0 {
		last = &snaps[0]
	}
	out := map[string]any{
		"snapshots": len(snaps), "packs": packs, "packs_size": packsSize,
		"retention": backupRetention, "dir": b.jobDir(),
	}
	if last != nil {
		out["last_id"] = last.ID
		out["last_at"] = last.CreatedAt
		out["last_files"] = len(last.Files)
		out["last_size"] = last.TotalSize
	}
	return out
}

// ---------- handlers ----------

func (a *App) handleBackupStatus(w http.ResponseWriter, r *http.Request) {
	snaps, _ := a.backup().snapshotFiles()
	list := make([]map[string]any, 0, len(snaps))
	for _, s := range snaps {
		list = append(list, map[string]any{
			"id": s.ID, "at": s.CreatedAt, "files": len(s.Files),
			"size": s.TotalSize, "note": s.Note,
		})
	}
	jsonWrite(w, http.StatusOK, map[string]any{
		"stats": a.backup().stats(), "snapshots": list,
	})
}

func (a *App) handleBackupRun(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&b)
	snap, added, err := a.runBackup(strings.TrimSpace(b.Note))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "backup_failed", "Falha no backup: "+err.Error())
		a.activity.Add(Activity{IP: clientIP(r), Action: "backup", OK: false, Detail: err.Error()})
		return
	}
	a.activity.Add(Activity{
		IP: clientIP(r), Action: "backup", OK: true,
		Detail: fmt.Sprintf("%s: %d arquivo(s), %d novo(s) em bloco", snap.ID, len(snap.Files), added),
	})
	jsonWrite(w, http.StatusOK, map[string]any{
		"ok": true, "id": snap.ID, "files": len(snap.Files),
		"added": added, "size": snap.TotalSize,
	})
}

// handleBackupFile serve um arquivo de um snapshot (download/restauração).
func (a *App) handleBackupFile(w http.ResponseWriter, r *http.Request) {
	snapID := r.URL.Query().Get("snapshot")
	rel := r.URL.Query().Get("path")
	b := a.backup()
	snaps, err := b.snapshotFiles()
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
	// Extrai para um temporário e serve como anexo.
	tmpDir := filepath.Join(a.backup().jobDir(), "tmp")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		jsonError(w, http.StatusInternalServerError, "tmp_failed", "Falha ao preparar restauração.")
		return
	}
	tmpFile, err := os.CreateTemp(tmpDir, "restore-*")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "tmp_failed", "Falha ao preparar restauração.")
		return
	}
	tmpName := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpName)

	if err := b.restoreFile(file.SHA, tmpName); err != nil {
		jsonError(w, http.StatusInternalServerError, "restore_failed", err.Error())
		return
	}
	f, err := os.Open(tmpName)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "open_failed", "Falha ao abrir restauração.")
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", contentDisposition(path.Base(file.Path), false))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, path.Base(file.Path), st.ModTime(), f)
	a.activity.Add(Activity{IP: clientIP(r), Action: "backup_restore", OK: true, Detail: snapID + " " + file.Path})
}

func (a *App) handleBackupDelete(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ID string `json:"id"`
	}
	if !decodeBody(w, r, &b) {
		return
	}
	id := b.ID
	if id == "" || strings.ContainsAny(id, `/\\`) || strings.Contains(id, "..") {
		jsonError(w, http.StatusBadRequest, "bad_id", "Snapshot inválido.")
		return
	}
	bm := a.backup()
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if err := os.Remove(filepath.Join(bm.snapsDir(), id+".json")); err != nil {
		jsonError(w, http.StatusNotFound, "not_found", "Snapshot não encontrado.")
		return
	}
	_ = bm.prune()
	a.activity.Add(Activity{IP: clientIP(r), Action: "backup_delete", OK: true, Detail: id})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

// startBackupScheduler roda um snapshot por dia, se configurado.
func (a *App) startBackupScheduler() {
	go func() {
		t := time.NewTicker(30 * time.Minute)
		defer t.Stop()
		for {
			cfg := a.config()
			if cfg.BackupDaily {
				if last := a.backup().lastRunAt(); time.Since(last) >= 23*time.Hour {
					if _, _, err := a.runBackup("automático"); err != nil {
						a.activity.Add(Activity{IP: "sistema", Action: "backup", OK: false, Detail: err.Error()})
					} else {
						a.activity.Add(Activity{IP: "sistema", Action: "backup", OK: true, Detail: "snapshot automático"})
					}
				}
			}
			<-t.C
		}
	}()
}

func (b *backupManager) lastRunAt() time.Time {
	snaps, err := b.snapshotFiles()
	if err != nil || len(snaps) == 0 {
		return time.Time{}
	}
	return snaps[0].CreatedAt
}

var _ = strconv.Itoa
