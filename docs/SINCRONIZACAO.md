# Sincronização

O PocketServe **não tem sincronização embutida**. Ele é um servidor de arquivos: mostra, envia, baixa, edita e compartilha. Manter duas cópias iguais é responsabilidade de outra ferramenta.

Este documento cobre as opções reais e os cuidados de cada uma.

## O que existe e o que não existe

| Recurso | Situação |
|---|---|
| upload/download pelo painel | implementado |
| edição de texto | implementado |
| links de compartilhamento | implementado |
| WebDAV | **não implementado** |
| aplicativo móvel nativo | **não implementado** |
| cliente de sincronização próprio | **não implementado** |
| Syncthing | opcional, instalado à parte |
| rsync/scp | via Termux, manual |
| nuvem | via rclone ou app de terceiros |

Não há resolução de conflitos, nem detecção de “qual lado está mais novo”. Duas ferramentas escrevendo no mesmo arquivo sempre podem gerar conflito.

## Syncthing

Syncthing é a opção mais próxima de “sincronização contínua” no Android. Ele usa TLS, não depende de nuvem e funciona entre celular, PC e outros aparelhos.

### Instalar

```bash
pkg install syncthing
```

Rode o instalador do PocketServe novamente para instalar o supervisor:

```bash
bash /sdcard/Download/PocketServe/install.sh
```

O instalador detecta o Syncthing e copia `syncthing-rc.sh` para `~/.pocketserve-sync/`. Se preferir, copie manualmente:

```bash
mkdir -p ~/.pocketserve-sync
cp /sdcard/Download/PocketServe/syncthing-rc.sh ~/.pocketserve-sync/
chmod 700 ~/.pocketserve-sync/syncthing-rc.sh
```

### Controlar

```bash
pocketserve sync start
pocketserve sync status
pocketserve sync log
pocketserve sync stop
pocketserve sync restart
pocketserve sync supervisor
pocketserve sync unsupervisor
pocketserve sync gui
```

O `pocketserve start` também tenta iniciar o supervisor do Syncthing, mas não falha se ele não estiver instalado.

Configuração usada pelo script:

| Item | Valor |
|---|---|
| pasta de configuração | `~/.syncthing-rc` |
| interface web | `http://127.0.0.1:8384` |
| porta de sincronização | `22000` |
| log | `~/.pocketserve-sync/syncthing.log` |
| supervisor | `~/.pocketserve-sync/supervisor.pid` |

As variáveis `SYNCTHING_HOME`, `SYNCTHING_GUI_PORT` e `SYNCTHING_PORT` podem mudar esses valores.

### Abrir a interface no computador

A interface escuta **somente no loopback**. Não a exponha na rede. Use um túnel SSH:

```bash
# no celular
pkg install openssh
sshd

# no computador
ssh -L 8384:127.0.0.1:8384 -p 8022 SEU_USUARIO@SEU_ENDERECO_IP
```

Depois abra `http://127.0.0.1:8384` no computador. A porta padrão do `sshd` do Termux é `8022`; confirme com `sshd` e `whoami`.

Alternativa: use o Tailscale e acesse `http://IP_TAILSCALE:8384` **somente** se você mudou o bind para a interface do Tailscale e entende o risco.

### Configurar as pastas

No Syncthing do celular:

1. **Add Folder**;
2. escolha uma pasta de conteúdo, por exemplo `/sdcard/PocketServe` ou `/sdcard/Documentos`;
3. **não** escolha `~/.pocketserve`, `~/.syncthing-rc`, `.pocketserve-trash` nem `.pocketserve-backups`;
4. compartilhe a pasta com o outro aparelho;
5. no PC, aceite e escolha uma pasta local.

Pastas que **nunca** devem ser sincronizadas:

```text
~/.pocketserve/                 estado, senha, tokens, sessões
<raiz>/.pocketserve-trash/      lixeira
<raiz>/.pocketserve-backups/    snapshots content-addressed
<raiz>/.pocketserve-tmp/        temporários
```

Sincronizar o estado pode corromper a configuração entre versões. Sincronizar a lixeira ou os snapshots duplica lixo e confunde a retenção.

### Padrões de exclusão

No Syncthing, em **Folder → Ignore Patterns**:

