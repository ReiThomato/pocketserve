package main

import (
	"bytes"
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web
var embeddedWeb embed.FS

type App struct {
	home    string
	logPath string

	mu  sync.RWMutex
	cfg Config

	started      time.Time
	limiter      *loginLimiter
	shareLimiter *loginLimiter
	activity     *ActivityStore
	shares       *ShareStore
	tokens       *TokenStore
	revoker      *sessionRevoker
	backups      *backupManager
	spaceAlerts  *spaceAlerter

	sysMu      sync.Mutex
	sysCache   *SystemInfo
	sysCacheAt time.Time
	cpuPrev    cpuTimes
	lastCPU    float64
}

func newApp(home string, cfg *Config) *App {
	a := &App{
		home:         home,
		logPath:      filepath.Join(home, "server.log"),
		cfg:          *cfg,
		started:      time.Now(),
		limiter:      newLoginLimiter(),
		shareLimiter: newLoginLimiter(),
		activity:     newActivityStore(home),
		shares:       newShareStore(home),
		tokens:       newTokenStore(home),
		spaceAlerts:  newSpaceAlerter(),
		revoker:      newSessionRevoker(home),
	}
	a.backups = newBackupManager(home, func() string { return a.rootPath() })
	return a
}

func (a *App) config() Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

func (a *App) updateConfig(fn func(*Config)) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := a.cfg
	fn(&cfg)
	if err := saveConfig(a.home, &cfg); err != nil {
		return err
	}
	a.cfg = cfg
	return nil
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)

	homeFlag := flag.String("home", defaultHome(), "pasta de configuração e dados")
	initFlag := flag.Bool("init", false, "cria/atualiza a configuração e sai")
	portFlag := flag.Int("port", 0, "porta HTTP (sobrescreve a configuração)")
	rootFlag := flag.String("root", "", "pasta raiz compartilhada (sobrescreve a configuração)")
	nameFlag := flag.String("name", "", "nome exibido do servidor")
	tsFlag := flag.String("ts-name", "", "nome MagicDNS do Tailscale (ex.: celular.tailXXXX.ts.net)")
	resetFlag := flag.Bool("reset-password", false, "remove a senha do administrador e sai")
	tlsCertFlag := flag.String("tls-cert", "", "certificado TLS (PEM) para servir HTTPS")
	tlsKeyFlag := flag.String("tls-key", "", "chave privada TLS (PEM)")
	publicHostFlag := flag.String("public-host", "", "domínio público usado nos links (ex.: sync.exemplo.com)")
	resetAccessFlag := flag.Bool("reset-access", false, "remove senha, 2FA e aparelhos confiáveis (recuperação de acesso)")
	versionFlag := flag.Bool("version", false, "mostra a versão e sai")
	flag.Parse()

	if *versionFlag {
		fmt.Println("PocketServe", Version)
		return
	}

	home := *homeFlag
	if err := os.MkdirAll(home, 0o700); err != nil {
		log.Fatalf("não foi possível criar %s: %v", home, err)
	}
	cfg, err := loadConfig(home)
	if err != nil {
		log.Fatalf("configuração: %v", err)
	}
	changed := false
	if *portFlag != 0 {
		if *portFlag < 1024 || *portFlag > 65535 {
			log.Fatal("porta inválida")
		}
		cfg.Port = *portFlag
		changed = true
	}
	if *rootFlag != "" {
		cfg.Root = normalizeRoot(*rootFlag)
		changed = true
	}
	if *nameFlag != "" {
		cfg.ServerName = *nameFlag
		changed = true
	}
	if *tlsCertFlag != "" {
		cfg.TLSCert = *tlsCertFlag
		changed = true
	}
	if *tlsKeyFlag != "" {
		cfg.TLSKey = *tlsKeyFlag
		changed = true
	}
	if *publicHostFlag != "" {
		cfg.PublicHost = strings.TrimSuffix(strings.TrimSpace(*publicHostFlag), ".")
		changed = true
	}
	if *tsFlag != "" {
		cfg.TailscaleName = strings.TrimSuffix(strings.TrimSpace(*tsFlag), ".")
		changed = true
	}
	if *resetFlag {
		cfg.PasswordHash = ""
		cfg.TOTPSecret = ""
		cfg.TOTPPending = ""
		cfg.RecoveryCodes = ""
		cfg.TrustedDevices = nil
		if err := saveConfig(home, cfg); err != nil {
			log.Fatalf("salvar configuração: %v", err)
		}
		fmt.Println("Senha do administrador removida. Reinicie o servidor e abra o painel para criar uma nova.")
		return
	}
	if *resetAccessFlag {
		cfg.PasswordHash = ""
		cfg.TOTPSecret = ""
		cfg.TOTPPending = ""
		cfg.RecoveryCodes = ""
		cfg.TrustedDevices = nil
		if err := saveConfig(home, cfg); err != nil {
			log.Fatalf("salvar configuração: %v", err)
		}
		fmt.Println("Acesso redefinido: senha, verificação em duas etapas e aparelhos confiáveis foram removidos.")
		fmt.Println("Reinicie o servidor e abra o painel para criar uma nova senha.")
		fmt.Println("Seus arquivos, compartilhamentos e backups NÃO foram alterados.")
		return
	}
	if changed {
		if err := saveConfig(home, cfg); err != nil {
			log.Fatalf("salvar configuração: %v", err)
		}
	}
	if *initFlag {
		fmt.Println("PocketServe configurado")
		fmt.Println("  configuração:", filepath.Join(home, "config.json"))
		fmt.Println("  raiz:        ", cfg.Root)
		fmt.Println("  porta:       ", cfg.Port)
		if cfg.TailscaleName != "" {
			fmt.Println("  tailscale:   ", cfg.TailscaleName)
		}
		return
	}

	// A raiz indisponível NÃO pode cair para "/": isso transformaria o painel
	// em acesso ao sistema de arquivos inteiro (um HD desconectado bastaria).
	// Aqui o processo encerra e o supervisor tenta de novo; se a raiz voltar,
	// o servidor sobe normalmente com os dados intactos.
	if st, err := os.Stat(cfg.Root); err != nil || !st.IsDir() {
		if os.Getenv("POCKETSERVE_ALLOW_MISSING_ROOT") == "1" {
			// Escape hatch consciente para diagnóstico: sobe na raiz atual do
			// processo, nunca em "/", e avisa em destaque no log.
			wd, _ := os.Getwd()
			safe, ferr := fallbackRoot(wd)
			if ferr != nil {
				log.Fatalf("raiz %q indisponível e %v; "+
					"POCKETSERVE_ALLOW_MISSING_ROOT não pode ser usado aqui", cfg.Root, ferr)
			}
			log.Printf("ATENÇÃO: raiz %q indisponível; subindo em %q por POCKETSERVE_ALLOW_MISSING_ROOT", cfg.Root, safe)
			cfg.Root = safe
		} else {
			log.Fatalf("raiz %q indisponível (HD desconectado?). Encerrando para evitar expor o sistema de arquivos. "+
				"Reconecte o disco e o supervisor reinicia sozinho; para diagnóstico use POCKETSERVE_ALLOW_MISSING_ROOT=1.", cfg.Root)
		}
	}

	a := newApp(home, cfg)
	a.pruneTrusted()
	a.startTrashJanitor()
	a.startBackupScheduler()
	a.startSpaceMonitor()
	a.startRevocationJanitor()
	mux := a.routes()
	addr := "0.0.0.0:" + strconv.Itoa(cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("não foi possível ouvir em %s: %v", addr, err)
	}
	srv := &http.Server{
		Handler:           a.withHeaders(mux),
		ReadHeaderTimeout: 20 * time.Second,
		IdleTimeout:       10 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}

	tlsCfg, tlsErr := loadTLSConfig(cfg.TLSCert, cfg.TLSKey)
	if tlsErr != nil {
		log.Printf("aviso: TLS desativado (%v)", tlsErr)
		tlsCfg = nil
	}

	log.Printf("PocketServe %s iniciado", Version)
	log.Printf("  local:     http://127.0.0.1:%d", cfg.Port)
	for _, n := range networkInterfaces() {
		if n.Reachable {
			log.Printf("  %-9s  http://%s:%d", n.Label, n.IPv4, cfg.Port)
		}
	}
	if cfg.TailscaleName != "" {
		log.Printf("  Tailscale  http://%s:%d", cfg.TailscaleName, cfg.Port)
	}
	if cfg.PublicHost != "" {
		scheme := "http"
		if tlsCfg != nil {
			scheme = "https"
		}
		log.Printf("  público    %s://%s", scheme, cfg.PublicHost)
	}
	if tlsCfg != nil {
		serveWithTLS(srv, tlsCfg, "0.0.0.0:"+strconv.Itoa(cfg.TLSPort))
	}
	if cfg.PasswordHash == "" {
		log.Printf("  primeiro acesso: crie a senha do administrador no navegador")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("servidor: %v", err)
	}
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	// public
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /api/state", a.handleState)
	mux.HandleFunc("POST /api/login", a.handleLogin)
	mux.HandleFunc("POST /api/setup", a.handleSetup)
	mux.HandleFunc("POST /api/logout", a.handleLogout)

	// authenticated
	mux.HandleFunc("POST /api/password", a.requireAuth(a.handleChangePassword))
	mux.HandleFunc("GET /api/system", a.requireAuth(a.handleSystem))
	mux.HandleFunc("GET /api/files", a.authAny(scopeFilesRead, a.handleList))
	mux.HandleFunc("GET /api/raw", a.authAny(scopeFilesRead, a.handleRaw))
	mux.HandleFunc("GET /api/download", a.authAny(scopeFilesRead, a.handleDownload))
	mux.HandleFunc("GET /api/zip", a.authAny(scopeFilesRead, a.handleZip))
	mux.HandleFunc("GET /api/text", a.authAny(scopeFilesRead, a.handleText))
	mux.HandleFunc("GET /api/thumb", a.authAny(scopeFilesRead, a.handleThumb))
	mux.HandleFunc("GET /api/search", a.authAny(scopeFilesRead, a.handleSearch))
	mux.HandleFunc("GET /api/checksum", a.authAny(scopeFilesRead, a.handleChecksum))
	mux.HandleFunc("POST /api/upload", a.authAny(scopeFilesWrite, a.handleUpload))
	mux.HandleFunc("POST /api/mkdir", a.authAny(scopeFilesWrite, a.handleMkdir))
	mux.HandleFunc("POST /api/rename", a.authAny(scopeFilesWrite, a.handleRename))
	mux.HandleFunc("POST /api/delete", a.authAny(scopeFilesDelete, a.handleDelete))
	mux.HandleFunc("POST /api/move", a.authAny(scopeFilesWrite, a.handleMove))
	mux.HandleFunc("POST /api/copy", a.authAny(scopeFilesWrite, a.handleCopy))
	mux.HandleFunc("POST /api/save", a.authAny(scopeFilesWrite, a.handleSave))
	mux.HandleFunc("POST /api/unzip", a.authAny(scopeFilesWrite, a.handleUnzip))
	mux.HandleFunc("GET /api/shares", a.authAny(scopeSharesManage, a.handleShareList))
	mux.HandleFunc("POST /api/shares", a.authAny(scopeSharesManage, a.handleShareCreate))
	mux.HandleFunc("DELETE /api/shares/{token}", a.authAny(scopeSharesManage, a.handleShareDelete))
	mux.HandleFunc("GET /api/activity", a.requireAuth(a.handleActivity))
	mux.HandleFunc("GET /api/totp", a.requireAuth(a.handleTOTPStatus))
	mux.HandleFunc("GET /api/trusted", a.requireAuth(a.handleTrustList))
	mux.HandleFunc("POST /api/trusted/revoke", a.requireAuth(a.handleTrustRevoke))
	mux.HandleFunc("POST /api/totp/setup", a.requireAuth(a.handleTOTPSetup))
	mux.HandleFunc("POST /api/totp/enable", a.requireAuth(a.handleTOTPEnable))
	mux.HandleFunc("POST /api/totp/disable", a.requireAuth(a.handleTOTPDisable))
	mux.HandleFunc("GET /api/totp/qr", a.requireAuth(a.handleTOTPQR))
	mux.HandleFunc("GET /api/backups", a.requireAuth(a.handleBackupStatus))
	mux.HandleFunc("POST /api/backups/run", a.requireAuth(a.handleBackupRun))
	mux.HandleFunc("GET /api/backups/file", a.requireAuth(a.handleBackupFile))
	mux.HandleFunc("POST /api/backups/delete", a.requireAuth(a.handleBackupDelete))
	mux.HandleFunc("GET /api/trash", a.requireAuth(a.handleTrashList))
	mux.HandleFunc("POST /api/trash/restore", a.requireAuth(a.handleTrashRestore))
	mux.HandleFunc("POST /api/trash/delete", a.requireAuth(a.handleTrashDelete))
	mux.HandleFunc("POST /api/trash/empty", a.requireAuth(a.handleTrashEmpty))
	mux.HandleFunc("GET /api/settings", a.requireAuth(a.handleGetSettings))
	mux.HandleFunc("POST /api/settings", a.requireAuth(a.handleSaveSettings))
	mux.HandleFunc("POST /api/restart", a.requireAuth(a.handleRestart))
	mux.HandleFunc("GET /api/log", a.requireAuth(a.handleLog))
	mux.HandleFunc("GET /api/qr", a.requireAuth(a.handleQR))

	// tokens de API
	mux.HandleFunc("GET /api/whoami", a.handleWhoAmI)
	mux.HandleFunc("GET /api/tokens", a.requireAuth(a.handleTokenList))
	mux.HandleFunc("POST /api/tokens", a.requireAuth(a.handleTokenCreate))
	mux.HandleFunc("POST /api/tokens/{id}/revoke", a.requireAuth(a.handleTokenRevoke))
	mux.HandleFunc("DELETE /api/tokens/{id}", a.requireAuth(a.handleTokenDelete))

	mux.HandleFunc("GET /api/duplicates", a.authAny(scopeFilesRead, a.handleDuplicates))
	mux.HandleFunc("POST /api/duplicates/cleanup", a.authAny(scopeFilesDelete, a.handleDuplicateCleanup))

	// histórico por arquivo (versões nos snapshots de backup)
	mux.HandleFunc("GET /api/versions", a.authAny(scopeFilesRead, a.handleFileHistory))
	mux.HandleFunc("GET /api/versions/download", a.authAny(scopeFilesRead, a.handleFileVersion))
	mux.HandleFunc("POST /api/versions/restore", a.authAny(scopeFilesDelete, a.handleFileVersionRestore))

	// public share pages
	mux.HandleFunc("GET /s/{token}", a.handleSharePage)
	mux.HandleFunc("POST /s/{token}/unlock", a.handleShareUnlock)
	mux.HandleFunc("GET /s/{token}/raw", a.handleShareRaw)
	mux.HandleFunc("GET /s/{token}/zip", a.handleShareZip)

	// web app
	mux.HandleFunc("GET /", a.handleStatic)
	return mux
}

