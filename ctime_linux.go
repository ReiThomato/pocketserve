package main

import (
	"os"
	"syscall"
)

// ctimeNsOf devolve o ctime em nanossegundos, quando o sistema informar.
//
// O Go não expõe ctime de forma portátil. No Linux (incluindo Android/Termux)
// o Sys() traz *syscall.Stat_t com Ctim. Quando não há, devolve 0 — e nesse
// caso o cache continua válido por mtime de nanossegundos.
//
// Por que ctime e não só mtime: gravar dois conteúdos diferentes no mesmo
// arquivo, no mesmo nanossegundo, é improvável, mas um editor que preserve o
// mtime (rsync -t, alguns "touch" agressivos) faria o backup reaproveitar
// hash antigo. O ctime muda em qualquer escrita, então fecha essa lacuna.
func ctimeNsOf(info os.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok && st != nil {
		return st.Ctim.Nano()
	}
	return 0
}
