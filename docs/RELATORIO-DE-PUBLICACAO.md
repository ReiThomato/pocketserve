# Relatório de publicação

Inventário do que foi feito para transformar o projeto privado em uma versão pública independente e reproduzível. Este documento responde a três perguntas: **o que foi criado**, **o que foi alterado** e **o que foi removido** — e por quê.

> **Estado:** a versão pública está pronta localmente, mas **não foi publicada**. Não houve `push` para repositório público, criação de release ou mudança de visibilidade. A publicação depende de autorização explícita.

## Resumo

| Item | Antes (privado) | Depois (público) |
|---|---|---|
| Nome do projeto | identidade privada | **PocketServe** |
| Código de versão | `1.10.x` | `1.0.0` |
| Histórico git | 31 commits, 9 tags | histórico novo, independente |
| Documentação | 22 arquivos internos, ~4.500 linhas | 19 arquivos públicos, ~6.200 linhas |
| Testes | 99 | **108** |
| Verificação HTTP | 80/80 | **80/80** |
| Scripts de túnel específicos | 6 | 0 (substituídos por 1 exemplo genérico) |
| Dados pessoais no repositório | presentes | nenhum encontrado na varredura |
| Licença | ausente | **MIT** |

O código funcional é o da linhagem privada 1.10.2, com renomeações de identidade e **quatro correções** encontradas na revisão final.

## Princípios seguidos

1. **A implementação é a autoridade.** Toda afirmação na documentação foi conferida contra o código, não contra documentos antigos.
2. **Não inventar resultados.** Toda medição registrada foi executada; o que não foi medido está marcado como não verificado.
3. **Não tocar na produção.** O repositório privado foi usado somente como leitura. Nenhum comando foi executado nele.
4. **Isolamento total.** O trabalho ocorreu em uma cópia separada (`pocketserve/`), sem o `.git` original.
5. **Placeholders, nunca credenciais.** Domínios, IPs, IDs e caminhos pessoais foram substituídos por `SEU_*` e por faixas de documentação (RFC 5737).
6. **Distinguir o que existe do que é ideia.** Funcionalidade planejada está no [ROADMAP](ROADMAP.md), nunca apresentada como implementada.

## O que foi criado

### Documentação pública (19 arquivos)

Todos os documentos foram escritos do zero. Nenhum é uma cópia do material interno.

| Arquivo | Linhas | Conteúdo |
|---|---:|---|
| `docs/INSTALACAO.md` | 344 | do zero ao primeiro login, com saídas esperadas |
| `docs/CONFIGURACAO.md` | 225 | flags, `config.json`, variáveis, validação |
| `docs/USO.md` | 229 | cada tela e operação do painel |
| `docs/ARQUITETURA.md` | 201 | módulos, fluxo de requisição, decisões |
| `docs/ARMAZENAMENTO.md` | 228 | raízes, apelidos, cartão, lixeira, reserva |
| `docs/BACKUP-E-RECUPERACAO.md` | 286 | snapshots, versões, desastres |
| `docs/OPERACAO.md` | 336 | supervisor, logs, atualização, rollback |
| `docs/SEGURANCA.md` | 227 | modelo de ameaças e riscos residuais |
| `docs/ACESSO-REMOTO.md` | 280 | LAN, Tailscale, Cloudflare, HTTPS |
| `docs/SINCRONIZACAO.md` | 244 | Syncthing, rsync, automação |
| `docs/API.md` | 850 | rotas, escopos, exemplos `curl`, erros |
| `docs/HARDWARE.md` | 301 | medições e limites, sem marca/modelo |
| `docs/HISTORIA-DO-DESENVOLVIMENTO.md` | 346 | defeitos reais, causa, correção, teste |
| `docs/LIMITACOES.md` | 158 | o que não existe ou não é garantido |
| `docs/FAQ.md` | 400 | dúvidas e erros comuns |
| `docs/ROADMAP.md` | 99 | ideias planejadas, separadas do que existe |
| `docs/TERCEIROS.md` | 116 | dependências, licenças e créditos |
| `docs/VERIFICACAO.md` | 456 | resultados reais e como reproduzir |

### Arquivos de projeto

