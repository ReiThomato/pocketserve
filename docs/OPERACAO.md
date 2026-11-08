# Operação

Como manter o PocketServe rodando no dia a dia: comandos, supervisor, boot, logs, atualizações, rollback e diagnóstico.

## Pré-requisitos no aparelho

| Item | Para quê | Como conferir |
|---|---|---|
| Termux | executar o binário e os scripts | `echo "$PREFIX"` deve mostrar um caminho dentro de `com.termux` |
| Armazenamento compartilhado | acessar `/sdcard` | `ls /sdcard` |
| Termux:Boot (opcional) | subir sozinho após reiniciar | app instalado e aberto uma vez |
| Wake lock | evitar que o Android suspenda o processo | `termux-wake-lock` disponível |
| Tailscale (opcional) | acesso remoto sem abrir porta | `tailscale status` |

Se `/sdcard` não existir:

```bash
termux-setup-storage
```

Aceite a permissão no Android. Sem ela, o servidor não inicia com a raiz padrão.

## Comandos do dia a dia

O instalador cria o comando curto `pocketserve` em `$PREFIX/bin`, que chama `~/.pocketserve/ctl.sh`.

```bash
pocketserve status
pocketserve start
pocketserve stop
pocketserve restart
pocketserve url
pocketserve log
pocketserve watchdog
pocketserve unwatchdog
pocketserve wdlog
pocketserve sync status
```

### `status`

```text
PocketServe: rodando (PID 12345)
Painel local:    http://127.0.0.1:8080
Rede Wi-Fi:      http://SEU_ENDERECO_IP:8080
Tailscale:       http://SEU_NOME.tailXXXX.ts.net:8080
Supervisor:  ativo (PID 12346, checagem a cada 15s)
```

Os endereços variam conforme as interfaces ativas e o nome MagicDNS configurado. Se o supervisor estiver inativo, o servidor não será reiniciado sozinho.

### `start` e `stop`

- `start` verifica se já existe processo, remove órfãos, ativa o wake lock e sobe o binário em segundo plano. O PID fica em `~/.pocketserve/server.pid`.
- `stop` encerra o processo com `SIGTERM`, espera até 5 segundos e usa `SIGKILL` se necessário. Também libera o wake lock.
- `restart` para o supervisor, para o servidor, sobe de novo e reativa o supervisor.

### `url`

Mostra apenas os endereços. É útil para abrir o painel rapidamente no computador.

### `log` e `wdlog`

- `pocketserve log` mostra as últimas 100 linhas de `~/.pocketserve/server.log`.
- `pocketserve wdlog` mostra as últimas 60 linhas de `~/.pocketserve/watchdog.log`.

O `server.log` é **append-only** e não é rotacionado automaticamente. Para limitar o tamanho:

```bash
pocketserve stop
tail -n 2000 ~/.pocketserve/server.log > ~/.pocketserve/server.log.tmp
mv ~/.pocketserve/server.log.tmp ~/.pocketserve/server.log
pocketserve start
```

Não truncar com o servidor rodando: o processo mantém o descritor aberto e continua gravando no mesmo inode.

## Supervisor

O supervisor é um segundo processo, iniciado por `pocketserve watchdog` (ou pelo boot), que:

1. verifica a cada **15 segundos** se o processo existe;
2. consulta `GET /healthz` na porta configurada;
3. reinicia o servidor se ele sumiu ou está vivo mas sem responder;
4. usa recuos de **30 s** após 2 falhas e **60 s** após 5 falhas;
5. registra tudo em `watchdog.log`.

O PID fica em `~/.pocketserve/watchdog.pid`. Para parar de vez, use `pocketserve unwatchdog`; `stop` também desativa o supervisor antes de encerrar o servidor.

O supervisor não é um serviço do Android. Se o próprio Termux for morto pelo sistema, os dois processos morrem e o boot só os traz de volta no próximo reinício. Por isso:

- permita o Termux em **Autostart**;
- configure a economia de bateria como **Sem restrições** para o Termux;
- mantenha o wake lock ativo (`start` já tenta ativar).

## Início automático

O instalador grava `~/.termux/boot/pocketserve.sh`, executado pelo **Termux:Boot** após o boot:

