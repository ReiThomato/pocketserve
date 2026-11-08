package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// HTTPS é necessário por dois motivos concretos:
//  1. o Chrome exige HTTPS por padrão e mostra ERR_SSL_PROTOCOL_ERROR em
//     servidores HTTP puros;
//  2. cookies marcados como Secure só trafegam em TLS.
//
// O certificado pode vir de duas fontes:
//   - arquivos locais (flag -tls-cert / -tls-key), por exemplo emitidos pelo
//     `tailscale cert` ou por um proxy;
//   - proxy reverso na frente (Cloudflare Tunnel, Caddy), que termina o TLS e
//     fala HTTP com o servidor; nesse caso basta confiar no X-Forwarded-Proto.

// tlsConfig carrega o par cert/chave quando ambos existem.
func loadTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	if certFile == "" || keyFile == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
	}, nil
}

// serveWithTLS sobe HTTP e HTTPS ao mesmo tempo quando há certificado.
// O HTTP continua disponível na porta configurada (LAN) para não quebrar
// nada; o HTTPS usa a porta indicada (padrão 8443).
func serveWithTLS(srv *http.Server, tlsCfg *tls.Config, tlsAddr string) {
	tlsSrv := &http.Server{
		Handler:           srv.Handler,
		ReadHeaderTimeout: 20 * time.Second,
		IdleTimeout:       3 * time.Minute,
		MaxHeaderBytes:    1 << 20,
		TLSConfig:         tlsCfg,
	}
	go func() {
		ln, err := net.Listen("tcp", tlsAddr)
		if err != nil {
			log.Printf("HTTPS indisponível em %s: %v (portas abaixo de 1024 exigem root)", tlsAddr, err)
			return
		}
		log.Printf("HTTPS em %s", tlsAddr)
		if err := tlsSrv.ServeTLS(ln, "", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTPS encerrado: %v", err)
		}
	}()
}

// resolvePublicURL monta os endereços exibidos no painel, priorizando HTTPS
// quando ele está ativo.
func publicScheme(tlsActive bool) string {
	if tlsActive {
		return "https"
	}
	return "http"
}

// trustProxyHeaders liga o reconhecimento de X-Forwarded-Proto. Deve ser usado
// apenas quando um proxy local confiável está na frente (Cloudflare Tunnel,
// Caddy). O clientIP já limita a confiança a conexões de loopback.
func proxyHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
	if proto != "https" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return isLoopbackHost(host)
}
