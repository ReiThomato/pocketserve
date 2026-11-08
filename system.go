package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	qrcode "github.com/skip2/go-qrcode"
)

type BatteryInfo struct {
	Available   bool    `json:"available"`
	Level       int     `json:"level"`
	Status      string  `json:"status"`
	Temperature float64 `json:"temperature"`
	Plugged     string  `json:"plugged"`
}

type NetInterface struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	IPv4      string `json:"ipv4"`
	Tailscale bool   `json:"tailscale"`
	Reachable bool   `json:"reachable"`
}

type StorageInfo struct {
	Path    string  `json:"path"`
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Free    uint64  `json:"free"`
	Percent float64 `json:"percent"`
}

type SystemInfo struct {
	Version       string         `json:"version"`
	ServerName    string         `json:"server_name"`
	ServerUptime  int64          `json:"server_uptime"`
	DeviceUptime  int64          `json:"device_uptime"`
	Cores         int            `json:"cores"`
	Load1         float64        `json:"load1"`
	Load5         float64        `json:"load5"`
	Load15        float64        `json:"load15"`
	CPUPercent    float64        `json:"cpu_percent"`
	MemTotal      uint64         `json:"mem_total"`
	MemUsed       uint64         `json:"mem_used"`
	MemFree       uint64         `json:"mem_free"`
	MemPercent    float64        `json:"mem_percent"`
	Storage       StorageInfo    `json:"storage"`
	Volumes       []VolumeInfo   `json:"volumes"`
	Battery       BatteryInfo    `json:"battery"`
	Interfaces    []NetInterface `json:"interfaces"`
	URLs          []string       `json:"urls"`
	TailscaleName string         `json:"tailscale_name"`
	Hostname      string         `json:"hostname"`
	ShortHost     string         `json:"short_host"`
	Port          int            `json:"port"`
}

type cpuTimes struct {
	total uint64
	idle  uint64
	at    time.Time
}

func readCPUTimes() cpuTimes {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}
	}
	line := strings.SplitN(string(b), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return cpuTimes{}
	}
	var total, idle uint64
	for i, f := range fields[1:] {
		v, _ := strconv.ParseUint(f, 10, 64)
		total += v
		if i == 3 || i == 4 { // idle + iowait
			idle += v
		}
	}
	return cpuTimes{total: total, idle: idle, at: time.Now()}
}

func (a *App) cpuPercentLocked() float64 {
	cur := readCPUTimes()
	if cur.total == 0 {
		// /proc/stat é bloqueado para apps no Android 15; o painel mostra a
		// carga do sistema no lugar do percentual de CPU.
		return -1
	}
	prev := a.cpuPrev
	a.cpuPrev = cur
	if prev.total == 0 || cur.total <= prev.total || cur.at.Sub(prev.at) > 2*time.Minute {
		return a.lastCPU
	}
	dt := float64(cur.total - prev.total)
	di := float64(cur.idle - prev.idle)
	if dt <= 0 {
		return a.lastCPU
	}
	p := (1 - di/dt) * 100
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	a.lastCPU = p
	return p
}

// sysinfo lê uptime e carga direto do kernel. No Android o /proc/loadavg e o
// /proc/uptime são negados aos apps, mas a syscall sysinfo continua liberada.
func sysinfo() (uptime int64, l1, l5, l15 float64) {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		return 0, 0, 0, 0
	}
	const shift = 16 // SI_LOAD_SHIFT
	div := float64(uint64(1) << shift)
	return si.Uptime, float64(si.Loads[0]) / div, float64(si.Loads[1]) / div, float64(si.Loads[2]) / div
}

func readLoad() (float64, float64, float64) {
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		f := strings.Fields(string(b))
		if len(f) >= 3 {
			l1, _ := strconv.ParseFloat(f[0], 64)
			l5, _ := strconv.ParseFloat(f[1], 64)
			l15, _ := strconv.ParseFloat(f[2], 64)
			return l1, l5, l15
		}
	}
	_, l1, l5, l15 := sysinfo()
	return l1, l5, l15
}

