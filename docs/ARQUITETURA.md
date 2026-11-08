# Arquitetura

Documento técnico do PocketServe: como o código está dividido, quais decisões foram tomadas e por quê.

## Visão geral

Um único processo Go serve a API JSON, as páginas públicas de compartilhamento e o frontend embutido. Não há banco de dados, não há build de frontend e não há chamadas externas em tempo de execução.

```
main.go        flags, inicialização, rotas, headers, HTTP/HTTPS, embed
├── auth.go    senha, sessão, CSRF, rate limit, IP real, login/setup/logout
├── sessions.go lista de revogação de sessões (persistente)
├── totp.go    TOTP RFC 6238, códigos de recuperação, QR
├── trust.go   aparelhos confiáveis ("lembrar por 30 dias")
├── files.go   listagem, upload, download, ZIP, busca, thumbs, editor
├── roots.go   múltiplas raízes, apelidos, resolução de caminho
├── shares.go  links públicos, senha, expiração, templates HTML
├── backup.go  snapshots content-addressed, deduplicação, retenção
├── versions.go histórico por arquivo nos snapshots
├── duplicates.go relatório e limpeza de duplicatas
├── trash.go   lixeira por volume, restauração, expiração
├── tokens.go  tokens de API com escopo, hash e revogação
├── guard.go   reserva de disco e alertas
├── checksum.go SHA-256 sob demanda
├── system.go  CPU, memória, disco, bateria, rede, QR, log
├── settings.go configuração persistida
├── config.go  estrutura e leitura/escrita do config.json
├── activity.go histórico local em JSONL
├── tls.go     HTTPS opcional e confiança em proxy local
├── ctime_*.go ctime por plataforma (cache do backup)
└── web/       frontend embutido via go:embed
```

## Fluxo de uma requisição

```
cliente
  │
  ▼
withHeaders        cabeçalhos de segurança, CSP, no-store em /api e /s
  │
  ▼
ServeMux           rota + método
  │
  ├─ pública       /healthz, /api/state, /api/login, /api/setup, /s/{token}
  │
  ├─ requireAuth   sessão + CSRF (para métodos mutáveis)
  │
  └─ authAny       sessão OU Authorization: Bearer <token> com escopo
        │
        ▼
     handler       valida entrada → resolve caminho → sistema de arquivos
        │
        ▼
     JSON / stream / HTML
```

### Autenticação em duas camadas

- **`requireAuth`** aceita somente cookie de sessão. É usado em configurações, backups, lixeira, tokens, TOTP e atividades — operações administrativas.
- **`authAny(scope, ...)`** aceita cookie de sessão **ou** token Bearer. É usado em arquivos, compartilhamentos, duplicatas e versões. Token tem precedência sobre cookie quando o cabeçalho `Authorization` está presente.
- Sessão concede todos os escopos (é o administrador humano). Token concede apenas o escopo declarado; `admin` é superset.

### CSRF

Toda mutação por cookie exige:

1. cabeçalho `X-PS: 1` (um formulário HTML cross-site não consegue enviar cabeçalho customizado);
2. se houver `Origin`, ele precisa bater com o `Host` da requisição.

Requisições autenticadas por Bearer não exigem CSRF: o navegador não anexa esse cabeçalho sozinho, então não existe requisição cross-site capaz de forjar a credencial.

## Armazenamento de estado

```
~/.pocketserve/                  (permissão 0700)
├── config.json                  0600 — configuração, hash da senha, segredos
├── shares.json                  0600 — links públicos
├── tokens.json                  0600 — tokens (somente hash)
├── activity.jsonl               0600 — últimas 500 atividades
├── revoked_sessions.json        0600 — hashes de IDs revogados
├── server.log                   log do processo
├── server.pid / watchdog.pid    controle do supervisor
├── pocketserve                  binário instalado
└── ctl.sh                       controle local
```

O estado **não** fica dentro da raiz de arquivos. A única exceção deliberada são os dados de backup e a lixeira, que precisam estar no mesmo volume dos arquivos:

```
<raiz>/.pocketserve-trash/<id>/
├── meta.json                    caminho original, volume, tamanho, data
└── data/                        conteúdo movido por rename

<raiz>/.pocketserve-backups/arquivos/
├── snapshots/<id>.json          manifesto (caminho → SHA-256)
├── packs/<sha256>.pack          conteúdo comprimido, compartilhado
├── index.json                   estado do último snapshot
└── tmp/                         extração temporária
```

Essas pastas são invisíveis na listagem, mesmo com "mostrar ocultos" ligado, e são excluídas do próprio backup.

## Decisões de projeto

### 1. Um binário, sem banco de dados

O alvo é um celular com Termux: menos partes móveis, menos memória, menos chance de corrupção. Arquivos JSON com escrita atômica (`tmp` + `rename`) resolvem o volume de dados de um usuário. O custo é não haver consultas concorrentes complexas — o que não é necessário aqui.

### 2. Sessão sem estado + lista de revogação

A sessão normal é um cookie assinado com HMAC (`payload.assinatura`), sem consulta a disco por requisição. Para o logout funcionar de verdade, cada sessão carrega um **ID aleatório** e o logout grava `sha256(ID)` em `revoked_sessions.json`. A verificação é um mapa em memória, O(1), e a lista expira junto com as sessões.

Sem o ID, dois logins no mesmo segundo gerariam o mesmo cookie e o logout de um derrubaria o outro. O ID elimina essa colisão.

### 3. Backup content-addressed

