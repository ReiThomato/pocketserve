package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// O servidor pode expor mais de uma pasta ao mesmo tempo — por exemplo o
// cartão SD e o armazenamento interno. Cada uma recebe um apelido curto:
//
//	cartao/Projetos/x.txt
//	interno/Download/y.pdf
//
// O primeiro segmento escolhe a raiz. Sem apelido, vale a raiz principal.
// Isso mantém a compatibilidade: quem usa só uma raiz continua vendo os
// caminhos como antes.

type RootEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// allRoots devolve as raízes ativas, com a principal primeiro.
func (a *App) allRoots() []RootEntry {
	cfg := a.config()
	out := make([]RootEntry, 0, len(cfg.Roots)+1)
	seen := map[string]bool{}
	if cfg.Root != "" {
		out = append(out, RootEntry{Name: "", Path: cfg.Root})
		seen[cfg.Root] = true
	}
	for _, r := range cfg.Roots {
		p := normalizeRoot(r.Path)
		if p == "" || seen[p] {
			continue
		}
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			continue
		}
		name := sanitizeAlias(r.Name)
		if name == "" {
			name = filepath.Base(p)
		}
		// Evita apelidos duplicados.
		for _, existing := range out {
			if existing.Name != "" && strings.EqualFold(existing.Name, name) {
				name = name + "-2"
			}
		}
		out = append(out, RootEntry{Name: name, Path: p})
		seen[p] = true
	}
	return out
}

func sanitizeAlias(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// rootAliases lista só as raízes que têm apelido (as secundárias).
func (a *App) rootAliases() []RootEntry {
	var out []RootEntry
	for _, r := range a.allRoots() {
		if r.Name != "" {
			out = append(out, r)
		}
	}
	return out
}

// rootsWithSpace devolve as raízes secundárias com o espaço livre de cada uma.
// O painel usa isso para distribuir uploads entre volumes.
func (a *App) rootsWithSpace() []map[string]any {
	var out []map[string]any
	for _, r := range a.rootAliases() {
		free, total := diskFree(r.Path)
		out = append(out, map[string]any{
			"name": r.Name, "path": r.Path, "free": free, "total": total,
		})
	}
	return out
}

// primaryFree é o espaço livre da raiz principal em bytes.
func (a *App) primaryFree() int64 {
	free, _ := diskFree(a.rootPath())
	return free
}

// lookupRoot encontra a raiz de um apelido.
func (a *App) lookupRoot(alias string) (string, bool) {
	if alias == "" {
		return a.rootPath(), true
	}
	for _, r := range a.allRoots() {
		if r.Name != "" && strings.EqualFold(r.Name, alias) {
			return r.Path, true
		}
	}
	return "", false
}

// splitAlias separa o primeiro segmento quando ele é um apelido conhecido.
//
// A ordem importa e já causou um bug real: antes, qualquer primeiro segmento
// "que parecia apelido" (sem ponto e sem espaço) era recusado quando havia
// mais de uma raiz. No celular, isso bloqueava DCIM, Download, Projetos,
// 01_DOCUMENTOS — o interno inteiro — enquanto o cartão (via "cartao/...")
// continuava funcionando. A regra correta:
//
//  1. apelido exato ganha;
//  2. pasta que existe de fato na raiz principal é pasta normal;
//  3. só recusa quando o nome não existe e é quase igual a um apelido.
func (a *App) splitAlias(rel string) (base, rest string, err error) {
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return a.rootPath(), "", nil
	}
	first, remainder, _ := strings.Cut(rel, "/")
	if path, ok := a.lookupRoot(first); ok && first != "" {
		return path, remainder, nil
	}
	primary := a.rootPath()
	// Pasta real na raiz principal: nunca é tratada como apelido, mesmo que
	// seja parecida com um. Isto mantém DCIM/Projetos/Download acessíveis.
	if first != "" && first != "." && first != ".." {
		if st, statErr := os.Stat(filepath.Join(primary, first)); statErr == nil && st.IsDir() {
			return primary, rel, nil
		}
	}
	// Nome inexistente e muito próximo de um apelido: quase sempre digitação
	// ("cartoa" em vez de "cartao"). Devolve uma mensagem útil em vez de
	// criar silenciosamente uma pasta no lugar errado.
	if suggestion := a.aliasSuggestion(first); suggestion != "" {
		return "", "", fmt.Errorf("pasta de armazenamento desconhecida: você quis dizer %q?", suggestion)
	}
	// Qualquer outro nome é uma pasta nova na raiz principal (inclusive em
	// uploads para uma pasta que ainda será criada).
	return primary, rel, nil
}

