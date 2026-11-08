package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Regressão visual: componentes "saindo para fora da tela" no celular.
//
// O bug real aconteceu no card de compartilhamentos: o grid usava
// `minmax(300px,1fr)` e, com o gap, ficava 27px mais largo que a viewport de
// 360px — a URL do link e a segunda coluna vazavam para fora. O mesmo padrão
// existia em access-grid (320px) e file-grid (150px).
//
// Estes testes são estáticos: leem o CSS e garantem que as regras que
// impedem o overflow continuam presentes. Não substituem a verificação no
// navegador, mas travam a regressão mais provável.

func readCSS(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("web/styles.css")
	if err != nil {
		t.Fatalf("não foi possível ler styles.css: %v", err)
	}
	return string(b)
}

// Nenhum grid auto-fill pode usar minmax com valor fixo sem o min(X,100%).
// Sem isso, uma viewport menor que o mínimo força scroll horizontal.
func TestNoFixedMinmaxInAutoFillGrids(t *testing.T) {
	css := readCSS(t)
	re := regexp.MustCompile(`grid-template-columns:\s*repeat\(auto-fill,\s*minmax\(([^,]+),`)
	for _, m := range re.FindAllStringSubmatch(css, -1) {
		arg := strings.TrimSpace(m[1])
		if strings.HasPrefix(arg, "min(") {
			continue
		}
		// minmax(0,1fr) e afins são seguros.
		if arg == "0" {
			continue
		}
		t.Fatalf("grid auto-fill com minmax(%s,...) pode estourar em telas estreitas; use minmax(min(%s,100%%),1fr)", arg, arg)
	}
}

// A página não pode rolar horizontalmente. É a rede de segurança final.
func TestBodyHasNoHorizontalScroll(t *testing.T) {
	css := readCSS(t)
	if !strings.Contains(css, "overflow-x:hidden") {
		t.Fatal("body precisa de overflow-x:hidden como rede de segurança contra overflow lateral")
	}
}

// Filhos de flex/grid precisam de min-width:0, senão texto e URL longos
// empurram o layout para fora.
func TestFlexChildrenCanShrink(t *testing.T) {
	css := readCSS(t)
	required := []string{
		".main,.content,.page,.card,.stat,.share-card,.settings-col,.settings-grid>*{min-width:0}",
	}
	for _, rule := range required {
		if !strings.Contains(css, rule) {
			t.Fatalf("regra ausente (filhos de flex/grid sem min-width:0): %s", rule)
		}
	}
}

// Componentes que já estouraram precisam manter min-width:0 / overflow.
func TestKnownOverflowComponents(t *testing.T) {
	css := readCSS(t)
	checks := map[string]string{
		"share-link":  ".share-link{display:flex;",
		"share-card":  ".share-card{background:var(--surface);",
		"share-meta":  ".share-meta{display:flex;",
		"dup-path":    ".dup-path{flex:1;",
		"token-head":  ".token-head{display:flex;",
		"ver-meta":    ".ver-meta{display:flex;",
		"dup-actions": ".dup-actions{display:flex;",
	}
	for name, prefix := range checks {
		i := strings.Index(css, prefix)
		if i < 0 {
			t.Fatalf("regra de .%s não encontrada", name)
		}
		// pega a declaração inteira até o fecha-chave
		end := strings.Index(css[i:], "}")
		if end < 0 {
			t.Fatalf("declaração de .%s malformada", name)
		}
		decl := css[i : i+end]
		if !strings.Contains(decl, "min-width:0") &&
			!strings.Contains(decl, "overflow") &&
			!strings.Contains(decl, "flex-wrap:wrap") &&
			name != "share-card" && name != "ver-meta" && name != "share-meta" {
			t.Fatalf(".%s pode estourar: sem min-width:0, overflow ou flex-wrap (%s)", name, decl)
		}
	}
}

// O menu de contexto não pode passar da viewport.
func TestContextMenuIsClamped(t *testing.T) {
	css := readCSS(t)
	if !strings.Contains(css, "max-width:min(320px,calc(100vw - 16px))") {
		t.Fatal("o menu de contexto precisa de max-width limitado pela viewport")
	}
}

// O card de versões e o de duplicados precisam quebrar linha em vez de
// empurrar os botões para fora.
func TestActionRowsWrap(t *testing.T) {
	css := readCSS(t)
	for _, sel := range []string{".ver-row{", ".dup-actions{", ".act-row{"} {
		i := strings.Index(css, sel)
		if i < 0 {
			t.Fatalf("%s não encontrado", sel)
		}
		end := strings.Index(css[i:], "}")
		decl := css[i : i+end]
		if !strings.Contains(decl, "flex-wrap:wrap") {
			t.Fatalf("%s precisa de flex-wrap:wrap para não estourar no celular", sel)
		}
	}
}

// Toda meta viewport precisa existir para o navegador móvel não simular
// uma janela de 980px.
func TestViewportMetaPresent(t *testing.T) {
	b, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)
	if !strings.Contains(html, `name="viewport"`) {
		t.Fatal("index.html sem meta viewport")
	}
	if !strings.Contains(html, "width=device-width") {
		t.Fatal("meta viewport sem width=device-width")
	}
}

// Regressão específica da tela de Links, que era a mais quebrada: o card
// precisa declarar min-width:0 em cada nível, senão a URL longa do link
// empurra o conteúdo para fora da viewport de 360px.
func TestShareCardChainCanShrink(t *testing.T) {
	css := readCSS(t)
	chain := []string{".share-card{", ".share-top{", ".share-link{", ".share-meta{", ".share-link code{"}
	for _, sel := range chain {
		i := strings.Index(css, sel)
		if i < 0 {
			t.Fatalf("%s ausente no CSS", sel)
		}
		end := strings.Index(css[i:], "}")
		decl := css[i : i+end]
		hasShrink := strings.Contains(decl, "min-width:0") ||
			strings.Contains(decl, "overflow:hidden") ||
			strings.Contains(decl, "overflow-x:hidden")
		if !hasShrink {
			t.Fatalf("%s sem min-width:0 nem overflow — a URL do link vai vazar", sel)
		}
	}
}

// O binário embute o CSS; se alguém editar o arquivo e esquecer de reinstalar,
// o teste não pega. Este aqui garante ao menos que o asset versionado está
// coerente com o que o servidor serve (mesmo arquivo, via go:embed).
func TestEmbeddedCSSMatchesFile(t *testing.T) {
	onDisk := readCSS(t)
	embedded, err := embeddedWeb.ReadFile("web/styles.css")
	if err != nil {
		t.Fatalf("CSS não está embutido: %v", err)
	}
	if string(embedded) != onDisk {
		t.Fatal("web/styles.css diverge do arquivo embutido no binário")
	}
	if !strings.Contains(string(embedded), "minmax(min(") {
		t.Fatal("o CSS embutido não contém a correção de overflow")
	}
}