func readUptime() int64 {
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		f := strings.Fields(string(b))
		if len(f) > 0 {
			if v, err := strconv.ParseFloat(f[0], 64); err == nil {
				return int64(v)
			}
		}
	}
	up, _, _, _ := sysinfo()
	return up
}

func readMem() (total, used, free uint64, percent float64) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}
	vals := map[string]uint64{}
	for _, line := range strings.Split(string(b), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		v, _ := strconv.ParseUint(parts[1], 10, 64)
		vals[key] = v * 1024
	}
	total = vals["MemTotal"]
	avail := vals["MemAvailable"]
	if avail == 0 {
		avail = vals["MemFree"] + vals["Buffers"] + vals["Cached"]
	}
	if total > 0 {
		used = total - avail
		free = avail
		percent = float64(used) / float64(total) * 100
	}
	// MemAvailable é a métrica correta (inclui cache recuperável). MemFree
	// sozinho daria a impressão de que o aparelho está sem memória.
	return
}

func diskInfo(path string) StorageInfo {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return StorageInfo{Path: path}
	}
	bs := uint64(st.Bsize)
	total := st.Blocks * bs
	free := st.Bavail * bs
	used := (st.Blocks - st.Bfree) * bs
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return StorageInfo{Path: path, Total: total, Used: used, Free: free, Percent: pct}
}

// ---------- battery ----------

type batteryCached struct {
	at time.Time
	b  BatteryInfo
}

var batteryMu sync.Mutex
var batteryCache batteryCached

func readBattery() BatteryInfo {
	batteryMu.Lock()
	defer batteryMu.Unlock()
	if time.Since(batteryCache.at) < 20*time.Second {
		return batteryCache.b
	}
	b := batteryFromSysfs()
	if !b.Available {
		b = batteryFromTermuxAPI()
	}
	batteryCache = batteryCached{at: time.Now(), b: b}
	return b
}

func batteryFromSysfs() BatteryInfo {
	read := func(p string) string {
		v, err := os.ReadFile(p)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(v))
	}
	capStr := read("/sys/class/power_supply/battery/capacity")
	if capStr == "" {
		return BatteryInfo{}
	}
	lvl, _ := strconv.Atoi(capStr)
	status := strings.ToUpper(read("/sys/class/power_supply/battery/status"))
	tempStr := read("/sys/class/power_supply/battery/temp")
	temp, _ := strconv.ParseFloat(tempStr, 64)
	if temp > 200 {
		temp /= 10
	}
	plugged := read("/sys/class/power_supply/battery/charge_type")
	b := BatteryInfo{Available: true, Level: lvl, Status: status, Temperature: temp, Plugged: plugged}
	return b
}

// lookPathNoAccess procura um executável no PATH sem usar access(2).
// O seccomp do Android bloqueia faccessat2, que o exec.LookPath usa no Go
// recente; um stat comum em cada candidato evita o SIGSYS.
func lookPathNoAccess(name string) string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		p := filepath.Join(dir, name)
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		if st.Mode().Perm()&0o111 != 0 {
			return p
		}
	}
	return ""
}

func batteryFromTermuxAPI() BatteryInfo {
	cmdPath := lookPathNoAccess("termux-battery-status")
	if cmdPath == "" {
		return BatteryInfo{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, cmdPath).Output()
	if err != nil {
		return BatteryInfo{}
	}
	var raw struct {
		Percentage  int     `json:"percentage"`
		Status      string  `json:"status"`
		Temperature float64 `json:"temperature"`
		Plugged     string  `json:"plugged"`
	}
	if json.Unmarshal(out, &raw) != nil {
		return BatteryInfo{}
	}
	return BatteryInfo{
		Available:   true,
		Level:       raw.Percentage,
		Status:      strings.ToUpper(raw.Status),
		Temperature: raw.Temperature,
		Plugged:     raw.Plugged,
	}
}

// ---------- network ----------

var netWarnOnce sync.Once

func networkInterfaces() []NetInterface {
	ifaces, err := net.Interfaces()
	if err != nil {
		netWarnOnce.Do(func() {
			log.Printf("net.Interfaces indisponível: %v (usando ioctl)", err)
		})
	} else {
		var out []NetInterface
		for _, ifc := range ifaces {
			if ifc.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := ifc.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				ipnet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}
				ip4 := ipnet.IP.To4()
				if ip4 == nil || !ipnet.IP.IsGlobalUnicast() {
					continue
				}
				label, ts, reachable := classifyInterface(ifc.Name, ip4)
				out = append(out, NetInterface{
					Name: ifc.Name, Label: label, IPv4: ip4.String(), Tailscale: ts, Reachable: reachable,
				})
			}
		}
		if len(out) > 0 {
			return sortInterfaces(out)
		}
	}
	if out := interfacesViaIoctl(); len(out) > 0 {
		return sortInterfaces(out)
	}
	return fallbackInterfaces()
}

