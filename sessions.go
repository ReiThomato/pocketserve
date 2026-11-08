package main

// Revogação de sessões.
//
// PROBLEMA QUE ISTO RESOLVE
//
// A sessão é sem estado: o cookie carrega {exp, versão da senha} assinado com
// HMAC. O servidor não guarda nada, então `handleLogout` só mandava o
// navegador descartar o cookie — o token antigo continuava válido no servidor.
// Quem tivesse copiado o cookie antes do logout mantinha o acesso.
//
// DESENHO ADOTADO
//
// Mantém a sessão sem estado como caminho normal (nenhuma consulta a disco por
// requisição) e adiciona uma lista de revogação só para o que foi encerrado.
//
//   - cada sessão ganha um identificador aleatório (ID) no payload;
//   - o logout grava sha256(ID) + expiração na lista;
//   - `validSession` consulta a lista em memória (mapa), O(1);
//   - a lista é persistida em <home>/revoked_sessions.json;
//   - entradas expiram junto com a sessão que revogam.
//
// POR QUE O ID É NECESSÁRIO
//
// Sem ele, dois logins no mesmo segundo produzem cookie idêntico (o payload só
// tem exp e versão). Revogar "o cookie" revogaria os dois — o usuário perderia
// a sessão do celular ao sair do notebook. O ID separa as sessões.
//
// LIMITE DE CRESCIMENTO
//
// A lista nunca pode passar do número de sessões encerradas dentro da janela
// de validade (SessionDays, no máximo 365). Cada entrada some sozinha quando
// expira; a limpeza roda ao carregar e a cada inserção. Não há acumulação.
//
// COMPATIBILIDADE
//
// Cookies emitidos antes desta versão não têm ID. Continuam aceitos até
// expirarem (ninguém é deslogado no deploy), mas não podem ser revogados
// individualmente — o que é inevitável, já que não há como distingui-los.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const revokedSessionsFile = "revoked_sessions.json"

// revokedSession é uma sessão encerrada. Guarda o hash do ID, nunca o ID.
type revokedSession struct {
	IDHash  string    `json:"id_hash"`
	Expires time.Time `json:"expires"`
	Revoked time.Time `json:"revoked"`
}

// sessionRevoker mantém a lista em memória e persiste em disco.
type sessionRevoker struct {
	mu      sync.RWMutex
	path    string
	items   map[string]time.Time // idHash -> expiração
	dirty   bool
	lastErr error
}

func newSessionRevoker(home string) *sessionRevoker {
	r := &sessionRevoker{
		path:  filepath.Join(home, revokedSessionsFile),
		items: map[string]time.Time{},
	}
	r.load()
	return r
}

// load lê o arquivo e descarta o que já expirou.
func (r *sessionRevoker) load() {
	b, err := os.ReadFile(r.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			r.lastErr = err
		}
		return
	}
	var list []revokedSession
	if err := json.Unmarshal(b, &list); err != nil {
		r.lastErr = err
		return
	}
	now := time.Now()
	for _, it := range list {
		if it.IDHash == "" || !it.Expires.After(now) {
			continue
		}
		r.items[it.IDHash] = it.Expires
	}
	// Se algo expirou na leitura, reescreve já compactado.
	if len(r.items) != len(list) {
		r.dirty = true
		r.saveLocked()
	}
}

// saveLocked grava o estado atual. Só entradas ainda válidas são escritas.
func (r *sessionRevoker) saveLocked() {
	now := time.Now()
	list := make([]revokedSession, 0, len(r.items))
	for hash, exp := range r.items {
		if !exp.After(now) {
			continue
		}
		list = append(list, revokedSession{IDHash: hash, Expires: exp, Revoked: now})
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		r.lastErr = err
		return
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		r.lastErr = err
		return
	}
	if err := os.Rename(tmp, r.path); err != nil {
		r.lastErr = err
		return
	}
	r.dirty = false
}

// revoke marca a sessão como encerrada até a expiração original.
func (r *sessionRevoker) revoke(sessionID string, expires time.Time) {
	if sessionID == "" {
		return
	}
	hash := sessionIDHash(sessionID)
	r.mu.Lock()
	defer r.mu.Unlock()
	// Sessão já expirada não precisa entrar na lista.
	if !expires.After(time.Now()) {
		return
	}
	r.items[hash] = expires
	r.pruneLocked()
	r.saveLocked()
}

// isRevoked informa se o ID foi encerrado.
func (r *sessionRevoker) isRevoked(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	hash := sessionIDHash(sessionID)
	r.mu.RLock()
	exp, ok := r.items[hash]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	// Entrada vencida não conta como revogada (a sessão já morreu sozinha).
	return exp.After(time.Now())
}

// pruneLocked remove entradas expiradas.
func (r *sessionRevoker) pruneLocked() int {
	now := time.Now()
	n := 0
	for hash, exp := range r.items {
		if !exp.After(now) {
			delete(r.items, hash)
			n++
		}
	}
	return n
}

// prune remove expirados e persiste se algo mudou. Usado pelo janitor.
func (r *sessionRevoker) prune() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := r.pruneLocked()
	if n > 0 {
		r.saveLocked()
	}
	return n
}

// count devolve quantas revogações estão ativas (para o painel).
func (r *sessionRevoker) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

// sessionIDHash guarda o hash do ID, não o ID. Se o arquivo vazar, ninguém
// consegue reconstruir o valor que está no cookie.
func sessionIDHash(id string) string {
	sum := sha256.Sum256([]byte("revoked:" + id))
	return hex.EncodeToString(sum[:])
}

// startRevocationJanitor limpa entradas vencidas de hora em hora. Mantém a
// lista pequena mesmo sem novos logouts.
func (a *App) startRevocationJanitor() {
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			if n := a.revoker.prune(); n > 0 {
				a.activity.Add(Activity{
					IP: "sistema", Action: "session_expire", OK: true,
					Detail: itoa(n) + " revogação(ões) expirada(s)",
				})
			}
			<-t.C
		}
	}()
}