| Arquivo | Para quê |
|---|---|
| `README.md` | porta de entrada: o que faz, requisitos, início rápido, links |
| `CHANGELOG.md` | versão 1.0.0 e resumo da linhagem privada |
| `CONTRIBUTING.md` | como propor mudança, padrão de commit e teste |
| `SECURITY.md` | como relatar vulnerabilidade e escopo de suporte |
| `LICENSE` | MIT |
| `Makefile` | `build`, `test`, `race`, `vet`, `fmt`, `cover`, `arm64`, `verify-http` |
| `.gitignore` | binários, estado de execução, segredos, artefatos |
| `deploy/ci.yml` | workflow de `gofmt`, `vet`, testes, `-race` e build arm64, pronto para ativar |

> **Defeito corrigido durante a revisão final:** o alvo `make arm64` falhava com `flag provided but not defined: -trimpath`, porque `-trimpath` estava dentro de `LDFLAGS` (que vai para o linker, não para o compilador). O `Makefile` foi ajustado e o alvo voltou a produzir o binário de 9,2 MiB.

### Scripts

| Arquivo | Origem | Observação |
|---|---|---|
| `deploy/verify-local.sh` | novo | sobe instância descartável e roda as 80 verificações |
| `deploy/restore-snapshot.sh` | novo | restaura snapshot para pasta de preparo, com validação de caminho e SHA-256 |
| `deploy/cloudflared.example.yml` | novo | modelo genérico, sem ID de túnel ou domínio |

### Testes novos

| Arquivo | Testes | Cobre |
|---|---:|---|
| `shares_test.go` | 6 | hash não exposto (2) e IP real atrás de proxy (4) |
| `coverage_test.go` | +3 | `trash_days` explícito zero, valor ausente e `fallbackRoot` |

## O que foi alterado

### Identidade

Renomeação consistente em código, frontend, scripts, testes e documentação:

| Antes | Depois |
|---|---|
| nome do projeto e binário | `pocketserve` |
| diretório de estado | `~/.pocketserve` |
| variável de ambiente | `POCKETSERVE_HOME` |
| variável de diagnóstico | `POCKETSERVE_ALLOW_MISSING_ROOT` |
| cookie de sessão | `ps_session` |
| cookie de aparelho confiável | `ps_trust` |
| cookie de share | `ps_sh_<token>` |
| prefixo de token de API | `ps_` |
| cabeçalho CSRF | `X-PS` |
| lixeira | `.pocketserve-trash` |
| backups | `.pocketserve-backups` |
| temporários | `.pocketserve-tmp`, `.ps-*` |
| diretório do Syncthing | `~/.pocketserve-sync` |
| chaves de `localStorage` | `ps_hidden`, `ps_view`, `ps_sort`, `ps_dir`, `ps_filter` |

A renomeação foi aplicada também nos **testes**, para que a suíte valide a identidade nova e não a antiga. 195 linhas de `.go` foram alteradas; 19 arquivos `.go` permaneceram idênticos ao privado.

### Versão

- `const Version = "1.0.0"` em `config.go`;
- `deploy/verify-all.sh` passou a esperar `1.0.0`;
- o CHANGELOG explica que a numeração pública é independente da linhagem 1.10.x.

### Quatro correções de código

#### 1. Hash de senha de link exposto

**Antes:** `GET /api/shares` e a resposta de `POST /api/shares` serializavam a struct inteira, incluindo `password_hash`. Qualquer sessão autenticada conseguia ler o hash PBKDF2 da senha de um link.

**Depois:** um método `Share.public()` devolve só os campos necessários, com `has_password` como booleano. O hash permanece apenas em `shares.json`. O frontend passou de `s.password_hash` para `s.has_password`.

**Testes:** `TestSharePublicDoesNotExposePasswordHash` e `TestShareListDoesNotExposePasswordHash`.

#### 2. IP real atrás de túnel

**Antes:** `clientIP` só olhava `X-Forwarded-For` e `X-Real-IP`, e somente de loopback. Um túnel Cloudflare que envia `CF-Connecting-IP` fazia todos os clientes aparecerem como `127.0.0.1`: o limite de login virava um balde único e um atacante podia bloquear o acesso de todos.

**Depois:** a ordem é `CF-Connecting-IP` → `X-Forwarded-For` → `X-Real-IP`, sempre apenas quando a conexão vem do loopback. Cabeçalho malformado é ignorado e cai para o próximo.

**Testes:** quatro casos em `shares_test.go` — cabeçalhos ignorados fora do loopback, `CF-Connecting-IP` aceito no loopback, fallback para `X-Forwarded-For` e cabeçalho malformado.

**Limite residual:** se o proxy local não encaminhar nenhum desses cabeçalhos, o problema persiste. A configuração do proxy é responsabilidade do operador.