func (a *App) handleActivity(w http.ResponseWriter, r *http.Request) {
	n := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 500 {
			n = parsed
		}
	}
	jsonWrite(w, http.StatusOK, map[string]any{"activity": a.activity.Recent(n)})
}

func (a *App) withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; manifest-src 'self'; font-src 'self'; frame-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		// Conteúdo privado e links de compartilhamento nunca devem ficar em cache
		// nem ser indexados por buscadores.
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/s/") {
			h.Set("Cache-Control", "no-store, max-age=0")
			h.Set("Pragma", "no-cache")
			h.Set("X-Robots-Tag", "noindex, nofollow, noarchive")
		}
		next.ServeHTTP(w, r)
	})
}

var webBuildTime = time.Now()

// assetVersion identifica esta execução do binário. É anexado aos assets no
// index.html para invalidar cache de versões anteriores após um deploy.
var assetVersion = strconv.FormatInt(webBuildTime.Unix(), 36)

// versionedAssets adiciona ?v=<build> aos assets referenciados no HTML, sem
// tocar em links que já tenham uma versão explícita.
func versionedAssets(html []byte) []byte {
	for _, name := range []string{"styles.css", "app.js", "pages.js", "files.js"} {
		html = bytes.ReplaceAll(html, []byte(`"/`+name+`"`), []byte(`"/`+name+`?v=`+assetVersion+`"`))
	}
	return html
}

