package main

import "testing"

func TestSysinfo(t *testing.T) {
	up, l1, l5, l15 := sysinfo()
	if up <= 0 {
		t.Fatalf("uptime inválido: %d", up)
	}
	if l1 < 0 || l5 < 0 || l15 < 0 {
		t.Fatalf("carga inválida: %v %v %v", l1, l5, l15)
	}
	t.Logf("uptime=%ds carga=%.2f %.2f %.2f", up, l1, l5, l15)
}

func TestInterfacesViaIoctl(t *testing.T) {
	out := interfacesViaIoctl()
	if len(out) == 0 {
		t.Skip("nenhuma interface IPv4 encontrada")
	}
	for _, n := range out {
		t.Logf("%s (%s) %s reachable=%v", n.Name, n.Label, n.IPv4, n.Reachable)
	}
}
