package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const sessionCookie = "ps_session"

// ---------- password hashing (PBKDF2-HMAC-SHA256, stdlib only) ----------

const pbkdf2Iterations = 210000

func pbkdf2Key(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var buf [4]byte
	dk := make([]byte, 0, numBlocks*hashLen)
	u := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)
		prf.Write(buf[:])
		dk = prf.Sum(dk)
		t := dk[len(dk)-hashLen:]
		copy(u, t)
		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for i := range t {
				t[i] ^= u[i]
			}
		}
	}
	return dk[:keyLen]
}

func hashPassword(password string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	key := pbkdf2Key([]byte(password), salt, pbkdf2Iterations, 32)
	return "pbkdf2$" + strconv.Itoa(pbkdf2Iterations) + "$" +
		base64.RawStdEncoding.EncodeToString(salt) + "$" +
		base64.RawStdEncoding.EncodeToString(key)
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1000 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[2])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got := pbkdf2Key([]byte(password), salt, iter, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ---------- sessions ----------

// passwordVersion deriva um identificador curto do hash da senha. Trocar a
// senha muda a versão, o que invalida todas as sessões existentes.
func passwordVersion(hash string) string {
	if hash == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(hash))
	return hex.EncodeToString(sum[:6])
}

type sessionPayload struct {
	Exp int64  `json:"exp"`
	Ver string `json:"v"`
	// ID identifica a sessão individualmente, permitindo revogar uma sem
	// derrubar as outras. Cookies emitidos antes da 1.11 não têm este campo
	// e continuam válidos até expirar (não são revogáveis individualmente).
	ID string `json:"id,omitempty"`
}

// sessionTTL devolve a duração efetiva da sessão e a expiração resultante.
// Centralizado para que setSession e a revogação usem o mesmo valor — se
// divergissem, a entrada de revogação poderia viver menos que o cookie.
func (a *App) sessionTTL(days int) (time.Duration, time.Time) {
	if days <= 0 {
		days = a.config().SessionDays
	}
	ttl := time.Duration(days) * 24 * time.Hour
	return ttl, time.Now().Add(ttl)
}

func (a *App) setSession(w http.ResponseWriter, r *http.Request, days int) {
	cfg := a.config()
	ttl, exp := a.sessionTTL(days)
	p := sessionPayload{
		Exp: exp.Unix(),
		Ver: passwordVersion(cfg.PasswordHash),
		ID:  randomToken(16),
	}
	raw, _ := json.Marshal(p)
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(cfg.SessionSecret))
	mac.Write([]byte(payload))
	value := payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) clearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// readSession valida a assinatura e devolve o payload. Não confere revogação
// nem expiração — quem chama decide.
func (a *App) readSession(r *http.Request) (sessionPayload, bool) {
	var p sessionPayload
	cfg := a.config()
	if cfg.PasswordHash == "" {
		return p, false
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return p, false
	}
	payload, sig, ok := strings.Cut(c.Value, ".")
	if !ok {
		return p, false
	}
	mac := hmac.New(sha256.New, []byte(cfg.SessionSecret))
	mac.Write([]byte(payload))
	want, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(mac.Sum(nil), want) {
		return p, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return p, false
	}
	if json.Unmarshal(raw, &p) != nil {
		return p, false
	}
	return p, true
}

func (a *App) validSession(r *http.Request) bool {
	cfg := a.config()
	p, ok := a.readSession(r)
	if !ok {
		return false
	}
	if p.Exp <= time.Now().Unix() {
		return false
	}
	if p.Ver != passwordVersion(cfg.PasswordHash) {
		return false
	}
	// Sessão encerrada explicitamente não vale mais, mesmo com assinatura boa.
	if a.revoker != nil && a.revoker.isRevoked(p.ID) {
		return false
	}
	return true
}

// ---------- login throttling ----------

type limitEntry struct {
	fails        int
	blockedUntil time.Time
	last         time.Time
}

type loginLimiter struct {
	mu sync.Mutex
	m  map[string]*limitEntry
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{m: make(map[string]*limitEntry)}
}

func (l *loginLimiter) wait(ip string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.m[ip]
	if e == nil {
		return 0
	}
	d := time.Until(e.blockedUntil)
	if d > 0 {
		return d
	}
	return 0
}

func (l *loginLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e := l.m[ip]
	if e == nil {
		e = &limitEntry{}
		l.m[ip] = e
	}
	// reset the counter if the last failure was a long time ago
	if now.Sub(e.last) > 30*time.Minute {
		e.fails = 0
	}
	e.fails++
	e.last = now
	if e.fails >= 5 {
		block := time.Minute << uint(minInt(e.fails-5, 4)) // 1m,2m,4m,8m,16m
		e.blockedUntil = now.Add(block)
	}
	if len(l.m) > 2000 {
		for k, v := range l.m {
			if now.Sub(v.last) > 24*time.Hour {
				delete(l.m, k)
			}
		}
	}
}

