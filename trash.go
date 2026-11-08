package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// A lixeira fica dentro da raiz (não no home) porque restaurar exige rename
// no mesmo sistema de arquivos. Em /sdcard e /data são FUSE diferentes.
//
// Com mais de uma raiz, cada volume tem a sua própria lixeira: mover do cartão
// para a lixeira do interno exigiria copiar o arquivo inteiro, e o rename
// entre volumes diferentes falha com EXDEV.
const trashDirName = ".pocketserve-trash"

type trashMeta struct {
	ID        string    `json:"id"`
	OrigPath  string    `json:"orig_path"`
	Name      string    `json:"name"`
	DeletedAt time.Time `json:"deleted_at"`
	Size      int64     `json:"size"`
	IsDir     bool      `json:"is_dir"`
	IP        string    `json:"ip,omitempty"`
	// Root guarda o volume onde o item estava. Sem isso, restaurar um arquivo
	// do cartão tentaria voltar por um caminho da raiz principal.
	Root string `json:"root,omitempty"`
}

type trashItem struct {
	trashMeta
	Error string `json:"error,omitempty"`
}

// trashRoot é a lixeira da raiz principal (compatibilidade e uso comum).
func (a *App) trashRoot() string {
	return filepath.Join(a.rootPath(), trashDirName)
}

// allTrashRoots devolve a lixeira de cada volume ativo, sem duplicar caminhos.
func (a *App) allTrashRoots() []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range a.allRoots() {
		dir := filepath.Join(r.Path, trashDirName)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	return out
}

// trashRootFor escolhe a lixeira do mesmo volume do arquivo, para que o
// rename continue sendo instantâneo. Fora de qualquer raiz, usa a principal.
func (a *App) trashRootFor(full string) (root, trash string) {
	if r, ok := a.rootForPath(full); ok {
		return r.Path, filepath.Join(r.Path, trashDirName)
	}
	root = a.rootPath()
	return root, filepath.Join(root, trashDirName)
}

// hiddenFromListing decide o que nunca aparece para o usuário, mesmo com
// "mostrar ocultos" ligado.
func hiddenFromListing(name string) bool {
	return name == trashDirName || name == ".pocketserve-tmp" || name == ".pocketserve-backups"
}

func newTrashID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

// moveToTrash move um item para a lixeira do próprio volume. Se falhar,
// devolve o erro para o chamador decidir (não deleta de verdade como fallback).
func (a *App) moveToTrash(full, rel string, ip string) error {
	root, trashRoot := a.trashRootFor(full)
	if !within(root, full) || filepath.Clean(full) == filepath.Clean(root) {
		return errors.New("caminho inválido")
	}
	st, err := os.Lstat(full)
	if err != nil {
		return err
	}
	id := newTrashID()
	base := filepath.Join(trashRoot, id)
	if err := os.MkdirAll(base, 0o700); err != nil {
		return err
	}
	dataDir := filepath.Join(base, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	meta := trashMeta{
		ID:        id,
		OrigPath:  rel,
		Name:      filepath.Base(full),
		DeletedAt: time.Now(),
		Size:      st.Size(),
		IsDir:     st.IsDir(),
		IP:        ip,
		Root:      root,
	}
	b, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(base, "meta.json"), b, 0o600); err != nil {
		_ = os.RemoveAll(base)
		return err
	}
	target := filepath.Join(dataDir, meta.Name)
	if err := os.Rename(full, target); err != nil {
		_ = os.RemoveAll(base)
		return err
	}
	return nil
}

// findTrashItem localiza um item pelo ID em qualquer lixeira ativa.
// Devolve a pasta do item e o meta lido.
func (a *App) findTrashItem(id string) (string, trashMeta, error) {
	if id == "" || len(id) > 64 {
		return "", trashMeta{}, errors.New("id inválido")
	}
	// O ID usa apenas dígitos, letras a-f e hífen (ver newTrashID).
	for _, r := range id {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || r == '-'
		if !ok {
			return "", trashMeta{}, errors.New("id inválido")
		}
	}
	for _, root := range a.allTrashRoots() {
		base := filepath.Join(root, id)
		if !within(root, base) {
			continue
		}
		if _, err := os.Stat(base); err != nil {
			continue
		}
		meta, err := a.readTrashMeta(base)
		if err != nil {
			continue
		}
		return base, meta, nil
	}
	return "", trashMeta{}, errors.New("item não encontrado")
}

