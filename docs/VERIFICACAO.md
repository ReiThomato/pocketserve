# Verificação

Resultados reais dos testes executados nesta versão pública, com os comandos para reproduzir cada um. Nada aqui é estimativa: os números foram coletados em **08/11/2026**, com Go 1.24.5 em Linux amd64, e o binário arm64 foi gerado por compilação cruzada.

> Se você rodar em outra máquina, alguns números mudam (tempo, cobertura por linha, tamanho do binário). O que deve permanecer igual é o **resultado**: testes passam, verificação HTTP 80/80, `gofmt` limpo e `vet` sem avisos.

Toda a bateria foi executada novamente **a partir de um clone do commit marcado com `v1.0.0`**, para garantir que o que está versionado valida sozinho — sem depender de arquivos não rastreados no diretório de trabalho.

## Resumo

| Verificação | Comando | Resultado |
|---|---|---|
| Formatação | `gofmt -l .` | nenhum arquivo listado |
| Análise estática | `go vet ./...` | sem avisos |
| Build nativo | `go build -trimpath` | ok, versão reportada `1.0.0` |
| Testes | `go test ./...` | **108 testes**, pacote ok |
| Corrida de dados | `go test -race ./...` | ok, sem corrida |
| Cobertura | `go test -coverprofile` | **34,9%** das instruções |
| Build arm64 | `GOOS=linux GOARCH=arm64` | binário estático, **9,2 MiB** |
| Verificação HTTP | `make verify-http` | **80 ok**, 0 falhas, 0 skip |
| Restauração de snapshot | `deploy/restore-snapshot.sh` | extrai e confere SHA-256 |
| Sintaxe dos scripts | `bash -n deploy/*.sh` | todos ok |

## Ambiente usado

```text
Go:       go1.24.5 linux/amd64
Módulo:   pocketserve (go 1.24)
Sistema:  Linux x86_64 (máquina de build)
Alvo:     linux/arm64, CGO desligado
```

O código não usa CGO. O binário arm64 é gerado no PC; nenhum SDK Android ou NDK é necessário.

## 1. Formatação

```bash
gofmt -l .
```

Saída esperada: **nenhuma linha**. Se algum arquivo aparecer, rode `gofmt -w .` e confira o diff antes de commitar.

O `Makefile` tem o mesmo teste:

```bash
make fmt
```

## 2. Análise estática

```bash
go vet ./...
```

Saída esperada: nenhuma. Um aviso do `vet` costuma indicar erro real de formatação de string, cópia de `mutex` ou uso incorreto de `Printf`.

## 3. Build e versão

```bash
go build -trimpath -o /tmp/pocketserve .
/tmp/pocketserve -version
```

Saída esperada:

```text
PocketServe 1.0.0
```

A versão vem de `const Version = "1.0.0"` em `config.go` e é a mesma reportada por `/healthz`.

## 4. Testes automatizados

```bash
go test ./...
```

Saída esperada:

```text
ok  	pocketserve	3.1s
```

Contagem:

```bash
go test -list '^Test' ./... | grep -c '^Test'
```

Resultado medido: **108**.

### Distribuição por arquivo

| Arquivo | Testes | Foco |
|---|---:|---|
| `coverage_test.go` | 26 | senha, TOTP, config, `fallbackRoot`, tokens, sessão, CSRF, limiter |
| `sessions_test.go` | 13 | revogação, isolamento, persistência, cookie legado |
| `investigation_test.go` | 10 | regressões encontradas na auditoria |
| `responsive_test.go` | 9 | layout e CSS embutido |
| `tokens_test.go` | 9 | escopos, expiração, revogação, persistência |
| `guard_test.go` | 7 | reserva de disco |
| `duplicates_test.go` | 6 | relatório e limpeza segura |
| `roots_test.go` | 6 | apelidos, traversal, `EXDEV` |
| `shares_test.go` | 6 | hash não exposto e IP real |
| `versions_test.go` | 6 | histórico e restauração |
| `routes_test.go` | 3 | registro de rotas, escalada de token |
| `volumes_test.go` | 3 | volumes e espaço |
| `backup_test.go` | 2 | cache de hash |
| `system_test.go` | 2 | sysinfo e ioctl |
| `testsupport_test.go` | 0 | helpers compartilhados |

### Testes de regressão que existem por causa de defeitos reais

Estes não são testes genéricos: cada um trava um defeito que já aconteceu.

