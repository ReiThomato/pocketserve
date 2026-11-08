# API HTTP

Referência dos endpoints reais do PocketServe 1.0.0. Todos os caminhos são relativos à base (`http://127.0.0.1:8080` por padrão).

## Convenções

- Respostas JSON usam `Content-Type: application/json; charset=utf-8`.
- Erros seguem o formato:

```json
{"error": "Mensagem para o usuário.", "code": "codigo_estavel"}
```

- Códigos de status usados: `200`, `303` (redirecionamento de share), `400`, `401`, `403`, `404`, `409`, `413`, `428`, `429`, `500`, `507`.
- `401`: sem credencial, sessão expirada/revogada, senha ou código incorretos.
- `403`: CSRF inválido, token sem escopo, permissão negada ou raiz protegida.
- `428 setup_required`: a senha do administrador ainda não foi criada.
- `507 no_space`: a escrita furaria a reserva de disco.
- Datas são RFC 3339 (`2026-11-08T10:38:11-03:00`), salvo indicação contrária.
- Tamanho máximo do corpo JSON: **1 MiB** (login: 4 KiB; setup: 8 KiB).
- Uploads são `multipart/form-data` e não passam pelo limite de 1 MiB.

## Autenticação

### Sessão (painel)

1. `POST /api/setup` (primeiro acesso) ou `POST /api/login`;
2. o servidor devolve o cookie `ps_session` (`HttpOnly`, `SameSite=Lax`);
3. toda mutação (`POST`, `PUT`, `PATCH`, `DELETE`) exige o cabeçalho `X-PS: 1`;
4. se houver `Origin`, ele precisa bater com o `Host`.

O cookie é assinado com HMAC-SHA256 e carrega expiração, versão da senha e um ID de sessão. Logout revoga o ID no servidor; o cookie antigo passa a devolver `401`.

### Token de API

```http
Authorization: Bearer ps_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

- não exige `X-PS` (o navegador não anexa esse cabeçalho sozinho);
- tem escopos;
- é exibido **uma única vez** na criação;
- o servidor guarda apenas o SHA-256;
- `admin` é superset dos escopos de arquivos e shares, mas **não** dá acesso a configurações, sistema, log, reinício, TOTP, aparelhos confiáveis ou outros tokens.

| Escopo | Permite |
|---|---|
| `files:read` | listar, baixar, visualizar, buscar, thumbnail, checksum, versões |
| `files:write` | upload, mkdir, rename, mover, copiar, salvar texto, extrair ZIP |
| `files:delete` | excluir, limpar duplicados, restaurar versão |
| `shares:manage` | listar, criar e revogar links |
| `admin` | tudo que os escopos acima permitem |

### Descobrir a credencial atual

```bash
curl -s -b cookies.txt http://127.0.0.1:8080/api/whoami
```

```json
{"auth":"session","scopes":["files:read","files:write","files:delete","shares:manage","admin"]}
```

Com token:

```json
{"auth":"token","name":"backup-pc","scopes":["files:read"],"expires_at":"2026-12-08T10:00:00Z"}
```

## Endpoints públicos

### `GET /healthz`

Sem autenticação, sem dados sensíveis.

```json
{"ok":true,"service":"pocketserve","version":"1.0.0"}
```

### `GET /api/state`

```json
{"needs_setup":false,"authenticated":true,"name":"PocketServe","version":"1.0.0"}
```

### `POST /api/setup`

Cria a senha do administrador. Só funciona enquanto `password_hash` está vazio.

Cabeçalhos: `Content-Type: application/json`, `X-PS: 1`.

```json
{"password":"senha-com-8-ou-mais","server_name":"PocketServe"}
```

Resposta `200`:

```json
{"ok":true,"name":"PocketServe"}
```

Erros: `403 already_setup`, `400 weak_password`, `400 bad_name`.

### `POST /api/login`

Cabeçalhos: `Content-Type: application/json`, `X-PS: 1`.

```json
{"password":"sua-senha","code":"123456","remember":true}
```

- `code` só é necessário com 2FA ativo e aparelho não confiável;
- `remember: true` grava o cookie de aparelho confiável por 30 dias.

Respostas:

```json
{"ok":true}
```

```json
{"error":"Código de verificação necessário.","code":"totp_required"}
```

Erros: `401 invalid_password`, `401 bad_code`, `429` com `Retry-After`.

### `POST /api/logout`

Cabeçalho `X-PS: 1`. Revoga a sessão no servidor e limpa o cookie.

```json
{"ok":true}
```

### `GET /api/whoami`

Aceita sessão ou token. `200` com a descrição; `401` sem credencial.

## Arquivos

### `GET /api/files`

Parâmetros:

| Nome | Padrão | Descrição |
|---|---|---|
| `path` | raiz | pasta a listar (vazio = raiz) |
| `hidden` | `0` | `1` mostra arquivos e pastas ocultos |

Resposta `200`:

```json
{
  "path": "docs",
  "roots": [{"name":"principal","path":"/sdcard","free":202000000000}],
  "root_label": "Interno",
  "primary_free": 202000000000,
  "trash_days": 30,
  "parent": "",
  "entries": [
    {
      "name": "a.txt",
      "path": "docs/a.txt",
      "is_dir": false,
      "size": 19,
      "mtime": "2026-11-08T10:00:00-03:00",
      "kind": "text",
      "mime": "text/plain; charset=utf-8"
    }
  ],
  "writable": true
}
```

`kind` é uma classificação para a interface: `folder`, `image`, `video`, `audio`, `pdf`, `archive`, `text`, `doc`, `sheet`, `slides` ou `file`. `writable` é testado criando um arquivo temporário.

Erros: `404 not_found`, `400 not_dir`, `403 permission` (ex.: `Android/data`).

### `GET /api/raw`

Serve o arquivo inline. Query: `path`. Ideal para imagens, PDF e vídeo. Respeita `Range`.

### `GET /api/download`

Serve o arquivo como anexo. Query: `path`. Se `path` for uma pasta, gera um ZIP.

### `GET /api/zip`

Gera um ZIP em streaming. Aceita `path` repetido:

```bash
curl -b cookies.txt -o selecao.zip \
  "http://127.0.0.1:8080/api/zip?path=docs&path=fotos"