func (a *App) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/s/") {
		http.NotFound(w, r)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		p = "index.html"
	}
	allowed := map[string]bool{
		"index.html": true, "app.js": true, "pages.js": true, "files.js": true,
		"styles.css": true, "icon.svg": true, "manifest.json": true,
		"favicon.ico": true, "apple-touch-icon.png": true,
		"icon-192.png": true, "icon-512.png": true,
	}
	if !allowed[p] {
		p = "index.html"
	}
	b, err := fs.ReadFile(embeddedWeb, "web/"+p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ct := mime.TypeByExtension(filepath.Ext(p))
	switch p {
	case "manifest.json":
		ct = "application/manifest+json"
	case "index.html":
		ct = "text/html; charset=utf-8"
	case "app.js":
		ct = "text/javascript; charset=utf-8"
	case "pages.js":
		ct = "text/javascript; charset=utf-8"
	case "files.js":
		ct = "text/javascript; charset=utf-8"
	case "styles.css":
		ct = "text/css; charset=utf-8"
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	switch filepath.Ext(p) {
	case ".png", ".ico", ".svg":
		// Ícones e imagens mudam pouco; o restante revalida a cada carga para
		// que uma atualização do servidor apareça sem limpar o cache.
		w.Header().Set("Cache-Control", "public, max-age=86400")
	default:
		w.Header().Set("Cache-Control", "no-cache")
	}
	if p == "index.html" {
		// Cada build ganha uma versão própria nos assets: assim o navegador
		// baixa o CSS/JS novo mesmo que tenha guardado o antigo em cache.
		b = versionedAssets(b)
	}
	http.ServeContent(w, r, p, webBuildTime, bytes.NewReader(b))
}