| Teste | Defeito que ele impede |
|---|---|
| `TestBackupDoesNotReuseHashAfterQuickEdit` | duas edições no mesmo segundo reutilizavam o hash antigo e o snapshot gravava conteúdo errado |
| `TestManifestCacheSkipsRecentFiles` | arquivo recém-modificado vir do cache de hash |
| `TestConfigExplicitZeroTrashDaysIsPreserved` | `trash_days: 0` virar 30 e a lixeira nunca desligar |
| `TestConfigMissingTrashDaysUsesDefault` | config sem a chave perder o padrão de 30 dias |
| `TestSharePublicDoesNotExposePasswordHash` | hash da senha do link voltar na resposta da API |
| `TestShareListDoesNotExposePasswordHash` | idem na listagem |
| `TestClientIPUsesCloudflareHeaderFromLoopback` | túnel local contar todos os clientes como `127.0.0.1` |
| `TestClientIPIgnoresProxyHeadersFromNetwork` | cliente da LAN falsificar o próprio IP |
| `TestReusedCookieAfterLogoutIsRejected` | logout apagar só o cookie e a sessão continuar válida |
| `TestRevocationSurvivesRestart` | revogação se perder ao reiniciar |
| `TestLogoutDoesNotAffectOtherSessions` | dois logins no mesmo segundo derrubarem um ao outro |
| `TestLegacyCookieWithoutIDStillWorks` | cookie antigo, sem ID, parar de funcionar de imediato |
| `TestSplitAliasKeepsPrimaryFolders` | `DCIM`, `Download` e `Projetos` recusados com múltiplas raízes |
| `TestMovePathAcrossFilesystems` | mover entre volumes falhar com `EXDEV` |
| `TestFallbackRootRefusesSystemTrees` | escape hatch de raiz ausente expor `/`, `/proc` ou `/data` |
| `TestDuplicateCleanupKeepsOneCopy` | limpeza de duplicatas apagar a última cópia |
| `TestUploadRejectedWhenReserveExceeded` | upload furar a reserva de disco |

## 5. Corrida de dados

```bash
go test -race ./...
```

Saída esperada:

```text
ok  	pocketserve	22.9s
```

Nenhum relatório de corrida. O tempo é maior porque o detector desacelera a execução.

## 6. Cobertura

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
```

Resultado medido:

```text
total:				(statements)			34,9%
```

### Cobertura por arquivo (instruções)

| Arquivo | Cobertura | Cobertas/Total |
|---|---:|---:|
| `duplicates.go` | 85,9% | 73/85 |
| `json.go` | 81,8% | 18/22 |
| `sessions.go` | 78,0% | 64/82 |
| `config.go` | 77,5% | 79/102 |
| `roots.go` | 74,4% | 163/219 |
| `versions.go` | 69,0% | 100/145 |
| `ctime_linux.go` | 66,7% | 2/3 |
| `guard.go` | 61,4% | 54/88 |
| `auth.go` | 59,0% | 173/293 |
| `tokens.go` | 56,1% | 110/196 |
| `activity.go` | 48,2% | 27/56 |
| `backup.go` | 41,3% | 135/327 |
| `trash.go` | 30,3% | 56/185 |
| `main.go` | 25,7% | 67/261 |
| `settings.go` | 21,0% | 21/100 |
| `checksum.go` | 18,8% | 9/48 |
| `files.go` | 16,3% | 139/853 |
| `totp.go` | 15,3% | 22/144 |
| `shares.go` | 10,7% | 33/307 |
| `system.go` | 10,6% | 37/349 |
| `trust.go` | 0,0% | 0/70 |
| `tls.go` | 0,0% | 0/27 |

### Como interpretar

34,9% é um número baixo se lido isoladamente, e é honesto dizer por quê:

- boa parte da cobertura vem de **testes unitários** de funções puras (hash, config, token, sessão);
- os handlers HTTP são exercitados pela **verificação funcional** (`deploy/verify-all.sh`), que **não** conta para a cobertura do `go test`;
- `files.go` (853 instruções) concentra listagem, upload, download, ZIP, busca, thumbnails e editor — o alvo mais caro de testar;
- `trust.go` e `tls.go` dependem de cookie e certificado reais; nenhum teste os cobre hoje;
- `system.go` lê `/proc`, `/sys` e bateria, que variam por Android.

Somando os dois mecanismos, todas as rotas são exercitadas ao menos uma vez. Mas cobertura de rota **não** é cobertura de comportamento: as lacunas estão listadas em [LIMITACOES.md](LIMITACOES.md) e no [ROADMAP](ROADMAP.md).

## 7. Build arm64

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags '-s -w' -o pocketserve-arm64 .
file pocketserve-arm64
```

Saída medida:

```text
pocketserve-arm64: ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV),
statically linked, BuildID[sha1]=503f71a8..., stripped
```

Tamanho: **9.633.976 bytes** (9,2 MiB; 9,6 MB em base decimal).

O build é **reprodutível** desde que as mesmas condições sejam usadas: mesmo código, mesma versão de Go e mesma presença ou ausência de metadados de VCS. Duas compilações seguidas a partir da mesma árvore, fora de um repositório git, geraram o mesmo SHA-256:

