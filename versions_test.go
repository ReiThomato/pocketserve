package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupVersionApp cria um App com backup funcional e raiz temporária.
func setupVersionApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	a := newGuardApp(t, 0)
	root := filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	a.cfg.Root = root
	a.cfg.TrashDays = 30
	a.backups = newBackupManager(dir, func() string { return root })
	return a, root
}

func writeAndBackup(t *testing.T, a *App, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.runBackup("teste"); err != nil {
		t.Fatalf("backup falhou: %v", err)
	}
	// Garante IDs distintos mesmo em execuções no mesmo segundo.
	timeSleepShort()
}

func TestFileHistoryDetectsVersions(t *testing.T) {
	a, root := setupVersionApp(t)

	writeAndBackup(t, a, root, "Projetos/nota.md", "versao 1")
	writeAndBackup(t, a, root, "Projetos/nota.md", "versao 2")
	writeAndBackup(t, a, root, "Projetos/nota.md", "versao 3")

	h := a.buildFileHistory("Projetos/nota.md")
	if h.Note != "" {
		t.Fatalf("não esperava nota: %q", h.Note)
	}
	if len(h.Versions) != 3 {
		t.Fatalf("esperava 3 versões, veio %d", len(h.Versions))
	}
	if h.Distinct != 3 {
		t.Fatalf("esperava 3 hashes distintos, veio %d", h.Distinct)
	}
	// A primeira é a atual.
	if !h.Versions[0].Current {
		t.Fatal("a versão mais recente deveria estar marcada como atual")
	}
	for _, v := range h.Versions {
		if !v.Recoverable {
			t.Fatalf("versão %s deveria ser recuperável", v.Snapshot)
		}
	}
}

// Arquivo que nunca mudou: várias entradas, mas um único hash.
func TestFileHistoryUnchangedFile(t *testing.T) {
	a, root := setupVersionApp(t)
	writeAndBackup(t, a, root, "a.txt", "igual")
	writeAndBackup(t, a, root, "a.txt", "igual")
	writeAndBackup(t, a, root, "a.txt", "igual")

	h := a.buildFileHistory("a.txt")
	if h.Distinct != 1 {
		t.Fatalf("esperava 1 hash distinto, veio %d", h.Distinct)
	}
	if len(h.Versions) != 3 {
		t.Fatalf("esperava 3 entradas de snapshot, veio %d", len(h.Versions))
	}
	if got := h.summary(); !strings.Contains(got, "sem alterações") {
		t.Fatalf("resumo inesperado: %q", got)
	}
}

func TestFileHistoryUnknownPath(t *testing.T) {
	a, _ := setupVersionApp(t)
	h := a.buildFileHistory("nao/existe.txt")
	if h.Note == "" {
		t.Fatal("caminho inexistente deveria trazer uma nota")
	}
	if len(h.Versions) != 0 {
		t.Fatalf("não deveria haver versões, veio %d", len(h.Versions))
	}
}

func TestFileHistoryWithoutBackup(t *testing.T) {
	a, _ := setupVersionApp(t)
	h := a.buildFileHistory("a.txt")
	if !strings.Contains(h.Note, "snapshot") {
		t.Fatalf("nota deveria orientar a rodar backup: %q", h.Note)
	}
}

// Restaurar precisa devolver o conteúdo antigo E preservar o atual na lixeira.
func TestVersionRestorePreservesCurrent(t *testing.T) {
	a, root := setupVersionApp(t)

	writeAndBackup(t, a, root, "doc.txt", "antigo")
	oldSnap := a.buildFileHistory("doc.txt").Versions[0].Snapshot

	writeAndBackup(t, a, root, "doc.txt", "atual")

	rec := postJSON(t, a, "/api/versions/restore", map[string]any{
		"snapshot": oldSnap, "path": "doc.txt",
	})
	if rec.Code != 200 {
		t.Fatalf("restore falhou: %d %s", rec.Code, rec.Body.String())
	}

	got, err := os.ReadFile(filepath.Join(root, "doc.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "antigo" {
		t.Fatalf("conteúdo restaurado = %q, esperado 'antigo'", got)
	}

	// A versão atual ("atual") precisa estar na lixeira.
	items, err := a.listTrash()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range items {
		if it.OrigPath == "doc.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("a versão substituída deveria ter ido para a lixeira")
	}
}

// Baixar uma versão específica precisa devolver o conteúdo daquele snapshot.
func TestVersionDownload(t *testing.T) {
	a, root := setupVersionApp(t)
	writeAndBackup(t, a, root, "v.txt", "primeira")
	oldSnap := a.buildFileHistory("v.txt").Versions[0].Snapshot
	writeAndBackup(t, a, root, "v.txt", "segunda")

	req := newTestRequest()
	req.URL.Path = "/api/versions/download"
	req.URL.RawQuery = "snapshot=" + oldSnap + "&path=v.txt"
	req.AddCookie(sessionCookieForTest(t, a))
	rec := newRecorder()
	a.routes().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("download falhou: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "primeira") {
		t.Fatalf("conteúdo inesperado: %q", rec.Body.String())
	}
}
