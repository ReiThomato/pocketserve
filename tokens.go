package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Tokens de API com escopo.
//
// Motivo: até a 1.5 a única forma de autenticar era o login com a senha do
// administrador, que devolve cookie de sessão. Um agente (script, rclone,
// WebDAV, LLM com ferramentas) precisaria da senha mestra — sem escopo, sem
// expiração própria e sem revogação individual.
//
// Modelo adotado (o mesmo de Stripe/GitHub e do SFTPGo):
//   - a chave é mostrada UMA vez, na criação;
//   - em disco fica apenas o SHA-256;
//   - guarda-se o prefixo (12 primeiros caracteres) para exibir na lista;
//   - revogação é imediata porque a validação é hash -> lookup;
//   - escopo e expiração são checados a cada requisição.
//
// SHA-256 puro (e não PBKDF2) porque o token tem 256 bits de entropia real:
// não há dicionário a proteger, e PBKDF2 custaria ~200 ms por requisição.

const tokenFile = "tokens.json"

// Escopos disponíveis. "admin" é superset e libera tudo.
const (
	scopeFilesRead    = "files:read"
	scopeFilesWrite   = "files:write"
	scopeFilesDelete  = "files:delete"
	scopeSharesManage = "shares:manage"
	scopeAdmin        = "admin"
)

var allScopes = []string{
	scopeFilesRead, scopeFilesWrite, scopeFilesDelete, scopeSharesManage, scopeAdmin,
}

func validScope(s string) bool {
	for _, v := range allScopes {
		if v == s {
			return true
		}
	}
	return false
}

