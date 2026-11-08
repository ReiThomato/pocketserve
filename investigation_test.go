package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Investigação pós-implementação: procurar problemas que os testes de
// funcionalidade não pegam. Cada teste aqui corresponde a uma hipótese
// levantada na revisão manual.

// HIPÓTESE 1: a limpeza de duplicatas usa a lixeira do volume do arquivo.
// Se o cartão estiver sem permissão ou a lixeira dele falhar, o item não pode
// ser apagado de vez como efeito colateral.
func TestDuplicateCleanupNeverHardDeletes(t *testing.T) {
	dir := t.TempDir()
	a := newGuardApp(t, 0)
	root := dir
	a.cfg.Root = root
	a.cfg.TrashDays = 0 // lixeira DESLIGADA: a limpeza deve ser recusada
	a.backups = newBackupManager(dir, func() string { return root })

	for _, rel := range []string{"a.txt", "b.txt"} {
		if err := writeFileForTest(root, rel, "igual"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := a.runBackup("t"); err != nil {
		t.Fatal(err)
	}
	rep := a.buildDuplicateReport()
	if rep.GroupCount != 1 {
		t.Skip("sem grupo formado")
	}
	var other string
	for _, f := range rep.Groups[0].Files {
		if f.Path != rep.Groups[0].Keep {
			other = f.Path
		}
	}
	rec := postJSON(t, a, "/api/duplicates/cleanup", map[string]any{
		"paths": []string{other},
	})
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	// Com lixeira desligada, o handler conta como skipped e NÃO apaga.
	if _, err := statForTest(root, other); err != nil {
		t.Fatal("com lixeira desligada o arquivo não deveria ser apagado pela limpeza de duplicatas")
	}
}

// HIPÓTESE 2: restaurar uma versão em um caminho que hoje é uma PASTA.
// O código checa IsDir antes de mover para a lixeira; precisa recusar em vez
// de apagar a árvore inteira.
func TestVersionRestoreRefusesDirectoryTarget(t *testing.T) {
	a, root := setupVersionApp(t)
	if err := writeFileForTest(root, "alvo.txt", "conteudo antigo"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.runBackup("t"); err != nil {
		t.Fatal(err)
	}
	snap := a.buildFileHistory("alvo.txt").Versions[0].Snapshot

	// Troca o arquivo por uma pasta com conteúdo.
	if err := removeForTest(root, "alvo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := mkdirForTest(root, "alvo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := writeFileForTest(root, "alvo.txt/dentro.txt", "importante"); err != nil {
		t.Fatal(err)
	}

	rec := postJSON(t, a, "/api/versions/restore", map[string]any{
		"snapshot": snap, "path": "alvo.txt",
	})
	// Qualquer que seja o status, a pasta e seu conteúdo devem sobreviver.
	if _, err := statForTest(root, "alvo.txt/dentro.txt"); err != nil {
		t.Fatalf("o conteúdo da pasta foi destruído pela restauração (status %d): %v", rec.Code, err)
	}
}

// HIPÓTESE 3: token revogado durante uma requisição em andamento.
// A validação é por requisição, então a próxima já deve falhar mesmo que a
// anterior tenha sido autorizada.
func TestTokenRevokedBetweenRequests(t *testing.T) {
	a := newGuardApp(t, 0)
	value := newTokenValue()
	a.tokens.add(&APIToken{
		ID: "x", Hash: tokenHash(value), Scopes: []string{scopeFilesRead},
		CreatedAt: nowForTest(),
	})
	if a.tokens.lookup(value) == nil {
		t.Fatal("deveria validar antes")
	}
	a.tokens.revoke("x")
	if a.tokens.lookup(value) != nil {
		t.Fatal("não deveria validar depois da revogação")
	}
}

// HIPÓTESE 4: /api/whoami é público na rota mas precisa devolver 401 sem
// credencial. Se devolvesse 200 vazio, um agente acharia que está autenticado.
func TestWhoAmIRequiresCredential(t *testing.T) {
	a := newGuardApp(t, 0)
	h := a.routes()
	req := httptest.NewRequest(http.MethodGet, "/api/whoami", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("whoami sem credencial = %d, esperado 401", rec.Code)
	}
}

// HIPÓTESE 5: o campo reserve exposto em /api/settings não pode vazar o
// caminho absoluto do volume para um token de leitura.
// (Hoje /api/settings exige sessão, mas o teste documenta a intenção.)
func TestSettingsStillRequiresSession(t *testing.T) {
	a := newGuardApp(t, 0)
	value := newTokenValue()
	a.tokens.add(&APIToken{
		ID: "admin", Hash: tokenHash(value),
		Scopes: []string{scopeAdmin}, CreatedAt: nowForTest(),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+value)
	rec := httptest.NewRecorder()
	a.routes().ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("token (mesmo admin) não deveria acessar ajustes — só sessão de navegador")
	}
}

// HIPÓTESE 6: o relatório de duplicatas não pode expor caminhos de arquivos
// que o token não pode ler. Como o token é global, o teste verifica que o
// relatório ao menos exige credencial.
func TestDuplicatesRequiresCredential(t *testing.T) {
	a := newGuardApp(t, 0)
	req := httptest.NewRequest(http.MethodGet, "/api/duplicates", nil)
	rec := httptest.NewRecorder()
	a.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("duplicates sem credencial = %d", rec.Code)
	}
}

// HIPÓTESE 7: min_free_mb negativo ou absurdo não pode ser aceito e deixar o
// servidor num estado onde nenhum upload passa por engano.
func TestSettingsRejectsBadReserve(t *testing.T) {
	a := newGuardApp(t, 2048)
	for _, v := range []int64{-1, 9999999} {
		rec := postJSON(t, a, "/api/settings", map[string]any{
			"server_name": "x", "port": 8080, "root": a.cfg.Root,
			"session_days": 30, "max_upload_mb": 0, "min_free_mb": v,
			"trash_days": 30, "backup_daily": false, "roots": []any{},
		})
		if rec.Code == http.StatusOK {
			t.Fatalf("reserva %d deveria ser recusada", v)
		}
	}
}

// HIPÓTESE 8: dois uploads simultâneos não podem, juntos, furar a reserva.
// A checagem pós-escrita existe para isso; aqui verificamos a lógica.
func TestConcurrentReserveChecks(t *testing.T) {
	dir := t.TempDir()
	a := &App{cfg: Config{Root: dir, MinFreeMB: 1024 * 1024}} // 1 TB: impossível
	done := make(chan bool, 4)
	for i := 0; i < 4; i++ {
		go func() {
			_, _, err := a.checkSpace(dir, 1)
			done <- err != nil
		}()
	}
	for i := 0; i < 4; i++ {
		if !<-done {
			t.Fatal("todos os uploads simultâneos deveriam ser recusados")
		}
	}
}

// HIPÓTESE 9: o histórico não pode quebrar com caminho contendo ".."
// ou barra inicial.
func TestFileHistoryPathNormalization(t *testing.T) {
	a, root := setupVersionApp(t)
	if err := writeFileForTest(root, "a.txt", "x"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.runBackup("t"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/a.txt", "a.txt", " a.txt "} {
		h := a.buildFileHistory(p)
		if len(h.Versions) == 0 {
			t.Fatalf("caminho %q deveria encontrar a versão", p)
		}
	}
	h := a.buildFileHistory("../../etc/passwd")
	if len(h.Versions) != 0 {
		t.Fatal("caminho com .. não deveria casar com nada")
	}
}

// HIPÓTESE 10: a página de duplicados precisa existir no roteador, senão o
// clique no menu navega para uma tela em branco.
func TestDuplicatesPageRegistered(t *testing.T) {
	src := readFileForTest("web/app.js")
	if !strings.Contains(src, "duplicates: 'Duplicados'") {
		t.Fatal("rota 'duplicates' não está no TITLES")
	}
	if !strings.Contains(src, "renderDuplicatesPage") {
		t.Fatal("renderRoute não chama renderDuplicatesPage")
	}
	html := readFileForTest("web/index.html")
	if !strings.Contains(html, `id="page-duplicates"`) {
		t.Fatal("seção page-duplicates ausente no index.html")
	}
	if !strings.Contains(html, `href="#/duplicates"`) {
		t.Fatal("link de navegação para duplicados ausente")
	}
}