```text
.pocketserve-trash
.pocketserve-backups
.pocketserve-tmp
.sync-conflict-*
```

Os arquivos `.sync-conflict-*` são criados pelo próprio Syncthing quando os dois lados editam o mesmo arquivo. Eles **não** são resolvidos automaticamente: abra, compare e escolha a versão correta.

### Sincronização não é backup

Syncthing propaga **exclusões**. Se você apagar uma pasta no celular, ela some no PC também. Para usar como backup:

- ative **File Versioning** no PC (Staggered ou Trash Can);
- mantenha o PC como cópia independente, não como espelho cego;
- faça snapshots locais no PocketServe;
- teste a restauração.

Uma topologia segura:

```text
celular (PocketServe)  →  PC (Syncthing receive-only + versionamento)
                       →  HD externo (cópia periódica)
```

### Consumo e bateria

- a sincronização inicial de muitos GB aquece o aparelho e consome bateria;
- deixe o celular carregando e na Wi-Fi;
- limite a sincronização a Wi-Fi, se possível;
- não sincronize a pasta de downloads inteira sem necessidade;
- monitore a temperatura e o espaço livre.

## rsync e scp

Para cópias pontuais ou agendadas, use o SSH do Termux.

### No celular

```bash
pkg install openssh rsync
sshd
whoami          # usuário do Termux
passwd          # defina uma senha se ainda não tiver
```

### No computador

```bash
rsync -av --progress \
  -e "ssh -p 8022" \
  SEU_USUARIO@SEU_ENDERECO_IP:/sdcard/PocketServe/ \
  ./PocketServe/
```

Cuidados:

- **sem** `--delete` na primeira cópia;
- com `--delete`, o comando vira espelho e propaga exclusões;
- exclua as pastas internas:

```bash
rsync -av --progress \
  --exclude '.pocketserve-trash/' \
  --exclude '.pocketserve-backups/' \
  --exclude '.pocketserve-tmp/' \
  -e "ssh -p 8022" \
  SEU_USUARIO@SEU_ENDERECO_IP:/sdcard/PocketServe/ \
  ./PocketServe/
```

O `sshd` do Termux escuta em `8022` por padrão. Não exponha essa porta na internet; use Tailscale ou a LAN.

## adb

Com depuração USB ativada:

```bash
adb devices
adb pull /sdcard/PocketServe ./PocketServe
```

Para enviar:

```bash
adb push ./PocketServe /sdcard/
```

O `adb` não preserva todos os metadados e pode ser lento com muitos arquivos, mas é uma cópia fiel o suficiente para recuperação.

## rclone e nuvem

```bash
pkg install rclone
rclone config
rclone copy /sdcard/PocketServe remote:PocketServe --progress
```

`rclone copy` não apaga o destino. `rclone sync` apaga — use com cautela. Para backup, prefira `copy` com versionamento no destino.

## Comparação

| Método | Contínuo | Bidirecional | Lida com conflito | Custo |
|---|---|---|---|---|
| Syncthing | sim | sim | cria `.sync-conflict-*` | app extra |
| rsync | não | não | sobrescreve conforme flags | manual/script |
| adb | não | não | sobrescreve | cabo USB |
| rclone | não | não | sobrescreve conforme flags | nuvem |
| painel (upload/download) | não | não | manual | nenhum |

## Regras práticas

1. **Não sincronize o estado do aplicativo.** `~/.pocketserve` é configuração, não conteúdo.
2. **Não sincronize a lixeira nem os snapshots.** Eles têm estrutura própria.
3. **Trate conflito como normal.** Duas cópias editáveis vão divergir.
4. **Tenha uma cópia que não seja espelho.** Um backup que recebe exclusões não é backup.
5. **Teste a restauração.** Sincronizar não prova que o arquivo abre.
6. **Cuidado com o espaço.** Sincronizar 100 GB para um cartão de 128 GB não cabe.
7. **Cuidado com a bateria.** Sincronização contínua e servidor ligado o tempo todo aquecem.

## Limitações

- não há cliente nativo do PocketServe;
- não há WebDAV;
- não há sincronização embutida;
- o Syncthing é um projeto separado, com configuração e riscos próprios;
- o script de supervisor do Syncthing não é um serviço do Android;
- conflitos não são resolvidos pelo PocketServe;
- a sincronização não substitui backup externo.