```text
3fc90cfe73846eb4f08fd5b828d90447f747b08791127092f535c8146f0c2942
```

**Atenção:** o Go 1.24 embute a versão e o commit do VCS no binário por padrão. Compilar dentro de um repositório git **muda o hash** em relação a compilar na mesma árvore sem git, e qualquer novo commit (ou arquivo modificado) muda de novo. Isso não é instabilidade: é informação gravada de propósito.

Dá para ver o que entrou:

```bash
go version -m pocketserve-arm64
```

Saída resumida de um build dentro do repositório, com a árvore limpa:

```text
mod	pocketserve	v1.0.0
dep	github.com/skip2/go-qrcode	v0.0.0-20200617195104-da1b6568686e
build	vcs=git
build	vcs.revision=<hash do commit>
build	vcs.time=<data do commit>
build	vcs.modified=false
```

Se houver alteração não commitada, aparecem `+dirty` no módulo e `vcs.modified=true`. Um binário publicado deve sair de uma árvore limpa, para que o `vcs.revision` identifique exatamente o código.

Para um hash estável independentemente do estado do repositório, desligue o VCS:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -buildvcs=false \
  -trimpath -ldflags '-s -w' -o pocketserve-arm64 .
```

Com `-buildvcs=false` o hash é estável (`3fc90cfe...`, medido nesta revisão). Com VCS ligado, o hash muda a cada commit e a cada alteração não commitada — mesmo tamanho e mesmo comportamento, identidade diferente. Ao publicar um binário, gere e divulgue o hash da sua própria compilação e diga qual comando foi usado.

## 8. Verificação HTTP de ponta a ponta

```bash
make verify-http
```

O comando chama `deploy/verify-local.sh`, que:

1. cria um diretório temporário com `home` e raiz vazios;
2. compila o binário (ou usa `POCKETSERVE_BIN`);
3. cria a configuração inicial e sobe o servidor na porta **19600**;
4. espera `/healthz` responder;
5. roda `deploy/verify-all.sh` contra essa instância;
6. encerra o processo e apaga o diretório.

Nada toca em uma instalação real.

### Resultado medido

```text
==================================================================
 resultado: 80 ok
