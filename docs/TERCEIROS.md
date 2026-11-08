# Terceiros e créditos

O PocketServe usa muito pouco código de terceiros por decisão de projeto: cada dependência extra é uma fonte de falha em um celular. Esta página lista tudo o que entra no binário, o que é usado apenas em desenvolvimento e o que foi criado no próprio repositório.

## Dependências em tempo de execução

### `github.com/skip2/go-qrcode`

| Item | Valor |
|---|---|
| Uso | gerar QR codes localmente (acesso rápido, TOTP) |
| Versão fixada | `v0.0.0-20200617195104-da1b6568686e` |
| Licença | MIT |
| Copyright | Copyright (c) 2014 Tom Harwood |
| Site | https://github.com/skip2/go-qrcode |

É a **única** dependência externa do binário. O QR é gerado no servidor, sem chamada a serviço externo.

Licença completa (MIT):

```text
Copyright (c) 2014 Tom Harwood

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
```

## Biblioteca padrão do Go

O binário é estático (`CGO_ENABLED=0`) e inclui a biblioteca padrão do Go: `net/http`, `crypto/tls`, `crypto/hmac`, `crypto/sha1`, `crypto/sha256`, `crypto/rand`, `encoding/json`, `archive/zip`, `image/*`, `os`, `syscall` e outras.

| Item | Valor |
|---|---|
| Licença | BSD 3-Clause |
| Copyright | Copyright 2009 The Go Authors |
| Termos adicionais | [`PATENTS`](https://go.dev/LICENSE) do projeto Go |

A licença BSD do Go exige a manutenção do aviso de copyright em redistribuições de código-fonte e em binários. Este documento cumpre esse papel para o binário distribuído sem os arquivos de licença do Go.

## Ferramentas de desenvolvimento

Não entram no binário e não são redistribuídas:

| Ferramenta | Uso | Licença |
|---|---|---|
| **Go 1.24+** | compilação e testes | BSD 3-Clause |
| **Make** | atalhos do `Makefile` | GPL (uso como ferramenta) |
| **curl** | verificação HTTP em `deploy/verify-all.sh` | MIT-like |
| **python3** | parsing de JSON na verificação e no `restore-snapshot.sh` | PSF License |
| **bash** | scripts de instalação e operação | GPL |
| **GitHub Actions** | CI (`.github/workflows/ci.yml`) | serviço |

## Serviços opcionais (não inclusos)

O PocketServe **não** depende deles para funcionar, mas a documentação cita:

| Serviço | Para quê | Observação |
|---|---|---|
| **Termux** | ambiente Linux no Android | projeto independente, sem vínculo |
| **Termux:API** | leitura de bateria em alguns aparelhos | opcional |
| **Termux:Boot** | iniciar no boot | opcional |
| **Tailscale** | VPN privada para acesso remoto | opcional; requer conta |
| **Cloudflare Tunnel** | exposição HTTPS sem abrir porta | opcional; requer domínio e conta |
| **Syncthing** | sincronização contínua | opcional; instalado à parte |
| **OpenSSH** | túnel para a interface do Syncthing | opcional |

Nenhum desses serviços recebe dados do PocketServe por padrão. O servidor não tem telemetria, não faz chamadas de saída e não envia analytics.

## Recursos criados no projeto

Os itens abaixo foram escritos especificamente para o PocketServe e não têm licença de terceiros:

- todo o código Go (`*.go`);
- todo o frontend (`web/`): HTML, CSS, JavaScript e o conjunto de ícones SVG inline;
- os ícones e o logotipo: SVG próprio com uma nuvem e uma seta para cima;
- os scripts de `deploy/`;
- toda a documentação em `docs/`;
- o `Makefile`, o workflow de CI e o exemplo de túnel.

O conjunto de ícones é desenhado como `path` SVG no `web/app.js` e no `web/icon.svg`, sem biblioteca de ícones.

## Tipografia

A interface usa apenas fontes do sistema, via `font-family: ui-monospace, SFMono-Regular, Menlo, monospace` e a pilha padrão de sans-serif do navegador. **Não há fontes baixadas, `@import` ou CDN.** Isso mantém o painel funcionando offline e evita requisições externas.

## Marcas citadas

Android, Termux, Tailscale, Cloudflare, Syncthing, Go e demais nomes citados pertencem aos seus respectivos donos. A citação é descritiva e não implica afiliação, patrocínio ou endosso.

O aparelho usado como referência é descrito em [HARDWARE.md](HARDWARE.md) sem marca, modelo comercial ou identificador único.

## Como atualizar esta página

Ao adicionar uma dependência:

1. verifique a licença e se ela é compatível com MIT;
2. fixe a versão no `go.mod`;
3. acrescente a entrada nesta página com copyright e link;
4. se a licença exigir texto no binário, inclua o texto completo.

Dependências com licença incompatível com MIT não entram no projeto.
