# Changelog

Todas as mudanças relevantes do PocketServe público são registradas aqui.

O projeto segue [Versionamento Semântico](https://semver.org/lang/pt-BR/). A numeração pública é independente da linhagem privada que originou este código: a versão **1.0.0** é o primeiro marco público e consolida a antiga linha 1.10.x.

## [1.0.0] — 2026-11-08

Primeira versão pública independente.

### Adicionado

- Servidor HTTP em Go com frontend embutido (`go:embed`), sem framework e sem banco de dados.
- Navegação por arquivos e pastas, upload (arrastar/soltar, seletor e pasta inteira), download, ZIP de pastas, renomear, copiar, mover, criar pasta e editor de texto.
- Múltiplas raízes simultâneas (até 8), com apelido curto por volume e detecção de armazenamentos.
- Lixeira por volume, com restauração, exclusão definitiva, esvaziamento e retenção configurável.
- Backup por snapshots content-addressed: manifesto por snapshot, packs `SHA-256`, deduplicação, retenção de 14 snapshots, execução manual e diária opcional.
- Histórico de versões por arquivo, com download e restauração; a versão substituída vai para a lixeira.
- Relatório de duplicatas por SHA-256, com sugestão de cópia a manter e limpeza que nunca apaga a última cópia.
- Compartilhamentos públicos somente leitura, com token aleatório, validade e senha opcional.
- Autenticação com PBKDF2-HMAC-SHA256 (210.000 iterações), sessão assinada com HMAC, ID único por login, CSRF e limite progressivo de tentativas.
- TOTP (RFC 6238), códigos de recuperação, aparelhos confiáveis por 30 dias e revogação de sessões persistente.
- Tokens de API com escopo, validade, hash em disco e revogação individual.
- Painel de sistema com CPU, memória, armazenamento por volume, bateria, interfaces de rede, endereços e QR code.
- Supervisor com reinício automático, verificação de `/healthz`, inicialização pelo Termux:Boot e comandos `start`, `stop`, `restart`, `status`, `url`, `log` e `watchdog`.
- HTTPS opcional por certificado local (portas HTTP e HTTPS simultâneas), com suporte a proxy reverso confiável.
- Reserva mínima de disco por volume (padrão 2 GB), com resposta `507 Insufficient Storage` e alertas.
- Verificação de integridade por SHA-256 sob demanda.
- Log de atividade local em JSONL.
- Documentação pública completa, Makefile, CI e scripts de verificação.

### Segurança

- Logout revoga a sessão no servidor; o cookie antigo passa a devolver `401`, inclusive após reinício.
- Cada login recebe um ID aleatório, impedindo que dois logins no mesmo segundo compartilhem o mesmo cookie.
- Sessões antigas sem ID continuam válidas até expirarem (compatibilidade), mas não são revogáveis individualmente.
- Cookies `HttpOnly` e `SameSite=Lax`, com `Secure` quando há TLS direto ou proxy local confiável.
- Cabeçalhos `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, `COOP`, `CORP`, CSP restritiva e `X-Robots-Tag: noindex` em APIs e links.
- Caminhos validados contra traversal e symlinks; raiz indisponível encerra o processo em vez de cair para `/`.
- Tokens de API são exibidos uma única vez e guardados apenas como SHA-256.

### Corrigido

- **Hash de senha de link exposto:** `GET /api/shares` e a resposta de criação devolviam `password_hash`. A API passou a devolver apenas `has_password`; o hash continua somente em `shares.json`, e o painel usa o novo campo.
- **IP real atrás de túnel:** conexões de um proxy local chegavam todas como `127.0.0.1` e o limite de tentativas virava um balde único. O servidor agora aceita `CF-Connecting-IP`, depois `X-Forwarded-For` e `X-Real-IP`, somente quando a conexão vem do loopback.
- **`trash_days: 0` não desligava a lixeira:** `fillDefaults` convertia `0` em `30` a cada carga, anulando a opção anunciada na interface. Agora a chave ausente recebe 30 e o zero explícito é preservado.
- Overflow horizontal em telas estreitas de celular.
- Cache de hash do backup que podia reutilizar conteúdo antigo quando duas edições ocorriam no mesmo segundo (agora usa nanossegundos, `ctime` e ignora arquivos recém-modificados).
- Memória exibida em base decimal, coerente com o Android (8 GB, não 7,5 GiB).
- Soma de todos os volumes no cartão de armazenamento, em vez de apenas a raiz principal.
- Uploads para pastas reais do interno quando havia múltiplas raízes (nomes como `DCIM`, `Download` e `Projetos` deixaram de ser confundidos com apelidos).
- Movimentação entre volumes diferentes (fallback de `rename` com `EXDEV` para cópia + remoção).
- Lixeira e busca em todos os volumes, com caminho exibível correto.
- Reserva de disco checada antes do upload, antes de cada arquivo, depois da gravação, no editor e no unzip.
- Leitura de bateria com fallback para Termux:API e mensagem explícita quando o Android bloqueia o acesso.

### Verificação da versão

| Item | Resultado |
|---|---|
| Testes automatizados | 108 |
| Cobertura de instruções | 34,9% |
| `go test -race` | limpo |
| Verificação HTTP | 80/80 |
| Binário arm64 estático | 9,2 MiB (9.633.976 bytes) |

Detalhes em [docs/VERIFICACAO.md](docs/VERIFICACAO.md).

### Limitações conhecidas nesta versão

- Um único usuário administrador; não há contas múltiplas.
- Sem WebDAV, aplicativo móvel nativo, sincronização própria ou restauração integral de snapshot pela interface.
- Backup não é enviado automaticamente para fora do aparelho.
- Arquivos maiores que 512 MB não entram nos snapshots.
- Relatório de duplicatas depende do último snapshot e compara apenas conteúdo exato.
- O desempenho e a durabilidade são os de um celular, não os de um NAS.

Veja [docs/LIMITACOES.md](docs/LIMITACOES.md) para a lista completa e [docs/VERIFICACAO.md](docs/VERIFICACAO.md) para os testes executados.

---

## Linhagem privada (resumo)

Antes da versão pública, o projeto foi desenvolvido e usado em um aparelho pessoal. Os marcos abaixo explicam a origem das funcionalidades; eles **não** são versões públicas e não devem ser confundidos com a numeração atual.

| Linhagem | Marco |
|---|---|
| 1.0 | Servidor Go/Termux, painel embutido, arquivos, compartilhamentos e painel de sistema. |
| 1.1 | Lixeira, snapshots com deduplicação, verificação SHA-256, TOTP e proteção contra raiz ausente. |
| 1.2 | Aparelhos confiáveis, recuperação de acesso, HTTPS opcional, supervisor do Syncthing. |
| 1.3 | Múltiplas raízes e cartão microSD. |
| 1.4 | Correções de memória, bateria e endereço curto. |
| 1.4.1 | Soma de todos os volumes no painel. |
| 1.5 | Correção do apelido de raiz, `EXDEV`, lixeira por volume e busca multi-volume. |
| 1.6 | Tokens de API com escopo. |
| 1.7 | Reserva de disco por volume. |
| 1.8 | Relatório de duplicatas. |
| 1.9 | Versões por arquivo e correção do cache de hash do backup. |
| 1.10 | Correção de overflow no celular e revogação de sessões no logout. |
| 1.10.2 | Último marco privado antes da publicação independente. |