func sortInterfaces(out []NetInterface) []NetInterface {
	rank := func(n NetInterface) int {
		switch {
		case strings.HasPrefix(n.Name, "wlan"):
			return 0
		case n.Tailscale:
			return 1
		case strings.HasPrefix(n.Name, "ap"), strings.HasPrefix(n.Name, "swlan"):
			return 2
		case strings.HasPrefix(n.Name, "eth"):
			return 3
		case strings.HasPrefix(n.Name, "rndis"), strings.HasPrefix(n.Name, "usb"):
			return 4
		case strings.HasPrefix(n.Name, "rmnet"), strings.HasPrefix(n.Name, "ccmni"):
			return 6
		}
		return 5
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
	return out
}

// interfacesViaIoctl enumera os IPv4 com SIOCGIFCONF. É o caminho usado no
// Android quando o netlink é negado pelo SELinux; ioctl é permitido.
func interfacesViaIoctl() []NetInterface {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return nil
	}
	defer syscall.Close(fd)
	syscall.CloseOnExec(fd)

	const (
		ifreqSize   = 40
		ifNameSize  = 16
		siocgifconf = 0x8912
	)
	buf := make([]byte, 32*1024)
	var ifc struct {
		Len int32
		_   int32
		Buf *byte
	}
	ifc.Len = int32(len(buf))
	ifc.Buf = &buf[0]
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(siocgifconf), uintptr(unsafe.Pointer(&ifc)))
	if errno != 0 {
		return nil
	}
	n := int(ifc.Len)
	if n > len(buf) {
		n = len(buf)
	}
	var out []NetInterface
	for off := 0; off+ifreqSize <= n; off += ifreqSize {
		rec := buf[off : off+ifreqSize]
		name := string(rec[:ifNameSize])
		if i := strings.IndexByte(name, 0); i >= 0 {
			name = name[:i]
		}
		if name == "" || name == "lo" {
			continue
		}
		family := uint16(rec[16]) | uint16(rec[17])<<8
		if family != syscall.AF_INET {
			continue
		}
		ip := net.IPv4(rec[20], rec[21], rec[22], rec[23]).To4()
		if ip == nil || !ip.IsGlobalUnicast() {
			continue
		}
		label, ts, reachable := classifyInterface(name, ip)
		out = append(out, NetInterface{
			Name: name, Label: label, IPv4: ip.String(), Tailscale: ts, Reachable: reachable,
		})
	}
	return out
}

// fallbackInterfaces cobre o Android, onde a listagem de interfaces via
// netlink pode ser negada pelo SELinux. O IP de saída é descoberto com um
// UDP dial (sem enviar pacotes) e o nome vem do sysfs.
func fallbackInterfaces() []NetInterface {
	ip := outboundIPv4()
	if ip == nil || !isPrivateIPv4(ip) {
		return nil
	}
	name, label := "rede", "Rede local"
	switch {
	case interfaceActive("wlan0"):
		name, label = "wlan0", "Wi-Fi"
	case interfaceActive("ap0"):
		name, label = "ap0", "Hotspot"
	case interfaceActive("swlan0"):
		name, label = "swlan0", "Hotspot"
	case interfaceActive("eth0"):
		name, label = "eth0", "Ethernet"
	}
	return []NetInterface{{Name: name, Label: label, IPv4: ip.String(), Reachable: true}}
}

