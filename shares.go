package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Share struct {
	Token        string     `json:"token"`
	Path         string     `json:"path"`
	Name         string     `json:"name"`
	IsDir        bool       `json:"is_dir"`
	Size         int64      `json:"size"`
	Created      time.Time  `json:"created"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	PasswordHash string     `json:"password_hash,omitempty"`
	Downloads    int        `json:"downloads"`
	LastAccess   *time.Time `json:"last_access,omitempty"`
}

type ShareStore struct {
	mu    sync.Mutex
	path  string
	items map[string]*Share
}

func newShareStore(home string) *ShareStore {
	s := &ShareStore{path: filepath.Join(home, "shares.json"), items: map[string]*Share{}}
	b, err := os.ReadFile(s.path)
	if err == nil {
		var list []*Share
		if json.Unmarshal(b, &list) == nil {
			for _, it := range list {
				if it != nil && it.Token != "" {
					s.items[it.Token] = it
				}
			}
		}
	}
	return s
}

func (s *ShareStore) saveLocked() {
	list := make([]*Share, 0, len(s.items))
	for _, it := range s.items {
		list = append(list, it)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Created.After(list[j].Created) })
	b, _ := json.MarshalIndent(list, "", "  ")
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		_ = os.Rename(tmp, s.path)
	}
}

func (s *ShareStore) get(token string) *Share {
	s.mu.Lock()
	defer s.mu.Unlock()
	it := s.items[token]
	if it == nil {
		return nil
	}
	cp := *it
	return &cp
}

func (s *ShareStore) add(it *Share) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[it.Token] = it
	s.saveLocked()
}

func (s *ShareStore) remove(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[token]; !ok {
		return false
	}
	delete(s.items, token)
	s.saveLocked()
	return true
}

func (s *ShareStore) list() []*Share {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Share, 0, len(s.items))
	for _, it := range s.items {
		cp := *it
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

func (s *ShareStore) increment(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it := s.items[token]; it != nil {
		now := time.Now()
		it.Downloads++
		it.LastAccess = &now
		s.saveLocked()
	}
}

func (it *Share) expired() bool {
	return it.ExpiresAt != nil && time.Now().After(*it.ExpiresAt)
}

// public devolve a representação enviada ao painel. O hash da senha fica
// apenas no shares.json: a interface só precisa saber se o link é protegido.
func (it *Share) public() map[string]any {
	out := map[string]any{
		"token":        it.Token,
		"path":         it.Path,
		"name":         it.Name,
		"is_dir":       it.IsDir,
		"size":         it.Size,
		"created":      it.Created,
		"downloads":    it.Downloads,
		"has_password": it.PasswordHash != "",
	}
	if it.ExpiresAt != nil {
		out["expires_at"] = it.ExpiresAt
	}
	if it.LastAccess != nil {
		out["last_access"] = it.LastAccess
	}
	return out
}

// ---------- authenticated share management ----------

func (a *App) handleShareCreate(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Path       string `json:"path"`
		ExpiresHrs int    `json:"expires_hours"`
		Password   string `json:"password"`
	}
	if !decodeBody(w, r, &b) {
		return
	}
	full, err := a.resolveExisting(b.Path)
	if err != nil {
		jsonError(w, http.StatusNotFound, "not_found", "Item não encontrado.")
		return
	}
	st, err := os.Stat(full)
	if err != nil {
		jsonError(w, http.StatusNotFound, "not_found", "Item não encontrado.")
		return
	}
	if b.ExpiresHrs < 0 || b.ExpiresHrs > 24*365 {
		jsonError(w, http.StatusBadRequest, "bad_expiry", "Validade inválida.")
		return
	}
	if b.Password != "" && len([]rune(b.Password)) < 4 {
		jsonError(w, http.StatusBadRequest, "weak_password", "A senha do link precisa ter pelo menos 4 caracteres.")
		return
	}
	share := &Share{
		Token:   randomToken(16),
		Path:    a.displayPath(full),
		Name:    filepath.Base(full),
		IsDir:   st.IsDir(),
		Size:    st.Size(),
		Created: time.Now(),
	}
	if b.ExpiresHrs > 0 {
		t := time.Now().Add(time.Duration(b.ExpiresHrs) * time.Hour)
		share.ExpiresAt = &t
	}
	if b.Password != "" {
		share.PasswordHash = hashPassword(b.Password)
	}
	a.shares.add(share)
	a.activity.Add(Activity{IP: clientIP(r), Action: "share_create", Detail: share.Path, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "share": share.public(), "url": "/s/" + share.Token})
}

func (a *App) handleShareList(w http.ResponseWriter, r *http.Request) {
	list := a.shares.list()
	out := make([]map[string]any, 0, len(list))
	for _, it := range list {
		out = append(out, it.public())
	}
	jsonWrite(w, http.StatusOK, map[string]any{"shares": out})
}

func (a *App) handleShareDelete(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if a.shares.remove(token) {
		a.activity.Add(Activity{IP: clientIP(r), Action: "share_revoke", Detail: token, OK: true})
	}
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------- public share ----------

func (a *App) shareUnlocked(r *http.Request, it *Share) bool {
	if it.PasswordHash == "" {
		return true
	}
	c, err := r.Cookie(shareCookieName(it.Token))
	if err != nil {
		return false
	}
	cfg := a.config()
	mac := hmac.New(sha256.New, []byte(cfg.SessionSecret))
	mac.Write([]byte("share:" + it.Token))
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(c.Value), []byte(want))
}

func shareCookieName(token string) string {
	if len(token) > 10 {
		token = token[:10]
	}
	return "ps_sh_" + token
}

func (a *App) setShareCookie(w http.ResponseWriter, r *http.Request, token string) {
	cfg := a.config()
	mac := hmac.New(sha256.New, []byte(cfg.SessionSecret))
	mac.Write([]byte("share:" + token))
	http.SetCookie(w, &http.Cookie{
		Name:     shareCookieName(token),
		Value:    hex.EncodeToString(mac.Sum(nil)),
		Path:     "/s/" + token,
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) handleShareUnlock(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	it := a.shares.get(token)
	if it == nil || it.expired() {
		a.renderShareError(w, "Compartilhamento não encontrado", "Este link não existe ou expirou.", http.StatusNotFound)
		return
	}
	ip := clientIP(r)
	if wait := a.shareLimiter.wait(ip); wait > 0 {
		a.renderShareError(w, "Muitas tentativas", "Aguarde alguns instantes e tente novamente.", http.StatusTooManyRequests)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderShareError(w, "Requisição inválida", "", http.StatusBadRequest)
		return
	}
	if !verifyPassword(r.FormValue("password"), it.PasswordHash) {
		a.shareLimiter.fail(ip)
		a.activity.Add(Activity{IP: ip, Action: "share_unlock_fail", Detail: token, OK: false})
		a.renderSharePage(w, r, it, true, "Senha incorreta. Tente novamente.")
		return
	}
	a.shareLimiter.reset(ip)
	a.setShareCookie(w, r, token)
	next := r.FormValue("next")
	if next == "" || !strings.HasPrefix(next, "/s/"+token) {
		next = "/s/" + token
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (a *App) handleSharePage(w http.ResponseWriter, r *http.Request) {
	it := a.shares.get(r.PathValue("token"))
	if it == nil || it.expired() {
		a.renderShareError(w, "Compartilhamento não encontrado", "Este link não existe, foi revogado ou expirou.", http.StatusNotFound)
		return
	}
	if !a.shareUnlocked(r, it) {
		a.renderSharePage(w, r, it, true, "")
		return
	}
	a.renderSharePage(w, r, it, false, "")
}

func (a *App) shareFullPath(it *Share, sub string) (string, error) {
	base, err := a.resolveExisting(it.Path)
	if err != nil {
		return "", err
	}
	if !it.IsDir {
		return base, nil
	}
	joined := path.Join(it.Path, strings.TrimPrefix(sub, "/"))
	full, err := a.resolveExisting(joined)
	if err != nil {
		return "", err
	}
	baseReal, _ := filepath.EvalSymlinks(base)
	fullReal, _ := filepath.EvalSymlinks(full)
	if !within(baseReal, fullReal) {
		return "", errors.New("fora do compartilhamento")
	}
	return full, nil
}

type shareEntry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    string
	Kind    string
	Badge   string
	ModTime time.Time
}

type shareCrumb struct {
	Name string
	Path string
}

type shareView struct {
	Token         string
	Share         *Share
	ServerName    string
	NeedsPassword bool
	Error         string
	IsDir         bool
	Entries       []shareEntry
	Crumbs        []shareCrumb
	CurrentPath   string
	ParentPath    string
	FileName      string
	FileSize      string
	Kind          string
	Mime          string
	ExpiresText   string
	Downloads     int
	InlineURL     string
	DownloadURL   string
	ZipURL        string
	ShareURL      string
	TotalSize     string
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func dateBR(t time.Time) string {
	return t.Local().Format("02/01/2006 15:04")
}

func fileBadge(kind, name string) string {
	switch kind {
	case "image":
		return "IMG"
	case "video":
		return "MP4"
	case "audio":
		return "AUD"
	case "pdf":
		return "PDF"
	case "archive":
		return "ZIP"
	case "text":
		return "TXT"
	case "doc":
		return "DOC"
	case "sheet":
		return "XLS"
	case "slides":
		return "PPT"
	}
	ext := strings.TrimPrefix(strings.ToUpper(filepath.Ext(name)), ".")
	if len(ext) > 4 {
		ext = ext[:4]
	}
	if ext == "" {
		ext = "FILE"
	}
	return ext
}

func (a *App) renderSharePage(w http.ResponseWriter, r *http.Request, it *Share, needsPassword bool, errMsg string) {
	cfg := a.config()
	v := shareView{
		Token: it.Token, Share: it, ServerName: cfg.ServerName, NeedsPassword: needsPassword,
		Error: errMsg, ExpiresText: "", Downloads: it.Downloads,
	}
	if it.ExpiresAt != nil {
		v.ExpiresText = "Expira em " + dateBR(*it.ExpiresAt)
	}
	if needsPassword {
		v.ShareURL = "/s/" + it.Token
		a.renderShareTemplate(w, v, http.StatusOK)
		return
	}
	full, err := a.shareFullPath(it, r.URL.Query().Get("path"))
	if err != nil {
		a.renderShareError(w, "Item não encontrado", "O caminho solicitado não faz parte deste compartilhamento.", http.StatusNotFound)
		return
	}
	st, err := os.Stat(full)
	if err != nil {
		a.renderShareError(w, "Item não encontrado", "O arquivo ou pasta não existe mais.", http.StatusNotFound)
		return
	}
	sub := a.shareRel(it, full)
	v.CurrentPath = sub
	v.ShareURL = "/s/" + it.Token
	v.DownloadURL = "/s/" + it.Token + "/raw"
	v.ZipURL = "/s/" + it.Token + "/zip"
	if st.IsDir() {
		v.IsDir = true
		v.Entries, v.TotalSize = a.shareEntries(it, full)
		v.Crumbs = buildCrumbs(it, sub)
		if sub != "" {
			v.ParentPath = path.Dir(sub)
			if v.ParentPath == "." {
				v.ParentPath = ""
			}
		}
	} else {
		v.FileName = filepath.Base(full)
		v.FileSize = humanSize(st.Size())
		v.Kind = kindOf(v.FileName, false)
		v.Mime = mimeOf(v.FileName)
		v.InlineURL = "/s/" + it.Token + "/raw?inline=1"
		v.DownloadURL = "/s/" + it.Token + "/raw"
	}
	a.renderShareTemplate(w, v, http.StatusOK)
}

func (a *App) shareBase(it *Share) string {
	full, err := a.resolveExisting(it.Path)
	if err != nil {
		return ""
	}
	return full
}

func (a *App) shareRel(it *Share, full string) string {
	base := a.shareBase(it)
	if base == "" || !within(base, full) {
		return ""
	}
	rel, err := filepath.Rel(base, full)
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

func (a *App) shareEntries(it *Share, dir string) ([]shareEntry, string) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, ""
	}
	base, _ := a.shareFullPath(it, "")
	var out []shareEntry
	var total int64
	for _, de := range des {
		name := de.Name()
		info, err := de.Info()
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(base, filepath.Join(dir, name))
		if err != nil {
			continue
		}
		out = append(out, shareEntry{
			Name: name, Path: filepath.ToSlash(rel), IsDir: info.IsDir(),
			Size: humanSize(info.Size()), Kind: kindOf(name, info.IsDir()),
			Badge: fileBadge(kindOf(name, info.IsDir()), name), ModTime: info.ModTime(),
		})
		if !info.IsDir() {
			total += info.Size()
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return naturalLess(out[i].Name, out[j].Name)
	})
	return out, humanSize(total)
}

func buildCrumbs(it *Share, sub string) []shareCrumb {
	crumbs := []shareCrumb{{Name: it.Name, Path: ""}}
	if sub == "" {
		return crumbs
	}
	parts := strings.Split(sub, "/")
	cur := ""
	for _, p := range parts {
		if cur == "" {
			cur = p
		} else {
			cur += "/" + p
		}
		crumbs = append(crumbs, shareCrumb{Name: p, Path: cur})
	}
	return crumbs
}

func (a *App) renderShareError(w http.ResponseWriter, title, msg string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = shareErrorTmpl.Execute(w, map[string]string{"Title": title, "Message": msg})
}

func (a *App) renderShareTemplate(w http.ResponseWriter, v shareView, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := shareTmpl.Execute(w, v); err != nil {
		http.Error(w, "erro ao renderizar", http.StatusInternalServerError)
	}
}

func (a *App) handleShareRaw(w http.ResponseWriter, r *http.Request) {
	it := a.shares.get(r.PathValue("token"))
	if it == nil || it.expired() {
		http.Error(w, "não encontrado", http.StatusNotFound)
		return
	}
	if !a.shareUnlocked(r, it) {
		http.Error(w, "senha necessária", http.StatusForbidden)
		return
	}
	full, err := a.shareFullPath(it, r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, "não encontrado", http.StatusNotFound)
		return
	}
	st, err := os.Stat(full)
	if err != nil || st.IsDir() {
		http.Error(w, "não encontrado", http.StatusNotFound)
		return
	}
	inline := r.URL.Query().Get("inline") == "1"
	if !inline {
		a.shares.increment(it.Token)
		a.activity.Add(Activity{IP: clientIP(r), Action: "share_download", Detail: it.Name, OK: true})
	}
	a.servePath(w, r, full, filepath.Base(full), inline)
}

func (a *App) handleShareZip(w http.ResponseWriter, r *http.Request) {
	it := a.shares.get(r.PathValue("token"))
	if it == nil || it.expired() {
		http.Error(w, "não encontrado", http.StatusNotFound)
		return
	}
	if !a.shareUnlocked(r, it) {
		http.Error(w, "senha necessária", http.StatusForbidden)
		return
	}
	sub := r.URL.Query().Get("path")
	full, err := a.shareFullPath(it, sub)
	if err != nil {
		http.Error(w, "não encontrado", http.StatusNotFound)
		return
	}
	rel := it.Path
	if sub != "" {
		rel = path.Join(it.Path, sub)
	}
	a.shares.increment(it.Token)
	a.streamZip(w, r, []string{rel})
	_ = full
}

// ---------- templates ----------

var shareFuncs = template.FuncMap{
	"q":         url.QueryEscape,
	"size":      humanSize,
	"date":      dateBR,
	"hasPwd":    func(s *Share) bool { return s.PasswordHash != "" },
	"filebadge": fileBadge,
}

var shareTmpl = template.Must(template.New("share").Funcs(shareFuncs).Parse(sharePageHTML))
var shareErrorTmpl = template.Must(template.New("shareerr").Parse(shareErrorHTML))

const sharePageHTML = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<meta name="theme-color" content="#0a0b0d">
<meta name="color-scheme" content="dark">
<title>{{.Share.Name}} · {{.ServerName}}</title>
<link rel="icon" href="/icon.svg?v=2" type="image/svg+xml">
<style>
:root{color-scheme:dark;--bg:#0a0b0d;--surface:#111316;--surface-2:#171a1e;--line:rgba(255,255,255,.08);--line-2:rgba(255,255,255,.12);--text:#eef1f4;--muted:#9aa3ad;--muted-2:#7f8892;--accent:#9be7c4;--accent-ink:#07100b;--danger:#ffb4ae}
*{box-sizing:border-box}
html,body{min-height:100%}
body{margin:0;background:var(--bg);color:var(--text);font:15px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;display:flex;flex-direction:column;min-height:100dvh}
.wrap{width:min(820px,100%);margin:0 auto;padding:24px 18px 48px;flex:1}
header{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:24px}
.brand{display:flex;align-items:center;gap:10px;font-weight:650;letter-spacing:-.02em;min-width:0}
.logo{width:34px;height:34px;border-radius:9px;flex:none}
.chip{font-size:11.5px;color:var(--muted);border:1px solid var(--line);padding:5px 10px;border-radius:999px;background:var(--surface);white-space:nowrap}
.card{background:var(--surface);border:1px solid var(--line);border-radius:16px;padding:22px}
h1{font-size:22px;margin:0 0 4px;letter-spacing:-.03em;word-break:break-word}
.sub{color:var(--muted);font-size:13px;margin:0 0 20px}
.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;border:1px solid transparent;border-radius:10px;padding:11px 16px;font-weight:600;font-size:14px;color:var(--accent-ink);background:var(--accent);text-decoration:none;cursor:pointer;transition:filter .15s ease}
.btn:hover{filter:brightness(.96)}
.btn:active{transform:translateY(1px)}
.btn.ghost{background:transparent;border-color:var(--line);color:var(--text)}
.btn.ghost:hover{background:var(--surface-2)}
.btn.block{width:100%}
.btn.small{padding:7px 11px;font-size:12px;border-radius:8px}
.crumbs{display:flex;flex-wrap:wrap;gap:6px;align-items:center;font-size:12.5px;color:var(--muted-2);margin-bottom:12px}
.crumbs a{color:var(--text);text-decoration:none;padding:3px 7px;border-radius:6px;background:var(--surface-2)}
.crumbs span.sep{opacity:.4}
.list{display:flex;flex-direction:column;border:1px solid var(--line);border-radius:12px;overflow:hidden;background:var(--bg)}
.row{display:grid;grid-template-columns:38px 1fr auto;gap:12px;align-items:center;padding:11px 14px;border-top:1px solid var(--line);text-decoration:none;color:var(--text)}
.row:first-child{border-top:0}
.row:hover{background:var(--surface-2)}
.badge{width:34px;height:34px;border-radius:9px;display:grid;place-items:center;font-size:9.5px;font-weight:750;letter-spacing:.03em;background:var(--surface-2);color:var(--accent);border:1px solid var(--line)}
.badge.image{background:rgba(155,231,196,.08);color:#a8ebca;border-color:rgba(155,231,196,.16)}
.badge.video{background:rgba(232,194,122,.08);color:#e8c27a;border-color:rgba(232,194,122,.16)}
.badge.audio{background:rgba(196,168,255,.08);color:#c4a8ff;border-color:rgba(196,168,255,.16)}
.badge.pdf{background:rgba(240,138,128,.08);color:#f08a80;border-color:rgba(240,138,128,.16)}
.badge.archive{background:rgba(143,180,255,.08);color:#8fb4ff;border-color:rgba(143,180,255,.16)}
.name{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-weight:600;font-size:14px}
.meta{color:var(--muted-2);font-size:12px;margin-top:2px}
.row .go{color:var(--muted-2);font-size:17px}
.file-hero{display:flex;flex-direction:column;align-items:center;text-align:center;gap:15px;padding:14px 4px 4px}
.file-hero .big{width:72px;height:72px;border-radius:18px;font-size:17px;display:grid;place-items:center;background:rgba(155,231,196,.08);border:1px solid rgba(155,231,196,.16);color:var(--accent);font-weight:750}
.preview{width:100%;border-radius:12px;margin-top:16px;background:#000;max-height:62vh;object-fit:contain;border:1px solid var(--line)}
audio.preview{background:transparent;border:0}
iframe.preview{height:62vh;border:1px solid var(--line);background:#fff}
.foot{margin-top:24px;text-align:center;color:var(--muted-2);font-size:11.5px}
.pw{max-width:400px;margin:8vh auto 0}
.pw h1,.pw p{text-align:center}
label{display:block;font-size:12px;color:var(--muted);margin:16px 0 6px;font-weight:600}
input[type=password]{width:100%;padding:12px 13px;border-radius:9px;border:1px solid var(--line);background:var(--bg);color:var(--text);font-size:15px;outline:none}
input[type=password]:focus{border-color:var(--accent);box-shadow:0 0 0 3px rgba(155,231,196,.14)}
.err{background:rgba(240,138,128,.08);border:1px solid rgba(240,138,128,.22);color:var(--danger);padding:10px 12px;border-radius:9px;font-size:13px;margin:14px 0}
.actions{display:flex;gap:10px;flex-wrap:wrap;margin-top:18px}
.actions .btn{flex:1;min-width:140px}
@media(max-width:520px){.wrap{padding:16px 12px 36px}.card{padding:16px;border-radius:14px}h1{font-size:19px}.row{padding:10px 11px;grid-template-columns:34px 1fr auto}.badge{width:30px;height:30px;font-size:8.5px}header{margin-bottom:18px}}
</style>
</head>
<body>
<div class="wrap">
<header>
  <div class="brand"><img class="logo" src="/icon.svg?v=2" alt=""><span>{{.ServerName}}</span></div>
  <div class="chip">Compartilhamento</div>
</header>
{{if .NeedsPassword}}
<div class="card pw">
  <h1>Link protegido</h1>
  <p class="sub">Digite a senha para acessar este compartilhamento.</p>
  {{if .Error}}<div class="err">{{.Error}}</div>{{end}}
  <form method="post" action="/s/{{.Token}}/unlock">
    <input type="hidden" name="next" value="{{.ShareURL}}">
    <label for="pw">Senha</label>
    <input id="pw" name="password" type="password" autocomplete="current-password" required autofocus>
    <div class="actions"><button class="btn block" type="submit">Desbloquear</button></div>
  </form>
</div>
{{else if .IsDir}}
<div class="card">
  <h1>{{.Share.Name}}</h1>
  <p class="sub">Pasta compartilhada · {{.TotalSize}} · {{len .Entries}} item(ns)</p>
  <div class="crumbs">
    {{range $i, $c := .Crumbs}}{{if $i}}<span class="sep">/</span>{{end}}<a href="/s/{{$.Token}}{{if $c.Path}}?path={{$c.Path | q}}{{end}}">{{$c.Name}}</a>{{end}}
  </div>
  <div class="list">
    {{if .ParentPath}}<a class="row" href="/s/{{.Token}}{{if .ParentPath}}?path={{.ParentPath | q}}{{end}}"><div class="badge">↑</div><div class="name">Voltar</div><div class="go">‹</div></a>{{end}}
    {{range .Entries}}
      {{if .IsDir}}
      <a class="row" href="/s/{{$.Token}}?path={{.Path | q}}">
        <div class="badge">DIR</div>
        <div><div class="name">{{.Name}}</div><div class="meta">Pasta</div></div>
        <div class="go">›</div>
      </a>
      {{else}}
      <a class="row" href="/s/{{$.Token}}/raw?path={{.Path | q}}">
        <div class="badge {{.Kind}}">{{.Badge}}</div>
        <div><div class="name">{{.Name}}</div><div class="meta">{{.Size}} · {{date .ModTime}}</div></div>
        <div class="go">↓</div>
      </a>
      {{end}}
    {{end}}
    {{if not .Entries}}<div class="row"><div class="badge">∅</div><div><div class="name">Pasta vazia</div></div><div></div></div>{{end}}
  </div>
  <div class="actions">
    <a class="btn" href="/s/{{.Token}}/zip{{if .CurrentPath}}?path={{.CurrentPath | q}}{{end}}">Baixar tudo (.zip)</a>
  </div>
</div>
{{else}}
<div class="card">
  <div class="file-hero">
    <div class="big">{{filebadge .Kind .FileName}}</div>
    <div>
      <h1>{{.FileName}}</h1>
      <p class="sub">{{.FileSize}}{{if .ExpiresText}} · {{.ExpiresText}}{{end}}</p>
    </div>
    <div class="actions" style="width:100%">
      <a class="btn" href="{{.DownloadURL}}">Baixar arquivo</a>
    </div>
  </div>
  {{if eq .Kind "image"}}<img class="preview" src="{{.InlineURL}}" alt="{{.FileName}}">
  {{else if eq .Kind "video"}}<video class="preview" controls preload="metadata" src="{{.InlineURL}}"></video>
  {{else if eq .Kind "audio"}}<audio class="preview" controls preload="metadata" src="{{.InlineURL}}"></audio>
  {{else if eq .Kind "pdf"}}<iframe class="preview" src="{{.InlineURL}}" title="{{.FileName}}"></iframe>{{end}}
</div>
{{end}}
<div class="foot">Compartilhado com {{.ServerName}}</div>
</div>
</body>
</html>`

const shareErrorHTML = `<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="theme-color" content="#0a0b0d"><meta name="color-scheme" content="dark">
<title>{{.Title}}</title>
<style>:root{color-scheme:dark}body{margin:0;min-height:100dvh;display:grid;place-items:center;background:#0a0b0d;color:#eef1f4;font:15px system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;text-align:center;padding:24px}
.box{max-width:420px}.logo{width:56px;height:56px;margin:0 auto 18px;border-radius:15px;display:block}
h1{font-size:22px;margin:0 0 8px;letter-spacing:-.02em}p{color:#9aa3ad;margin:0 0 20px;line-height:1.6}a{color:#9be7c4}</style></head>
<body><div class="box"><img class="logo" src="/icon.svg?v=2" alt=""><h1>{{.Title}}</h1><p>{{.Message}}</p><a href="/">Ir para o início</a></div></body></html>`
