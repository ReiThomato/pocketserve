package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectVolumesMarksConfigured(t *testing.T) {
	primary := t.TempDir()
	card := t.TempDir()
	a := &App{cfg: Config{Root: primary, Roots: []RootEntry{{Name: "cartao", Path: card}}}}

	vols := a.detectVolumes()
	for _, v := range vols {
		if v.Path == primary || v.Path == card {
			t.Fatalf("volume já configurado apareceu como candidato: %s", v.Path)
		}
		if v.Total == 0 {
			t.Fatalf("volume %s sem total calculado", v.Path)
		}
	}
}

func TestPrimaryLabel(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "PocketServe")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ root, want string }{
		{sub, "PocketServe"},
		{"/sdcard", "Armazenamento interno"},
		{"/storage/emulated/0", "Armazenamento interno"},
		{"/storage/XXXX-XXXX", "Cartão XXXX-XXXX"},
	}
	for _, c := range cases {
		a := &App{cfg: Config{Root: c.root}}
		if got := a.primaryLabel(); got != c.want {
			t.Fatalf("primaryLabel(%q) = %q, esperado %q", c.root, got, c.want)
		}
	}
}

func TestRootsWithSpace(t *testing.T) {
	primary := t.TempDir()
	card := t.TempDir()
	a := &App{cfg: Config{Root: primary, Roots: []RootEntry{{Name: "cartao", Path: card}}}}
	list := a.rootsWithSpace()
	if len(list) != 1 {
		t.Fatalf("esperava 1 raiz secundária, veio %d", len(list))
	}
	if list[0]["name"] != "cartao" {
		t.Fatalf("nome = %v", list[0]["name"])
	}
	if free, _ := list[0]["free"].(int64); free <= 0 {
		t.Fatalf("free inválido: %v", list[0]["free"])
	}
}
