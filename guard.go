package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Reserva mínima de disco.
//
// Motivo: com dois volumes (interno + cartão) nada impedia um upload de encher
// o cartão até o último byte. Em exFAT isso degrada a alocação, e o Android
// pode ficar instável quando o armazenamento compartilhado zera. O servidor
// também quebra de formas confusas: o CreateTemp falha, o rename falha, e o
// usuário só vê "não foi possível salvar".
//
// A regra: nenhuma escrita pode deixar um volume abaixo da reserva. A reserva
// é configurável em Ajustes (padrão 2 GB) e vale por volume.

const defaultMinFreeMB int64 = 2048

// alertThresholdPercent é o uso a partir do qual o painel avisa.
const alertThresholdPercent = 90

// errNoSpace é devolvido quando a escrita violaria a reserva.
var errNoSpace = errors.New("espaço insuficiente respeitando a reserva de segurança")

// freeSpace devolve o espaço livre em bytes no volume que contém o caminho.
// Se o caminho ainda não existe, sobe até o ancestral mais próximo: é o mesmo
// volume, e evita falhar só porque a pasta de destino vai ser criada agora.
func freeSpace(path string) (int64, error) {
	probe := filepath.Clean(path)
	for {
		st, err := os.Stat(probe)
		if err == nil && st.IsDir() {
			free, _ := diskFree(probe)
			return free, nil
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return 0, errors.New("volume não encontrado")
		}
		probe = parent
	}
}

// minFreeBytes é a reserva configurada em bytes.
func (a *App) minFreeBytes() int64 {
	mb := a.config().MinFreeMB
	if mb <= 0 {
		return 0
	}
	return mb * 1024 * 1024
}

// checkSpace confere se há espaço para "needed" bytes sem furar a reserva.
// needed == 0 valida apenas se o volume já está abaixo da reserva (útil antes
// de começar um upload, para não receber gigabytes por rede à toa).
// Devolve o espaço livre atual e o limite utilizável.
func (a *App) checkSpace(path string, needed int64) (free, usable int64, err error) {
	free, ferr := freeSpace(path)
	if ferr != nil {
		// Volume ilegível: deixa o erro real de I/O aparecer mais adiante,
		// em vez de culpar a reserva.
		return 0, 0, nil
	}
	reserve := a.minFreeBytes()
	usable = free - reserve
	if usable < 0 {
		usable = 0
	}
	// Volume já abaixo da reserva: recusa mesmo que o arquivo seja pequeno.
	// Sem esta checagem, needed == 0 passaria e o upload começaria num volume
	// que já está no limite.
	if reserve > 0 && free < reserve {
		return free, usable, errNoSpace
	}
	if needed > usable {
		return free, usable, errNoSpace
	}
	return free, usable, nil
}

// spaceMessage devolve uma mensagem útil para o usuário.
func (a *App) spaceMessage(free, usable int64) string {
	reserve := a.minFreeBytes()
	if reserve == 0 {
		return "Não há espaço livre suficiente neste volume."
	}
	return fmt.Sprintf(
		"Sem espaço: %s livres, mas %s são reserva de segurança. "+
			"Libere espaço ou reduza a reserva em Ajustes.",
		humanBytes(free), humanBytes(a.minFreeBytes()))
}

func humanBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 4; v /= unit {
		div *= unit
		exp++
	}
	suffix := []string{"KB", "MB", "GB", "TB", "PB"}[exp]
	// Uma casa decimal abaixo de 10, inteiro acima: "1,8 GB" / "202 GB".
	val := float64(n) / float64(div)
	if val < 10 {
		return fmt.Sprintf("%.1f %s", val, suffix)
	}
	return fmt.Sprintf("%.0f %s", val, suffix)
}

// ---------- alertas ----------

type volumeAlertState struct {
	alerted bool
	lastAt  time.Time
}

type spaceAlerter struct {
	mu     sync.Mutex
	states map[string]*volumeAlertState
}

func newSpaceAlerter() *spaceAlerter {
	return &spaceAlerter{states: map[string]*volumeAlertState{}}
}

// check percorre os volumes e registra um aviso quando um deles cruza o limite.
// Avisa uma vez ao entrar no estado crítico e depois no máximo a cada 24 h,
// para não poluir o histórico de atividade.
func (s *spaceAlerter) check(a *App) {
	for _, r := range a.allRoots() {
		free, total := diskFree(r.Path)
		if total == 0 {
			continue
		}
		used := total - free
		pct := float64(used) / float64(total) * 100
		label := r.Name
		if label == "" {
			label = a.primaryLabel()
		}
		s.mu.Lock()
		st := s.states[r.Path]
		if st == nil {
			st = &volumeAlertState{}
			s.states[r.Path] = st
		}
		shouldAlert := pct >= alertThresholdPercent &&
			(!st.alerted || time.Since(st.lastAt) > 24*time.Hour)
		if pct < alertThresholdPercent {
			st.alerted = false
		}
		if shouldAlert {
			st.alerted = true
			st.lastAt = time.Now()
		}
		s.mu.Unlock()
		if shouldAlert {
			a.activity.Add(Activity{
				IP: "sistema", Action: "disk_alert", OK: false,
				Detail: fmt.Sprintf("%s com %.0f%% de uso (%s livres de %s)",
					label, pct, humanBytes(free), humanBytes(total)),
			})
		}
	}
}

// startSpaceMonitor verifica o espaço a cada hora enquanto o servidor roda.
func (a *App) startSpaceMonitor() {
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		// Primeira checagem logo após subir, para pegar o estado atual.
		a.spaceAlerts.check(a)
		for {
			<-t.C
			a.spaceAlerts.check(a)
		}
	}()
}

// spaceReport devolve o estado de cada volume para o painel.
func (a *App) spaceReport() []map[string]any {
	reserve := a.minFreeBytes()
	var out []map[string]any
	for _, r := range a.allRoots() {
		free, total := diskFree(r.Path)
		if total == 0 {
			continue
		}
		used := total - free
		pct := float64(used) / float64(total) * 100
		label := r.Name
		if label == "" {
			label = a.primaryLabel()
		}
		out = append(out, map[string]any{
			"name": label, "path": r.Path,
			"free": free, "total": total, "used": used, "percent": pct,
			"reserve": reserve,
			"low":     free < reserve,
			"alert":   pct >= alertThresholdPercent,
		})
	}
	return out
}
