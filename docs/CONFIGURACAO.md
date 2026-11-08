# Configuração

Como o PocketServe é configurado, onde os valores ficam e o que cada um faz.

## Prioridade dos valores

Na inicialização, nesta ordem:

1. **flags de linha de comando** (`-port`, `-root`, `-name`, ...);
2. **`config.json`** já existente;
3. **padrões do código**, quando o campo não existe.

As flags de porta, raiz, nome, Tailscale e TLS são gravadas no `config.json` quando usadas. As flags de recuperação (`-reset-password`, `-reset-access`) alteram o arquivo e encerram o processo. O painel (**Configurações**) grava no mesmo arquivo em tempo de execução.

## Variáveis de ambiente

| Variável | Efeito | Padrão |
|---|---|---|
| `POCKETSERVE_HOME` | diretório de configuração e dados | `$HOME/.pocketserve` |
| `HOME` | usado para montar o padrão acima | — |
| `POCKETSERVE_ALLOW_MISSING_ROOT` | `1` permite subir mesmo com a raiz ausente, usando o diretório atual | desligado |

`POCKETSERVE_ALLOW_MISSING_ROOT` é uma saída de diagnóstico. Em uso normal, uma raiz ausente encerra o processo de propósito: cair para `/` transformaria o painel em acesso ao sistema de arquivos inteiro. Mesmo com a variável ligada, o servidor recusa `/`, `/system`, `/proc`, `/sys`, `/dev`, `/vendor`, `/etc`, `/root` e qualquer caminho sob `/data` fora do diretório do Termux.

## Flags

```text
pocketserve [flags]
```

| Flag | Valor | O que faz |
|---|---|---|
| `-home` | caminho | pasta de configuração, estado, logs e binário instalado |
| `-init` | — | cria/atualiza a configuração e sai (usado pelo instalador) |
| `-port` | 1024–65535 | porta HTTP; grava no `config.json` |
| `-root` | caminho | pasta raiz principal; grava no `config.json` |
| `-name` | texto | nome exibido no painel (até 40 caracteres) |
| `-ts-name` | nome DNS | MagicDNS do Tailscale mostrado nos endereços |
| `-tls-cert` | arquivo PEM | certificado para HTTPS |
| `-tls-key` | arquivo PEM | chave privada do certificado |
| `-public-host` | domínio | domínio público mostrado nos links |
| `-reset-password` | — | remove a senha do administrador e sai |
| `-reset-access` | — | remove senha, TOTP e aparelhos confiáveis, e sai |
| `-version` | — | mostra a versão e sai |

Exemplos:

```bash
# criar a configuração sem subir o servidor
~/.pocketserve/pocketserve -home ~/.pocketserve -init \
  -root /sdcard -port 8080 -name PocketServe

# subir em primeiro plano, em outra porta, para teste
~/.pocketserve/pocketserve -home /tmp/ps-home -root /tmp/ps-root -port 18080

# recuperar o acesso (senha, 2FA e aparelhos confiáveis)
~/.pocketserve/pocketserve -home ~/.pocketserve -reset-access
```

> `-reset-password` e `-reset-access` **não apagam arquivos, compartilhamentos, lixeira nem backups**. Eles só limpam credenciais.

## `config.json`

Fica em `$POCKETSERVE_HOME/config.json`, com permissão `0600`. A pasta tem permissão `0700`. O arquivo é gravado de forma atômica (`config.json.tmp` + `rename`), então uma queda no meio da escrita não deixa JSON pela metade.

### Campos

| Campo | Tipo | Padrão | Descrição |
|---|---|---|---|
| `server_name` | texto | `PocketServe` | nome exibido no painel e nos QR codes |
| `port` | inteiro | `8080` | porta HTTP (1024–65535) |
| `root` | caminho | `/sdcard` ou diretório atual | raiz principal |
| `password_hash` | texto | vazio | PBKDF2-HMAC-SHA256; vazio = primeiro acesso pendente |
| `session_secret` | texto | gerado | segredo HMAC das sessões, cookies de share e confiança de aparelho |
| `session_days` | inteiro | `30` | validade da sessão (1–365) |
| `tailscale_name` | texto | vazio | MagicDNS, ex.: `celular.tailXXXX.ts.net` |
| `max_upload_mb` | inteiro | `0` | limite por arquivo; `0` = sem limite |
| `min_free_mb` | inteiro | `2048` | reserva de disco por volume; `0` = desligada |
| `trash_days` | inteiro | `30` | retenção da lixeira; `0` = exclusão definitiva |
| `backup_daily` | booleano | `false` | um snapshot por dia (enquanto o servidor estiver ligado) |
| `totp_secret` | texto | vazio | segredo TOTP ativo |
| `totp_pending` | texto | vazio | segredo em processo de ativação |
| `recovery_codes` | texto | vazio | códigos de recuperação com hash |
| `trusted_devices` | lista | vazia | aparelhos que não pedem código por 30 dias |
| `tls_cert` | caminho | vazio | certificado PEM |
| `tls_key` | caminho | vazio | chave PEM |
| `tls_port` | inteiro | `8443` | porta HTTPS quando há certificado |
| `public_host` | texto | vazio | domínio público exibido nos links |
| `roots` | lista | vazia | raízes extras: `[{"name":"cartao","path":"/storage/XXXX-XXXX/PocketServe"}]` |
| `created_at` | data | agora | criação da configuração |

Validações aplicadas pelo painel e pela API:

