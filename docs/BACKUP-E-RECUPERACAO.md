# Backup e recuperação

O que o PocketServe protege, o que ele **não** protege e como recuperar cada tipo de perda.

## Quatro mecanismos diferentes

| Mecanismo | Onde fica | Protege contra | Não protege contra |
|---|---|---|---|
| **Lixeira** | `<volume>/.pocketserve-trash/` | exclusão acidental | edição, corrupção, perda do volume |
| **Snapshots** | `<raiz principal>/.pocketserve-backups/` | sobrescrita, exclusão, edição | falha física do aparelho/cartão |
| **Versões por arquivo** | lê os snapshots | recuperar uma versão específica | nada além do que está nos snapshots |
| **Cópia externa** | fora do aparelho | roubo, queima, cartão morto, enchente | erro humano na cópia |

O backup embutido é **local**. Ele fica no mesmo aparelho e, na prática, no mesmo cartão ou armazenamento interno. Se o aparelho for perdido ou o cartão morrer, os snapshots vão junto. Uma cópia externa periódica é obrigatória para recuperação de desastre.

## Como o snapshot funciona

O snapshot é incremental e **content-addressed**:

```text
<raiz principal>/.pocketserve-backups/arquivos/
├── snapshots/
│   ├── 20261108-103811.123456789.json
│   └── ...
├── packs/
│   ├── <sha256>.pack
│   └── ...
└── index.json
```

- cada snapshot é um **manifesto JSON** com `caminho → tamanho, sha256, permissão`;
- o conteúdo de cada arquivo é gravado uma única vez em `packs/<sha256>.pack`;
- um pack é um `.tar.gz` com **um** arquivo dentro;
- se dois snapshots têm o mesmo conteúdo, o pack é reutilizado;
- o nome do pack é o SHA-256 do conteúdo, então a integridade é verificável;
- a retenção é de **14 snapshots**; os mais antigos são podados;
- ao podar, packs que nenhum snapshot restante referencia são removidos.

### O que entra no backup

- arquivos regulares de **todas as raízes ativas**;
- caminhos com apelido para volumes extras (`cartao/04_MIDIA/aula.mp4`);
- arquivos de até **512 MB**.

### O que fica de fora

| Item | Motivo |
|---|---|
| arquivos maiores que 512 MB | limite fixo `backupMaxFileMB`; aparecem em `missing` no manifesto |
| `.pocketserve-trash` | é lixeira, não conteúdo ativo |
| `.pocketserve-backups` | o backup não se copia |
| `.pocketserve-tmp` | temporários de upload |
| symlinks | não são seguidos |
| diretórios vazios | não há entrada para pasta vazia |
| metadados como dono/ACL/xattr | não são preservados; a permissão é registrada, mas a restauração aplica `0644` |
| Android/data e Android/obb | bloqueados pelo Android, nem entram na varredura |

O campo `missing` do manifesto é usado para listar os arquivos **ignorados por tamanho**, não arquivos que sumiram.

## Quando o backup roda

- **Manual**: painel → **Configurações → Backups → Rodar agora**, ou `POST /api/backups/run`.
- **Diário**: ligando **Backup automático** em Configurações. A cada 30 minutos o servidor verifica; se o último snapshot tem mais de 23 horas, roda um novo.
- **Não há agendamento externo**: se o servidor estiver desligado, o snapshot não acontece.

O snapshot pode demorar. Ele lê e faz hash de todos os arquivos; arquivos recém-modificados não usam cache. Em um cartão grande e lento, isso consome I/O e bateria. Rode manualmente em um horário em que o aparelho esteja carregando.

## Acompanhar e verificar

Pela interface:

1. **Configurações → Backups**;
2. confira **snapshots**, **packs**, **tamanho dos packs**, **retenção** e o último snapshot;
3. cada linha permite **baixar** um arquivo ou **excluir** o snapshot.

Pela API:

```bash
curl -s -b cookies.txt http://127.0.0.1:8080/api/backups | python3 -m json.tool
```

Resposta resumida:

```json
{
  "stats": {
    "snapshots": 3,
    "packs": 120,
    "packs_size": 2345678901,
    "retention": 14,
    "dir": "/sdcard/.pocketserve-backups/arquivos",
    "last_id": "20261108-103811.123456789",
    "last_at": "2026-11-08T10:38:11-03:00",
    "last_files": 118,
    "last_size": 4567890123
  },
  "snapshots": [
    {"id": "...", "at": "...", "files": 118, "size": 4567890123, "note": "manual"}
  ]
}
```

