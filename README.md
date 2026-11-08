# PocketServe

**Seu servidor pessoal de bolso.**

Servidor de arquivos que roda **dentro de um celular Android**, no [Termux](https://termux.dev/). Binário Go único, frontend embutido, sem banco de dados, sem framework de frontend, sem nuvem obrigatória e sem telemetria.

O painel abre no navegador de qualquer aparelho na mesma rede Wi-Fi ou em uma VPN privada. Os arquivos ficam no armazenamento do próprio celular — interno ou cartão microSD.

> **Versão pública 1.0.0.** Esta é a primeira versão independente do PocketServe. Ela consolida a linhagem privada 1.10.x em uma numeração pública própria; veja o [CHANGELOG](CHANGELOG.md).

---

## O que ele faz

- **Arquivos** — navegar, enviar, baixar, renomear, copiar, mover, criar pastas, editar texto e baixar pastas em ZIP.
- **Até 8 pastas raiz** — interno e cartão microSD ao mesmo tempo, cada um com um apelido curto (`cartao/...`).
- **Lixeira por volume** — exclusão vira restauração; retenção configurável (padrão 30 dias).
- **Backups por snapshot** — conteúdo endereçado por SHA-256, deduplicado, 14 snapshots, um por dia se ativado.
- **Versões por arquivo** — histórico extraído dos snapshots; baixar ou restaurar uma versão antiga.
- **Relatório de duplicatas** — grupos por SHA-256, com sugestão de cópia a manter e limpeza segura.
- **Compartilhamentos** — link público com token, validade e senha opcional, somente leitura.
- **Autenticação** — senha com PBKDF2, sessão assinada, CSRF, limite de tentativas, TOTP (RFC 6238), códigos de recuperação e aparelhos confiáveis.
- **Revogação de sessão** — logout invalida o cookie no servidor e sobrevive a reinícios.
- **Tokens de API** — escopos (`files:read`, `files:write`, `files:delete`, `shares:manage`, `admin`), validade e revogação individual.
- **Painel de sistema** — CPU, memória, armazenamento por volume, bateria, rede, endereços de acesso e QR code.
- **Operação** — supervisor com reinício automático, inicialização no boot, log local e atualização com rollback.

## O que ele **não** faz

Para evitar expectativa errada:

- não é multiusuário;
- não tem WebDAV;
- não tem aplicativo móvel nativo;
- não faz sincronização própria (use Syncthing ou scripts pela API);
- não tem interface de restauração completa de um snapshot (a restauração é por arquivo);
- não é um serviço de nuvem pública e não deve ser exposto sem TLS, senha forte e 2FA;
- não substitui um NAS ou um PC servidor em desempenho, redundância ou durabilidade.

A lista completa está em [docs/LIMITACOES.md](docs/LIMITACOES.md).

---

## Como funciona

```
 Navegador (LAN / Tailscale / proxy HTTPS)
        │  HTTP :8080  (ou HTTPS :8443)
        ▼
┌──────────────────────────────────────────────────┐
│  PocketServe — um processo Go, binário estático  │
│  rotas · auth · arquivos · shares · backups      │
│  frontend embutido via go:embed                  │
└──────────────────────────────────────────────────┘
        │
        ▼
  /sdcard  →  cartão microSD (opcional)
  ~/.pocketserve/config.json · shares.json
                 tokens.json · activity.jsonl
```

- **Um único processo.** A API JSON e o painel são servidos pelo mesmo binário.
- **Sem banco de dados.** O estado fica em arquivos JSON no diretório `~/.pocketserve`.
- **Sem build de frontend.** HTML, CSS e JavaScript são embutidos no binário com `go:embed`.
- **Uma dependência externa:** [`skip2/go-qrcode`](https://github.com/skip2/go-qrcode), usada apenas para gerar QR codes localmente.

Detalhes em [docs/ARQUITETURA.md](docs/ARQUITETURA.md).

---

## Requisitos

### No celular

- Android com **arm64-v8a** (a grande maioria dos aparelhos dos últimos anos).
- [Termux](https://fido.termux.dev/) instalado (testado com 0.118.3).
- Acesso ao armazenamento compartilhado (`termux-setup-storage`).
- Para uso contínuo: carregador conectado, Termux sem restrição de bateria e, de preferência, uma rede Wi-Fi estável.

Não é necessário root. O servidor roda como um aplicativo comum dentro do Termux.

### No computador (para compilar)

- **Go 1.24 ou superior**.
- `adb` (Android Platform Tools) para enviar o binário, ou qualquer forma de copiar arquivos para o celular.
- Nenhum SDK Android, NDK ou compilador C é necessário: o build é Go puro com `CGO_ENABLED=0`.

---

## Início rápido

### 1. Compilar no computador

```bash
git clone https://github.com/SEU_USUARIO/pocketserve.git
cd pocketserve

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags "-s -w" -o pocketserve-arm64 .
```

O resultado é um binário estático para Android/Termux. Confira:

```bash
file pocketserve-arm64
# pocketserve-arm64: ELF 64-bit LSB executable, ARM aarch64, statically linked, ...
```

### 2. Preparar o celular

No Termux:

```bash
termux-setup-storage     # aceite o pedido de acesso às fotos/arquivos
termux-wake-lock         # mantém o processo vivo com a tela apagada
pkg update && pkg upgrade
```

No Android, abra **Configurações → Apps → Termux** e:

- permita **Autostart** (se o fabricante oferecer);
- escolha **Sem restrições** em economia de bateria;
- trave o Termux no menu de apps recentes (evita encerramento acidental).

### 3. Enviar e instalar

No computador, com o celular conectado por USB e depuração ativada:

```bash
adb shell mkdir -p /sdcard/Download/PocketServe
adb push pocketserve-arm64 /sdcard/Download/PocketServe/
adb push deploy/install.sh deploy/ctl.sh deploy/syncthing-rc.sh \
  deploy/rollback-guardar.sh deploy/backup-estado.sh \
  /sdcard/Download/PocketServe/
```

Se você não usa Syncthing, pode omitir `deploy/syncthing-rc.sh`; o instalador avisa e segue sem supervisioná-lo.

No Termux:

```bash
bash /sdcard/Download/PocketServe/install.sh
```

Saída esperada (resumida):

```
== PocketServe - instalacao ==
Destino: /data/data/com.termux/files/home/.pocketserve
...
PocketServe iniciado (PID ...).
Painel local:    http://127.0.0.1:8080
Rede Wi-Fi:      http://SEU_ENDERECO_IP:8080
...
Instalacao concluida.
```

O instalador **preserva** um `config.json` existente. Para uma instalação limpa, remova `~/.pocketserve` antes.

### 4. Primeiro acesso

No navegador de outro aparelho na mesma Wi-Fi:

```
http://SEU_ENDERECO_IP:8080
```

Na primeira abertura, o painel pede para criar a senha do administrador (mínimo de 8 caracteres). Depois disso, entre com a senha.

Para descobrir o endereço no próprio celular:

```bash
pocketserve status
pocketserve url
```

### 5. Atualizar

```bash
# no PC: recompilar
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags "-s -w" -o pocketserve-arm64 .

# no Termux: guardar o binário atual para rollback (opcional, mas recomendado)
bash /sdcard/Download/PocketServe/rollback-guardar.sh

# no PC: enviar o binário novo
adb push pocketserve-arm64 /sdcard/Download/PocketServe/

# no Termux: instalar por cima, preservando o config.json
bash /sdcard/Download/PocketServe/install.sh
```

O `config.json` é preservado e o supervisor reinicia o serviço. Passo a passo completo, com verificação e rollback, em [docs/OPERACAO.md](docs/OPERACAO.md).

---

## Segurança em uma frase

O modelo assume **um único usuário, acesso por rede local ou VPN privada e nenhuma porta aberta no roteador**. Se você decidir expor o servidor à internet, leia [docs/SEGURANCA.md](docs/SEGURANCA.md) e [docs/ACESSO-REMOTO.md](docs/ACESSO-REMOTO.md) antes: use HTTPS, senha forte e 2FA.

Recursos de segurança já implementados:

- senha com PBKDF2-HMAC-SHA256 (210.000 iterações);
- cookie de sessão `HttpOnly`, `SameSite=Lax`, assinado com HMAC e com ID único por login;
- CSRF por cabeçalho customizado + verificação de `Origin`;
- bloqueio progressivo após 5 tentativas de login falhas;
- TOTP de 6 dígitos, janela de tolerância de 30 s, códigos de recuperação com hash;
- tokens de API com escopo, validade e hash em disco;
- validação de caminho contra traversal, symlinks e raízes não configuradas;
- cabeçalhos de segurança e CSP restritiva;
- logs de atividade locais (500 entradas em memória, arquivo JSONL).

---

## Estrutura do repositório

```
.
├── *.go                  código do servidor (Go 1.24, sem CGO)
├── *_test.go             testes automatizados
├── web/                  frontend embutido (HTML/CSS/JS + ícones)
├── deploy/
│   ├── install.sh        instalador para Termux
│   ├── ctl.sh            start/stop/status/supervisor
│   ├── syncthing-rc.sh   supervisor opcional do Syncthing
│   ├── backup-estado.sh  backup do estado do servidor
│   ├── rollback-guardar.sh  guarda o binário antes de atualizar
│   ├── restore-snapshot.sh  restaura snapshot em pasta de preparo
│   ├── verify-all.sh     verificação HTTP de ponta a ponta
│   ├── verify-local.sh   sobe instância descartável e roda a verificação
│   ├── ci.yml            workflow de CI pronto para ativar
│   └── cloudflared.example.yml  modelo genérico de túnel
├── docs/                 documentação completa
├── Makefile
├── go.mod / go.sum
├── CONTRIBUTING.md
├── SECURITY.md
├── CHANGELOG.md
└── LICENSE (MIT)
```

---

## Documentação

| Documento | Para quê |
|---|---|
| [Instalação](docs/INSTALACAO.md) | do zero ao primeiro login, com comandos e saídas esperadas |
| [Configuração](docs/CONFIGURACAO.md) | flags, `config.json`, variáveis de ambiente e ajustes |
| [Uso](docs/USO.md) | cada tela e operação do painel |
| [Arquitetura](docs/ARQUITETURA.md) | módulos, fluxo de requisição, decisões e limites |
| [Armazenamento](docs/ARMAZENAMENTO.md) | raízes, apelidos, cartão SD, lixeira e reserva de disco |
| [Backup e recuperação](docs/BACKUP-E-RECUPERACAO.md) | snapshots, versões, restauração e desastres |
| [Operação](docs/OPERACAO.md) | supervisor, logs, atualização, rollback e diagnóstico |
| [Segurança](docs/SEGURANCA.md) | modelo de ameaças, autenticação e riscos residuais |
| [Acesso remoto](docs/ACESSO-REMOTO.md) | LAN, Tailscale, Cloudflare Tunnel e HTTPS |
| [Sincronização](docs/SINCRONIZACAO.md) | Syncthing, rsync e automação pela API |
| [API](docs/API.md) | rotas, escopos, exemplos `curl` e códigos de erro |
| [Hardware](docs/HARDWARE.md) | medições, limites térmicos e o que é declarado/não verificado |
| [História do desenvolvimento](docs/HISTORIA-DO-DESENVOLVIMENTO.md) | problemas reais, causas, soluções e resultados |
| [Limitações](docs/LIMITACOES.md) | o que não existe ou não é garantido |
| [FAQ](docs/FAQ.md) | dúvidas e problemas comuns |
| [Roadmap](docs/ROADMAP.md) | ideias planejadas (não implementadas) |
| [Terceiros](docs/TERCEIROS.md) | dependências e créditos |
| [Verificação](docs/VERIFICACAO.md) | testes executados e como reproduzir |
| [Relatório de publicação](docs/RELATORIO-DE-PUBLICACAO.md) | o que mudou nesta versão pública |

---

## Testes

```bash
make test        # go test ./...
make race        # go test -race ./...
make vet         # go vet ./...
make cover       # cobertura
make arm64       # binário arm64 para Termux
```

A verificação HTTP completa precisa de uma instância limpa. O script `deploy/verify-local.sh` sobe uma instância temporária, roda as 80 verificações e encerra:

```bash
bash deploy/verify-local.sh
```

Resultados da última execução e o que cada teste cobre: [docs/VERIFICACAO.md](docs/VERIFICACAO.md).

---

## Licença e créditos

Distribuído sob a licença **MIT**. Veja [LICENSE](LICENSE).

O PocketServe nasceu de um projeto pessoal de servidor de arquivos em celular. Esta versão pública foi reorganizada, renomeada e documentada de forma independente. Créditos de bibliotecas e ferramentas em [docs/TERCEIROS.md](docs/TERCEIROS.md).

**Não há vínculo com fabricantes de celulares, com o Termux, com o Tailscale ou com a Cloudflare.** Marcas citadas pertencem aos seus respectivos donos.