// aliasSuggestion encontra um apelido a uma edição de distância. Serve apenas
// para melhorar a mensagem de erro; nunca bloqueia uma pasta que existe.
//
// Nomes que claramente derivam do apelido ("cartao-bkp", "usb2") seguem como
// pastas normais: o usuário quis mesmo criar algo novo ao lado da raiz.
func (a *App) aliasSuggestion(name string) string {
	lower := strings.ToLower(name)
	if len(lower) < 4 {
		return ""
	}
	for _, r := range a.rootAliases() {
		alias := strings.ToLower(r.Name)
		if len(alias) < 4 {
			continue
		}
		if derivedFrom(lower, alias) {
			continue
		}
		if editDistanceAtMostOne(lower, alias) {
			return r.Name
		}
	}
	return ""
}

// derivedFrom diz se name é claramente o apelido + sufixo/prefixo de uso
// ("cartao-bkp", "usb2", "meu-cartao"). A separação por '-', '_' ou dígito
// distingue uma pasta nova de um erro de digitação como "cartaoo".
func derivedFrom(name, alias string) bool {
	isSep := func(b byte) bool {
		return b == '-' || b == '_' || (b >= '0' && b <= '9')
	}
	if strings.HasPrefix(name, alias) {
		rest := name[len(alias):]
		return rest != "" && isSep(rest[0])
	}
	if strings.HasSuffix(name, alias) {
		rest := name[:len(name)-len(alias)]
		return rest != "" && isSep(rest[len(rest)-1])
	}
	return false
}

// editDistanceAtMostOne compara duas strings aceitando no máximo uma edição
// (inserção, remoção, troca ou duas letras adjacentes invertidas). Barato e
// suficiente para sugerir apelidos.
func editDistanceAtMostOne(a, b string) bool {
	if a == b {
		return false // nome igual não é sugestão: já teria casado antes
	}
	la, lb := len(a), len(b)
	if la-lb > 1 || lb-la > 1 {
		return false
	}
	if la == lb {
		// Uma troca simples ou uma inversão de duas letras vizinhas
		// ("cartoa" → "cartao" é o erro de digitação mais comum).
		var diff []int
		for i := 0; i < la; i++ {
			if a[i] != b[i] {
				diff = append(diff, i)
				if len(diff) > 2 {
					return false
				}
			}
		}
		if len(diff) == 1 {
			return true
		}
		// Inversão adjacente: a[i]=b[j] e a[j]=b[i] com j=i+1.
		return len(diff) == 2 && diff[1] == diff[0]+1 &&
			a[diff[0]] == b[diff[1]] && a[diff[1]] == b[diff[0]]
	}
	// Garante que "a" seja o menor para simplificar.
	if la > lb {
		a, b = b, a
	}
	i, j, diff := 0, 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			i++
			j++
			continue
		}
		diff++
		if diff > 1 {
			return false
		}
		j++
	}
	return true
}

// rootForPath devolve a raiz ativa que contém o caminho real. Quando uma raiz
// está dentro de outra (ex.: principal=/sdcard, cartao=/sdcard/PocketServe),
// vence a mais específica — senão o arquivo seria atribuído ao volume errado.
func (a *App) rootForPath(full string) (RootEntry, bool) {
	var best RootEntry
	found := false
	for _, r := range a.allRoots() {
		if !within(r.Path, full) {
			continue
		}
		if !found || len(r.Path) > len(best.Path) {
			best = r
			found = true
		}
	}
	return best, found
}

// displayPath converte um caminho absoluto no caminho exibido ao usuário,
// prefixando o apelido quando o arquivo não está na raiz principal.
func (a *App) displayPath(full string) string {
	r, ok := a.rootForPath(full)
	if !ok {
		return relPath(a.rootPath(), full)
	}
	sub := relPath(r.Path, full)
	if r.Name == "" {
		return sub
	}
	if sub == "" {
		return r.Name
	}
	return r.Name + "/" + sub
}

// countRoots informa quantas raízes estão ativas (usado no painel).
func (a *App) countRoots() int { return len(a.allRoots()) }

// primaryLabel é o nome mostrado para a raiz principal na interface. Usa o
// último segmento do caminho (ex.: "PocketServe") em vez de um rótulo genérico.
// Caminhos do sistema Android recebem nomes legíveis: mostrar "sdcard" ou
// "0" como nome de volume não diz nada a quem usa.
func (a *App) primaryLabel() string {
	p := a.rootPath()
	if p == "" {
		return "Armazenamento"
	}
	clean := filepath.Clean(p)
	system := map[string]string{
		"/sdcard":                          "Armazenamento interno",
		"/storage/emulated/0":              "Armazenamento interno",
		"/storage/self/primary":            "Armazenamento interno",
		"/storage/emulated":                "Armazenamento interno",
		"/mnt/sdcard":                      "Armazenamento interno",
		"/data/data/com.termux/files/home": "Início do Termux",
	}
	if label, ok := system[clean]; ok {
		return label
	}
	// Volume removível: /storage/XXXX-XXXX vira "Cartão XXXX-XXXX".
	if strings.HasPrefix(clean, "/storage/") {
		base := filepath.Base(clean)
		if len(base) == 9 && base[4] == '-' {
			return "Cartão " + base
		}
	}
	base := filepath.Base(clean)
	if base == "/" || base == "." || base == string(filepath.Separator) {
		return "Armazenamento"
	}
	return base
}