```

Sem `path`: `400` em texto puro.

### `GET /api/text`

Lê até **2 MiB** para o editor.

```json
{"path":"docs/a.txt","content":"...","size":19,"truncated":false,"binary":false}
```

Arquivos binários vêm com `binary: true` e `content: ""`. Acima de 2 MiB, `truncated: true`.

### `GET /api/thumb`

Gera miniatura de imagem. Query: `path`. Resposta é imagem; `404` para não-imagem ou falha.

### `GET /api/search`

| Nome | Descrição |
|---|---|
| `q` | termo, mínimo **2 caracteres** |
| `path` | pasta inicial; vazio = raiz |
| `all` | `1` força varrer todos os volumes |

Limites: até **400 resultados** e **6 segundos**. Resposta:

```json
{"entries":[...],"query":"au"}
```

`400 short_query` para menos de 2 caracteres. Se o cliente desconectar, a varredura para.

### `GET /api/checksum`

Calcula SHA-256. Query:

| Nome | Descrição |
|---|---|
| `path` | arquivo |
| `expect` | hash esperado, opcional |

```json
{"path":"docs/a.txt","size":19,"sha256":"...","match":true,"millis":1}
```

`match` só aparece quando `expect` é enviado. `400 not_file`, `404`, `403 permission`.

### `POST /api/upload`

Query:

| Nome | Descrição |
|---|---|
| `path` | pasta de destino; vazio = raiz |
| `overwrite` | `1` sobrescreve; padrão cria `nome (1)` |

Corpo `multipart/form-data` com um ou mais campos `file`. Vários arquivos podem ir na mesma requisição.

```bash
curl -b cookies.txt -H 'X-PS: 1' \
  -F "file=@relatorio.pdf" \
  -F "file=@foto.jpg" \
  "http://127.0.0.1:8080/api/upload?path=docs"