func outboundIPv4() net.IP {
	conn, err := net.Dial("udp4", "1.1.1.1:53")
	if err != nil {
		return nil
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.To4()
	}
	return nil
}

func interfaceActive(name string) bool {
	// A tabela de rotas principais lista a interface quando ela tem endereço.
	if b, err := os.ReadFile("/proc/net/route"); err == nil {
		for _, line := range strings.Split(string(b), "\n")[1:] {
			f := strings.Fields(line)
			if len(f) > 0 && f[0] == name {
				return true
			}
		}
		return false
	}
	// Se /proc não estiver acessível, usa a presença no sysfs.
	_, err := os.Stat("/sys/class/net/" + name)
	return err == nil
}

func isPrivateIPv4(ip net.IP) bool {
	if len(ip) != 4 {
		return false
	}
	return ip[0] == 10 ||
		(ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31) ||
		(ip[0] == 192 && ip[1] == 168)
}

func classifyInterface(name string, ip net.IP) (label string, tailscale bool, reachable bool) {
	switch {
	case strings.HasPrefix(name, "tailscale"):
		return "Tailscale", true, true
	case strings.HasPrefix(name, "wlan"):
		return "Wi-Fi", false, true
	case strings.HasPrefix(name, "ap"), strings.HasPrefix(name, "swlan"):
		return "Hotspot", false, true
	case strings.HasPrefix(name, "eth"):
		return "Ethernet", false, true
	case strings.HasPrefix(name, "rndis"), strings.HasPrefix(name, "usb"):
		return "USB", false, true
	case strings.HasPrefix(name, "rmnet"), strings.HasPrefix(name, "ccmni"):
		return "Dados móveis", false, false
	}
	if ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127 {
		return "Tailscale", true, true
	}
	return name, false, true
}

func (a *App) buildSystemInfo() SystemInfo {
	cfg := a.config()
	a.sysMu.Lock()
	cpu := a.cpuPercentLocked()
	a.sysMu.Unlock()

	l1, l5, l15 := readLoad()
	memTotal, memUsed, memFree, memPct := readMem()
	storage, volumes := a.storageSummary()
	host, _ := os.Hostname()
	ifaces := networkInterfaces()
	port := cfg.Port
	var urls []string
	for _, n := range ifaces {
		if n.Reachable {
			urls = append(urls, "http://"+n.IPv4+":"+strconv.Itoa(port))
		}
	}
	if cfg.TailscaleName != "" {
		urls = append(urls, "http://"+cfg.TailscaleName+":"+strconv.Itoa(port))
	}
	return SystemInfo{
		Version:       Version,
		ServerName:    cfg.ServerName,
		ServerUptime:  int64(time.Since(a.started).Seconds()),
		DeviceUptime:  readUptime(),
		Cores:         runtime.NumCPU(),
		Load1:         l1,
		Load5:         l5,
		Load15:        l15,
		CPUPercent:    round1(cpu),
		MemTotal:      memTotal,
		MemUsed:       memUsed,
		MemFree:       memFree,
		MemPercent:    round1(memPct),
		Storage:       storage,
		Volumes:       volumes,
		Battery:       readBattery(),
		Interfaces:    ifaces,
		URLs:          urls,
		TailscaleName: cfg.TailscaleName,
		Hostname:      host,
		ShortHost:     shortHost(cfg.TailscaleName, host),
		Port:          port,
	}
}

func round1(v float64) float64 {
	return float64(int(v*10)) / 10
}

func (a *App) handleSystem(w http.ResponseWriter, r *http.Request) {
	a.sysMu.Lock()
	if a.sysCache != nil && time.Since(a.sysCacheAt) < 2*time.Second {
		info := *a.sysCache
		a.sysMu.Unlock()
		jsonWrite(w, http.StatusOK, info)
		return
	}
	a.sysMu.Unlock()

	info := a.buildSystemInfo()

	a.sysMu.Lock()
	a.sysCache = &info
	a.sysCacheAt = time.Now()
	a.sysMu.Unlock()
	jsonWrite(w, http.StatusOK, info)
}