Baixar um arquivo de um snapshot:

```bash
curl -b cookies.txt -o recuperado.txt \
  "http://127.0.0.1:8080/api/backups/file?snapshot=ID&path=docs/relatorio.txt"
```

O servidor extrai o pack, confere o SHA-256 e envia o conteúdo. Se o pack estiver corrompido, a resposta é erro — ele não entrega bytes errados.

## Recuperar um arquivo pela interface

1. **Arquivos** → menu **⋮** do arquivo → **Versões (backup)**.
2. A lista mostra cada versão distinta, com data, tamanho, snapshot e se é recuperável.
3. **Baixar** salva aquela versão sem tocar na atual.
4. **Restaurar**:
   - se a lixeira está ligada, a versão atual vai para a lixeira;
   - se a lixeira está desligada, a versão atual é **sobrescrita**;
   - o conteúdo restaurado é extraído para um temporário, verificado por SHA-256 e renomeado de forma atômica;
   - a permissão final é `0644`.

Se o arquivo não aparece em nenhum snapshot, o painel avisa: ele pode ter sido criado depois do último backup.

## Recuperar manualmente um arquivo

Útil quando o painel não está disponível. Primeiro descubra o hash no manifesto:

```bash
python3 - <<'PY'
import glob, json
alvo = "docs/relatorio.txt"   # caminho exibido no painel
for arq in sorted(glob.glob("/sdcard/.pocketserve-backups/arquivos/snapshots/*.json"), reverse=True):
    s = json.load(open(arq))
    for f in s.get("files", []):
        if f["path"] == alvo:
            print("snapshot:", s["id"])
            print("sha256:  ", f["sha"])
            print("tamanho: ", f["size"])
            raise SystemExit
print("não encontrado")
PY
```

Depois extraia o pack (o `.pack` é um tar.gz com um único arquivo):

```bash
PACK=/sdcard/.pocketserve-backups/arquivos/packs/SHA256_AQUI.pack
tar -xOzf "$PACK" > /sdcard/recuperado.txt
sha256sum /sdcard/recuperado.txt
```

O hash impresso precisa ser igual ao nome do pack. Se não for, o pack está corrompido; tente um snapshot anterior.

## Restaurar um snapshot inteiro para uma pasta de preparo

Não existe botão de “restaurar tudo” na interface. O script `deploy/restore-snapshot.sh` extrai o snapshot para uma pasta separada, sem tocar nos arquivos atuais:

```bash
bash deploy/restore-snapshot.sh \
  /sdcard/.pocketserve-backups/arquivos/snapshots/20261108-103811.123456789.json \
  /sdcard/restaurado
```

Saída esperada:

```text
snapshot 20261108-103811.123456789: 118 arquivo(s) restaurado(s), 0 falha(s)
destino: /sdcard/restaurado
```

O script:

- valida cada caminho e recusa `..` ou caminho absoluto;
- extrai para `PASTA/…`, preservando o caminho exibido (inclusive `cartao/...`);
- verifica o SHA-256 de cada arquivo antes de renomear;
- não sobrescreve nada fora da pasta de destino;
- retorna código diferente de zero se algum pack faltar ou estiver corrompido.

Depois de conferir o conteúdo, mova manualmente para os lugares definitivos. Para os arquivos de uma raiz extra, o prefixo `cartao/` faz parte do caminho de preparo e **não** deve ser copiado para dentro da raiz.

## Backup e restauração do estado

Os snapshots cobrem **arquivos do usuário**. O estado do aplicativo fica em `~/.pocketserve`:

```text
~/.pocketserve/
├── config.json            senha (hash), segredos, ajustes
├── shares.json            links compartilhados
├── tokens.json            tokens de API (hash)
├── revoked_sessions.json  sessões revogadas
├── activity.jsonl         histórico local
└── server.log             log
```

Os aparelhos confiáveis e os códigos de recuperação ficam dentro do próprio `config.json` (`trusted_devices` e `recovery_codes`), não em arquivo separado.

### Guardar

```bash
bash deploy/backup-estado.sh
```

