# Segurança

Modelo de ameaças, controles implementados, riscos residuais e o que fazer antes de expor o servidor.

## Premissa

O PocketServe é um servidor **pessoal, de um único usuário**. O desenho assume uma destas redes:

- Wi-Fi local considerada confiável;
- VPN privada (Tailscale/WireGuard);
- proxy HTTPS de confiança falando com o servidor pelo loopback.

Ele **não** foi desenhado para ser um serviço público multiusuário. Se você decidir expô-lo à internet, precisa de HTTPS, senha forte e 2FA — e ainda assim os riscos residuais continuam maiores do que em uma VPN.

## O que está implementado

### Senha do administrador

- PBKDF2-HMAC-SHA256;
- **210.000 iterações**;
- salt aleatório de 16 bytes por senha;
- mínimo de 8 caracteres (a recomendação é 12+);
- comparação em tempo constante pelo `subtle.ConstantTimeCompare`;
- troca de senha exige a senha atual.

O hash fica em `config.json` (permissão `0600`). A senha em claro nunca é gravada.

### Sessão

- cookie `ps_session`, `HttpOnly`, `SameSite=Lax`, `Path=/`;
- `Secure` quando há TLS direto ou proxy local confiável com `X-Forwarded-Proto: https`;
- payload assinado com HMAC-SHA256 usando `session_secret`;
- payload contém expiração, versão da senha e um **ID aleatório por login**;
- validade padrão de 30 dias (1–365);
- trocar a senha invalida cookies antigos (a versão muda).

### Revogação de sessão

O logout **não** confia apenas em apagar o cookie no navegador:

1. o servidor lê o ID da sessão;
2. grava `sha256(ID)` em `~/.pocketserve/revoked_sessions.json`;
3. o cookie antigo passa a devolver `401`;
4. a lista sobrevive a reinícios;
5. entradas expiram junto com a sessão que revogam.

Sem o ID, dois logins no mesmo segundo gerariam o mesmo cookie e o logout de um derrubaria o outro. O ID elimina a colisão.

Cookies emitidos por versões antigas, sem ID, continuam válidos até expirarem — mas não podem ser revogados individualmente. É uma compatibilidade consciente.

### CSRF

Toda mutação autenticada por **cookie** exige:

1. cabeçalho `X-PS: 1` (um formulário HTML cross-site não consegue enviar cabeçalho customizado);
2. se houver `Origin`, ele precisa bater com o `Host` da requisição.

Requisições autenticadas por **Bearer token** dispensam CSRF: o navegador não anexa esse cabeçalho sozinho, então não existe requisição cross-site capaz de forjar a credencial.

### Limite de tentativas

- 5 falhas seguidas → bloqueio de 1 minuto;
- novas falhas: 2, 4, 8 e 16 minutos;
- o contador zera após 30 minutos sem falhas;
- o limite vale por IP e também para o código TOTP;
- mapa limitado a 2.000 entradas, com limpeza de registros antigos.

O IP é obtido de `RemoteAddr`. Quando a conexão vem de **loopback** (proxy local), o servidor aceita, nesta ordem: `CF-Connecting-IP`, `X-Forwarded-For` e `X-Real-IP`. Um cliente da rede não consegue falsificar o próprio IP nos logs, porque esses cabeçalhos são ignorados fora do loopback.

O suporte a `CF-Connecting-IP` cobre o Cloudflare Tunnel, em que o `cloudflared` conecta em `127.0.0.1`. Se o proxy local não encaminhar nenhum desses cabeçalhos, todas as requisições do túnel chegam como `127.0.0.1` e o limite vira um balde único.

### Verificação em duas etapas (TOTP)

- RFC 6238, SHA-1, 6 dígitos, período de 30 s, janela ±1;
- segredo gerado localmente e mostrado uma vez;
- QR code gerado pelo próprio servidor;
- ativação exige um código válido;
- desativação exige a senha;
- **códigos de recuperação** de uso único, guardados com hash;
- sem dependência de serviço externo.

Com 2FA ativo, o código só é pedido **depois** da senha correta, para não revelar a existência do segundo fator a quem não sabe a senha.

### Aparelhos confiáveis

- opção “Lembrar deste aparelho por 30 dias” no login;
- cookie `ps_trust` assinado com HMAC;
- lista em **Configurações → Verificação em duas etapas**;
- revogar um aparelho invalida o cookie;
- trocar a senha ou desativar o 2FA limpa a lista.

Um aparelho confiável **não** substitui a senha: ele só dispensa o código TOTP.

### Tokens de API

- valor de **256 bits** aleatórios, prefixo `ps_`;
- exibido **uma única vez** na criação;
- guardado apenas como **SHA-256** em `tokens.json` (permissão `0600`);
- escopos: `files:read`, `files:write`, `files:delete`, `shares:manage`, `admin`;
- validade de 0 a 3.650 dias (`0` = nunca expira);
- revogação individual e imediata;
- último uso e último IP registrados;
- token **não** acessa configurações, sistema, log, reinício, TOTP, aparelhos confiáveis ou outros tokens.

`admin` é um superset dos escopos de arquivos e shares; **não** transforma o token em administrador do servidor.

### Caminhos