```sh
#!/data/data/com.termux/files/usr/bin/sh
termux-wake-lock 2>/dev/null || true
sleep 8
"$HOME/.pocketserve/ctl.sh" watchdog >> "$HOME/.pocketserve/boot.log" 2>&1
...
```

O atraso de 8 segundos dá tempo ao Android de montar o armazenamento. O log do boot fica em `~/.pocketserve/boot.log`.

Para testar sem reiniciar:

```bash
bash ~/.termux/boot/pocketserve.sh
pocketserve status
```

## Tarefas automáticas

| Tarefa | Frequência | O que faz |
|---|---|---|
| Supervisor | 15 s | mantém o processo vivo e respondendo |
| Limpeza da lixeira | 6 h | remove itens mais antigos que `trash_days` |
| Snapshot automático | a cada 30 min verifica; roda se passou de 23 h | só com `backup_daily` ligado |
| Alerta de disco | 1 h e no boot | grava `disk_alert` acima de 90% de uso |
| Limpeza de sessões revogadas | 1 h | remove entradas expiradas de `revoked_sessions.json` |
| Sessões/aparelhos vencidos | no boot e no uso | remove entradas expiradas |

Os snapshots automáticos só acontecem enquanto o servidor está ligado. Um aparelho desligado à noite não “recupera” o snapshot perdido no dia seguinte: ele roda quando o servidor voltar e a última execução tiver mais de 23 h.

## Atualização

O princípio é: **preservar `config.json` e o estado**, trocar apenas binário e scripts.

### 1. Guardar o binário atual

No aparelho, se o script estiver disponível:

```bash
bash /sdcard/Download/PocketServe/rollback-guardar.sh
```

Saída esperada:

```text
binario guardado para rollback:
  /sdcard/Download/PocketServe/pocketserve-arm64-ANTES-1.0.0
```

O script lê a versão com `pocketserve -version` e copia o binário. Se o caminho do script não existir, copie manualmente:

```bash
cp ~/.pocketserve/pocketserve /sdcard/Download/pocketserve-ANTES-$(~/.pocketserve/pocketserve -version | awk '{print $NF}')
```

### 2. Guardar o estado

```bash
bash /sdcard/Download/PocketServe/backup-estado.sh
```

O script cria `pocketserve-estado-AAAAMMDD-HHMMSS.tar.gz` em `/sdcard/Download/PocketServe`, com a pasta `~/.pocketserve` inteira: `config.json`, shares, tokens, atividade, sessões revogadas e logs. Mostra o SHA-256 no final. Guarde esse hash.

> Os snapshots de arquivos **não** ficam em `~/.pocketserve`, e sim em `<raiz>/.pocketserve-backups`. Para levá-los junto, copie essa pasta separadamente.

### 3. Compilar a nova versão

No computador, com Go 1.24+:

```bash
git pull
make test
make arm64
```

O binário sai como `pocketserve-arm64` (ELF arm64 estático).

### 4. Enviar e instalar

Envie para o aparelho, na mesma pasta:

- `pocketserve-arm64`
- `install.sh`
- `ctl.sh`
- `syncthing-rc.sh` (se usar Syncthing)

No Termux:

```bash
cd /sdcard/Download/PocketServe
bash install.sh
```

O instalador:

1. para a instância antiga;
2. substitui o binário de forma atômica (`pocketserve.new` + `mv`);
3. copia o `ctl.sh`;
4. **preserva** `config.json` existente;
5. recria o comando `pocketserve`;
6. atualiza o script de boot;
7. sobe o supervisor.

Confira:

```bash
pocketserve status
curl -s http://127.0.0.1:8080/healthz
```

Resposta esperada:

```json
{"ok":true,"service":"pocketserve","version":"1.0.0"}
```

### 5. Rollback

```bash
pocketserve stop
cp /sdcard/Download/PocketServe/pocketserve-arm64-ANTES-1.0.0 ~/.pocketserve/pocketserve
chmod 700 ~/.pocketserve/pocketserve
pocketserve start
```

Se houver mudança incompatível de configuração, restaure também o tar.gz do estado **com o servidor parado**. O formato atual é compatível com a linhagem 1.10.x; versões futuras podem exigir migração.