- nome: 1 a 40 caracteres;
- porta: 1024 a 65535;
- raiz: precisa existir e ser pasta;
- sessão: 1 a 365 dias;
- reserva: 0 a 1.048.576 MB;
- lixeira: 0 a 3650 dias;
- raízes extras: até 8, cada uma precisa existir e não pode repetir a principal;
- apelido: normalizado, curto, sem barras.

### Exemplo seguro

Este exemplo é **fictício**. Nunca copie segredos reais para issues, prints ou repositórios. Os valores abaixo são placeholders.

```json
{
  "server_name": "PocketServe",
  "port": 8080,
  "root": "/sdcard",
  "password_hash": "SUBSTITUA_PELO_HASH_GERADO_PELO_PAINEL",
  "session_secret": "SUBSTITUA_POR_UM_SEGREDO_ALEATORIO",
  "session_days": 30,
  "tailscale_name": "celular.tailXXXX.ts.net",
  "max_upload_mb": 0,
  "min_free_mb": 2048,
  "trash_days": 30,
  "backup_daily": false,
  "tls_port": 8443,
  "roots": [
    { "name": "cartao", "path": "/storage/XXXX-XXXX/PocketServe" }
  ],
  "created_at": "2026-01-01T12:00:00Z"
}
```

Campos opcionais ausentes (`totp_secret`, `tls_cert`, `public_host`, `roots`, ...) simplesmente não aparecem.

### Editar à mão

Editar o JSON funciona, mas exige cuidado:

```bash
pocketserve stop
cp ~/.pocketserve/config.json ~/.pocketserve/config.json.bak
# edite com um editor de texto
pocketserve start
pocketserve log
```

Se o JSON estiver inválido, o processo encerra com `configuração: config.json inválido`. Restaure o backup e tente de novo. **Não** edite o arquivo com o servidor rodando: a próxima gravação do painel sobrescreve a alteração.

Para mudar porta ou raiz, preferir **Configurações → Servidor**, que valida os valores e oferece o botão **Reiniciar agora**. Essas duas mudanças exigem reinício; as demais valem na hora.

## Configurações do painel

### Servidor

| Campo | Efeito |
|---|---|
| Nome do servidor | nome exibido e usado no TOTP |
| Porta HTTP | 1024–65535; reinício necessário |
| Pasta raiz | raiz principal; reinício necessário |
| Sessão (dias) | validade de novos cookies |
| Limite de upload (MB) | `0` = ilimitado |
| Reserva de disco (MB) | espaço intocável por volume; `0` desliga |
| Lixeira (dias) | retenção; `0` desliga a lixeira |
| Backup automático | um snapshot por dia |
| Nome MagicDNS | usado só para exibir o endereço do Tailscale |

### Armazenamento

- lista a raiz principal e as extras, com espaço livre;
- descobre volumes do aparelho (`/storage/XXXX-XXXX`, `~/storage/external-*`, `/sdcard`);
- adiciona uma raiz extra com caminho e apelido;
- remove uma raiz extra **sem apagar o conteúdo**.

Detalhes em [ARMAZENAMENTO.md](ARMAZENAMENTO.md).

### Segurança

- troca de senha (mínimo 8 caracteres, exige a senha atual);
- ativação/desativação de TOTP;
- códigos de recuperação;
- aparelhos confiáveis (30 dias);
- tokens de API.

Detalhes em [SEGURANCA.md](SEGURANCA.md) e [API.md](API.md).

### Manutenção

- mostra o caminho do `config.json` e do `server.log`;
- **Ver log** (últimas linhas);
- **Reiniciar servidor** (o supervisor sobe de novo);
- **Sair do painel** (revoga a sessão no servidor).

## TLS e proxy

Com `tls_cert` e `tls_key` válidos, o servidor mantém o HTTP na porta configurada **e** sobe HTTPS em `tls_port` (padrão 8443). O HTTP continua disponível na LAN de propósito, para não quebrar acesso existente.

Se um proxy reverso local termina o TLS (Cloudflare Tunnel, Caddy), não é preciso certificado local. O servidor confia em `X-Forwarded-Proto` e `X-Forwarded-For` **somente quando a conexão vem de loopback**. Um cliente da rede não consegue falsificar esses cabeçalhos.

Receita completa em [ACESSO-REMOTO.md](ACESSO-REMOTO.md).

## Redefinir a configuração

### Manter os dados, trocar a senha

```bash
pocketserve stop
~/.pocketserve/pocketserve -home ~/.pocketserve -reset-access
pocketserve start
```

### Começar do zero (apaga credenciais, shares, tokens e atividade)

```bash
pocketserve stop
mv ~/.pocketserve ~/.pocketserve.old
bash /sdcard/Download/PocketServe/install.sh
```

O `mv` preserva o estado antigo para inspeção. **Seus arquivos em `/sdcard` não são tocados.**

## Problemas comuns de configuração

| Sintoma | Causa | Solução |
|---|---|---|
| `porta inválida` | flag fora de 1024–65535 | use uma porta alta |
| `raiz "..." indisponível` | cartão/HD desconectado | reconecte ou ajuste a raiz |
| `config.json inválido` | JSON quebrado | restaure o backup |
| mudou a porta e o painel não abre | ainda não reiniciou | `pocketserve restart` |
| endereço do Tailscale errado | nome desatualizado | corrija em Configurações |
| `POCKETSERVE_HOME` ignorado | flag `-home` tem prioridade | remova a flag ou ajuste |