func (l *loginLimiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.m, ip)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------- request helpers ----------

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// Só confia em cabeçalhos de proxy quando a conexão vem do próprio aparelho
	// (proxy local) ou de loopback, evitando falsificação por clientes da rede.
	if isLoopbackHost(host) {
		// Cloudflare Tunnel entrega o IP original neste cabeçalho. Sem ele,
		// todas as requisições do túnel seriam contadas como 127.0.0.1 e o
		// limite de tentativas viraria um balde único.
		if ip := net.ParseIP(strings.TrimSpace(strings.Trim(r.Header.Get("CF-Connecting-IP"), "[]"))); ip != nil {
			return ip.String()
		}
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			first := strings.TrimSpace(strings.Split(fwd, ",")[0])
			if ip := net.ParseIP(strings.TrimSpace(strings.Trim(first, "[]"))); ip != nil {
				return ip.String()
			}
		}
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	return host
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(strings.TrimSpace(strings.Trim(host, "[]")))
	return ip != nil && ip.IsLoopback()
}

// requestIsHTTPS informa se o cookie pode ser marcado como Secure. Em HTTP puro
// na LAN isso quebraria o login, então o flag só é ativado em TLS direto ou
// quando um proxy local confirma o encaminhamento HTTPS.
func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return isLoopbackHost(host) && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func isMutating(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// checkCSRF requires a custom header (impossible for a cross-site form) and,
// when present, a same-origin Origin header.
func checkCSRF(r *http.Request) bool {
	if r.Header.Get("X-PS") != "1" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	return true
}

func (a *App) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := a.config()
		if cfg.PasswordHash == "" {
			jsonError(w, http.StatusPreconditionRequired, "setup_required", "Configure a senha do administrador primeiro.")
			return
		}
		if !a.validSession(r) {
			jsonError(w, http.StatusUnauthorized, "unauthorized", "Sessão expirada. Entre novamente.")
			return
		}
		if isMutating(r) && !checkCSRF(r) {
			jsonError(w, http.StatusForbidden, "csrf", "Requisição inválida.")
			return
		}
		next(w, r)
	}
}

// authAny aceita duas formas de credencial na mesma rota:
//
//   - cookie de sessão (o painel no navegador), com CSRF como sempre;
//   - Authorization: Bearer <token> (agentes, scripts, WebDAV), exigindo o
//     escopo indicado e sem CSRF — o navegador não anexa esse header sozinho,
//     então não existe requisição cross-site capaz de forjar a credencial.
//
// A sessão concede todos os escopos: é o administrador humano.
func (a *App) authAny(scope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := a.config()
		if cfg.PasswordHash == "" {
			jsonError(w, http.StatusPreconditionRequired, "setup_required", "Configure a senha do administrador primeiro.")
			return
		}
		// Token tem precedência: um cliente que mandou Bearer quer ser tratado
		// como token, mesmo que também carregue um cookie antigo.
		if bearerToken(r) != "" {
			t := a.tokenRequest(r)
			if t == nil {
				jsonError(w, http.StatusUnauthorized, "unauthorized", "Token ausente, inválido ou revogado.")
				return
			}
			if !t.allows(scope) {
				jsonWrite(w, http.StatusForbidden, map[string]any{
					"error":   "Este token não tem o escopo necessário.",
					"code":    "missing_scope",
					"missing": scope,
				})
				return
			}
			next(w, r)
			return
		}
		if !a.validSession(r) {
			jsonError(w, http.StatusUnauthorized, "unauthorized", "Sessão expirada. Entre novamente.")
			return
		}
		if isMutating(r) && !checkCSRF(r) {
			jsonError(w, http.StatusForbidden, "csrf", "Requisição inválida.")
			return
		}
		next(w, r)
	}
}

// ---------- handlers ----------

// handleHealth é deliberadamente público e sem dados sensíveis: serve apenas
// para monitoramento externo (Uptime Kuma, systemd, scripts) saber se o
// processo está respondendo.
func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "service": "pocketserve", "version": Version})
}

