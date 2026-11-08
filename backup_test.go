package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Regressão: duas edições do mesmo arquivo, com o mesmo tamanho, dentro do
// mesmo segundo, faziam o cache de hash reaproveitar o conteúdo antigo. O
// backup gravava um pack com o hash velho e a restauração devolvia o texto
// errado — perda de dados silenciosa.
func TestBackupDoesNotReuseHashAfterQuickEdit(t *testing.T) {
	dir := t.TempDir()
	a := newGuardApp(t, 0)
	root := filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	a.cfg.Root = root
	a.backups = newBackupManager(dir, func() string { return root })
	full := filepath.Join(root, "x.txt")

	// "AAAA" e "BBBB" têm o mesmo tamanho: só o conteúdo muda.
	if err := os.WriteFile(full, []byte("AAAA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.runBackup("1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("BBBB"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.runBackup("2"); err != nil {
		t.Fatal(err)
	}

	snaps, err := a.backup().snapshotFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) < 2 {
		t.Fatalf("esperava 2 snapshots, veio %d", len(snaps))
	}
	// O snapshot mais recente precisa ter o hash de "BBBB".
	latest := snaps[0]
	var sha string
	for _, f := range latest.Files {
		if f.Path == "x.txt" {
			sha = f.SHA
		}
	}
	if sha == "" {
		t.Fatal("x.txt não está no snapshot mais recente")
	}
	expected, err := a.hashFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if sha != expected {
		t.Fatalf("hash do snapshot = %s, esperado %s (conteúdo velho reaproveitado)", sha, expected)
	}

	// E o histórico precisa mostrar 2 versões distintas.
	h := a.buildFileHistory("x.txt")
	if h.Distinct != 2 {
		t.Fatalf("esperava 2 versões distintas, veio %d", h.Distinct)
	}
}

// Arquivo recém-modificado não pode vir do cache: no exFAT a granularidade do
// timestamp é grossa, então o cache só é confiável depois de alguns segundos.
func TestManifestCacheSkipsRecentFiles(t *testing.T) {
	a := newGuardApp(t, 0)
	now := time.Now().UnixNano()
	a.manifestCachePut("recente.txt", 10, now, now, "hash-qualquer")
	if _, ok := a.manifestCacheGet("recente.txt", 10, now, now); ok {
		t.Fatal("arquivo recém-modificado não deveria vir do cache")
	}

	// Depois da janela de acomodação, o cache volta a valer.
	old := time.Now().Add(-time.Minute).UnixNano()
	a.manifestCachePut("antigo.txt", 10, old, old, "hash-ok")
	if sha, ok := a.manifestCacheGet("antigo.txt", 10, old, old); !ok || sha != "hash-ok" {
		t.Fatalf("arquivo estável deveria vir do cache, veio %q (ok=%v)", sha, ok)
	}
}
