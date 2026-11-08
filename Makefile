# PocketServe — atalhos de desenvolvimento e build
#
# Requisitos: Go 1.24 ou superior. Para o build do celular, apenas o Go do PC
# (compilação cruzada, sem CGO).

BINARY      ?= pocketserve
GO          ?= go
GOFLAGS     ?=
CGO_ENABLED ?= 0
LDFLAGS     ?= -s -w

.PHONY: all build test race vet fmt cover arm64 verify-http clean

all: fmt vet test build

build:
	$(GO) build $(GOFLAGS) -trimpath -o $(BINARY) .

test:
	$(GO) test $(GOFLAGS) ./...

race:
	$(GO) test $(GOFLAGS) -race ./...

vet:
	$(GO) vet $(GOFLAGS) ./...

fmt:
	@test -z "$$(gofmt -l .)" || { echo "Arquivos fora do gofmt:"; gofmt -l .; exit 1; }

cover:
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

# Binário estático para Android/Termux em aparelhos arm64.
arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) \
		-trimpath -ldflags '$(LDFLAGS)' -o $(BINARY)-arm64 .

# Sobe um servidor limpo e roda a verificação HTTP de ponta a ponta.
# Uso: make verify-http   (o script inicia e encerra a instância de teste)
verify-http:
	bash deploy/verify-local.sh

clean:
	rm -f $(BINARY) $(BINARY)-arm64 coverage.out
	rm -rf dist
