//go:build !linux

package main

import "os"

// ctimeNsOf é um no-op em sistemas sem Stat_t.Ctim. O cache continua
// funcionando por mtime de nanossegundos.
func ctimeNsOf(info os.FileInfo) int64 { return 0 }
