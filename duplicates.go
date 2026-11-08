package main

import (
	"net/http"
	"sort"
	"strings"
)

// Relatório de duplicatas.
//
// O backup já calcula SHA-256 de todos os arquivos dos volumes ativos e guarda
// um manifesto por snapshot. Isso é exatamente a informação necessária para
// achar duplicatas — o motor está pronto, só faltava expor.
//
// A leitura é do snapshot mais recente (o mais barato) e sob demanda. Não há
// índice novo em memória nem varredura do disco: se o backup nunca rodou, o
// relatório avisa em vez de varrer 300 GB.
//
// Ação sugerida é sempre não destrutiva: o usuário revisa e escolhe. Nada é
// apagado automaticamente.

type duplicateFile struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"mtime,omitempty"`
	Root    string `json:"root,omitempty"`
}

type duplicateGroup struct {
	SHA    string          `json:"sha"`
	Size   int64           `json:"size"`
	Count  int             `json:"count"`
	Wasted int64           `json:"wasted"`
	Files  []duplicateFile `json:"files"`
	// Keep sugere qual manter: o caminho mais curto e, em empate, o mais
	// antigo. É só uma sugestão — a decisão é do usuário.
	Keep string `json:"keep"`
}

type duplicateReport struct {
	Snapshot   string           `json:"snapshot,omitempty"`
	At         string           `json:"at,omitempty"`
	Groups     []duplicateGroup `json:"groups"`
	TotalWaste int64            `json:"total_waste"`
	FileCount  int              `json:"file_count"`
	GroupCount int              `json:"group_count"`
	Note       string           `json:"note,omitempty"`
}

// buildDuplicateReport agrupa o manifesto do último snapshot por SHA.
func (a *App) buildDuplicateReport() duplicateReport {
	rep := duplicateReport{Groups: []duplicateGroup{}}

	snaps, err := a.backup().snapshotFiles()
	if err != nil || len(snaps) == 0 {
		rep.Note = "Nenhum snapshot de backup ainda. Rode um backup em Ajustes → Backups para gerar o relatório."
		return rep
	}
	last := snaps[0]
	rep.Snapshot = last.ID
	rep.At = last.CreatedAt.Format("2006-01-02 15:04")

	bySHA := map[string][]backupFile{}
	var order []string
	for _, f := range last.Files {
		// Arquivos vazios compartilham o mesmo hash e não desperdiçam espaço
		// real: agrupar milhares deles só polui o relatório.
		if f.SHA == "" || f.Size == 0 {
			continue
		}
		if _, ok := bySHA[f.SHA]; !ok {
			order = append(order, f.SHA)
		}
		bySHA[f.SHA] = append(bySHA[f.SHA], f)
	}

	for _, sha := range order {
		files := bySHA[sha]
		if len(files) < 2 {
			continue
		}
		g := duplicateGroup{SHA: sha, Size: files[0].Size, Count: len(files)}
		g.Wasted = g.Size * int64(len(files)-1)
		// Ordena por caminho para a sugestão ser determinística.
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		for _, f := range files {
			g.Files = append(g.Files, duplicateFile{
				Path: f.Path, Size: f.Size, Root: rootOfPath(f.Path),
			})
		}
		g.Keep = suggestKeep(g.Files)
		rep.Groups = append(rep.Groups, g)
		rep.TotalWaste += g.Wasted
		rep.FileCount += len(files)
	}

	// Mais desperdício primeiro: é o que interessa resolver.
	sort.Slice(rep.Groups, func(i, j int) bool {
		if rep.Groups[i].Wasted != rep.Groups[j].Wasted {
			return rep.Groups[i].Wasted > rep.Groups[j].Wasted
		}
		return rep.Groups[i].Files[0].Path < rep.Groups[j].Files[0].Path
	})
	rep.GroupCount = len(rep.Groups)
	return rep
}

// suggestKeep escolhe o arquivo a manter: caminho mais curto (mais perto da
// raiz, mais "oficial") e, no empate, o primeiro em ordem alfabética.
func suggestKeep(files []duplicateFile) string {
	best := files[0].Path
	for _, f := range files[1:] {
		if len(f.Path) < len(best) {
			best = f.Path
		}
	}
	return best
}

// rootOfPath devolve o apelido do volume a partir do caminho exibível.
func rootOfPath(p string) string {
	if i := strings.Index(p, "/"); i > 0 {
		first := p[:i]
		if first != ".." && !strings.Contains(first, ".") {
			return first
		}
	}
	return "principal"
}

func (a *App) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	rep := a.buildDuplicateReport()
	a.activity.Add(Activity{
		IP: clientIP(r), Action: "duplicates_scan", OK: true,
		Detail: itoa(rep.GroupCount) + " grupo(s), " + itoa(rep.FileCount) + " arquivo(s)",
	})
	jsonWrite(w, http.StatusOK, rep)
}

// handleDuplicateCleanup move para a lixeira os caminhos indicados, com duas
// travas: nunca apaga tudo de um grupo (mantém pelo menos uma cópia) e nunca
// apaga o arquivo sugerido como "manter".
func (a *App) handleDuplicateCleanup(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Paths []string `json:"paths"`
	}
	if !decodeBody(w, r, &b) {
		return
	}
	if len(b.Paths) == 0 {
		jsonError(w, http.StatusBadRequest, "empty", "Nada selecionado.")
		return
	}

	// Recalcula o relatório para validar contra o estado atual: um pedido
	// forjado não pode apagar originais.
	rep := a.buildDuplicateReport()
	byPath := map[string]duplicateGroup{}
	for _, g := range rep.Groups {
		for _, f := range g.Files {
			byPath[f.Path] = g
		}
	}

	var trashed, skipped int
	removedPerGroup := map[string]int{}
	for _, p := range b.Paths {
		g, ok := byPath[p]
		if !ok {
			skipped++
			continue
		}
		if p == g.Keep {
			skipped++ // protege a cópia sugerida
			continue
		}
		// Mantém ao menos uma cópia de cada grupo.
		if removedPerGroup[g.SHA] >= g.Count-1 {
			skipped++
			continue
		}
		full, err := a.resolveExisting(p)
		if err != nil {
			skipped++
			continue
		}
		if a.config().TrashDays > 0 {
			if err := a.moveToTrash(full, p, clientIP(r)); err == nil {
				trashed++
				removedPerGroup[g.SHA]++
				continue
			}
		}
		skipped++
	}
	a.activity.Add(Activity{
		IP: clientIP(r), Action: "duplicates_cleanup", OK: trashed > 0,
		Detail: itoa(trashed) + " para a lixeira, " + itoa(skipped) + " ignorado(s)",
	})
	jsonWrite(w, http.StatusOK, map[string]any{
		"ok": true, "trashed": trashed, "skipped": skipped,
	})
}