// APIToken é um token persistido. Nunca contém a chave em claro.
type APIToken struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Hash      string     `json:"hash"`
	Prefix    string     `json:"prefix"`
	Scopes    []string   `json:"scopes"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	// LastUsed fica em memória e é gravado com atraso (ver markUsed), para não
	// reescrever o JSON a cada requisição de um cliente ativo.
	LastUsed *time.Time `json:"last_used,omitempty"`
	LastIP   string     `json:"last_ip,omitempty"`
}

func (t *APIToken) expired() bool {
	return t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt)
}

func (t *APIToken) revoked() bool { return t.RevokedAt != nil }

func (t *APIToken) active() bool { return !t.expired() && !t.revoked() }

// allows informa se o token tem o escopo pedido. "admin" libera qualquer um.
func (t *APIToken) allows(scope string) bool {
	for _, s := range t.Scopes {
		if s == scopeAdmin || s == scope {
			return true
		}
	}
	return false
}

// public devolve a visão segura do token para a API (sem o hash).
func (t *APIToken) public() map[string]any {
	out := map[string]any{
		"id": t.ID, "name": t.Name, "prefix": t.Prefix,
		"scopes": t.Scopes, "created_at": t.CreatedAt,
		"active": t.active(),
	}
	if t.ExpiresAt != nil {
		out["expires_at"] = t.ExpiresAt
	}
	if t.RevokedAt != nil {
		out["revoked_at"] = t.RevokedAt
	}
	if t.LastUsed != nil {
		out["last_used"] = t.LastUsed
	}
	if t.LastIP != "" {
		out["last_ip"] = t.LastIP
	}
	return out
}

// ---------- geração ----------

const tokenPrefixLiteral = "ps_"

func newTokenValue() string {
	// 32 bytes aleatórios -> 256 bits de entropia, 43 caracteres base64url.
	return tokenPrefixLiteral + randomToken(32)
}

func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// displayPrefix devolve o trecho mostrado na lista: "ps_AbCdEf..." .
// Guardar o prefixo permite identificar qual token é qual sem exibir a chave.
func displayPrefix(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

// ---------- store ----------

type TokenStore struct {
	mu    sync.Mutex
	path  string
	items map[string]*APIToken // por ID

	dirty    bool
	lastSave time.Time
}

func newTokenStore(home string) *TokenStore {
	s := &TokenStore{path: filepath.Join(home, tokenFile), items: map[string]*APIToken{}}
	b, err := os.ReadFile(s.path)
	if err == nil {
		var list []*APIToken
		if json.Unmarshal(b, &list) == nil {
			for _, it := range list {
				if it != nil && it.ID != "" && it.Hash != "" {
					s.items[it.ID] = it
				}
			}
		}
	}
	return s
}

func (s *TokenStore) saveLocked() {
	list := make([]*APIToken, 0, len(s.items))
	for _, it := range s.items {
		list = append(list, it)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return
	}
	s.dirty = false
	s.lastSave = time.Now()
}

// flush grava se houver alteração pendente há mais de 30 s. Chamado pelo
// marcador de uso para não escrever o arquivo a cada requisição.
func (s *TokenStore) flush(force bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return
	}
	if !force && time.Since(s.lastSave) < 30*time.Second {
		return
	}
	s.saveLocked()
}

func (s *TokenStore) add(t *APIToken) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[t.ID] = t
	s.saveLocked()
}

func (s *TokenStore) list() []*APIToken {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*APIToken, 0, len(s.items))
	for _, it := range s.items {
		cp := *it
		cp.Scopes = append([]string(nil), it.Scopes...)
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *TokenStore) get(id string) *APIToken {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it := s.items[id]; it != nil {
		cp := *it
		cp.Scopes = append([]string(nil), it.Scopes...)
		return &cp
	}
	return nil
}

func (s *TokenStore) revoke(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	it := s.items[id]
	if it == nil || it.RevokedAt != nil {
		return false
	}
	now := time.Now()
	it.RevokedAt = &now
	s.saveLocked()
	return true
}

func (s *TokenStore) remove(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items[id] == nil {
		return false
	}
	delete(s.items, id)
	s.saveLocked()
	return true
}

// lookup encontra o token pelo valor apresentado. Compara pelo hash, então
// a busca não expõe a chave nem permite reconstruí-la pelo arquivo.
func (s *TokenStore) lookup(value string) *APIToken {
	if value == "" {
		return nil
	}
	want := tokenHash(value)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, it := range s.items {
		if it.Hash == want && it.active() {
			cp := *it
			cp.Scopes = append([]string(nil), it.Scopes...)
			return &cp
		}
	}
	return nil
}

// markUsed atualiza o último uso em memória e agenda a gravação.
func (s *TokenStore) markUsed(id, ip string) {
	s.mu.Lock()
	it := s.items[id]
	if it == nil {
		s.mu.Unlock()
		return
	}
	now := time.Now()
	it.LastUsed = &now
	it.LastIP = ip
	s.dirty = true
	s.mu.Unlock()
	s.flush(false)
}

// ---------- autenticação ----------

// bearerToken extrai o valor de "Authorization: Bearer <chave>".
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const p = "bearer "
	if len(h) <= len(p) || !strings.EqualFold(h[:len(p)], p) {
		return ""
	}
	return strings.TrimSpace(h[len(p):])
}

// tokenRequest resolve o token de uma requisição, se houver.
func (a *App) tokenRequest(r *http.Request) *APIToken {
	value := bearerToken(r)
	if value == "" {
		return nil
	}
	t := a.tokens.lookup(value)
	if t == nil {
		return nil
	}
	a.tokens.markUsed(t.ID, clientIP(r))
	return t
}

// ---------- middleware ----------

// ---------- handlers ----------

func (a *App) handleTokenList(w http.ResponseWriter, r *http.Request) {
	list := a.tokens.list()
	out := make([]map[string]any, 0, len(list))
	for _, t := range list {
		out = append(out, t.public())
	}
	jsonWrite(w, http.StatusOK, map[string]any{
		"tokens": out,
		"scopes": allScopes,
	})
}

func (a *App) handleTokenCreate(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name      string   `json:"name"`
		Scopes    []string `json:"scopes"`
		ExpiresIn int      `json:"expires_days"` // 0 = nunca expira
	}
	if !decodeBody(w, r, &b) {
		return
	}
	name := strings.TrimSpace(b.Name)
	if name == "" || len([]rune(name)) > 60 {
		jsonError(w, http.StatusBadRequest, "bad_name", "Dê um nome ao token (1 a 60 caracteres).")
		return
	}
	if len(b.Scopes) == 0 {
		jsonError(w, http.StatusBadRequest, "no_scopes", "Escolha pelo menos um escopo.")
		return
	}
	scopes := make([]string, 0, len(b.Scopes))
	seen := map[string]bool{}
	for _, s := range b.Scopes {
		s = strings.TrimSpace(s)
		if !validScope(s) {
			jsonError(w, http.StatusBadRequest, "bad_scope", "Escopo desconhecido: "+s)
			return
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		scopes = append(scopes, s)
	}
	if b.ExpiresIn < 0 || b.ExpiresIn > 3650 {
		jsonError(w, http.StatusBadRequest, "bad_expiry", "Validade deve ficar entre 0 e 3650 dias.")
		return
	}

	value := newTokenValue()
	now := time.Now()
	t := &APIToken{
		ID:        randomToken(9),
		Name:      name,
		Hash:      tokenHash(value),
		Prefix:    displayPrefix(value),
		Scopes:    scopes,
		CreatedAt: now,
	}
	if b.ExpiresIn > 0 {
		exp := now.Add(time.Duration(b.ExpiresIn) * 24 * time.Hour)
		t.ExpiresAt = &exp
	}
	a.tokens.add(t)
	a.activity.Add(Activity{
		IP: clientIP(r), Action: "token_create", OK: true,
		Detail: name + " (" + strings.Join(scopes, ",") + ")",
	})
	// A chave em claro aparece só aqui, uma única vez.
	jsonWrite(w, http.StatusOK, map[string]any{
		"ok":    true,
		"token": value,
		"item":  t.public(),
	})
}

func (a *App) handleTokenRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, "bad_id", "Token inválido.")
		return
	}
	if !a.tokens.revoke(id) {
		jsonError(w, http.StatusNotFound, "not_found", "Token não encontrado ou já revogado.")
		return
	}
	a.tokens.flush(true)
	a.activity.Add(Activity{IP: clientIP(r), Action: "token_revoke", Detail: id, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleTokenDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, "bad_id", "Token inválido.")
		return
	}
	if !a.tokens.remove(id) {
		jsonError(w, http.StatusNotFound, "not_found", "Token não encontrado.")
		return
	}
	a.activity.Add(Activity{IP: clientIP(r), Action: "token_delete", Detail: id, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

// handleWhoAmI permite a um agente confirmar o que pode fazer.
func (a *App) handleWhoAmI(w http.ResponseWriter, r *http.Request) {
	if t := a.tokenRequest(r); t != nil {
		jsonWrite(w, http.StatusOK, map[string]any{
			"auth": "token", "name": t.Name, "scopes": t.Scopes,
			"expires_at": t.ExpiresAt,
		})
		return
	}
	if a.validSession(r) {
		jsonWrite(w, http.StatusOK, map[string]any{"auth": "session", "scopes": allScopes})
		return
	}
	jsonError(w, http.StatusUnauthorized, "unauthorized", "Não autenticado.")
}