==================================================================
exit=0
```

Zero falhas, zero skips.

### Distribuição das 80 verificações

| Bloco | Verificações | O que cobre |
|---|---:|---|
| [1] básico | 8 | `/healthz`, `/api/state`, versão, painel, CSS, JS, manifest |
| [2] autenticação | 9 | setup, login certo/errado, CSRF ausente, whoami, rotas protegidas, TOTP, confiança |
| [3] raízes | 3 | `GET`/`POST` de settings, settings sem sessão |
| [4] listagem | 4 | raiz, subpasta, inexistente, arquivo como pasta |
| [5] upload | 4 | raiz, subpasta, overwrite, criação de pasta |
| [6] leitura | 5 | download, raw, text, checksum, thumb inválida |
| [7] busca | 3 | query curta, query válida, subpasta |
| [8] operações | 7 | mkdir, duplicado, rename, copy, move, save, delete |
| [9] ZIP | 3 | pasta, múltiplos, sem `path` |
| [10] lixeira | 3 | listar, restaurar, esvaziar |
| [11] backup | 3 | rodar, status, baixar arquivo |
| [12] versões | 4 | listar, inexistente, download inválido, restore inválido |
| [13] duplicatas | 2 | relatório, limpeza vazia |
| [14] shares | 6 | criar, listar, página pública, ZIP público, revogar, caminho inválido |
| [15] tokens | 6 | criar, listar, sem escopo, escopo inválido, revogar, apagar |
| [16] sistema | 4 | sistema, atividade, log, QR |
| [17] segurança | 4 | traversal simples, traversal aninhado, mutação sem CSRF, nome de upload malicioso |
| [18] logout | 2 | logout, cookie rejeitado depois |

### O que essa verificação **não** prova

- não testa o navegador: layout, foco, acessibilidade e JavaScript não são cobertos;
- não mede desempenho nem uso de memória;
- não testa o Android, o Termux ou o supervisor;
- não simula falha de disco, queda de energia ou cartão removido;
- não cobre TLS nem proxy, porque a instância é HTTP em loopback;
- o teste de traversal verifica a recusa (404), não todas as formas de escape possíveis.

Ela prova que as rotas existem, respondem com o código esperado e que os controles básicos (CSRF, sessão, traversal) estão ativos.

## 9. Restauração de snapshot

O script `deploy/restore-snapshot.sh` foi testado com três casos.

### Caso válido

```bash
mkdir -p "$T/snapshots" "$T/packs"
printf 'conteudo restaurado\n' > "$T/file.txt"
SHA=$(sha256sum "$T/file.txt" | awk '{print $1}')
tar -czf "$T/packs/$SHA.pack" -C "$T" file.txt
# manifesto apontando para docs/a.txt
bash deploy/restore-snapshot.sh "$T/snapshots/test.json" "$T/out"
cmp "$T/out/docs/a.txt" "$T/file.txt"
```

Saída:

```text
snapshot test: 1 arquivo(s) restaurado(s), 0 falha(s)
destino: .../out
conteudo restaurado identico: ok
exit=0
```

### Pack ausente ou hash inválido

Saída:

```text
FALTA pack: x.txt (0000...)
snapshot bad: 0 arquivo(s) restaurado(s), 1 falha(s)
exit=1
```

O script **não** grava o arquivo e retorna código diferente de zero.

### Caminho inseguro

Saída:

```text
IGNORADO (caminho inseguro): '../../etc/passwd'
snapshot evil: 0 arquivo(s) restaurado(s), 1 falha(s)
exit=1
```

Caminhos com `..`, absolutos ou vazios são recusados antes de qualquer escrita.

### Sem argumentos

```text
uso: deploy/restore-snapshot.sh SNAPSHOT.json PASTA_DE_DESTINO
exit=2
```

## 10. Sintaxe dos scripts

```bash
for f in deploy/*.sh; do bash -n "$f"; done
```

Todos passam: `backup-estado.sh`, `ctl.sh`, `install.sh`, `restore-snapshot.sh`, `rollback-guardar.sh`, `syncthing-rc.sh`, `verify-all.sh`, `verify-local.sh`.

`bash -n` verifica apenas a sintaxe. Não prova que os scripts funcionam no Termux, onde o `bash` vem de `$PREFIX` e não de `/bin`.

## 11. Verificações que **não** foram executadas

Para não dar a impressão de cobertura maior do que existe:

| Item | Situação |
|---|---|
| Teste no aparelho Android desta versão pública | **não executado** — o binário arm64 foi só compilado, não instalado |
| Teste de navegador automatizado | não existe |
| Teste de carga e vazão | não existe |
| Teste de corrupção de cartão | não existe |
| Teste de queda de energia | não existe |
| Teste de TLS com certificado real | não existe na suíte |
| Teste de proxy Cloudflare/Tailscale | não existe automatizado |
| Auditoria de acessibilidade (WCAG) | não existe |
| Teste de restauração de estado completo (`tar.gz`) | executado manualmente na linhagem privada, não automatizado |
| Varredura de dependências por CVE | não executada nesta revisão |

O aparelho de referência e as medições de hardware estão em [HARDWARE.md](HARDWARE.md). O histórico de defeitos e correções está em [HISTORIA-DO-DESENVOLVIMENTO.md](HISTORIA-DO-DESENVOLVIMENTO.md).

## 12. Como reproduzir tudo

Do zero, em uma máquina Linux com Go 1.24+:

```bash
git clone https://github.com/SEU_USUARIO/pocketserve.git
cd pocketserve

gofmt -l .
go vet ./...
go build -trimpath -o /tmp/pocketserve .
/tmp/pocketserve -version

go test ./...
go test -race ./...
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1

make verify-http

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags '-s -w' -o pocketserve-arm64 .
file pocketserve-arm64
sha256sum pocketserve-arm64
```

A verificação HTTP precisa de `curl` e `python3`. Sem eles, `verify-local.sh` encerra com mensagem explícita.

Se você não tiver Go, mas tiver o binário arm64, ainda pode rodar a verificação HTTP em um aparelho arm64:

```bash
POCKETSERVE_BIN=./pocketserve-arm64 bash deploy/verify-local.sh
```

## 13. Interpretação honesta

O que os números **permitem** afirmar:

- o pacote compila, passa no `vet` e não tem corrida de dados detectada;
- 108 testes cobrem as partes puras e os defeitos históricos mais graves;
- as 80 verificações HTTP confirmam que as rotas respondem e que os controles básicos estão ativos;
- o build arm64 é estático, reprodutível e pequeno o bastante para um celular.

O que os números **não** permitem afirmar:

- que o sistema é livre de bugs;
- que resiste a um ataque determinado;
- que o desempenho é suficiente para o seu acervo;
- que o backup funciona no seu aparelho sem teste;
- que a interface é acessível;
- que a instalação funciona em todas as versões de Android e Termux.

A cobertura de 34,9% é o indicador mais claro da lacuna: quase dois terços das instruções não são exercitados por teste automatizado. A verificação HTTP cobre rotas, não ramos internos de erro.

Se você depende deste servidor, faça o seu próprio teste de restauração. É a única verificação que prova o que realmente importa.