O script grava um tar.gz em `/sdcard/Download/PocketServe` e mostra o SHA-256. Ele inclui a pasta inteira, inclusive snapshots se estiverem dentro dela. **Não** há criptografia: o tar.gz contém o hash da senha e os segredos de sessão. Trate-o como credencial.

### Restaurar

Com o servidor parado:

```bash
pocketserve stop
mv ~/.pocketserve ~/.pocketserve.antes
mkdir -p ~/.pocketserve
tar -xzf /sdcard/Download/PocketServe/pocketserve-estado-AAAAMMDD-HHMMSS.tar.gz \
  -C "$HOME" --strip-components=1
chmod 700 ~/.pocketserve
chmod 600 ~/.pocketserve/*.json ~/.pocketserve/*.jsonl 2>/dev/null || true
pocketserve start
```

Confira a versão e o login. Se o tar.gz foi criado por outra versão, o `config.json` pode precisar de migração; restaure o binário antigo junto com o estado.

## Cópia externa

O snapshot local não é backup de desastre. Opções, da mais simples para a mais robusta:

| Método | Prós | Contras |
|---|---|---|
| copiar pelo painel (ZIP) | simples, seletivo | manual, não versiona |
| Syncthing | contínuo, sem nuvem | sincroniza também exclusões; configure versionamento |
| `adb pull /sdcard /pasta` | cópia fiel pelo cabo | precisa de PC e depuração USB |
| MTP/USB | sem instalar nada | lento, pode falhar com muitos arquivos |
| nuvem (rclone etc.) | fora do local | consome dados, requer configuração |

Regra prática **3-2-1**: três cópias, em dois meios, uma fora do aparelho. No mínimo, uma cópia semanal em um PC ou HD externo.

A cópia externa deve incluir:

- os arquivos que você não pode perder;
- `~/.pocketserve/config.json` (ou o tar.gz de estado);
- `shares.json` e `tokens.json` se quiser preservar links e tokens;
- opcionalmente `.pocketserve-backups`.

## Matriz de desastres

| Situação | O que fazer |
|---|---|
| Arquivo excluído pelo painel | **Lixeira** → restaurar (se `trash_days > 0`) |
| Arquivo sobrescrito/editado | **Versões (backup)** → baixar/restaurar |
| Arquivo corrompido | comparar versões, baixar a boa, restaurar |
| Snapshot corrompido | usar um snapshot anterior; packs são verificados por hash |
| Cartão removido sem ejeção | parar, reinserir, verificar `fsck` no PC se necessário |
| Cartão morto | restaurar da cópia externa; o snapshot local morreu junto |
| Aparelho perdido/roubado | copiar de backup externo; revogar acessos em outro dispositivo |
| Senha esquecida | `-reset-access` no Termux (exige acesso físico) |
| 2FA perdido | código de recuperação; senão `-reset-access` |
| Configuração quebrada | restaurar `config.json` do tar.gz de estado |
| Atualização ruim | voltar o binário guardado; restaurar estado se necessário |
| Espaço esgotado | esvaziar lixeira, reduzir retenção, apagar snapshots antigos, liberar arquivos |

## Teste de restauração

Faça um ensaio, não confie no backup sem testar:

1. escolha um arquivo pequeno e conhecido;
2. rode um snapshot manual;
3. edite o arquivo (ou exclua e restaure da lixeira);
4. abra **Versões (backup)** e restaure a versão anterior;
5. confira o conteúdo e o SHA-256;
6. repita o processo com `deploy/restore-snapshot.sh` em uma pasta de preparo;
7. guarde a data e o resultado do teste.

Se o teste falhar, corrija antes de precisar do backup de verdade.

## Limitações conhecidas

- backup **local**, no mesmo aparelho;
- retenção fixa de **14 snapshots**;
- arquivos acima de **512 MB** ficam fora;
- não há restauração integral pela interface;
- o script de restauração integral é auxiliar e restaura para uma pasta de preparo, não para as raízes ao vivo;
- permissões e timestamps não são restaurados fielmente (aplica `0644`);
- não há criptografia nem compactação adicional além do gzip dos packs;
- não há verificação automática de todos os packs; a verificação ocorre no download/restauração;
- snapshots automáticos dependem do servidor ligado;
- a lixeira e os snapshots podem ser apagados por qualquer pessoa com acesso ao Termux ou ao armazenamento.
