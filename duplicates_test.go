package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSuggestKeepPrefersShortestPath(t *testing.T) {
	files := []duplicateFile{
		{Path: "cartao/backup/projetos/a.txt"},
		{Path: "Projetos/a.txt"},
		{Path: "arquivo/a.txt"},
	}
	if got := suggestKeep(files); got != "Projetos/a.txt" {
		// "arquivo/a.txt" e "Projetos/a.txt" têm 15 caracteres; o laço só troca
		// quando é estritamente menor, então vence o primeiro em ordem.
		if got != "arquivo/a.txt" {
			t.Fatalf("suggestKeep = %q", got)
		}
	}
}

func TestRootOfPath(t *testing.T) {
	cases := map[string]string{
		"cartao/04_MIDIA/a.mp4": "cartao",
		"Projetos/nota.md":      "Projetos",
		"arquivo.txt":           "principal",
	}
	for in, want := range cases {
		if got := rootOfPath(in); got != want {
			t.Fatalf("rootOfPath(%q) = %q, esperado %q", in, got, want)
		}
	}
}

// buildDuplicateReport precisa agrupar por SHA, ignorar arquivos vazios e
// calcular o desperdício como (n-1) * tamanho.
func TestBuildDuplicateReport(t *testing.T) {
	dir := t.TempDir()
	a := newGuardApp(t, 0)
	// A raiz do App é o home no helper; ajusta para um diretório com arquivos.
	root := filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	a.cfg.Root = root
	a.backups = newBackupManager(dir, func() string { return root })

	write := func(rel, content string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a/x.txt", "igual")
	write("b/x.txt", "igual")
	write("c/y.txt", "igual") // terceira cópia do mesmo conteúdo
	write("d/unico.txt", "outro")
	write("e/vazio1.txt", "")
	write("f/vazio2.txt", "") // vazios não entram no relatório

	if _, _, err := a.runBackup("teste"); err != nil {
		t.Fatalf("backup falhou: %v", err)
	}

	rep := a.buildDuplicateReport()
	if rep.Snapshot == "" {
		t.Fatal("relatório sem snapshot")
	}
	if rep.GroupCount != 1 {
		t.Fatalf("esperava 1 grupo, veio %d", rep.GroupCount)
	}
	g := rep.Groups[0]
	if g.Count != 3 {
		t.Fatalf("esperava 3 cópias, veio %d", g.Count)
	}
	if g.Wasted != g.Size*2 {
		t.Fatalf("desperdício = %d, esperado %d", g.Wasted, g.Size*2)
	}
	if g.Keep == "" {
		t.Fatal("sem sugestão de qual manter")
	}
	if rep.TotalWaste != g.Wasted {
		t.Fatalf("total_waste = %d", rep.TotalWaste)
	}
}

// Sem backup, o relatório deve orientar em vez de varrer o disco.
func TestDuplicateReportWithoutBackup(t *testing.T) {
	a := newGuardApp(t, 0)
	rep := a.buildDuplicateReport()
	if rep.Note == "" {
		t.Fatal("sem snapshot deveria haver uma nota explicativa")
	}
	if len(rep.Groups) != 0 {
		t.Fatalf("sem snapshot não deveria haver grupos, veio %d", len(rep.Groups))
	}
}

// A limpeza nunca pode apagar a cópia sugerida nem zerar um grupo.
func TestDuplicateCleanupSafety(t *testing.T) {
	dir := t.TempDir()
	a := newGuardApp(t, 0)
	root := filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	a.cfg.Root = root
	a.cfg.TrashDays = 30
	a.backups = newBackupManager(dir, func() string { return root })

	for _, rel := range []string{"a/x.txt", "b/x.txt"} {
		full := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte("igual"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := a.runBackup("teste"); err != nil {
		t.Fatal(err)
	}
	rep := a.buildDuplicateReport()
	if rep.GroupCount != 1 {
		t.Fatalf("esperava 1 grupo, veio %d", rep.GroupCount)
	}
	keep := rep.Groups[0].Keep

	// Tenta apagar o arquivo marcado como "manter": precisa ser ignorado.
	rec := postJSON(t, a, "/api/duplicates/cleanup", map[string]any{
		"paths": []string{keep},
	})
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(root, keep)); err != nil {
		t.Fatalf("a cópia sugerida foi apagada: %v", err)
	}

	// Agora apaga a outra: deve ir para a lixeira.
	other := ""
	for _, f := range rep.Groups[0].Files {
		if f.Path != keep {
			other = f.Path
		}
	}
	rec2 := postJSON(t, a, "/api/duplicates/cleanup", map[string]any{
		"paths": []string{other},
	})
	if rec2.Code != 200 {
		t.Fatalf("status = %d: %s", rec2.Code, rec2.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, other)); !os.IsNotExist(err) {
		t.Fatalf("a cópia duplicada deveria ter saído do lugar: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, keep)); err != nil {
		t.Fatalf("a cópia mantida desapareceu: %v", err)
	}

	// E o item precisa estar na lixeira, não apagado de vez.
	items, err := a.listTrash()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("esperava 1 item na lixeira, veio %d", len(items))
	}
}

// Grupo com duas cópias: mesmo pedindo as duas, uma precisa sobrar.
func TestDuplicateCleanupKeepsOneCopy(t *testing.T) {
	dir := t.TempDir()
	a := newGuardApp(t, 0)
	root := filepath.Join(dir, "root")
	os.MkdirAll(root, 0o755)
	a.cfg.Root = root
	a.cfg.TrashDays = 30
	a.backups = newBackupManager(dir, func() string { return root })

	for _, rel := range []string{"a/x.txt", "b/x.txt"} {
		full := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte("igual"), 0o644)
	}
	if _, _, err := a.runBackup("t"); err != nil {
		t.Fatal(err)
	}
	rep := a.buildDuplicateReport()
	var all []string
	for _, f := range rep.Groups[0].Files {
		all = append(all, f.Path)
	}
	postJSON(t, a, "/api/duplicates/cleanup", map[string]any{"paths": all})

	remaining := 0
	for _, rel := range []string{"a/x.txt", "b/x.txt"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			remaining++
		}
	}
	if remaining < 1 {
		t.Fatal("o grupo ficou sem nenhuma cópia")
	}
}