Cada snapshot é um manifesto `caminho → SHA-256`; o conteúdo vive em `packs/<sha>.pack`, compartilhado entre snapshots. Um arquivo idêntico em 14 snapshots ocupa espaço de um só. O custo fica próximo ao de um espelho, mas com histórico.

O cache de hash usa **nanossegundos**, **ctime** e uma janela de 10 segundos para arquivos recém-modificados. Isso evita o bug em que duas edições no mesmo segundo (especialmente em exFAT, com granularidade de 2 s) reutilizavam o hash antigo.

### 4. Lixeira por volume

`os.Rename` só é instantâneo dentro do mesmo sistema de arquivos. Se a lixeira ficasse apenas na raiz principal, excluir um arquivo do cartão exigiria copiar o conteúdo inteiro — ou seria definitivo. Por isso cada volume tem sua própria `.pocketserve-trash`, e o `meta.json` guarda o volume de origem para restaurar no lugar certo.

### 5. Múltiplas raízes com apelido

O primeiro segmento de um caminho pode escolher a raiz (`cartao/...`). A ordem de resolução é deliberada:

1. apelido exato ganha;
2. pasta que existe de fato na raiz principal é pasta normal;
3. só recusa quando o nome não existe e é quase igual a um apelido (sugere a correção).

Isso corrigiu um bug real: com duas raízes, nomes como `DCIM`, `Download` e `Projetos` eram confundidos com apelidos inválidos e o interno deixava de aceitar upload.

### 6. Reserva de disco por volume

O Android fica instável quando o armazenamento compartilhado zera, e exFAT degrada a alocação. A reserva padrão de **2 GB** é checada antes de receber o upload, antes de cada arquivo, depois da gravação, no editor e no unzip. A resposta é `507 Insufficient Storage` com mensagem clara.

### 7. Supervisor externo

O processo Go não tenta se auto-supervisionar. Um script shell (`ctl.sh`) mantém o servidor vivo, verifica `/healthz` e reinicia com recuo progressivo. Isso separa responsabilidades e evita que um bug no servidor derrube também o supervisor.

### 8. Frontend embutido sem build

HTML, CSS e JavaScript são servidos diretamente de `web/` via `go:embed`, com `?v=<build>` para invalidar cache após deploy. Não há Node.js, bundler ou etapa de build. O custo é não usar frameworks modernos; o benefício é um deploy de um arquivo só.

### 9. Proxy confiável apenas em loopback

`X-Forwarded-For` e `X-Forwarded-Proto` só são considerados quando a conexão vem de loopback (`127.0.0.1`/`::1`). Assim, um cliente da rede não consegue falsificar o IP nos logs nem forçar o cookie `Secure` em HTTP puro. Cloudflare Tunnel e Caddy locais se encaixam nesse modelo; um proxy em outra máquina não.

### 10. HTTPS opcional, HTTP preservado

Quando há certificado, o servidor sobe HTTP na porta configurada **e** HTTPS na porta `8443` (configurável). O HTTP continua disponível na LAN para não quebrar acesso existente. Em produção, a recomendação é usar Tailscale ou um túnel HTTPS e não encaminhar portas no roteador.

## Limites e constantes

| Item | Valor |
|---|---|
| Porta HTTP padrão | 8080 |
| Porta HTTPS padrão | 8443 |
| Sessão | 30 dias (1–365) |
| PBKDF2 | 210.000 iterações, SHA-256, salt de 16 bytes |
| TOTP | 6 dígitos, período 30 s, janela ±1 |
| Aparelho confiável | 30 dias |
| Tentativas de login | 5 falhas → 1 min; depois 2, 4, 8, 16 min |
| Raízes | até 8 extras + principal |
| Lixeira | 30 dias (0–3650; 0 desliga) |
| Backup | 14 snapshots, arquivo máximo 512 MB, diário opcional |
| Reserva de disco | 2.048 MB (0–1.048.576 MB) |
| Upload | sem limite artificial (0 = ilimitado; configurável) |
| Editor de texto | leitura de 2 MiB; gravação de 5 MiB |
| Unzip | 20 GiB e 20.000 arquivos |
| Miniatura | 64 MiB e 80 megapixels; 4 decodificações simultâneas |
| Busca | 400 resultados, 6 s, mínimo de 2 caracteres |
| Hash | 4 leituras simultâneas |
| Corpo de login/setup | 4 KiB / 8 KiB |
| Corpo de operação | 1 MiB |
| Cabeçalhos | 1 MiB; leitura de cabeçalho 20 s |
| Idle HTTP | 10 min |
| Shutdown gracioso | 6 s |

## Concorrência

- `App.mu` protege a configuração em memória.
- Cada store (`ShareStore`, `TokenStore`, `ActivityStore`, `sessionRevoker`, `backupManager`) tem seu próprio mutex.
- `thumbSem` limita decodificações de imagem a 4.
- `hashSem` limita leituras SHA-256 a 4.
- A verificação de espaço e o upload usam checagem otimista + checagem pós-escrita; dois uploads simultâneos podem passar juntos, mas o que furar a reserva é removido.
- O servidor HTTP usa `ReadHeaderTimeout`, `IdleTimeout`, `MaxHeaderBytes` e encerramento gracioso.

## O que não existe por decisão

- Banco de dados, ORM, migrations.
- WebDAV, SMB, FTP.
- Multiusuário, papéis, quotas por pasta.
- Criptografia própria de arquivos (a criptografia de repouso é a do Android).
- Sincronização embutida (Syncthing é opcional e externo).
- Aplicativo móvel nativo (o painel é uma PWA sem service worker).

Essas ausências estão detalhadas em [LIMITACOES.md](LIMITACOES.md).