```

Resposta:

```json
{"ok":true,"files":[...],"bytes":123456}
```

O arquivo é gravado em temporário no mesmo diretório e renomeado; a reserva é conferida antes, entre arquivos e depois da escrita. Erros: `413 too_large`, `507 no_space`, `400 empty`, `400 bad_path`.

### `POST /api/mkdir`

```json
{"path":"docs","name":"nova pasta"}
```

`200`:

```json
{"ok":true,"path":"docs/nova pasta"}
```

Erros: `409 exists`, `400 bad_path`.

### `POST /api/rename`

```json
{"path":"docs/a.txt","name":"b.txt"}
```

`200`:

```json
{"ok":true,"path":"docs/b.txt"}
```

Não renomeia a raiz (`403 forbidden`). `404`, `409`, `400`.

### `POST /api/delete`

```json
{"paths":["docs/a.txt","fotos/antiga"]}
```

`200`:

```json
{"ok":true,"deleted":2,"trashed":2}
```

Com lixeira ligada, os itens vão para `.pocketserve-trash` do volume. Com `trash_days = 0`, são removidos definitivamente. Itens que não existem ou são a raiz são ignorados.

### `POST /api/move`

```json
{"paths":["docs/a.txt"],"dest":"backup"}
```

`200`:

```json
{"ok":true,"moved":1}
```

Entre volumes diferentes, copia e só remove a origem após a cópia completa. Não move uma pasta para dentro dela mesma.

### `POST /api/copy`

Mesmo corpo de `move`. Resposta:

```json
{"ok":true,"copied":1}
```

### `POST /api/save`

Salva texto no editor.

```json
{"path":"docs/nota.txt","content":"linha 1\nlinha 2\n"}
```

Limite: **5 MiB**. Grava em temporário e renomeia. Resposta `{"ok":true}`. Erros: `413 too_large`, `507 no_space`, `400 bad_path`.

### `POST /api/unzip`

```json
{"path":"docs/arquivo.zip"}
```

Extrai na mesma pasta do ZIP. Limites: **20 GiB** descompactados e **20.000 arquivos**. Caminhos internos com `..` são normalizados e contidos no destino. Resposta:

```json
{"ok":true,"files":123}
```

Erros: `400 bad_zip`, `507 no_space`.

## Compartilhamentos

### `GET /api/shares`

```json
{
  "shares": [
    {
      "token": "ABC...",
      "path": "docs",
      "name": "docs",
      "is_dir": true,
      "size": 4096,
      "created": "2026-11-08T10:00:00-03:00",
      "expires_at": "2026-11-09T10:00:00-03:00",
      "downloads": 3,
      "last_access": "2026-11-08T11:00:00-03:00",
      "has_password": true
    }
  ]
}
```

O hash da senha **nunca** é devolvido. `has_password` indica se o link exige senha.

### `POST /api/shares`

```json
{"path":"docs","expires_hours":24,"password":"opcional"}
```

- `expires_hours`: 0 a 8760 (0 = sem validade);
- `password`: mínimo 4 caracteres; vazio = link aberto.

Resposta:

```json
{"ok":true,"share":{...},"url":"/s/TOKEN"}
```

### `DELETE /api/shares/{token}`

Revoga imediatamente.

```json
{"ok":true}
```

### Páginas públicas

| Rota | O que faz |
|---|---|
| `GET /s/{token}` | página somente leitura; pasta navegável |
| `POST /s/{token}/unlock` | formulário `password`; grava cookie de desbloqueio |
| `GET /s/{token}/raw` | arquivo inline |
| `GET /s/{token}/zip` | pasta em ZIP |

O cookie de desbloqueio é assinado com `session_secret`, vale 24 horas e é limitado ao caminho `/s/{token}`. Link expirado ou revogado devolve `404`.

## Lixeira

### `GET /api/trash`

```json
{
  "items": [
    {
      "id": "20261108-103811-ab12cd34",
      "orig_path": "docs/a.txt",
      "name": "a.txt",
      "deleted_at": "2026-11-08T10:38:11-03:00",
      "size": 19,
      "is_dir": false,
      "root": "/sdcard"
    }
  ],
  "days": 30
}
```

### `POST /api/trash/restore`

```json
{"id":"20261108-103811-ab12cd34"}
```

Restaura no caminho original. Se o nome já existir, cria `nome (1)`. Resposta:

```json
{"ok":true,"path":"docs/a.txt"}
```

### `POST /api/trash/delete`

```json
{"id":"..."}
```

Apaga definitivamente um item.

### `POST /api/trash/empty`

Sem corpo. Esvazia todas as lixeiras.

```json
{"ok":true,"deleted":12}
```

## Backups

### `GET /api/backups`

```json
{
  "stats": {
    "snapshots": 3,
    "packs": 120,
    "packs_size": 2345678901,
    "retention": 14,
    "dir": "/sdcard/.pocketserve-backups/arquivos",
    "last_id": "...",
    "last_at": "...",
    "last_files": 118,
    "last_size": 4567890123
  },
  "snapshots": [
    {"id":"...","at":"...","files":118,"size":4567890123,"note":"manual"}
  ]
}
```

### `POST /api/backups/run`

```json
{"note":"antes de atualizar"}
```

Corpo vazio também é aceito. Resposta:

```json
{"ok":true,"id":"...","files":118,"added":4,"size":4567890123}
```

### `GET /api/backups/file`

Query: `snapshot`, `path`. Baixa o arquivo daquele snapshot após verificar o SHA-256.

### `POST /api/backups/delete`

```json
{"id":"20261108-103811.123456789"}
```

Remove o manifesto e poda packs órfãos.

## Versões por arquivo

### `GET /api/versions`

Query: `path`.

```json
{
  "path": "docs/a.txt",
  "versions": [
    {
      "snapshot": "20261108-103811.123456789",
      "at": "2026-11-08 10:38",
      "sha": "...",
      "size": 19,
      "current": true,
      "recoverable": true
    }
  ],
  "distinct": 2,
  "note": ""
}
```

`current` marca a versão mais recente encontrada nos snapshots. `recoverable` indica se o pack existe.

### `GET /api/versions/download`

Query: `snapshot`, `path`. Baixa aquela versão.

### `POST /api/versions/restore`

```json
{"snapshot":"...","path":"docs/a.txt"}
```

- se a lixeira está ligada, a versão atual vai para a lixeira;
- se está desligada, a versão atual é sobrescrita;
- extrai para temporário, verifica SHA-256 e renomeia;
- aplica permissão `0644`.

Resposta:

```json
{"ok":true,"path":"docs/a.txt","snapshot":"..."}
```

## Duplicados

### `GET /api/duplicates`

Calcula a partir do **último snapshot**. Resposta:

```json
{
  "snapshot": "...",
  "at": "2026-11-08 10:38",
  "groups": [
    {
      "sha": "...",
      "size": 123456,
      "count": 2,
      "wasted": 123456,
      "files": [{"path":"docs/a.txt","size":123456,"root":"principal"}],
      "keep": "docs/a.txt"
    }
  ],
  "total_waste": 123456,
  "file_count": 2,
  "group_count": 1,
  "note": ""
}
```

Arquivos vazios são ignorados. `keep` é a cópia sugerida (caminho mais curto; empate alfabético).

### `POST /api/duplicates/cleanup`

```json
{"paths":["fotos/copia.jpg"]}
```

Move para a lixeira. Duas travas: nunca apaga a cópia sugerida e mantém ao menos uma cópia por grupo. Resposta:

```json
{"ok":true,"trashed":1,"skipped":0}
```

## Tokens de API

### `GET /api/tokens`

```json
{
  "tokens": [
    {"id":"...","name":"backup-pc","prefix":"ps_AbCdEf...","scopes":["files:read"],"created_at":"...","active":true}
  ],
  "scopes": ["files:read","files:write","files:delete","shares:manage","admin"]
}
```

### `POST /api/tokens`

```json
{"name":"backup-pc","scopes":["files:read"],"expires_days":90}
```

- nome: 1 a 60 caracteres;
- pelo menos um escopo;
- `expires_days`: 0 a 3650 (0 = nunca expira).

Resposta (a chave aparece **só aqui**):

```json
{"ok":true,"token":"ps_...","item":{...}}
```

### `POST /api/tokens/{id}/revoke`

Revoga sem apagar o registro.

### `DELETE /api/tokens/{id}`

Remove o registro.

## Configuração e sistema

### `GET /api/settings`

Devolve ajustes, raízes, volumes e espaço. Não devolve segredos.

```json
{
  "server_name": "PocketServe",
  "port": 8080,
  "root": "/sdcard",
  "session_days": 30,
  "tailscale_name": "",
  "max_upload_mb": 0,
  "min_free_mb": 2048,
  "trash_days": 30,
  "backup_daily": false,
  "version": "1.0.0",
  "home": "/data/data/com.termux/files/home/.pocketserve",
  "roots": [...],
  "volumes": [...],
  "space": [...]
}
```

### `POST /api/settings`

```json
{
  "server_name": "PocketServe",
  "port": 8080,
  "root": "/sdcard",
  "session_days": 30,
  "tailscale_name": "",
  "max_upload_mb": 0,
  "min_free_mb": 2048,
  "trash_days": 30,
  "backup_daily": false,
  "roots": [{"name":"cartao","path":"/storage/XXXX-XXXX/PocketServe"}]
}
```

Validações: nome 1–40; porta 1024–65535; raiz existente; sessão 1–365; reserva 0–1048576 MB; lixeira 0–3650; até 8 raízes extras. Resposta:

```json
{"ok":true,"restart_required":false}
```

`restart_required` é `true` quando porta ou raiz mudam; o servidor **não** reinicia sozinho. Use `POST /api/restart` ou reinicie pelo Termux.

### `POST /api/restart`

Responde `{"ok":true}` e, 450 ms depois, reexecuta o próprio binário (`syscall.Exec`). Só funciona se o processo tiver permissão e o supervisor estiver ativo para o caso de falha.

### `GET /api/system`

Informações do aparelho e do servidor:

```json
{
  "version":"1.0.0",
  "server_name":"PocketServe",
  "server_uptime":3600,
  "device_uptime":86400,
  "cores":8,
  "load1":0.4,
  "load5":0.3,
  "load15":0.2,
  "cpu_percent":12.5,
  "mem_total":8053063680,
  "mem_used":1234567890,
  "mem_free":6818495790,
  "mem_percent":15.3,
  "storage":{"total":...,"used":...,"free":...,"percent":...},
  "volumes":[...],
  "battery":{"available":true,"level":100,"status":"Carregando","temperature":35,"plugged":"USB"},
  "interfaces":[{"name":"wlan0","label":"Wi-Fi","ipv4":"SEU_ENDERECO_IP","tailscale":false,"reachable":true}],
  "urls":["http://SEU_ENDERECO_IP:8080"],
  "tailscale_name":"",
  "hostname":"localhost",
  "short_host":"",
  "port":8080
}
```

Campos podem vir zerados quando o Android bloqueia a leitura (`/proc/stat`, bateria etc.).

### `GET /api/activity`

Query `limit` (1–500, padrão 100), mais recentes primeiro.

```json
{"activity":[{"t":"...","ip":"SEU_ENDERECO_IP","action":"login","ok":true}]}
```

### `GET /api/log`

Últimas linhas de `server.log`, limitado aos **160 KiB** finais.

```json
{"log":"...","size":12345}
```

### `GET /api/qr`

Query `text` (até 1024 caracteres). Devolve PNG 420×420. `400` sem texto.

## Segurança e 2FA

### `GET /api/totp`

```json
{"enabled":false,"recovery_codes":0}
```

### `POST /api/totp/setup`

Gera segredo pendente (não ativa ainda).

```json
{"secret":"BASE32...","uri":"otpauth://totp/..."}
```

`409 already_enabled` se já estiver ativo.

### `GET /api/totp/qr`

Devolve o QR do segredo pendente. Query `secret` opcional para renderizar um segredo específico.

### `POST /api/totp/enable`

```json
{"code":"123456"}
```

Ativa e devolve os códigos de recuperação **em texto claro, uma única vez**:

```json
{"ok":true,"recovery_codes":["ABCD-EFGH","..."]}
```

### `POST /api/totp/disable`

```json
{"password":"sua-senha","code":"123456"}
```

Exige senha **e** código atual.

### `GET /api/trusted`

```json
{"devices":[{"label":"Chrome no PC","added_at":"...","expires":"..."}],"current":true}
```

### `POST /api/trusted/revoke`

Sem corpo. Revoga todos os aparelhos confiáveis.

### `POST /api/password`

```json
{"current":"senha-atual","new":"senha-nova-com-8+"}
```

Troca a senha, incrementa a versão e emite uma sessão nova. Cookies antigos passam a devolver `401`.

## Exemplos completos

### Login e upload com cookie

```bash
BASE=http://127.0.0.1:8080
curl -s -c cookies.txt -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"password":"sua-senha"}' "$BASE/api/login"

curl -s -b cookies.txt -H 'X-PS: 1' \
  -F "file=@./foto.jpg" "$BASE/api/upload?path=fotos"

curl -s -b cookies.txt "$BASE/api/files?path=fotos" | python3 -m json.tool
```

### Token somente leitura

```bash
TOKEN=ps_...
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/files?path="
curl -s -H "Authorization: Bearer $TOKEN" -o backup.zip "$BASE/api/zip?path=docs"
```

### Criar link com senha

```bash
curl -s -b cookies.txt -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"path":"docs","expires_hours":24,"password":"segredo"}' \
  "$BASE/api/shares"
```

### Rodar backup

```bash
curl -s -b cookies.txt -H 'X-PS: 1' -H 'Content-Type: application/json' \
  -d '{"note":"manual"}' "$BASE/api/backups/run"
```

## Limitações da API

- não há versionamento de API (`/v1`); a versão é a do servidor;
- não há paginação em listagens de arquivos;
- busca tem teto de 400 resultados e 6 segundos;
- não há WebDAV;
- não há endpoint para restaurar um snapshot inteiro;
- tokens não acessam configuração, sistema, log, TOTP nem outros tokens;
- não há CORS: a API é feita para o mesmo origin.
