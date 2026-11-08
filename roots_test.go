package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A regressão central: com mais de uma raiz configurada, pastas comuns da raiz
// principal (DCIM, Projetos, 01_DOCUMENTOS) eram recusadas como "apelido
// desconhecido" e o usuário só conseguia gravar no cartão.
func TestSplitAliasKeepsPrimaryFolders(t *testing.T) {
	primary := t.TempDir()
	card := t.TempDir()

	if err := os.MkdirAll(filepath.Join(primary, "DCIM"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(primary, "01_DOCUMENTOS", "Estudos"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Pasta com nome igual ao apelido existe no interno: o apelido ganha,
	// porque é uma escolha explícita do usuário.
	if err := os.MkdirAll(filepath.Join(primary, "cartao"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := &App{cfg: Config{Root: primary, Roots: []RootEntry{{Name: "cartao", Path: card}}}}

	cases := []struct {
		rel      string
		wantBase string
		wantRest string
	}{
		{"DCIM", primary, "DCIM"},
		{"01_DOCUMENTOS", primary, "01_DOCUMENTOS"},
		{"01_DOCUMENTOS/Estudos", primary, "01_DOCUMENTOS/Estudos"},
		{"Projetos/novo.txt", primary, "Projetos/novo.txt"},
		{"cartao", card, ""},
		{"cartao/04_MIDIA", card, "04_MIDIA"},
		{"", primary, ""},
		// Nome que não existe e não é apelido: pasta nova na raiz principal.
		{"NovaPasta", primary, "NovaPasta"},
	}

	for _, c := range cases {
		base, rest, err := a.splitAlias(c.rel)
		if err != nil {
			t.Fatalf("splitAlias(%q) devolveu erro: %v", c.rel, err)
		}
		if base != c.wantBase || rest != c.wantRest {
			t.Fatalf("splitAlias(%q) = (%q, %q), esperado (%q, %q)",
				c.rel, base, rest, c.wantBase, c.wantRest)
		}
	}
}

// Um apelido digitado errado precisa de mensagem útil, não de uma pasta
// criada silenciosamente no volume errado.
func TestSplitAliasSuggestsTypo(t *testing.T) {
	primary := t.TempDir()
	card := t.TempDir()
	a := &App{cfg: Config{Root: primary, Roots: []RootEntry{{Name: "cartao", Path: card}}}}

	for _, typo := range []string{"cartoa", "carto", "cartaoo", "kartao"} {
		_, _, err := a.splitAlias(typo)
		if err == nil {
			t.Fatalf("splitAlias(%q) deveria recusar com sugestão", typo)
		}
		if !strings.Contains(err.Error(), "cartao") {
			t.Fatalf("mensagem sem sugestão para %q: %v", typo, err)
		}
	}

	// Nomes derivados do apelido são pastas legítimas, não erro de digitação.
	for _, ok := range []string{"cartao-bkp", "cartao2026", "usb"} {
		if _, _, err := a.splitAlias(ok); err != nil {
			t.Fatalf("splitAlias(%q) deveria ser aceito: %v", ok, err)
		}
	}
}

func TestDisplayPathRoundTrip(t *testing.T) {
	primary := t.TempDir()
	card := t.TempDir()
	a := &App{cfg: Config{Root: primary, Roots: []RootEntry{{Name: "cartao", Path: card}}}}

	if err := os.MkdirAll(filepath.Join(card, "04_MIDIA"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(card, "04_MIDIA", "a.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(card, "04_MIDIA", "a.mp4")
	shown := a.displayPath(full)
	if shown != "cartao/04_MIDIA/a.mp4" {
		t.Fatalf("displayPath = %q", shown)
	}
	back, err := a.resolve(shown)
	if err != nil {
		t.Fatal(err)
	}
	if back != full {
		t.Fatalf("resolve(displayPath) = %q, esperado %q", back, full)
	}
}

func TestEditDistanceAtMostOne(t *testing.T) {
	yes := [][2]string{
		{"cartao", "cartoa"},  // inversão adjacente
		{"cartao", "carto"},   // remoção
		{"cartao", "cartaoo"}, // inserção
		{"cartao", "kartao"},  // troca
	}
	for _, p := range yes {
		if !editDistanceAtMostOne(p[0], p[1]) {
			t.Fatalf("%q e %q deveriam estar a uma edição", p[0], p[1])
		}
	}
	no := [][2]string{
		{"cartao", "cartao"},  // igual não é sugestão
		{"cartao", "cartoes"}, // duas edições
		{"cartao", "usb"},
		{"cartao", "c"},
	}
	for _, p := range no {
		if editDistanceAtMostOne(p[0], p[1]) {
			t.Fatalf("%q e %q não deveriam estar a uma edição", p[0], p[1])
		}
	}
}

// Mover entre volumes distintos devolve EXDEV no rename; movePath precisa cair
// para cópia + remoção sem perder o arquivo.
func TestMovePathAcrossFilesystems(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "arquivo.txt")
	if err := os.WriteFile(src, []byte("conteudo"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dstDir, "arquivo.txt")
	if err := movePath(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "conteudo" {
		t.Fatalf("conteúdo no destino = %q (err=%v)", got, err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("origem deveria ter sido removida: %v", err)
	}
}

func TestMovePathDirectory(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	tree := filepath.Join(srcDir, "projeto")
	if err := os.MkdirAll(filepath.Join(tree, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "sub", "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := movePath(tree, filepath.Join(dstDir, "projeto")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "projeto", "sub", "a.txt")); err != nil {
		t.Fatalf("árvore não copiada: %v", err)
	}
}