- `filepath.Clean` + verificação de containment contra a raiz escolhida;
- `EvalSymlinks` no caminho existente e no ancestral mais próximo na criação;
- symlink que escapa da raiz é recusado;
- upload usa `filepath.Base` + `sanitizeName` (remove `/`, `\`, `\x00`, limita a 180 caracteres);
- ZIP: normaliza caminhos e recusa entradas que escapariam do destino;
- raiz indisponível **encerra** o processo em vez de cair para `/`;
- a própria raiz não pode ser excluída ou renomeada.

### Cabeçalhos

Em todas as respostas:

```text
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: no-referrer
Permissions-Policy: camera=(), microphone=(), geolocation=()
Cross-Origin-Opener-Policy: same-origin
Cross-Origin-Resource-Policy: same-origin
Content-Security-Policy: default-src 'self'; img-src 'self' data: blob:; ...
```

Em `/api/` e `/s/`:

```text
Cache-Control: no-store, max-age=0
Pragma: no-cache
X-Robots-Tag: noindex, nofollow, noarchive
```

A CSP não permite `script-src 'unsafe-inline'`. O frontend usa `textContent`/escape consistente; o `html/template` protege as páginas públicas de share.

### TLS

- HTTP e HTTPS podem coexistir;
- HTTPS em `tls_port` (padrão 8443) quando há certificado;
- TLS mínimo 1.2;
- certificado local (`-tls-cert`/`-tls-key`) ou proxy reverso;
- `X-Forwarded-Proto` só é aceito de loopback.

### Outros

- corpo de login/setup limitado a 4/8 KiB;
- operações limitadas a 1 MiB;
- cabeçalhos limitados a 1 MiB;
- timeout de leitura de cabeçalho de 20 s;
- timeout ocioso de 10 min;
- encerramento gracioso de 6 s;
- log de atividade local com 500 entradas em memória e arquivo JSONL limitado a 512 KiB;
- `config.json`, `shares.json`, `tokens.json`, `activity.jsonl` e `revoked_sessions.json` com permissão `0600`; a pasta, `0700`.

## Riscos residuais

| Risco | Situação | Mitigação |
|---|---|---|
| Um único administrador | por design | senha longa + 2FA |
| 2FA opcional | por design | ative antes de expor |
| HTTP puro na LAN | por design | use Tailscale/túnel; não encaminhe portas |
| Sem WAF/CAPTCHA | por design | rate limit + VPN |
| Malware no Android | fora do controle do app | sandbox do Android, PIN/biometria |
| Aparelho perdido | risco físico | criptografia do Android + bloqueio |
| Backup no mesmo aparelho | não protege contra falha física | cópia externa periódica |
| Logs contêm IPs e caminhos | local | proteja o acesso ao Termux |
| Sem criptografia própria | por design | criptografia de repouso do Android |
| Sem multiusuário/quotas | por design | uso pessoal |
| Rate limit atrás de proxy que não envia IP | limitação do proxy | use um proxy que preserve `CF-Connecting-IP`/`X-Forwarded-For` |
| Arquivos acima de 512 MB fora do backup | limitação | cópia manual |
| Sem restauração integral de snapshot | limitação | restore por arquivo |

### Sobre links de compartilhamento

- token de **128 bits**;
- validade opcional;
- senha opcional (mínimo 4 caracteres — use 8+);
- somente leitura;
- revogável;
- a página pública não lista o restante do servidor;
- o cookie de desbloqueio é assinado com o `session_secret`.

Quem tem o link e a senha tem acesso ao item compartilhado. Trate o link como uma credencial.

## Antes de expor à internet

1. **Ative 2FA.**
2. Use senha longa e exclusiva (12+ caracteres, gerenciador de senhas).
3. Prefira **Tailscale** ou **WireGuard** a abrir porta no roteador.
4. Se usar túnel HTTPS, termine o TLS no túnel e faça o proxy conectar em `127.0.0.1`.
5. Não encaminhe a porta 8080 no roteador.
6. Mantenha Android e Termux atualizados.
7. Revise **Configurações → Verificação em duas etapas** e revogue aparelhos confiáveis que não reconhecer.
8. Faça backup externo periódico.
9. Monitore **Atividade** e o log.
10. Se o aparelho for perdido, troque a senha em outro acesso ou use `-reset-access` no Termux (exige acesso físico).

## Recuperação de acesso

```bash
pocketserve stop
~/.pocketserve/pocketserve -home ~/.pocketserve -reset-access
pocketserve start
```

Remove senha, TOTP e aparelhos confiáveis. **Não** apaga arquivos, shares, lixeira ou backups. Exige acesso ao Termux no aparelho — é o “controle físico” que autoriza a recuperação. Não há caminho pela rede, de propósito.

## O que não é proteção

- esconder a porta ou usar um nome “difícil”;
- HTTPS sozinho;
- senha curta “porque é só na LAN”;
- confiar no bloqueio de tentativas como única defesa;
- achar que o backup local protege contra roubo ou falha do cartão.

## Divulgação responsável

Falhas de segurança **não** devem ser abertas como issue pública. Use **Security → Report a vulnerability** no GitHub ou o canal indicado na página do projeto. Inclua impacto, versão e passos de reprodução, sem dados reais. Veja [SECURITY.md](../SECURITY.md).

## Sem garantias

Projeto pessoal distribuído sob MIT, **sem garantia**. Leia [LICENSE](../LICENSE) e [LIMITACOES.md](LIMITACOES.md).