#### 3. `trash_days: 0` não desligava a lixeira

**Antes:** `fillDefaults` fazia `if c.TrashDays == 0 { c.TrashDays = 30 }` em toda carga. A interface e vários documentos diziam que `0` desliga a lixeira, mas o valor era anulado antes de qualquer uso. Na prática, a lixeira nunca podia ser desligada.

**Depois:** `fillDefaults` só trata valores negativos. `loadConfig` inspeciona as chaves presentes no JSON: se `trash_days` não existe, aplica 30; se existe com `0`, preserva. O mesmo padrão já usado em `min_free_mb`.

**Testes:** `TestConfigExplicitZeroTrashDaysIsPreserved` (inclui uma volta completa de `saveConfig` + `loadConfig`) e `TestConfigMissingTrashDaysUsesDefault`.

**Impacto na documentação:** este defeito invalidava o texto de `LIMITACOES.md`, `ARMAZENAMENTO.md`, `CONFIGURACAO.md` e `USO.md`. Como a correção foi feita no código, os documentos permanecem com a promessa original — agora verdadeira.

#### 4. Escape hatch de raiz podia expor `/`

**Antes:** o comentário e a documentação diziam que `POCKETSERVE_ALLOW_MISSING_ROOT=1` sobe no diretório atual e "nunca em `/`", mas o código apenas usava `os.Getwd()` sem validar. Se o processo fosse iniciado com `cwd=/`, o painel exporia o sistema de arquivos inteiro — exatamente o cenário que a checagem de raiz deveria impedir.

**Depois:** uma função `fallbackRoot` valida o diretório: recusa `/`, `/system`, `/proc`, `/sys`, `/dev`, `/vendor`, `/etc`, `/root` e qualquer caminho sob `/data` **fora** do Termux. O diretório do Termux (`/data/data/com.termux/...`) é aceito de propósito, porque é o `cwd` normal do app.

**Teste:** `TestFallbackRootRefusesSystemTrees` cobre dez caminhos, incluindo `/`, `/proc/self`, `/data/other` (recusados) e `/sdcard`, `/tmp/ps`, `/data/data/com.termux/files/home` (aceitos).

**Limite:** o escape hatch continua sendo diagnóstico. Ele expõe o diretório de trabalho e o próprio `~/.pocketserve`; o log avisa em destaque.

### Scripts ajustados

| Arquivo | Mudança |
|---|---|
| `deploy/ctl.sh` | renomeação de identidade e caminhos |
| `deploy/install.sh` | renomeação; continua preservando `config.json` existente |
| `deploy/syncthing-rc.sh` | renomeação do diretório de estado |
| `deploy/verify-all.sh` | `X-PS`, versão `1.0.0`, identidade nova |
| `deploy/rollback-guardar.sh` | caminhos e nome do binário |
| `deploy/backup-estado.sh` | caminhos e comentário sem referência interna |

### Documentação corrigida após auditoria final

Inconsistências encontradas ao comparar documentos com o código:

| Documento | Problema | Correção |
|---|---|---|
| `BACKUP-E-RECUPERACAO.md` | listava `trusted.json`, arquivo que não existe | aparelhos confiáveis e códigos de recuperação ficam em `config.json` |
| `BACKUP-E-RECUPERACAO.md` | erro de digitação ("mais simple") | "mais simples" |
| `OPERACAO.md` | dizia que o backup de estado incluía snapshots | snapshots ficam na raiz, não em `~/.pocketserve` |
| `INSTALACAO.md` | dizia que o instalador exige "quatro arquivos" | exige três; `syncthing-rc.sh` é opcional |
| `README.md` | comando de `adb push` não copiava os scripts de rollback e backup | comando atualizado |
| `CHANGELOG.md` | não registrava a correção do `trash_days` | seção "Corrigido" e tabela de verificação |

## O que foi removido

### Documentação interna

Vinte e dois arquivos do repositório privado **não** foram para a versão pública:

| Arquivo | Motivo |
|---|---|
| `RELATORIO.md` | auditoria interna com topologia e riscos de produção |
| `docs/AGENTES.md` | instruções internas de trabalho |
| `docs/AUDITORIA.md` | contém dados do ambiente real |
| `docs/DOMINIO-CLOUDFLARE.md` | domínio, zona, nameservers, procedimento real |
| `docs/ESTADO-DO-DOMINIO.md` | estado de DNS e certificados reais |
| `docs/NOVIDADES-1.1.md` … `1.9.md` (9 arquivos) | notas de versão privadas com caminhos e IPs |
| `docs/RECUPERACAO.md` | procedimento com caminhos de produção |
| `docs/RELATORIO-LOTE-1.md` | planejamento interno |
| `docs/REVOGACAO-DE-SESSOES.md` | relatório interno de deploy |
| `docs/TOKENS-DE-API.md` | versão antiga, substituída por `API.md` |
| `docs/USO-E-SINCRONIZACAO.md` | substituído por `USO.md` e `SINCRONIZACAO.md` |
| `docs/VERIFICACAO-FUNCOES.md` | verificação contra a produção real |

O conteúdo útil desses documentos foi **reescrito**, não copiado: a história real está em `HISTORIA-DO-DESENVOLVIMENTO.md`, as notas de versão viraram o `CHANGELOG`, e a API foi documentada de novo a partir das rotas.

### Scripts de túnel específicos

| Arquivo removido | Motivo |
|---|---|
| `deploy/cloudflared.yml` | continha ID de túnel e domínio reais |
| `deploy/install-tunnel.sh` | instalava o túnel com dados do ambiente |
| `deploy/tunel.sh` | script específico do provedor |
| `deploy/tunel-rapido.sh` | idem |
| `deploy/tunel-hosts.sh` | manipulava `/etc/hosts` para o domínio real |
| `deploy/fix-termux-dns.sh` | correção de DNS específica da rede doméstica |

Substituídos por **um** exemplo genérico: `deploy/cloudflared.example.yml`, com `SEU_TUNNEL_ID` e `SEU_DOMINIO`. O procedimento completo, genérico, está em `ACESSO-REMOTO.md`.

### Artefatos e estado

Não foram copiados:

- `.git` original (histórico com dados pessoais);
- binários compilados;
- `config.json`, `shares.json`, `tokens.json`, `activity.jsonl`, `revoked_sessions.json`;
- logs, PIDs e arquivos de controle;
- dados de execução, snapshots e lixeira;
- `coverage.out` e artefatos de build.

## Dados sensíveis: verificação

Foi feita varredura por padrões de domínio, IP, identificador de túnel, marca do aparelho, nome de usuário e caminho pessoal, além de chaves, tokens e e-mails.

| Categoria | Resultado |
|---|---|
| Domínio privado e subdomínios | nenhuma ocorrência |
| IP da rede local de produção | nenhuma ocorrência |
| ID de túnel / zona / nameservers | nenhuma ocorrência |
| Marca e modelo do aparelho | nenhuma ocorrência |
| Nome de usuário do sistema | nenhuma ocorrência |
| Caminho pessoal do workspace | nenhuma ocorrência |
| Chaves, tokens, senhas em claro | nenhuma ocorrência |
| E-mails reais | nenhuma ocorrência |
| Metadados EXIF/texto em PNG/ICO | nenhum (apenas ícones gerados) |
| Sequência de UUID de produção | nenhuma ocorrência |

IPs que aparecem no repositório são **deliberadamente fictícios**:

- `SEU_ENDERECO_IP` — placeholder de LAN em documentação e saída de terminal;
- `203.0.113.7`, `198.51.100.2`, `198.51.100.3` — faixas de documentação (RFC 5737) em testes;
- `192.168.0.10` — endereço de LAN fictício, usado apenas em teste de unidade de `clientIP`;
- `1.2.3.4`, `5.6.7.8`, `10.0.0.1`, `10.0.0.2` — testes de unidade;
- `1.1.1.1` — verificação de conectividade em `system.go`.

Nenhum deles identifica a rede ou o aparelho de origem.

## Histórico git

O histórico privado **não** foi reaproveitado. Ele contém 31 commits e 9 tags com mensagens que citam domínio, IP e procedimentos de produção — republicá-lo vazaria dados mesmo que os arquivos atuais estejam limpos.

O que foi feito:

- repositório novo inicializado dentro de `pocketserve/`, branch `main`;
- **5 commits** organizados por assunto;
- tag anotada `v1.0.0` no commit final;
- autor único `ReiThomato <119749494+ReiThomato@users.noreply.github.com>`, usando o e-mail *noreply* do GitHub (não expõe e-mail real);
- nenhum remote configurado;
- nenhum push, release ou alteração de visibilidade.

```text
<commit> (HEAD -> main, tag: v1.0.0) docs: documentacao publica completa
<commit> docs: licenca MIT, README, changelog e politicas
<commit> chore: Makefile e alvos de verificacao
<commit> chore(deploy): instalador, controle e verificacao para Termux
<commit> feat: servidor de arquivos PocketServe
```

