package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
)

type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`
	Kind    string    `json:"kind"`
	Mime    string    `json:"mime,omitempty"`
	Symlink bool      `json:"symlink,omitempty"`
}

// ---------- path safety ----------

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func (a *App) resolve(rel string) (string, error) {
	root, rest, err := a.splitAlias(rel)
	if err != nil {
		return "", err
	}
	rest = filepath.FromSlash(rest)
	full := filepath.Clean(filepath.Join(root, rest))
	if !within(root, full) {
		return "", errors.New("caminho inválido")
	}
	return full, nil
}

// pathErrorText escolhe a mensagem devolvida ao usuário. Erros de resolução já
// trazem texto útil (por exemplo a sugestão de apelido); os demais recebem a
// mensagem genérica do contexto.
func pathErrorText(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	msg := err.Error()
	if strings.Contains(msg, "você quis dizer") {
		return msg
	}
	return fallback
}

// permissionHint explica por que o Android negou acesso, em vez de sugerir que
// a pasta não existe. Android/data e Android/obb são restritos a apps desde o
// Android 11 (scoped storage); nem o Termux nem o PocketServe têm o grupo
// ext_data_rw necessário para atravessá-los.
func permissionHint(rel string) string {
	clean := strings.ToLower(strings.Trim(strings.ReplaceAll(rel, `\`, "/"), "/"))
	if clean == "android/data" || strings.HasPrefix(clean, "android/data/") ||
		clean == "android/obb" || strings.HasPrefix(clean, "android/obb/") {
		return "O Android bloqueia Android/data e Android/obb para todos os apps. Use Android/media ou outra pasta."
	}
	if clean == "android" || strings.HasPrefix(clean, "android/") {
		return "O Android bloqueia parte da pasta Android (data e obb). Tente Android/media."
	}
	return "Sem permissão do Android para acessar esta pasta."
}

// resolveExisting additionally resolves symlinks so a link inside the root
// cannot be used to escape it.
func (a *App) resolveExisting(rel string) (string, error) {
	full, err := a.resolve(rel)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}
	if !a.withinAnyRoot(real) {
		return "", errors.New("caminho fora da pasta permitida")
	}
	return full, nil
}

// resolveCreate resolves the parent directory for a path that may not exist yet.
func (a *App) resolveCreate(rel string) (string, error) {
	full, err := a.resolve(rel)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(full)
	real, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !a.withinAnyRoot(real) {
		return "", errors.New("caminho fora da pasta permitida")
	}
	return full, nil
}

func (a *App) rootPath() string { return a.config().Root }

// ensureDir resolves rel inside the root and creates it (and missing parents)
// when needed. The deepest existing ancestor is checked after resolving
// symlinks so a link inside the root cannot lead outside of it.
func (a *App) ensureDir(rel string) (string, error) {
	full, err := a.resolve(rel)
	if err != nil {
		return "", err
	}
	probe := full
	for {
		if _, err := os.Lstat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
		probe = parent
	}
	real, err := filepath.EvalSymlinks(probe)
	if err != nil {
		return "", err
	}
	if !a.withinAnyRoot(real) {
		return "", errors.New("caminho fora da pasta permitida")
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		return "", err
	}
	st, err := os.Stat(full)
	if err != nil || !st.IsDir() {
		return "", errors.New("pasta inválida")
	}
	return full, nil
}

func relPath(root, full string) string {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return ""
	}
	if rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

// ---------- listing ----------

func kindOf(name string, isDir bool) string {
	if isDir {
		return "folder"
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg", ".heic", ".heif", ".avif", ".ico":
		return "image"
	case ".mp4", ".mkv", ".webm", ".mov", ".m4v", ".3gp", ".avi", ".ts", ".flv", ".wmv":
		return "video"
	case ".mp3", ".m4a", ".aac", ".wav", ".flac", ".ogg", ".opus", ".amr", ".mid", ".midi", ".wma":
		return "audio"
	case ".pdf":
		return "pdf"
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".apk", ".aab", ".iso", ".zst":
		return "archive"
	case ".txt", ".md", ".json", ".js", ".mjs", ".tsx", ".jsx", ".css", ".html", ".htm", ".xml", ".yaml", ".yml", ".ini", ".conf", ".cfg", ".log", ".csv", ".srt", ".vtt", ".go", ".py", ".java", ".kt", ".sh", ".bash", ".c", ".cpp", ".h", ".hpp", ".sql", ".rs", ".toml", ".env", ".properties":
		return "text"
	case ".doc", ".docx", ".odt", ".rtf", ".epub":
		return "doc"
	case ".xls", ".xlsx", ".ods":
		return "sheet"
	case ".ppt", ".pptx", ".odp":
		return "slides"
	}
	if m := mime.TypeByExtension(ext); strings.HasPrefix(m, "image/") {
		return "image"
	} else if strings.HasPrefix(m, "video/") {
		return "video"
	} else if strings.HasPrefix(m, "audio/") {
		return "audio"
	}
	return "file"
}

func mimeOf(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".3gp":
		return "video/3gpp"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".ogg", ".opus":
		return "audio/ogg"
	case ".amr":
		return "audio/amr"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".apk":
		return "application/vnd.android.package-archive"
	case ".txt", ".md", ".log", ".csv":
		return "text/plain; charset=utf-8"
	case ".json":
		return "application/json"
	}
	if m := mime.TypeByExtension(ext); m != "" {
		return m
	}
	return "application/octet-stream"
}

func naturalLess(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	i, j := 0, 0
	for i < len(la) && j < len(lb) {
		if unicode.IsDigit(rune(la[i])) && unicode.IsDigit(rune(lb[j])) {
			si, sj := i, j
			for i < len(la) && unicode.IsDigit(rune(la[i])) {
				i++
			}
			for j < len(lb) && unicode.IsDigit(rune(lb[j])) {
				j++
			}
			na, _ := strconv.ParseUint(la[si:i], 10, 64)
			nb, _ := strconv.ParseUint(lb[sj:j], 10, 64)
			if na != nb {
				return na < nb
			}
			continue
		}
		if la[i] != lb[j] {
			return la[i] < lb[j]
		}
		i++
		j++
	}
	return len(la) < len(lb)
}

func (a *App) handleList(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	full, err := a.resolveExisting(rel)
	if err != nil {
		// Evita dizer "não encontrada" quando o Android bloqueou o acesso. Isso
		// acontece em Android/data e Android/obb: a pasta existe, mas o Termux
		// não tem o grupo ext_data_rw para lê-la.
		if os.IsPermission(err) || errors.Is(err, fs.ErrPermission) {
			jsonError(w, http.StatusForbidden, "permission", permissionHint(rel))
			return
		}
		jsonError(w, http.StatusNotFound, "not_found", pathErrorText(err, "Pasta não encontrada."))
		return
	}
	st, err := os.Stat(full)
	if err != nil || !st.IsDir() {
		jsonError(w, http.StatusBadRequest, "not_dir", "Isso não é uma pasta.")
		return
	}
	showHidden := r.URL.Query().Get("hidden") == "1"
	des, err := os.ReadDir(full)
	if err != nil {
		if os.IsPermission(err) {
			jsonError(w, http.StatusForbidden, "permission", permissionHint(rel))
			return
		}
		jsonError(w, http.StatusForbidden, "permission", "Sem permissão para ler esta pasta.")
		return
	}
	entries := make([]FileEntry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		if hiddenFromListing(name) {
			continue
		}
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		// Raiz secundária dentro desta pasta: navegue por ela pela aba do
		// volume, não como subpasta duplicada (o mesmo arquivo apareceria em
		// dois caminhos diferentes).
		if a.isSecondaryRootChild(filepath.Join(full, name)) {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		isDir := info.IsDir()
		kind := kindOf(name, isDir)
		entries = append(entries, FileEntry{
			Name: name, Path: a.displayPath(filepath.Join(full, name)),
			IsDir: isDir, Size: info.Size(), ModTime: info.ModTime(),
			Kind: kind, Mime: mimeOf(name), Symlink: info.Mode()&os.ModeSymlink != 0,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return naturalLess(entries[i].Name, entries[j].Name)
	})
	shown := a.displayPath(full)
	parent := ""
	if shown != "" {
		// Numa raiz secundária, o pai é o próprio apelido.
		parts := strings.Split(shown, "/")
		if len(parts) > 1 {
			parent = strings.Join(parts[:len(parts)-1], "/")
		} else if len(parts) == 1 {
			parent = ""
		}
	}
	jsonWrite(w, http.StatusOK, map[string]any{
		"path":         shown,
		"roots":        a.rootsWithSpace(),
		"root_label":   a.primaryLabel(),
		"primary_free": a.primaryFree(),
		"trash_days":   a.config().TrashDays,
		"parent":       parent,
		"entries":      entries,
		"writable": func() bool {
			f, err := os.CreateTemp(full, ".ps-write-test-*")
			if err != nil {
				return false
			}
			name := f.Name()
			_ = f.Close()
			_ = os.Remove(name)
			return true
		}(),
	})
}

// ---------- serving files ----------

func unsafeInline(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".htm", ".xhtml", ".svg", ".svgz", ".xml", ".js", ".mjs":
		return true
	}
	return false
}

func (a *App) servePath(w http.ResponseWriter, r *http.Request, full, name string, inline bool) {
	f, err := os.Open(full)
	if err != nil {
		http.Error(w, "arquivo não encontrado", http.StatusNotFound)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.Error(w, "arquivo inválido", http.StatusBadRequest)
		return
	}
	ct := mimeOf(name)
	if inline && unsafeInline(name) {
		// HTML/SVG/XML podem executar scripts se abertos diretamente. Servir
		// como texto mantém a pré-visualização inofensiva no mesmo domínio.
		ct = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", contentDisposition(name, inline))
	w.Header().Set("Cache-Control", "private, max-age=30")
	if inline {
		// Allow the authenticated same-origin preview to embed PDFs and
		// media in an iframe without opening the endpoint to other sites.
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
	}
	http.ServeContent(w, r, name, st.ModTime(), f)
}

func (a *App) handleRaw(w http.ResponseWriter, r *http.Request) {
	full, err := a.resolveExisting(r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, "não encontrado", http.StatusNotFound)
		return
	}
	a.servePath(w, r, full, filepath.Base(full), true)
}

func (a *App) handleDownload(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	full, err := a.resolveExisting(rel)
	if err != nil {
		http.Error(w, "não encontrado", http.StatusNotFound)
		return
	}
	st, err := os.Stat(full)
	if err != nil {
		http.Error(w, "não encontrado", http.StatusNotFound)
		return
	}
	if st.IsDir() {
		a.streamZip(w, r, []string{rel})
		return
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "download", Detail: rel, OK: true})
	a.servePath(w, r, full, filepath.Base(full), false)
}

// ---------- zip streaming ----------

func isCompressible(name string) bool {
	switch kindOf(name, false) {
	case "image", "video", "audio", "archive", "pdf":
		return false
	}
	return true
}

type zipItem struct {
	full string
	base string
}

func (a *App) streamZip(w http.ResponseWriter, r *http.Request, rels []string) {
	var items []zipItem
	for _, rel := range rels {
		full, err := a.resolveExisting(rel)
		if err != nil {
			continue
		}
		items = append(items, zipItem{full: full, base: filepath.Base(full)})
	}
	if len(items) == 0 {
		http.Error(w, "nada para baixar", http.StatusNotFound)
		return
	}
	name := "arquivos.zip"
	if len(items) == 1 {
		name = items[0].base + ".zip"
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition(name, false))
	zw := zip.NewWriter(w)
	defer zw.Close()

	added := 0
	for _, it := range items {
		base := it.base
		err := filepath.WalkDir(it.full, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			rel, err := filepath.Rel(it.full, p)
			if err != nil {
				return nil
			}
			nameInZip := base
			if rel != "." {
				nameInZip = filepath.ToSlash(filepath.Join(base, rel))
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if rel == "." {
					return nil
				}
				_, err = zw.Create(nameInZip + "/")
				added++
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			hdr := &zip.FileHeader{Name: nameInZip, Method: zip.Deflate}
			if !isCompressible(nameInZip) || info.Size() > 24<<20 {
				hdr.Method = zip.Store
			}
			hdr.Modified = info.ModTime()
			fw, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			f, err := os.Open(p)
			if err != nil {
				return nil
			}
			_, copyErr := io.Copy(fw, f)
			_ = f.Close()
			if copyErr != nil {
				return copyErr
			}
			added++
			if added%50 == 0 {
				w.(http.Flusher).Flush()
			}
			return nil
		})
		if err != nil {
			break
		}
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "download_zip", Detail: fmt.Sprintf("%d item(ns)", len(items)), OK: true})
}

func (a *App) handleZip(w http.ResponseWriter, r *http.Request) {
	rels := r.URL.Query()["path"]
	if len(rels) == 0 {
		http.Error(w, "nada para baixar", http.StatusBadRequest)
		return
	}
	a.streamZip(w, r, rels)
}

// ---------- upload ----------

func uniquePath(dir, name string, overwrite bool) string {
	candidate := filepath.Join(dir, name)
	if overwrite {
		return candidate
	}
	if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
		return candidate
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; i < 10000; i++ {
		c := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Lstat(c); errors.Is(err, os.ErrNotExist) {
			return c
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, time.Now().UnixNano(), ext))
}

func (a *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	relDir := r.URL.Query().Get("path")
	dir, err := a.ensureDir(relDir)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "bad_path", pathErrorText(err, "Pasta de destino inválida."))
		return
	}
	// Antes de receber um byte: recusa se o volume já está abaixo da reserva.
	// Evita receber gigabytes por rede para descobrir no fim que não cabe.
	if free, usable, serr := a.checkSpace(dir, 0); serr != nil {
		jsonError(w, http.StatusInsufficientStorage, "no_space", a.spaceMessage(free, usable))
		return
	}
	cfg := a.config()
	maxBytes := cfg.MaxUploadMB * 1024 * 1024
	overwrite := r.URL.Query().Get("overwrite") == "1"

	mr, err := r.MultipartReader()
	if err != nil {
		jsonError(w, http.StatusBadRequest, "bad_request", "Envio inválido.")
		return
	}
	var saved []FileEntry
	var total int64
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			jsonError(w, http.StatusBadRequest, "upload_failed", "Falha ao receber o arquivo.")
			return
		}
		if part.FileName() == "" {
			_ = part.Close()
			continue
		}
		name := sanitizeName(filepath.Base(part.FileName()))
		dest := uniquePath(dir, name, overwrite)
		// Reconfere antes de cada arquivo: o primeiro da fila pode ter
		// consumido o espaço que o segundo esperava.
		if free, usable, serr := a.checkSpace(dir, 0); serr != nil {
			_ = part.Close()
			jsonError(w, http.StatusInsufficientStorage, "no_space", a.spaceMessage(free, usable))
			return
		}
		tmp, err := os.CreateTemp(dir, ".ps-upload-*")
		if err != nil {
			_ = part.Close()
			jsonError(w, http.StatusInsufficientStorage, "write_failed", "Não foi possível gravar na pasta. Verifique o espaço livre.")
			return
		}
		var src io.Reader = part
		if maxBytes > 0 {
			src = io.LimitReader(part, maxBytes+1)
		}
		n, copyErr := io.Copy(tmp, src)
		closeErr := tmp.Close()
		_ = part.Close()
		if copyErr != nil || closeErr != nil || (maxBytes > 0 && n > maxBytes) {
			_ = os.Remove(tmp.Name())
			// Espaço esgotado no meio da escrita aparece aqui como erro de I/O.
			if copyErr != nil {
				if free, usable, serr := a.checkSpace(dir, 0); serr != nil {
					jsonError(w, http.StatusInsufficientStorage, "no_space", a.spaceMessage(free, usable))
					return
				}
			}
			jsonError(w, http.StatusRequestEntityTooLarge, "too_large", "Arquivo maior que o limite configurado.")
			return
		}
		// Depois de gravar: o arquivo não pode deixar o volume abaixo da
		// reserva. Se deixou, desfaz — melhor recusar do que encher o disco.
		if free, usable, serr := a.checkSpace(dir, 0); serr != nil && n > 0 {
			_ = os.Remove(tmp.Name())
			jsonError(w, http.StatusInsufficientStorage, "no_space", a.spaceMessage(free, usable))
			return
		}
		if err := os.Rename(tmp.Name(), dest); err != nil {
			_ = os.Remove(tmp.Name())
			jsonError(w, http.StatusInternalServerError, "write_failed", "Não foi possível salvar o arquivo.")
			return
		}
		total += n
		st, _ := os.Stat(dest)
		var size int64
		var mt time.Time
		if st != nil {
			size = st.Size()
			mt = st.ModTime()
		}
		saved = append(saved, FileEntry{
			Name: filepath.Base(dest), Path: a.displayPath(dest), IsDir: false,
			Size: size, ModTime: mt, Kind: kindOf(dest, false), Mime: mimeOf(dest),
		})
	}
	if len(saved) == 0 {
		jsonError(w, http.StatusBadRequest, "empty", "Nenhum arquivo recebido.")
		return
	}
	detail := saved[0].Path
	if len(saved) > 1 {
		detail = fmt.Sprintf("%d arquivos em %s", len(saved), relDir)
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "upload", Detail: detail, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "files": saved, "bytes": total})
}

// ---------- operations ----------

type opBody struct {
	Path  string   `json:"path"`
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
	Dest  string   `json:"dest"`
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		jsonError(w, http.StatusBadRequest, "bad_request", "Dados inválidos.")
		return false
	}
	return true
}

func (a *App) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var b opBody
	if !decodeBody(w, r, &b) {
		return
	}
	name := sanitizeName(b.Name)
	full, err := a.resolveCreate(path.Join(b.Path, name))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "bad_path", "Nome inválido.")
		return
	}
	if err := os.Mkdir(full, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			jsonError(w, http.StatusConflict, "exists", "Já existe uma pasta com esse nome.")
			return
		}
		jsonError(w, http.StatusInternalServerError, "mkdir_failed", "Não foi possível criar a pasta.")
		return
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "mkdir", Detail: a.displayPath(full), OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "path": a.displayPath(full)})
}

func (a *App) handleRename(w http.ResponseWriter, r *http.Request) {
	var b opBody
	if !decodeBody(w, r, &b) {
		return
	}
	full, err := a.resolveExisting(b.Path)
	if err != nil {
		jsonError(w, http.StatusNotFound, "not_found", "Item não encontrado.")
		return
	}
	if a.isRootPath(full) {
		jsonError(w, http.StatusForbidden, "forbidden", "Não é possível renomear a raiz.")
		return
	}
	name := sanitizeName(b.Name)
	newFull, err := a.resolveCreate(path.Join(path.Dir(b.Path), name))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "bad_path", "Nome inválido.")
		return
	}
	if newFull != full {
		if _, err := os.Lstat(newFull); err == nil {
			jsonError(w, http.StatusConflict, "exists", "Já existe um item com esse nome.")
			return
		}
	}
	if err := os.Rename(full, newFull); err != nil {
		jsonError(w, http.StatusInternalServerError, "rename_failed", "Não foi possível renomear.")
		return
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "rename", Detail: b.Path + " → " + name, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "path": a.displayPath(newFull)})
}

func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	var b opBody
	if !decodeBody(w, r, &b) {
		return
	}
	if len(b.Paths) == 0 {
		jsonError(w, http.StatusBadRequest, "empty", "Nada selecionado.")
		return
	}
	deleted := 0
	trashed := 0
	for _, rel := range b.Paths {
		full, err := a.resolveExisting(rel)
		if err != nil || a.isRootPath(full) {
			continue
		}
		// Soft delete: vai para a lixeira. Se a lixeira falhar (disco cheio,
		// permissão), o arquivo permanece intacto em vez de sumir.
		if a.config().TrashDays > 0 {
			if err := a.moveToTrash(full, rel, clientIP(r)); err == nil {
				trashed++
				deleted++
				continue
			}
		}
		if err := os.RemoveAll(full); err == nil {
			deleted++
		}
	}
	detail := fmt.Sprintf("%d item(ns)", deleted)
	if trashed > 0 {
		detail += fmt.Sprintf(", %d na lixeira", trashed)
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "delete", Detail: detail, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "deleted": deleted, "trashed": trashed})
}

func (a *App) handleMove(w http.ResponseWriter, r *http.Request) {
	var b opBody
	if !decodeBody(w, r, &b) {
		return
	}
	dest, err := a.resolveExisting(b.Dest)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "bad_dest", "Destino inválido.")
		return
	}
	if st, err := os.Stat(dest); err != nil || !st.IsDir() {
		jsonError(w, http.StatusBadRequest, "bad_dest", "Destino inválido.")
		return
	}
	moved := 0
	for _, rel := range b.Paths {
		full, err := a.resolveExisting(rel)
		if err != nil {
			continue
		}
		if within(full, dest) {
			continue // cannot move a folder into itself
		}
		target := uniquePath(dest, filepath.Base(full), false)
		if err := movePath(full, target); err == nil {
			moved++
		}
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "move", Detail: fmt.Sprintf("%d item(ns) → %s", moved, b.Dest), OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "moved": moved})
}

// movePath move um arquivo ou pasta. Quando origem e destino estão em sistemas
// de arquivos diferentes — o caso normal entre o interno (/storage/emulated) e
// o cartão (/storage/XXXX-XXXX) — o rename(2) devolve EXDEV. Antes isso era
// engolido como "0 itens movidos". Aqui cai para cópia + remoção.
func movePath(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !isCrossDevice(err) {
		return err
	}
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		if err := copyTree(src, dst); err != nil {
			return err
		}
	} else if err := copyFile(src, dst); err != nil {
		return err
	}
	// Só remove a origem depois que a cópia terminou por inteiro.
	return os.RemoveAll(src)
}

// isCrossDevice reconhece EXDEV, que no Android/FUSE aparece embrulhado.
func isCrossDevice(err error) bool {
	if errors.Is(err, syscall.EXDEV) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "cross-device")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		return copyFile(p, target)
	})
}

func (a *App) handleCopy(w http.ResponseWriter, r *http.Request) {
	var b opBody
	if !decodeBody(w, r, &b) {
		return
	}
	dest, err := a.resolveExisting(b.Dest)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "bad_dest", "Destino inválido.")
		return
	}
	if st, err := os.Stat(dest); err != nil || !st.IsDir() {
		jsonError(w, http.StatusBadRequest, "bad_dest", "Destino inválido.")
		return
	}
	copied := 0
	for _, rel := range b.Paths {
		full, err := a.resolveExisting(rel)
		if err != nil {
			continue
		}
		target := uniquePath(dest, filepath.Base(full), false)
		st, err := os.Stat(full)
		if err != nil {
			continue
		}
		if st.IsDir() {
			if within(full, dest) {
				continue
			}
			if err := copyTree(full, target); err == nil {
				copied++
			}
		} else if copyFile(full, target) == nil {
			copied++
		}
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "copy", Detail: fmt.Sprintf("%d item(ns) → %s", copied, b.Dest), OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "copied": copied})
}

// ---------- text editor ----------

func (a *App) handleText(w http.ResponseWriter, r *http.Request) {
	full, err := a.resolveExisting(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, http.StatusNotFound, "not_found", "Arquivo não encontrado.")
		return
	}
	st, err := os.Stat(full)
	if err != nil || st.IsDir() {
		jsonError(w, http.StatusBadRequest, "bad_file", "Arquivo inválido.")
		return
	}
	const limit = 2 << 20
	f, err := os.Open(full)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "read_failed", "Não foi possível abrir o arquivo.")
		return
	}
	defer f.Close()
	buf, _ := io.ReadAll(io.LimitReader(f, limit+1))
	truncated := len(buf) > limit
	if truncated {
		buf = buf[:limit]
	}
	binary := bytes.IndexByte(buf, 0) >= 0
	content := ""
	if !binary {
		content = string(buf)
	}
	jsonWrite(w, http.StatusOK, map[string]any{
		"path": a.displayPath(full), "content": content, "size": st.Size(),
		"truncated": truncated, "binary": binary,
	})
}

func (a *App) handleSave(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if !decodeBody(w, r, &b) {
		return
	}
	if len(b.Content) > 5<<20 {
		jsonError(w, http.StatusRequestEntityTooLarge, "too_large", "Conteúdo grande demais.")
		return
	}
	full, err := a.resolveCreate(b.Path)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "bad_path", "Caminho inválido.")
		return
	}
	if free, usable, serr := a.checkSpace(filepath.Dir(full), int64(len(b.Content))); serr != nil {
		jsonError(w, http.StatusInsufficientStorage, "no_space", a.spaceMessage(free, usable))
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(full), ".ps-save-*")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "write_failed", "Não foi possível salvar.")
		return
	}
	if _, err := tmp.WriteString(b.Content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		jsonError(w, http.StatusInternalServerError, "write_failed", "Não foi possível salvar.")
		return
	}
	_ = tmp.Close()
	if err := os.Rename(tmp.Name(), full); err != nil {
		_ = os.Remove(tmp.Name())
		jsonError(w, http.StatusInternalServerError, "write_failed", "Não foi possível salvar.")
		return
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "save", Detail: b.Path, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- unzip ----------

func (a *App) handleUnzip(w http.ResponseWriter, r *http.Request) {
	var b opBody
	if !decodeBody(w, r, &b) {
		return
	}
	full, err := a.resolveExisting(b.Path)
	if err != nil {
		jsonError(w, http.StatusNotFound, "not_found", "Arquivo não encontrado.")
		return
	}
	zr, err := zip.OpenReader(full)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "bad_zip", "Arquivo ZIP inválido.")
		return
	}
	defer zr.Close()
	// A cota de extração não pode furar a reserva do volume. Se o ZIP
	// descompactado não couber respeitando a reserva, recusa antes de criar
	// qualquer arquivo.
	var declared int64
	for _, f := range zr.File {
		declared += int64(f.UncompressedSize64)
	}
	if free, usable, serr := a.checkSpace(filepath.Dir(full), declared); serr != nil {
		jsonError(w, http.StatusInsufficientStorage, "no_space", a.spaceMessage(free, usable))
		return
	}
	base := strings.TrimSuffix(filepath.Base(full), filepath.Ext(full))
	dest := uniquePath(filepath.Dir(full), base, false)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, "mkdir_failed", "Não foi possível criar a pasta.")
		return
	}
	const maxUnzipTotal = int64(20 << 30) // 20 GiB
	const maxUnzipCount = 20000
	var total int64
	count := 0
	aborted := false
	for _, f := range zr.File {
		if count >= maxUnzipCount || total >= maxUnzipTotal {
			aborted = true
			break
		}
		name := filepath.Clean(filepath.FromSlash(f.Name))
		if name == "." || strings.HasPrefix(name, ".."+string(filepath.Separator)) || filepath.IsAbs(name) {
			continue
		}
		target := filepath.Join(dest, name)
		if !within(dest, target) {
			continue
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0o755)
			continue
		}
		// Confere a cota antes de criar o arquivo: um ZIP bomba não pode
		// preencher o armazenamento antes de ser interrompido.
		remaining := maxUnzipTotal - total
		if int64(f.UncompressedSize64) > remaining {
			aborted = true
			break
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			_ = rc.Close()
			continue
		}
		// Mesmo que o cabeçalho minta sobre o tamanho, a escrita é limitada
		// ao espaço restante da cota.
		n, _ := io.Copy(out, io.LimitReader(rc, remaining+1))
		_ = out.Close()
		_ = rc.Close()
		if n > remaining || total+n > maxUnzipTotal {
			_ = os.Remove(target)
			aborted = true
			break
		}
		total += n
		count++
	}
	if aborted {
		_ = os.RemoveAll(dest)
		jsonError(w, http.StatusRequestEntityTooLarge, "too_large", "O ZIP excede o limite de extração (20 GiB ou 20 mil arquivos).")
		return
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "unzip", Detail: b.Path, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "path": a.displayPath(dest)})
}

// ---------- thumbnails ----------

var thumbMu sync.Mutex
var thumbCache = map[string][]byte{}
var thumbOrder []string

const (
	maxThumbFile   = 64 << 20 // 64 MiB
	maxThumbPixels = 80_000_000
)

// O aparelho é dedicado ao servidor: usamos mais paralelismo. O limite existe
// apenas para evitar picos de memória com muitas imagens grandes ao mesmo tempo.
var thumbSem = make(chan struct{}, 4)

func canThumb(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif":
		return true
	}
	return false
}

func (a *App) handleThumb(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	full, err := a.resolveExisting(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	st, err := os.Stat(full)
	if err != nil || st.IsDir() || !canThumb(full) {
		http.NotFound(w, r)
		return
	}
	if st.Size() > maxThumbFile {
		http.NotFound(w, r)
		return
	}
	key := fmt.Sprintf("%s|%d|%d", full, st.Size(), st.ModTime().Unix())
	thumbMu.Lock()
	data, ok := thumbCache[key]
	thumbMu.Unlock()
	if !ok {
		thumbSem <- struct{}{}
		data, err = buildThumb(full)
		<-thumbSem
		if err != nil {
			http.NotFound(w, r)
			return
		}
		thumbMu.Lock()
		thumbCache[key] = data
		thumbOrder = append(thumbOrder, key)
		for len(thumbOrder) > 600 {
			old := thumbOrder[0]
			thumbOrder = thumbOrder[1:]
			delete(thumbCache, old)
		}
		thumbMu.Unlock()
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = w.Write(data)
}

func buildThumb(full string) (out []byte, err error) {
	defer func() {
		if p := recover(); p != nil {
			out, err = nil, fmt.Errorf("thumbnail: %v", p)
		}
	}()
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(io.LimitReader(f, 1<<20))
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxThumbPixels {
		return nil, fmt.Errorf("thumbnail: dimensões não suportadas")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(io.LimitReader(f, maxThumbFile))
	if err != nil {
		return nil, err
	}
	dst := scaleImage(img, 420)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// scaleImage downsamples with bilinear sampling to fit max px on the long edge.
func scaleImage(src image.Image, max int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return src
	}
	scale := 1.0
	if sw > max || sh > max {
		if sw > sh {
			scale = float64(max) / float64(sw)
		} else {
			scale = float64(max) / float64(sh)
		}
	}
	dw, dh := int(float64(sw)*scale), int(float64(sh)*scale)
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	bg := color.RGBA{R: 12, G: 17, B: 25, A: 255}
	for y := 0; y < dh; y++ {
		sy := (float64(y) + 0.5) / scale
		for x := 0; x < dw; x++ {
			sx := (float64(x) + 0.5) / scale
			r, g, bl, al := bilinear(src, b, sx, sy)
			if al < 255 {
				r = uint8((uint16(r)*uint16(al) + uint16(bg.R)*uint16(255-al)) / 255)
				g = uint8((uint16(g)*uint16(al) + uint16(bg.G)*uint16(255-al)) / 255)
				bl = uint8((uint16(bl)*uint16(al) + uint16(bg.B)*uint16(255-al)) / 255)
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i] = r
			dst.Pix[i+1] = g
			dst.Pix[i+2] = bl
			dst.Pix[i+3] = 255
		}
	}
	return dst
}

func bilinear(src image.Image, b image.Rectangle, fx, fy float64) (uint8, uint8, uint8, uint8) {
	x0 := int(fx)
	y0 := int(fy)
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x0 >= b.Max.X {
		x0 = b.Max.X - 1
	}
	if y0 >= b.Max.Y {
		y0 = b.Max.Y - 1
	}
	x1, y1 := x0+1, y0+1
	if x1 >= b.Max.X {
		x1 = b.Max.X - 1
	}
	if y1 >= b.Max.Y {
		y1 = b.Max.Y - 1
	}
	dx, dy := fx-float64(x0), fy-float64(y0)
	c00 := color.RGBAModel.Convert(src.At(x0, y0)).(color.RGBA)
	c10 := color.RGBAModel.Convert(src.At(x1, y0)).(color.RGBA)
	c01 := color.RGBAModel.Convert(src.At(x0, y1)).(color.RGBA)
	c11 := color.RGBAModel.Convert(src.At(x1, y1)).(color.RGBA)
	mix := func(a, b, c, d uint8) uint8 {
		v := float64(a)*(1-dx)*(1-dy) + float64(b)*dx*(1-dy) + float64(c)*(1-dx)*dy + float64(d)*dx*dy
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		return uint8(v)
	}
	return mix(c00.R, c10.R, c01.R, c11.R), mix(c00.G, c10.G, c01.G, c11.G),
		mix(c00.B, c10.B, c01.B, c11.B), mix(c00.A, c10.A, c01.A, c11.A)
}

// ---------- search ----------

func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("q")))
	if len(q) < 2 {
		jsonError(w, http.StatusBadRequest, "short_query", "Digite pelo menos 2 caracteres.")
		return
	}
	start := r.URL.Query().Get("path")
	full, err := a.resolveExisting(start)
	if err != nil {
		full = a.rootPath()
	}
	// Escopo: por padrão a busca parte da pasta pedida. Com scope=all — ou
	// quando a busca começa na raiz — varre todos os volumes, para que um
	// arquivo no cartão apareça sem precisar trocar de aba antes.
	searchRoots := []string{full}
	if r.URL.Query().Get("all") == "1" || full == a.rootPath() {
		searchRoots = nil
		for _, rt := range a.allRoots() {
			searchRoots = append(searchRoots, rt.Path)
		}
	}
	deadline := time.Now().Add(6 * time.Second)
	ctx := r.Context()
	var results []FileEntry
	seenPaths := map[string]bool{}
	cancelled := false
	for _, searchRoot := range searchRoots {
		if cancelled || time.Now().After(deadline) || len(results) >= 400 {
			break
		}
		_ = filepath.WalkDir(searchRoot, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			// Para de varrer assim que o cliente desiste da busca (nova consulta,
			// troca de página ou aba fechada).
			if ctx.Err() != nil {
				cancelled = true
				return fs.SkipAll
			}
			if time.Now().After(deadline) || len(results) >= 400 {
				return fs.SkipAll
			}
			name := d.Name()
			if hiddenFromListing(name) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if p != searchRoot && strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if name == "Android" && filepath.Dir(p) == searchRoot {
					return fs.SkipDir
				}
				// Raiz sobreposta: se o cartão está dentro da pasta principal e
				// a busca já vai visitá-lo, não varra duas vezes.
				if p != searchRoot && a.isRootPath(p) {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.Contains(strings.ToLower(name), q) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			shown := a.displayPath(p)
			if seenPaths[shown] {
				return nil
			}
			seenPaths[shown] = true
			results = append(results, FileEntry{
				Name: name, Path: shown, IsDir: false, Size: info.Size(),
				ModTime: info.ModTime(), Kind: kindOf(name, false), Mime: mimeOf(name),
			})
			return nil
		})
	}
	if cancelled {
		return
	}
	if results == nil {
		results = []FileEntry{}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].ModTime.After(results[j].ModTime) })
	jsonWrite(w, http.StatusOK, map[string]any{"entries": results, "query": q})
}