func (a *App) handleQR(w http.ResponseWriter, r *http.Request) {
	text := strings.TrimSpace(r.URL.Query().Get("text"))
	if text == "" || len(text) > 1024 {
		http.Error(w, "texto inválido", http.StatusBadRequest)
		return
	}
	qr, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		http.Error(w, "não foi possível gerar o QR", http.StatusInternalServerError)
		return
	}
	qr.DisableBorder = false
	png, err := qr.PNG(420)
	if err != nil {
		http.Error(w, "não foi possível gerar o QR", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(png)
}

func (a *App) handleLog(w http.ResponseWriter, r *http.Request) {
	const max = 160 * 1024
	f, err := os.Open(a.logPath)
	if err != nil {
		jsonWrite(w, http.StatusOK, map[string]any{"log": "Sem registro de log ainda."})
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		jsonWrite(w, http.StatusOK, map[string]any{"log": ""})
		return
	}
	start := int64(0)
	if st.Size() > max {
		start = st.Size() - max
	}
	if _, err := f.Seek(start, 0); err != nil {
		jsonWrite(w, http.StatusOK, map[string]any{"log": ""})
		return
	}
	buf := make([]byte, st.Size()-start)
	n, _ := f.Read(buf)
	jsonWrite(w, http.StatusOK, map[string]any{"log": string(buf[:n]), "size": st.Size()})
}

// shortHost devolve o nome curto que funciona dentro do tailnet.
// No Android, os.Hostname() devolve "localhost", então o nome curto precisa
// vir do MagicDNS configurado: "celular.tail1234.ts.net" -> "celular".
func shortHost(tailscaleName, host string) string {
	ts := strings.TrimSpace(tailscaleName)
	if ts != "" {
		if i := strings.Index(ts, "."); i > 0 {
			return ts[:i]
		}
		return ts
	}
	h := strings.TrimSpace(host)
	if h == "" || h == "localhost" {
		return ""
	}
	if i := strings.Index(h, "."); i > 0 {
		return h[:i]
	}
	return h
}

// VolumeInfo descreve um armazenamento individual (interno, cartão...).
type VolumeInfo struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Free    uint64  `json:"free"`
	Percent float64 `json:"percent"`
	Primary bool    `json:"primary"`
	// Reserve é a reserva de segurança configurada; Low indica que o volume
	// já está abaixo dela (uploads serão recusados) e Alert que passou do
	// limite de aviso de uso.
	Reserve int64 `json:"reserve"`
	Low     bool  `json:"low"`
	Alert   bool  `json:"alert"`
}

// storageSummary soma todos os volumes configurados e devolve o agregado.
// Antes o painel mostrava apenas a raiz principal, então o cartão SD sumia
// da conta: 216 GB em vez dos 331 GB realmente disponíveis.
func (a *App) storageSummary() (StorageInfo, []VolumeInfo) {
	var vols []VolumeInfo
	seenPath := map[string]bool{}

	for _, r := range a.allRoots() {
		clean := filepath.Clean(r.Path)
		if seenPath[clean] {
			continue
		}
		info := diskInfo(r.Path)
		if info.Total == 0 {
			continue
		}
		seenPath[clean] = true
		name := r.Name
		if name == "" {
			name = "Interno"
			if len(vols) > 0 {
				name = "Principal"
			}
		}
		vols = append(vols, VolumeInfo{
			Name: name, Path: r.Path,
			Total: info.Total, Used: info.Used, Free: info.Free,
			Percent: info.Percent, Primary: r.Name == "",
		})
	}
	if len(vols) == 0 {
		return diskInfo(a.rootPath()), nil
	}
	reserve := a.minFreeBytes()
	for i := range vols {
		vols[i].Reserve = reserve
		vols[i].Low = reserve > 0 && int64(vols[i].Free) < reserve
		vols[i].Alert = vols[i].Percent >= alertThresholdPercent
	}

	sum := StorageInfo{Path: "todos"}
	for _, v := range vols {
		sum.Total += v.Total
		sum.Used += v.Used
		sum.Free += v.Free
	}
	if sum.Total > 0 {
		sum.Percent = float64(sum.Used) / float64(sum.Total) * 100
	}
	return sum, vols
}