// listTrash lê todas as lixeiras dos volumes ativos e devolve os itens
// ordenados do mais recente para o mais antigo.
func (a *App) listTrash() ([]trashItem, error) {
	var items []trashItem
	any := false
	for _, trashRoot := range a.allTrashRoots() {
		des, err := os.ReadDir(trashRoot)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			continue
		}
		any = true
		for _, de := range des {
			if !de.IsDir() {
				continue
			}
			base := filepath.Join(trashRoot, de.Name())
			b, err := os.ReadFile(filepath.Join(base, "meta.json"))
			if err != nil {
				continue
			}
			var it trashItem
			if json.Unmarshal(b, &it) != nil || it.ID == "" {
				continue
			}
			// Itens antigos não guardavam Root: deduz pelo diretório da lixeira.
			if it.Root == "" {
				it.Root = filepath.Dir(trashRoot)
			}
			items = append(items, it)
		}
	}
	if !any {
		return []trashItem{}, nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].DeletedAt.After(items[j].DeletedAt)
	})
	return items, nil
}

func (a *App) readTrashMeta(base string) (trashMeta, error) {
	var m trashMeta
	b, err := os.ReadFile(filepath.Join(base, "meta.json"))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

// ---------- handlers ----------

func (a *App) handleTrashList(w http.ResponseWriter, r *http.Request) {
	items, err := a.listTrash()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "read_failed", "Não foi possível ler a lixeira.")
		return
	}
	cfg := a.config()
	jsonWrite(w, http.StatusOK, map[string]any{
		"items": items,
		"days":  cfg.TrashDays,
	})
}

func (a *App) handleTrashRestore(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ID string `json:"id"`
	}
	if !decodeBody(w, r, &b) {
		return
	}
	base, meta, err := a.findTrashItem(b.ID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "not_found", "Item não encontrado na lixeira.")
		return
	}
	dest := meta.OrigPath
	full, err := a.resolve(dest)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "bad_path", "Caminho original inválido.")
		return
	}
	// Garante que a pasta de destino ainda existe.
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, "mkdir_failed", "Não foi possível recriar a pasta de destino.")
		return
	}
	// Se já existe algo no lugar, restaura com sufixo em vez de sobrescrever.
	full = uniquePath(filepath.Dir(full), filepath.Base(full), false)
	src := filepath.Join(base, "data", meta.Name)
	if _, err := os.Stat(src); err != nil {
		jsonError(w, http.StatusInternalServerError, "missing", "O conteúdo do item sumiu da lixeira.")
		return
	}
	// A lixeira pode estar em outro volume que o destino (item antigo movido
	// junto com a mudança de raiz): cai para cópia + remoção.
	if err := movePath(src, full); err != nil {
		jsonError(w, http.StatusInternalServerError, "restore_failed", "Não foi possível restaurar: "+err.Error())
		return
	}
	_ = os.RemoveAll(base)
	rel := a.displayPath(full)
	a.activity.Add(Activity{IP: clientIP(r), Action: "restore", Detail: rel, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "path": rel})
}

func (a *App) handleTrashDelete(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ID string `json:"id"`
	}
	if !decodeBody(w, r, &b) {
		return
	}
	base, _, err := a.findTrashItem(b.ID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "not_found", "Item não encontrado na lixeira.")
		return
	}
	if err := os.RemoveAll(base); err != nil {
		jsonError(w, http.StatusInternalServerError, "delete_failed", "Não foi possível apagar definitivamente.")
		return
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "trash_purge", Detail: b.ID, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleTrashEmpty(w http.ResponseWriter, r *http.Request) {
	n := 0
	for _, root := range a.allTrashRoots() {
		des, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, de := range des {
			if de.IsDir() {
				if os.RemoveAll(filepath.Join(root, de.Name())) == nil {
					n++
				}
			}
		}
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "trash_empty", Detail: itoa(n) + " item(ns)", OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "deleted": n})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// purgeTrash remove itens mais antigos que o limite configurado.
func (a *App) purgeTrash(olderThan time.Duration) int {
	if olderThan <= 0 {
		return 0
	}
	items, err := a.listTrash()
	if err != nil {
		return 0
	}
	cut := time.Now().Add(-olderThan)
	n := 0
	for _, it := range items {
		if it.DeletedAt.Before(cut) {
			base, _, err := a.findTrashItem(it.ID)
			if err != nil {
				continue
			}
			if os.RemoveAll(base) == nil {
				n++
			}
		}
	}
	return n
}

// startTrashJanitor limpa a lixeira uma vez por dia.
func (a *App) startTrashJanitor() {
	go func() {
		t := time.NewTicker(6 * time.Hour)
		defer t.Stop()
		for {
			cfg := a.config()
			if cfg.TrashDays > 0 {
				if n := a.purgeTrash(time.Duration(cfg.TrashDays) * 24 * time.Hour); n > 0 {
					a.activity.Add(Activity{
						IP: "sistema", Action: "trash_expire",
						Detail: itoa(n) + " item(ns) com mais de " + itoa(cfg.TrashDays) + " dias", OK: true,
					})
				}
			}
			<-t.C
		}
	}()
}

// joinRel é um helper para compor caminhos relativos com barra.
func joinRel(dir, name string) string {
	if dir == "" {
		return name
	}
	return path.Join(dir, name)
}

var _ = strings.TrimSpace