## Saúde e diagnóstico

### Verificação rápida

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/api/state
```

`/healthz` é público e não revela dados. `/api/state` também é público, mas informa apenas nome, versão, se falta configurar a senha e se a sessão atual está autenticada.

### Porta em uso

Sintoma no log: `não foi possível ouvir em 0.0.0.0:8080: address already in use`.

```bash
pocketserve status
ss -ltnp 2>/dev/null | grep 8080 || netstat -ltnp 2>/dev/null | grep 8080
```

O `start` já tenta remover processos órfãos comparando o executável real em `/proc/<pid>/exe`. Se outro aplicativo ocupa a porta, troque a porta em **Configurações → Servidor** ou use `-port`.

### Raiz indisponível

Sintoma: o processo encerra com `raiz "..." indisponível`.

Causas: cartão removido, HD desconectado, permissão de armazenamento revogada. Reconecte o volume; o supervisor tenta de novo. Para diagnóstico consciente:

```bash
POCKETSERVE_ALLOW_MISSING_ROOT=1 ~/.pocketserve/pocketserve -home ~/.pocketserve
```

Isso sobe usando o diretório atual como raiz e **não deve ser usado em produção**: expõe outra árvore de arquivos. O servidor recusa `/`, `/system`, `/proc`, `/sys`, `/dev`, `/vendor`, `/etc`, `/root` e `/data` fora do Termux, encerrando com mensagem de erro.

### Servidor sobe, mas o painel não abre

1. `pocketserve status` mostra o PID?
2. `curl -v http://127.0.0.1:8080/healthz` responde?
3. O celular e o computador estão na mesma rede?
4. O firewall do roteador está bloqueando a porta?
5. Se usa Tailscale, o nome MagicDNS resolve? `tailscale status`.

### Servidor é morto pelo Android

Sintomas: o supervisor reinicia várias vezes, ou tudo para quando a tela apaga.

- verifique a economia de bateria do Termux;
- mantenha o carregador conectado;
- ative o wake lock;
- evite “limpar tudo” em otimizadores de memória;
- em fabricantes agressivos, trave o app recente (ainda assim não há garantia).

### Relógio errado e TOTP

O TOTP depende do relógio. Se o horário do aparelho estiver errado, códigos válidos serão recusados. Ative **hora automática** no Android e confira:

```bash
date -u
```

### Supervisor não sobe

```bash
cat ~/.pocketserve/watchdog.log
cat ~/.pocketserve/watchdog.pid
```

Um `watchdog.stop` residual impede o laço. `pocketserve watchdog` remove esse arquivo antes de subir. Se o PID existir mas o processo não, remova o PID manualmente:

```bash
rm -f ~/.pocketserve/watchdog.pid
pocketserve watchdog
```

## Manutenção programada

| Intervalo | Ação |
|---|---|
| Semanal | conferir `pocketserve status`, espaço livre e atividade |
| Mensal | testar restauração de um arquivo pela interface |
| Trimestral | copiar os dados para fora do aparelho e testar o restore |
| A cada atualização | guardar binário e estado antes de trocar |
| Quando o cartão encher | revisar lixeira, snapshots e duplicados |

## Parada segura

```bash
pocketserve stop
```

O servidor recebe `SIGTERM`, para de aceitar conexões e tenta concluir requisições em até 6 segundos. Escritas atômicas (config, snapshots, shares) não ficam pela metade. Ainda assim, para desligar o aparelho:

```bash
pocketserve stop
termux-wake-unlock
```

Não é necessário parar o servidor para recarregar a bateria ou reiniciar o Android; o boot script e o supervisor cuidam disso. Parar antes evita que uma gravação seja interrompida no desligamento.

## Checklist de operação

- [ ] Termux com armazenamento e wake lock liberados;
- [ ] supervisor ativo (`pocketserve status`);
- [ ] boot automático testado;
- [ ] porta e raiz corretas no `config.json`;
- [ ] espaço livre acima da reserva;
- [ ] backup diário decidido (ligado ou não);
- [ ] cópia externa periódica;
- [ ] log e atividade revisados;
- [ ] atualização com binário e estado guardados.