// rootsView é a lista pronta para o JSON de configurações.
func (a *App) rootsView() []map[string]any {
	var out []map[string]any
	for _, r := range a.allRoots() {
		label := r.Name
		if label == "" {
			label = "principal"
		}
		free, total := diskFree(r.Path)
		out = append(out, map[string]any{
			"name": label, "alias": r.Name, "path": r.Path,
			"free": free, "total": total,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i]["alias"].(string) == ""
	})
	return out
}

// diskFree devolve espaço livre e total em bytes.
func diskFree(path string) (free, total int64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	bs := int64(st.Bsize)
	return int64(st.Bavail) * bs, int64(st.Blocks) * bs
}

// VolumeCandidate é um armazenamento encontrado no aparelho e ainda não
// configurado. O painel usa isto para oferecer o cartão com um clique, em vez
// de exigir que o usuário descubra e digite /storage/XXXX-XXXX à mão.
type VolumeCandidate struct {
	Path       string `json:"path"`
	Label      string `json:"label"`
	Free       int64  `json:"free"`
	Total      int64  `json:"total"`
	Removable  bool   `json:"removable"`
	Configured bool   `json:"configured"`
}

// detectVolumes varre os locais onde o Android monta armazenamento externo.
// No Termux, o cartão costuma aparecer como /storage/XXXX-XXXX e também como
// ~/storage/external-1 depois de `termux-setup-storage`.
func (a *App) detectVolumes() []VolumeCandidate {
	configured := map[string]bool{}
	for _, r := range a.allRoots() {
		configured[filepath.Clean(r.Path)] = true
	}

	var out []VolumeCandidate
	seen := map[string]bool{}
	add := func(p, label string, removable bool) {
		real, err := filepath.EvalSymlinks(p)
		if err != nil {
			return
		}
		clean := filepath.Clean(real)
		// Já configurado ou repetido: não oferece de novo.
		if configured[clean] || seen[clean] {
			return
		}
		st, err := os.Stat(clean)
		if err != nil || !st.IsDir() {
			return
		}
		free, total := diskFree(clean)
		if total == 0 {
			return
		}
		seen[clean] = true
		if label == "" {
			label = filepath.Base(clean)
		}
		out = append(out, VolumeCandidate{
			Path: clean, Label: label, Free: free, Total: total,
			Removable: removable, Configured: false,
		})
	}

	// /storage/XXXX-XXXX são volumes removíveis montados pelo sistema.
	if des, err := os.ReadDir("/storage"); err == nil {
		for _, de := range des {
			name := de.Name()
			if name == "emulated" || name == "self" || name == "sdcard0" {
				continue
			}
			// UUID de volume: 4 hexadecimal, hífen, 4 hexadecimal.
			if len(name) != 9 || name[4] != '-' {
				continue
			}
			add(filepath.Join("/storage", name), "Cartão "+name, true)
		}
	}

	// Links criados por termux-setup-storage (~/storage/external-*).
	if home, err := os.UserHomeDir(); err == nil {
		if des, err := os.ReadDir(filepath.Join(home, "storage")); err == nil {
			for _, de := range des {
				name := de.Name()
				if !strings.HasPrefix(name, "external-") {
					continue
				}
				label := "Cartão (external)"
				if i := strings.TrimPrefix(name, "external-"); i != "" {
					label = "Cartão " + i
				}
				add(filepath.Join(home, "storage", name), label, true)
			}
		}
	}

	// Armazenamento interno explícito, útil quando a raiz principal é uma
	// pasta dedicada e o usuário quer ver o /sdcard inteiro como opção.
	add("/sdcard", "Armazenamento interno", false)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Removable != out[j].Removable {
			return out[i].Removable
		}
		return out[i].Path < out[j].Path
	})
	if out == nil {
		return []VolumeCandidate{}
	}
	return out
}

// withinAnyRoot informa se um caminho real está dentro de alguma raiz ativa.
func (a *App) withinAnyRoot(real string) bool {
	for _, r := range a.allRoots() {
		if within(r.Path, real) {
			return true
		}
	}
	return false
}

// isRootPath evita que a própria raiz seja excluída ou renomeada.
func (a *App) isRootPath(full string) bool {
	clean := filepath.Clean(full)
	for _, r := range a.allRoots() {
		if filepath.Clean(r.Path) == clean {
			return true
		}
	}
	return false
}

// isSecondaryRootChild informa se o caminho é a raiz de um volume secundário
// que está fisicamente dentro da pasta listada. Nesse caso a entrada é omitida
// da listagem: o volume já tem aba própria, e mostrá-lo como subpasta faria o
// mesmo arquivo aparecer em dois caminhos.
func (a *App) isSecondaryRootChild(full string) bool {
	clean := filepath.Clean(full)
	for _, r := range a.rootAliases() {
		if filepath.Clean(r.Path) == clean {
			return true
		}
	}
	return false
}