Para ver os hashes reais do seu repositório:

```bash
git log --oneline --decorate
git rev-parse HEAD
```

O `.gitignore` bloqueia binários, `config.json`, `shares.json`, `tokens.json`, `activity.jsonl`, `revoked_sessions.json`, logs, PIDs, `coverage.out`, `.env`, certificados e chaves. Nenhum desses arquivos entrou em commit.

A autoria foi aplicada a **todos** os commits, não só ao último, para que o histórico não misture identidades:

```bash
git rebase -r --root --exec 'git commit --amend --no-edit --reset-author'
```

O e-mail usado é o *noreply* do GitHub (`119749494+ReiThomato@users.noreply.github.com`), que vincula os commits à conta sem publicar um endereço real.

## Validação executada

| Verificação | Resultado |
|---|---|
| `gofmt -l .` | limpo |
| `go vet ./...` | sem avisos |
| `go build` | ok; `-version` mostra `1.0.0` |
| `go test ./...` | 108 testes, ok |
| `go test -race ./...` | ok, 22,9 s |
| Cobertura | 34,9% |
| Build arm64 estático | 9,2 MiB (9.633.976 bytes), ELF aarch64 |
| Reprodutibilidade do build | SHA-256 idêntico em duas compilações fora de um repositório git; dentro do git o Go 1.24 embute o commit e o hash muda |
| `make verify-http` | 80/80, 0 falhas, 0 skip |
| `bash -n deploy/*.sh` | todos ok |
| `restore-snapshot.sh` | caso válido, pack ausente e caminho inseguro |
| Varredura de dados sensíveis | nenhuma ocorrência |
| Links internos | verificados ao final |

Detalhes e comandos de reprodução em [VERIFICACAO.md](VERIFICACAO.md).

Além da execução na árvore de trabalho, a bateria foi repetida em um **clone do próprio commit `v1.0.0`** — para provar que o repositório valida sozinho, sem depender de arquivos não rastreados:

| Verificação no clone | Resultado |
|---|---|
| `gofmt -l .` | limpo |
| `go vet ./...` | sem avisos |
| `go test ./...` | 108 testes, ok |
| `make verify-http` | 80/80 |
| `make arm64` | 9.633.976 bytes, ELF aarch64 |
| Links internos | 0 quebrados |
| Varredura de dados sensíveis | 0 ocorrências |
| `git log -p --all` | 0 ocorrências de termos privados |

## O que **não** foi feito

Para não superestimar a entrega:

- **não** houve publicação: nenhum push, release ou mudança de visibilidade;
- **não** houve teste do binário arm64 em um aparelho Android nesta versão pública;
- **não** há teste automatizado de navegador;
- **não** há teste de carga, vazão ou estabilidade prolongada;
- **não** há auditoria de acessibilidade;
- **não** há varredura de CVE nas dependências;
- **não** foi adicionado suporte a WebDAV, multiusuário, app nativo ou sincronização;
- **não** foi alterada a funcionalidade existente além das quatro correções descritas;
- **não** foi alterado nada no repositório privado nem na produção.

## Riscos e pendências antes de publicar

| Item | Situação | Recomendação |
|---|---|---|
| Testar o binário no aparelho | pendente | instalar via `install.sh` e rodar o checklist da instalação |
| Revisar textos como falante nativo | pendente | leitura final de `README` e `INSTALACAO` |
| Definir URL do repositório | placeholder `SEU_USUARIO` | substituir em `README`, `INSTALACAO` e `CONTRIBUTING` |
| Titular do copyright | `ReiThomato` | já definido em `LICENSE` |
| Aprovar a licença MIT | escolhida, não confirmada | confirmar antes de publicar |
| Publicar a tag `v1.0.0` | criada localmente | `git push origin main --tags` só após autorização |
| Adicionar remote | nenhum configurado | `git remote add origin URL` no momento da publicação |

## Conclusão

A versão pública está **funcionalmente equivalente** à linhagem privada 1.10.2, com identidade própria, quatro correções, documentação reescrita e nenhum dado pessoal identificado na varredura.

O repositório local está pronto: 5 commits, tag `v1.0.0`, 87 arquivos rastreados e nenhum remote. O que falta é decisão, não código: testar no aparelho, revisar os textos e autorizar a publicação. Nenhum desses passos foi executado sem pedido explícito.