func (a *App) handleState(w http.ResponseWriter, r *http.Request) {
	cfg := a.config()
	jsonWrite(w, http.StatusOK, map[string]any{
		"needs_setup":   cfg.PasswordHash == "",
		"authenticated": a.validSession(r),
		"name":          cfg.ServerName,
		"version":       Version,
	})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !checkCSRF(r) {
		jsonError(w, http.StatusForbidden, "csrf", "Requisição inválida.")
		return
	}
	ip := clientIP(r)
	if wait := a.limiter.wait(ip); wait > 0 {
		secs := int(wait.Seconds()) + 1
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		jsonWrite(w, http.StatusTooManyRequests, map[string]any{
			"error":       "Muitas tentativas. Tente novamente em " + humanSeconds(secs) + ".",
			"retry_after": secs,
		})
		return
	}
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
		Remember bool   `json:"remember"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "bad_request", "Dados inválidos.")
		return
	}
	cfg := a.config()
	if cfg.PasswordHash == "" {
		jsonError(w, http.StatusPreconditionRequired, "setup_required", "Crie a senha do administrador primeiro.")
		return
	}
	if !verifyPassword(body.Password, cfg.PasswordHash) {
		a.limiter.fail(ip)
		a.activity.Add(Activity{IP: ip, Action: "login_fail", Detail: "", OK: false})
		jsonError(w, http.StatusUnauthorized, "invalid_password", "Senha incorreta. Verifique maiúsculas, acentos e se o teclado não adicionou espaço no fim.")
		return
	}
	// Segundo fator: só depois da senha correta, para não revelar a existência
	// do 2FA a quem não sabe a senha. O rate limit vale para as duas etapas.
	if cfg.TOTPSecret != "" && !a.trustedValid(r) {
		if strings.TrimSpace(body.Code) == "" {
			jsonWrite(w, http.StatusUnauthorized, map[string]any{
				"error": "Código de verificação necessário.",
				"code":  "totp_required",
			})
			return
		}
		if !a.checkSecondFactor(body.Code) {
			a.limiter.fail(ip)
			a.activity.Add(Activity{IP: ip, Action: "login_totp_fail", Detail: "", OK: false})
			jsonError(w, http.StatusUnauthorized, "bad_code", "Código incorreto ou expirado.")
			return
		}
		a.activity.Add(Activity{IP: ip, Action: "login_totp", Detail: "", OK: true})
		if body.Remember {
			if err := a.trustDevice(w, r); err != nil {
				// Falhar ao lembrar o aparelho não impede o login.
				_ = err
			}
		}
	}
	a.limiter.reset(ip)
	a.setSession(w, r, 0)
	a.activity.Add(Activity{IP: ip, Action: "login", Detail: "", OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleSetup(w http.ResponseWriter, r *http.Request) {
	if !checkCSRF(r) {
		jsonError(w, http.StatusForbidden, "csrf", "Requisição inválida.")
		return
	}
	cfg := a.config()
	if cfg.PasswordHash != "" {
		jsonError(w, http.StatusForbidden, "already_setup", "O administrador já foi configurado.")
		return
	}
	var body struct {
		Password   string `json:"password"`
		ServerName string `json:"server_name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "bad_request", "Dados inválidos.")
		return
	}
	if len(strings.TrimSpace(body.Password)) < 8 {
		jsonError(w, http.StatusBadRequest, "weak_password", "A senha precisa ter pelo menos 8 caracteres.")
		return
	}
	name := strings.TrimSpace(body.ServerName)
	if name == "" {
		name = "PocketServe"
	}
	if len([]rune(name)) > 40 {
		jsonError(w, http.StatusBadRequest, "bad_name", "O nome do servidor é longo demais.")
		return
	}
	err := a.updateConfig(func(c *Config) {
		c.PasswordHash = hashPassword(body.Password)
		c.ServerName = name
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "save_failed", "Não foi possível salvar a configuração.")
		return
	}
	a.setSession(w, r, 0)
	a.activity.Add(Activity{IP: clientIP(r), Action: "setup", Detail: name, OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !checkCSRF(r) {
		jsonError(w, http.StatusForbidden, "csrf", "Requisição inválida.")
		return
	}
	// Revoga no servidor antes de limpar o cookie. Sem isto, quem tivesse
	// copiado o cookie continuaria entrando depois do logout.
	if p, ok := a.readSession(r); ok && p.ID != "" {
		a.revoker.revoke(p.ID, time.Unix(p.Exp, 0))
	}
	a.clearSession(w, r)
	a.activity.Add(Activity{IP: clientIP(r), Action: "logout", OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "bad_request", "Dados inválidos.")
		return
	}
	cfg := a.config()
	if !verifyPassword(body.Current, cfg.PasswordHash) {
		jsonError(w, http.StatusForbidden, "invalid_password", "Senha atual incorreta.")
		return
	}
	if len(strings.TrimSpace(body.New)) < 8 {
		jsonError(w, http.StatusBadRequest, "weak_password", "A nova senha precisa ter pelo menos 8 caracteres.")
		return
	}
	if err := a.updateConfig(func(c *Config) { c.PasswordHash = hashPassword(body.New) }); err != nil {
		jsonError(w, http.StatusInternalServerError, "save_failed", "Não foi possível salvar a senha.")
		return
	}
	a.setSession(w, r, 0)
	a.activity.Add(Activity{IP: clientIP(r), Action: "password", Detail: "", OK: true})
	jsonWrite(w, http.StatusOK, map[string]any{"ok": true})
}

func humanSeconds(s int) string {
	if s < 60 {
		return strconv.Itoa(s) + "s"
	}
	m := s / 60
	if m < 60 {
		return strconv.Itoa(m) + " min"
	}
	return strconv.Itoa(m/60) + " h"
}
