# Contribuindo com o PocketServe

Obrigado pelo interesse. Este projeto é pequeno e prioriza simplicidade, previsibilidade e operação em celular.

## Antes de abrir uma issue

1. Leia [docs/FAQ.md](docs/FAQ.md) e [docs/LIMITACOES.md](docs/LIMITACOES.md).
2. Verifique se o problema acontece na versão mais recente.
3. Para falhas de segurança, **não abra issue pública**: siga [SECURITY.md](SECURITY.md).

## Ambiente de desenvolvimento

Requisitos: Go 1.24+, `curl` e `python3` (usados pelo script de verificação HTTP). Não é necessário Android para desenvolver ou rodar os testes; o servidor também funciona em Linux/macOS.

```bash
git clone https://github.com/SEU_USUARIO/pocketserve.git
cd pocketserve

go build ./...
go test ./...
go test -race ./...
```

Para subir uma instância local descartável:

```bash
mkdir -p /tmp/pocketserve-home /tmp/pocketserve-root
go run . -home /tmp/pocketserve-home -root /tmp/pocketserve-root -port 18080
```

Abra `http://127.0.0.1:18080`, crie a senha e use. O diretório `-home` guarda configuração, tokens e atividade; `-root` é a pasta exposta.

> Em produção no celular, a raiz padrão é `/sdcard`. Em Linux, se `/sdcard` não existir, o padrão é o diretório atual — sempre passe `-root` explicitamente em testes.

## Antes de enviar uma mudança

Execute:

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
```

E, se a mudança tocar rotas, autenticação, upload/download ou frontend:

```bash
bash deploy/verify-local.sh
```

O script sobe uma instância limpa, executa a verificação HTTP completa e encerra. Se ele não existir no seu checkout, use `bash deploy/verify-all.sh` contra um servidor já em execução, definindo `BASE` e `ROOT`.

## Estilo de código

- Go idiomático, `gofmt` obrigatório.
- Mensagens de erro e comentários em português, coerentes com o restante do projeto.
- Sem dependências novas se a biblioteca padrão resolver. A única dependência externa atual é `skip2/go-qrcode`.
- Mantenha o frontend sem framework e sem build step: os arquivos de `web/` são embutidos diretamente.
- Preserve a compatibilidade de `config.json`: campos novos devem ter padrão seguro e não podem apagar dados existentes.
- Toda funcionalidade nova precisa de teste automatizado quando for lógica de servidor.
- Não inclua dados reais, domínios, IPs, tokens, cookies, capturas de tela ou caminhos pessoais em código, testes, issues ou commits.

## Commits e pull requests

- Commits pequenos, com mensagem no imperativo e escopo quando fizer sentido: `fix(auth): ...`, `docs: ...`, `test(backup): ...`.
- Um PR deve explicar **o problema**, **a solução**, **como testar** e **quais riscos/limitações** permanecem.
- Mudanças de segurança devem descrever o modelo de ameaça e o impacto.
- Mudanças de interface devem incluir antes/depois em texto ou imagem **sem dados pessoais**.

## Licença

Ao contribuir, você concorda que sua contribuição será licenciada sob a **MIT**, a mesma do projeto.
